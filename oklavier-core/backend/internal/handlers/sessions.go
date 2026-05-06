package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"oklavier-api/internal/agent"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/cache"
	"oklavier-api/internal/db"
	"oklavier-api/internal/metrics"
	"oklavier-api/internal/middleware"
	"oklavier-api/internal/models"
	mtlsLib "oklavier-api/internal/mtls"
)

// agentHTTPClient is a singleton http.Client used for core→agent calls.
// When MTLS_* env vars are configured, it presents a client certificate and
// verifies the agent's cert against the configured CA. Otherwise it falls
// back to the default transport (plain HTTPS / HTTP).
var agentHTTPClient = func() *http.Client {
	transport, err := mtlsLib.ClientTransport()
	if err != nil {
		log.Printf("mtls client transport disabled: %v", err)
		return &http.Client{Timeout: 30 * time.Second}
	}
	if transport == nil {
		return &http.Client{Timeout: 30 * time.Second}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}()

type SessionHandler struct {
	DB          *db.DB
	Agent       *agent.Agent
	RateLimiter *auth.RateLimiter
	Cache       *cache.Cache
}

// mintAndAdmitBearer generates an opaque random session bearer (32 bytes hex)
// and pushes it to the agent server-to-server via /api/admit-bearer. The
// browser only ever sees this opaque value — no JWT, no claims, fully
// revocable, scoped to one session, TTL 5 minutes.
func mintAndAdmitBearer(agentEndpoint, agentToken, sessionID, userID, role string) (string, error) {
	if agentEndpoint == "" || agentToken == "" {
		return "", fmt.Errorf("agent endpoint not configured")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	bearer := hex.EncodeToString(b)
	body, _ := json.Marshal(map[string]interface{}{
		"bearer":      bearer,
		"session_id":  sessionID,
		"user_id":     userID,
		"role":        role,
		"ttl_seconds": 300,
	})
	if _, err := callAgent(agentEndpoint, agentToken, "/api/admit-bearer", body); err != nil {
		return "", err
	}
	return bearer, nil
}

// revokeBearer asks the agent to drop all bearers for a session.
func revokeBearer(agentEndpoint, agentToken, sessionID string) {
	if agentEndpoint == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"session_id": sessionID})
	_, _ = callAgent(agentEndpoint, agentToken, "/api/revoke-bearer", body)
}

// GenerateGuestSessionBearer is the guest-flow variant.
func GenerateGuestSessionBearer(agentEndpoint, agentToken, guestUserID, sessionID string) (string, error) {
	return mintAndAdmitBearer(agentEndpoint, agentToken, sessionID, guestUserID, "guest")
}

func (h *SessionHandler) GetWorkspaces(c *fiber.Ctx) error {
	// Use authenticated user from middleware only — no query param fallback (IDOR prevention)
	userID, _ := c.Locals("user_id").(string)
	isAdmin := false
	if role, _ := c.Locals("user_role").(string); role == "admin" {
		isAdmin = true
	}

	var workspaces []models.Workspace
	if h.Cache == nil || !h.Cache.Get("workspaces:enabled", &workspaces) {
		var err error
		workspaces, err = h.DB.GetWorkspaces()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error_message": "Failed to get workspaces"})
		}
		if h.Cache != nil {
			h.Cache.Set("workspaces:enabled", workspaces, 30*time.Second)
		}
	}

	images := make([]fiber.Map, 0)
	for _, w := range workspaces {
		// Filter by group access (admin sees all)
		if !isAdmin && userID != "" && !h.DB.UserCanAccessWorkspace(userID, w.ID) {
			continue
		}

		// Get groups for this workspace
		groups, _ := h.DB.GetWorkspaceGroups(w.ID)
		groupNames := make([]string, len(groups))
		for j, g := range groups {
			groupNames[j] = g.Name
		}

		images = append(images, fiber.Map{
			"image_id":                w.ID,
			"friendly_name":           w.FriendlyName,
			"name":                    w.DockerImage,
			"description":             w.Description,
			"image_src":               w.ImageSrc,
			"cores":                   w.Cores,
			"memory":                  w.Memory,
			"categories":              []string{w.Category},
			"default_category":        w.Category,
			"groups":                  groupNames,
			"workspace_type":          w.WorkspaceType,
			"server_auth_mode":        w.ServerAuthMode,
			"server_protocol":         w.ServerProtocol,
			"server_allow_remember":   w.ServerAllowRemember,
			"server_default_settings": w.ServerDefaultSettings,
		})
	}

	return c.JSON(fiber.Map{"images": images})
}

func (h *SessionHandler) GetUserSessions(c *fiber.Ctx) error {
	// Use authenticated user only — no query param fallback (IDOR prevention)
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return c.JSON(fiber.Map{"sessions": []interface{}{}})
	}

	sessions, err := h.DB.GetUserSessions(userID)
	if err != nil {
		return c.JSON(fiber.Map{"sessions": []interface{}{}})
	}

	result := make([]fiber.Map, len(sessions))
	for i, s := range sessions {
		// Get agent public_url if available
		var agentPublicURL string
		if s.AgentID != "" {
			h.DB.Get(&agentPublicURL, "SELECT COALESCE(public_url, '') FROM agent WHERE id = $1", s.AgentID)
		}
		// SECURITY: no bearer is minted at list time. The dashboard fetches
		// the screenshot thumbnail through GET /api/sessions/:id/screenshot
		// (proxied by the core, owner-checked via the user cookie). The bearer
		// is server-side only — the browser never holds it.
		result[i] = fiber.Map{
			"session_id":         s.ID,
			"operational_status": s.Status,
			"start_date":         s.CreatedAt,
			"expiration_date":    s.ExpiresAt,
			"keepalive_date":     s.KeepaliveAt,
			"container_ip":       s.ContainerIP,
			"agent_id":           s.AgentID,
			"agent_vnc_url":      agentPublicURL,
			"session_type":       s.SessionType,
			"workspace_type":     s.WorkspaceType,
			"image": fiber.Map{
				"image_id":      s.WorkspaceID,
				"friendly_name": s.ImageName,
				"image_src":     s.ImageSrc,
			},
		}
	}

	return c.JSON(fiber.Map{"sessions": result})
}

func (h *SessionHandler) RequestSession(c *fiber.Ctx) error {
	// Rate limit session creation (10 per minute per IP)
	if h.RateLimiter != nil && !h.RateLimiter.Allow(c.IP()) {
		return c.Status(429).JSON(fiber.Map{"error_message": "Too many session requests. Please wait."})
	}
	var req struct {
		ImageID        string `json:"image_id" validate:"required"`
		AgentID        string `json:"agent_id"`
		Lang           string `json:"lang"`
		ServerUsername string `json:"server_username"`
		ServerPassword string `json:"server_password"`
		ServerDomain   string `json:"server_domain"`
	}
	if err := c.BodyParser(&req); err != nil {
		log.Printf("CreateSession: bad request - err=%v", err)
		return c.Status(400).JSON(fiber.Map{"error_message": "Invalid body"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		log.Printf("CreateSession: validation failed - %v", err)
		return c.Status(400).JSON(fiber.Map{"error_message": "image_id required"})
	}
	// SECURITY: identity comes from the authenticated session, NEVER from the body.
	// Trusting authUserID let any logged-in user impersonate another (incl. admins).
	authUserID, _ := c.Locals("user_id").(string)
	if authUserID == "" {
		return c.Status(401).JSON(fiber.Map{"error_message": "Not authenticated"})
	}
	log.Printf("CreateSession: image_id=%s user_id=%s", req.ImageID, authUserID)

	// Get workspace from our DB
	workspace, err := h.DB.GetWorkspace(req.ImageID)
	if err != nil {
		log.Printf("CreateSession: workspace not found: %v", err)
		return c.Status(404).JSON(fiber.Map{"error_message": "Workspace not found"})
	}
	log.Printf("CreateSession: workspace=%s image=%s", workspace.FriendlyName, workspace.DockerImage)

	// Check group access - admins bypass
	isAdmin := false
	if authUserID != "" {
		var role string
		err := h.DB.QueryRow(`SELECT COALESCE(role,'user') FROM "user" WHERE id = $1`, authUserID).Scan(&role)
		log.Printf("CreateSession: user role query err=%v role=%s", err, role)
		if err == nil {
			isAdmin = role == "admin"
		}
	}
	log.Printf("CreateSession: isAdmin=%v", isAdmin)
	if !isAdmin && authUserID != "" && !h.DB.UserCanAccessWorkspace(authUserID, req.ImageID) {
		return c.Status(403).JSON(fiber.Map{"error_message": "You don't have access to this workspace"})
	}

	// Check quotas (server workspaces don't consume CPU/memory, only session count)
	if workspace.WorkspaceType == "server" {
		if err := h.checkQuotas(authUserID, 0, 0); err != nil {
			return c.Status(429).JSON(fiber.Map{"error_message": err.Error()})
		}
	} else {
		if err := h.checkQuotas(authUserID, workspace.Cores, workspace.Memory); err != nil {
			return c.Status(429).JSON(fiber.Map{"error_message": err.Error()})
		}
	}

	// Find the agent to use
	agentID := req.AgentID
	var agentEndpoint, agentToken string
	if workspace.RestrictToAgent != "" {
		agentID = workspace.RestrictToAgent
	}
	if agentID == "" {
		if workspace.RestrictToRegion != "" {
			// Region-aware agent selection: only pick agents in the specified region
			err = h.DB.QueryRow(`SELECT id, endpoint, token FROM agent
				WHERE status = 'connected'
				AND last_heartbeat > NOW() - INTERVAL '2 minutes'
				AND region = $1
				ORDER BY active_sessions ASC
				LIMIT 1`, workspace.RestrictToRegion).Scan(&agentID, &agentEndpoint, &agentToken)
		} else {
			// Global agent selection: pick least-loaded agent with fresh heartbeat
			err = h.DB.QueryRow(`SELECT id, endpoint, token FROM agent
				WHERE status = 'connected'
				AND last_heartbeat > NOW() - INTERVAL '2 minutes'
				ORDER BY active_sessions ASC
				LIMIT 1`).Scan(&agentID, &agentEndpoint, &agentToken)
		}
	} else {
		err = h.DB.QueryRow(`SELECT endpoint, token FROM agent WHERE id = $1`, agentID).Scan(&agentEndpoint, &agentToken)
	}
	if err != nil || agentEndpoint == "" {
		return c.Status(500).JSON(fiber.Map{"error_message": "No agent available"})
	}

	// Generate session ID
	sessionID := uuid.New().String()

	if workspace.WorkspaceType == "server" {
		// --- Server workspace: connect via guacd, no K8s pod ---
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}
		// Use user-provided credentials for TSE/prompt mode
		srvUser := workspace.ServerUsername
		srvPass := workspace.ServerPassword
		srvDomain := workspace.ServerDomain
		if workspace.ServerAuthMode == "prompt" {
			srvUser = req.ServerUsername
			srvPass = req.ServerPassword
			if req.ServerDomain != "" {
				srvDomain = req.ServerDomain
			}
		}
		agentBody := map[string]interface{}{
			"session_id":       sessionID,
			"user_id":          authUserID,
			"lang":             lang,
			"protocol":         workspace.ServerProtocol,
			"hostname":         workspace.ServerHostname,
			"port":             workspace.ServerPort,
			"username":         srvUser,
			"password":         srvPass,
			"domain":           srvDomain,
			"ignore_cert":      workspace.ServerIgnoreCert,
			"security":         workspace.ServerSecurity,
			"default_settings": workspace.ServerDefaultSettings,
			"width":            workspace.XRes,
			"height":           workspace.YRes,
			"record_sessions":  workspace.RecordSessions,
			"workspace_name":   workspace.FriendlyName,
		}
		// Pass S3 config if recording is enabled
		if workspace.RecordSessions {
			agentBody["s3_endpoint"] = h.DB.GetSetting("s3.endpoint")
			agentBody["s3_access_key"] = h.DB.GetSetting("s3.access_key")
			agentBody["s3_secret_key"] = h.DB.GetSetting("s3.secret_key")
			agentBody["s3_bucket"] = h.DB.GetSetting("s3.bucket")
			agentBody["s3_region"] = h.DB.GetSetting("s3.region")
		}
		agentReqBody, _ := json.Marshal(agentBody)

		_, err := callAgent(agentEndpoint, agentToken, "/api/create-server-session", agentReqBody)
		if err != nil {
			h.DB.Exec(`UPDATE agent SET status = 'disconnected' WHERE id = $1`, agentID)
			log.Printf("Marked agent %s as disconnected after callAgent failure", agentID)
			return c.Status(500).JSON(fiber.Map{"error_message": "Failed to connect to agent"})
		}

		expiration := time.Now().Add(1 * time.Hour)
		if workspace.SessionTimeLimit > 0 {
			expiration = time.Now().Add(time.Duration(workspace.SessionTimeLimit) * time.Second)
		}
		dbSession := &models.WorkspaceSession{
			ID:          sessionID,
			UserID:      authUserID,
			WorkspaceID: workspace.ID,
			ContainerIP: workspace.ServerHostname,
			Status:      "running",
			AgentID:     agentID,
			SessionType: "server",
			ExpiresAt:   &expiration,
		}
		if err := h.DB.CreateWorkspaceSession(dbSession); err != nil {
			cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
			callAgent(agentEndpoint, agentToken, "/api/destroy-server-session", cleanupBody)
			return c.Status(500).JSON(fiber.Map{"error_message": "Failed to save session"})
		}

		var agentPublicURL string
		h.DB.QueryRow("SELECT COALESCE(public_url, '') FROM agent WHERE id = $1", agentID).Scan(&agentPublicURL)

		userRole, _ := c.Locals("user_role").(string)
		sessionToken, err := mintAndAdmitBearer(agentEndpoint, agentToken, sessionID, authUserID, userRole)
		if err != nil {
			cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
			callAgent(agentEndpoint, agentToken, "/api/destroy-server-session", cleanupBody)
			return c.Status(500).JSON(fiber.Map{"error_message": "Failed to issue session bearer"})
		}
		h.DB.LogAudit(authUserID, "", "create", "session", sessionID, workspace.FriendlyName, c.IP())
		metrics.SessionsCreatedTotal.Add(1)
		return c.JSON(fiber.Map{
			"session_id":    sessionID,
			"status":        "running",
			"agent_url":     agentPublicURL,
			"session_type":  "server",
			"session_token": sessionToken,
		})
	}

	// --- Container workspace: existing flow ---
	agentReqBody, _ := json.Marshal(map[string]interface{}{
		"session_id":      sessionID,
		"docker_image":    workspace.DockerImage,
		"cores":           workspace.Cores,
		"memory":          workspace.Memory,
		"shm_size":        workspace.SHMSize,
		"persistent":      workspace.Persistent,
		"persistent_size": workspace.PersistentSize,
		"user_id":         authUserID,
		"workspace_id":    workspace.ID,
		"run_config":      json.RawMessage(workspace.RunConfig),
		"exec_config":     json.RawMessage(workspace.ExecConfig),
		"volume_mappings": json.RawMessage(workspace.VolumeMappings),
		"docker_registry": workspace.DockerRegistry,
		"docker_user":     workspace.DockerUser,
		"docker_password": workspace.DockerPassword,
		"gpu_count":       workspace.GPUCount,
	})

	agentResp, err := callAgent(agentEndpoint, agentToken, "/api/create-session", agentReqBody)
	if err != nil {
		h.DB.Exec(`UPDATE agent SET status = 'disconnected' WHERE id = $1`, agentID)
		log.Printf("Marked agent %s as disconnected after callAgent failure", agentID)
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to connect to agent"})
	}

	var sessionResp struct {
		PodName     string `json:"pod_name"`
		ServiceName string `json:"service_name"`
		PodIP       string `json:"pod_ip"`
		VNCPassword string `json:"vnc_password"`
		Status      string `json:"status"`
	}
	json.Unmarshal(agentResp, &sessionResp)

	// Register container session with guacd (VNC TCP, default port 5900)
	lang := req.Lang
	if lang == "" {
		lang = "en"
	}
	vncPort := 5900
	guacBody := map[string]interface{}{
		"session_id":       sessionID,
		"user_id":          authUserID,
		"lang":             lang,
		"protocol":         "vnc",
		"hostname":         sessionResp.PodIP,
		"port":             vncPort,
		"username":         "",
		"password":         "",
		"domain":           "",
		"ignore_cert":      false,
		"security":         "",
		"default_settings": "{}",
		"width":            workspace.XRes,
		"height":           workspace.YRes,
		"record_sessions":  workspace.RecordSessions,
		"workspace_name":   workspace.FriendlyName,
	}
	if workspace.RecordSessions {
		guacBody["s3_endpoint"] = h.DB.GetSetting("s3.endpoint")
		guacBody["s3_access_key"] = h.DB.GetSetting("s3.access_key")
		guacBody["s3_secret_key"] = h.DB.GetSetting("s3.secret_key")
		guacBody["s3_bucket"] = h.DB.GetSetting("s3.bucket")
		guacBody["s3_region"] = h.DB.GetSetting("s3.region")
	}
	guacReqBody, _ := json.Marshal(guacBody)
	_, err = callAgent(agentEndpoint, agentToken, "/api/create-server-session", guacReqBody)
	if err != nil {
		log.Printf("Failed to register container session %s with guacd: %v", sessionID, err)
		// Cleanup pod
		cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
		callAgent(agentEndpoint, agentToken, "/api/destroy-session", cleanupBody)
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to register session with display proxy"})
	}

	expiration := time.Now().Add(1 * time.Hour)
	if workspace.SessionTimeLimit > 0 {
		expiration = time.Now().Add(time.Duration(workspace.SessionTimeLimit) * time.Second)
	}
	dbSession := &models.WorkspaceSession{
		ID:          sessionID,
		UserID:      authUserID,
		WorkspaceID: workspace.ID,
		PodName:     sessionResp.PodName,
		ServiceName: sessionResp.ServiceName,
		ContainerIP: sessionResp.PodIP,
		VNCPassword: sessionResp.VNCPassword,
		Status:      "running",
		AgentID:     agentID,
		SessionType: "container",
		ExpiresAt:   &expiration,
	}
	if err := h.DB.CreateWorkspaceSession(dbSession); err != nil {
		cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
		callAgent(agentEndpoint, agentToken, "/api/destroy-session", cleanupBody)
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to save session"})
	}

	var agentPublicURL string
	h.DB.QueryRow("SELECT COALESCE(public_url, '') FROM agent WHERE id = $1", agentID).Scan(&agentPublicURL)

	userRole, _ := c.Locals("user_role").(string)
	sessionToken, err := mintAndAdmitBearer(agentEndpoint, agentToken, sessionID, authUserID, userRole)
	if err != nil {
		cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
		callAgent(agentEndpoint, agentToken, "/api/destroy-session", cleanupBody)
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to issue session bearer"})
	}
	h.DB.LogAudit(authUserID, "", "create", "session", sessionID, workspace.FriendlyName, c.IP())
	metrics.SessionsCreatedTotal.Add(1)
	return c.JSON(fiber.Map{
		"session_id":    sessionID,
		"status":        "running",
		"agent_url":     agentPublicURL,
		"session_type":  "container",
		"session_token": sessionToken,
	})
}

// Check group quotas before creating session.
//
// SECURITY: serializes concurrent quota checks per user with a Postgres
// advisory lock keyed on the user_id hash. Without the lock, two parallel
// /api/sessions create requests could both pass the count<max_sessions
// check and both INSERT, exceeding the quota by N. The advisory lock is
// session-scoped (pg_advisory_lock) so we MUST release it; we use a
// transaction-scoped lock (pg_advisory_xact_lock) and have the caller wrap
// the subsequent INSERT in the same transaction. Callers that don't have
// a tx still benefit from the COUNT inside the same lock.
func (h *SessionHandler) checkQuotas(userID string, requestedCPU float64, requestedMemory int64) error {
	type GroupQuota struct {
		MaxSessions int     `db:"max_sessions"`
		MaxCPU      float64 `db:"max_cpu"`
		MaxMemory   int64   `db:"max_memory"`
	}

	// Take a short-lived xact-scoped advisory lock so two concurrent quota
	// checks for the same user are serialized.
	tx, err := h.DB.Beginx()
	if err != nil {
		return nil // fail open — DB issue, will surface elsewhere
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
		return nil
	}

	var quotas []GroupQuota
	tx.Select(&quotas, `
		SELECT COALESCE(g.max_sessions, 0) as max_sessions,
		       COALESCE(g.max_cpu, 0) as max_cpu,
		       COALESCE(g.max_memory, 0) as max_memory
		FROM oklavier_group g
		JOIN user_group ug ON ug.group_id = g.id
		WHERE ug.user_id = $1 AND (g.max_sessions > 0 OR g.max_cpu > 0 OR g.max_memory > 0)
	`, userID)

	if len(quotas) == 0 {
		return nil // No quotas defined
	}

	// Get current usage — inside the locked transaction so concurrent
	// quota checks for the same user see consistent counts.
	var currentSessions int
	tx.Get(&currentSessions, `SELECT COUNT(*) FROM workspace_session WHERE user_id = $1 AND status IN ('running', 'starting')`, userID)

	var currentCPU float64
	var currentMemory int64
	tx.QueryRow(`
		SELECT COALESCE(SUM(w.cores), 0), COALESCE(SUM(w.memory), 0)
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		WHERE ws.user_id = $1 AND ws.status IN ('running', 'starting')
	`, userID).Scan(&currentCPU, &currentMemory)

	for _, q := range quotas {
		if q.MaxSessions > 0 && currentSessions+1 > q.MaxSessions {
			return fmt.Errorf("Session limit reached (%d/%d)", currentSessions, q.MaxSessions)
		}
		if q.MaxCPU > 0 && currentCPU+requestedCPU > q.MaxCPU {
			return fmt.Errorf("CPU quota exceeded (%.1f/%.1f cores)", currentCPU+requestedCPU, q.MaxCPU)
		}
		if q.MaxMemory > 0 && currentMemory+requestedMemory > q.MaxMemory {
			return fmt.Errorf("Memory quota exceeded")
		}
	}

	return nil
}

// CallAgent forwards a request to an agent endpoint with retry logic.
func (h *SessionHandler) CallAgent(endpoint, token, path string, body []byte) ([]byte, error) {
	return callAgent(endpoint, token, path, body)
}

func callAgent(endpoint, token, path string, body []byte) ([]byte, error) {
	const maxAttempts = 3
	url := fmt.Sprintf("%s%s", strings.TrimRight(endpoint, "/"), path)

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s
			log.Printf("callAgent: retry %d/%d after %v for %s", attempt+1, maxAttempts, delay, path)
			time.Sleep(delay)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", token)

		resp, err := agentHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("callAgent: attempt %d failed (network): %v", attempt+1, err)
			continue // retry on network error
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("agent returned %d", resp.StatusCode)
			log.Printf("callAgent: attempt %d failed (status %d): %s", attempt+1, resp.StatusCode, string(respBody))
			continue // retry on 5xx
		}

		if resp.StatusCode >= 400 {
			log.Printf("callAgent: agent returned %d: %s", resp.StatusCode, string(respBody))
			return respBody, fmt.Errorf("agent error %d", resp.StatusCode)
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("callAgent failed after %d attempts: %w", maxAttempts, lastErr)
}

func (h *SessionHandler) GetSessionReadiness(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "session_id required"})
	}

	session, err := h.DB.GetSession(req.SessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}

	// SECURITY: enforce ownership. Anyone with a known session_id was previously
	// able to read pod_ip / phase and trigger UpdateSessionIP for arbitrary sessions.
	authUserID, _ := c.Locals("user_id").(string)
	authRole, _ := c.Locals("user_role").(string)
	if authRole != "admin" && session.UserID != authUserID {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}

	// Server sessions are immediately ready (no pod to wait for)
	if session.SessionType == "server" {
		return c.JSON(fiber.Map{"phase": "ready", "progress": 100, "pod_ip": session.ContainerIP})
	}

	// Route through agent if available
	if session.AgentID != "" {
		var endpoint, token string
		h.DB.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", session.AgentID).Scan(&endpoint, &token)
		if endpoint != "" {
			body, _ := json.Marshal(map[string]interface{}{
				"session_id": req.SessionID,
				"pod_name":   session.PodName,
				"pod_ip":     session.ContainerIP,
			})
			resp, err := callAgent(endpoint, token, "/api/session-readiness", body)
			if err == nil {
				var readiness map[string]interface{}
				json.Unmarshal(resp, &readiness)
				// Update DB with pod IP if available
				if ip, ok := readiness["pod_ip"].(string); ok && ip != "" && ip != session.ContainerIP {
					h.DB.UpdateSessionIP(session.ID, ip)
				}
				if phase, ok := readiness["phase"].(string); ok && phase == "ready" {
					h.DB.UpdateSessionStatus(session.ID, "running")
				}
				return c.JSON(readiness)
			}
		}
	}

	// Fallback: check directly (legacy)
	readiness := h.Agent.CheckReadiness(session.PodName, session.ContainerIP)
	if readiness.PodIP != "" && readiness.PodIP != session.ContainerIP {
		h.DB.UpdateSessionIP(session.ID, readiness.PodIP)
	}
	if readiness.Phase == "ready" && session.Status != "running" {
		h.DB.UpdateSessionStatus(session.ID, "running")
	}
	return c.JSON(readiness)
}

func (h *SessionHandler) DestroySession(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error_message": "Invalid body"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error_message": "session_id required"})
	}

	// Get session from our DB
	session, err := h.DB.GetSession(req.SessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error_message": "Session not found"})
	}

	// Ownership check: user must own the session or be admin
	userID, _ := c.Locals("user_id").(string)
	userRole, _ := c.Locals("user_role").(string)
	if session.UserID != userID && userRole != "admin" {
		return c.Status(403).JSON(fiber.Map{"error_message": "Access denied"})
	}

	if session.AgentID != "" {
		var endpoint, token string
		h.DB.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", session.AgentID).Scan(&endpoint, &token)
		if endpoint != "" {
			// Revoke any outstanding session bearers held by the agent.
			revokeBearer(endpoint, token, req.SessionID)
			body, _ := json.Marshal(map[string]string{"session_id": req.SessionID})
			destroyPath := "/api/destroy-session"
			if session.SessionType == "server" {
				destroyPath = "/api/destroy-server-session"
			}
			if _, err := callAgent(endpoint, token, destroyPath, body); err != nil {
				log.Printf("DestroySession: callAgent error for session %s: %v", req.SessionID, err)
			}
		}
	} else if session.PodName != "" {
		h.Agent.DestroySessionByPodName(session.PodName)
	}

	h.DB.DeleteSession(req.SessionID)
	h.DB.LogAudit(userID, "", "destroy", "session", req.SessionID, "", c.IP())
	return c.JSON(fiber.Map{"status": "ok"})
}

// GetSessionScreenshot returns the JPEG/PNG thumbnail for a session the
// caller owns. The bearer used to authenticate against the agent is minted
// and admitted server-side and is never sent to the browser. The caller
// authenticates via the standard user cookie (AuthRequired upstream).
func (h *SessionHandler) GetSessionScreenshot(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	authUserID, _ := c.Locals("user_id").(string)
	authRole, _ := c.Locals("user_role").(string)
	if sessionID == "" || authUserID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	session, err := h.DB.GetSession(sessionID)
	if err != nil {
		// Don't leak existence — return placeholder PNG so callers see the
		// same response whether the session exists or not.
		return sendPlaceholderPNG(c)
	}
	if authRole != "admin" && session.UserID != authUserID {
		return sendPlaceholderPNG(c)
	}
	if session.AgentID == "" {
		return sendPlaceholderPNG(c)
	}

	var agentEndpoint, agentToken string
	h.DB.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", session.AgentID).
		Scan(&agentEndpoint, &agentToken)
	if agentEndpoint == "" {
		return sendPlaceholderPNG(c)
	}

	bearer, err := mintAndAdmitBearer(agentEndpoint, agentToken, session.ID, session.UserID, authRole)
	if err != nil {
		return sendPlaceholderPNG(c)
	}

	// Fetch the screenshot from the agent. The bearer is in a header so it
	// doesn't appear in agent access logs.
	url := fmt.Sprintf("%s/api/screenshot/%s", strings.TrimRight(agentEndpoint, "/"), session.ID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return sendPlaceholderPNG(c)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := agentHTTPClient.Do(req)
	if err != nil {
		return sendPlaceholderPNG(c)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return sendPlaceholderPNG(c)
	}
	body, _ := io.ReadAll(resp.Body)
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "no-store")
	return c.Send(body)
}

// 1x1 transparent PNG, served when the caller can't see this session.
var placeholderPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x01, 0xE5, 0x27, 0xDE, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
	0x60, 0x82,
}

func sendPlaceholderPNG(c *fiber.Ctx) error {
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "no-store")
	return c.Send(placeholderPNG)
}

// ConnectSession mints a fresh short-lived session bearer for an existing
// session and pushes it to the agent. Called by the frontend right before
// opening the viewer, replacing the previous "embed a 2h JWT in the listing
// response and pass it through the URL fragment" pattern.
func (h *SessionHandler) ConnectSession(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error_message": "Invalid body"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error_message": "session_id required"})
	}
	authUserID, _ := c.Locals("user_id").(string)
	authRole, _ := c.Locals("user_role").(string)

	session, err := h.DB.GetSession(req.SessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error_message": "Session not found"})
	}
	if authRole != "admin" && session.UserID != authUserID {
		return c.Status(404).JSON(fiber.Map{"error_message": "Session not found"})
	}
	if session.AgentID == "" {
		return c.Status(400).JSON(fiber.Map{"error_message": "Session has no agent"})
	}

	var agentEndpoint, agentToken, agentPublicURL string
	h.DB.QueryRow("SELECT endpoint, token, COALESCE(public_url, '') FROM agent WHERE id = $1", session.AgentID).
		Scan(&agentEndpoint, &agentToken, &agentPublicURL)
	if agentEndpoint == "" {
		return c.Status(503).JSON(fiber.Map{"error_message": "Agent not available"})
	}

	bearer, err := mintAndAdmitBearer(agentEndpoint, agentToken, session.ID, session.UserID, authRole)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to issue session bearer"})
	}
	return c.JSON(fiber.Map{
		"session_id":    session.ID,
		"agent_url":     agentPublicURL,
		"session_token": bearer,
		"expires_in":    300,
	})
}

// ShadowSession creates a read-only shadow connection to an active session (admin only).
func (h *SessionHandler) ShadowSession(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "session_id required"})
	}

	// Get the original session
	session, err := h.DB.GetSession(req.SessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}
	if session.Status != "running" {
		return c.Status(400).JSON(fiber.Map{"error": "Session is not running"})
	}
	if session.AgentID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Session has no agent"})
	}

	// Get the agent endpoint
	var agentEndpoint, agentToken, agentPublicURL string
	h.DB.QueryRow("SELECT endpoint, token, COALESCE(public_url, '') FROM agent WHERE id = $1", session.AgentID).Scan(&agentEndpoint, &agentToken, &agentPublicURL)
	if agentEndpoint == "" {
		return c.Status(500).JSON(fiber.Map{"error": "Agent not available"})
	}

	// Generate a shadow session ID
	shadowSessionID := "shadow-" + uuid.New().String()
	adminUserID, _ := c.Locals("user_id").(string)

	// Call agent to create shadow session
	agentBody, _ := json.Marshal(map[string]string{
		"original_session_id": req.SessionID,
		"shadow_session_id":   shadowSessionID,
		"user_id":             adminUserID,
	})

	_, err = callAgent(agentEndpoint, agentToken, "/api/shadow-session", agentBody)
	if err != nil {
		log.Printf("ShadowSession: callAgent error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create shadow session"})
	}

	adminRole, _ := c.Locals("user_role").(string)
	sessionToken, err := mintAndAdmitBearer(agentEndpoint, agentToken, shadowSessionID, adminUserID, adminRole)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to issue session bearer"})
	}

	h.DB.LogAudit(adminUserID, "", "shadow", "session", req.SessionID, fmt.Sprintf("shadow_id=%s", shadowSessionID), c.IP())

	return c.JSON(fiber.Map{
		"status":        "ok",
		"shadow_id":     shadowSessionID,
		"agent_url":     agentPublicURL,
		"session_token": sessionToken,
	})
}
