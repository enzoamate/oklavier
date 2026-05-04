package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"oklavier-agent/internal/guacamole"
	"oklavier-agent/internal/heartbeat"
	"oklavier-agent/internal/provisioner"
)

var agentLogBuffer []string
var agentLogMu sync.Mutex

type agentLogWriter struct{}

func (w *agentLogWriter) Write(p []byte) (n int, err error) {
	agentLogMu.Lock()
	defer agentLogMu.Unlock()
	agentLogBuffer = append(agentLogBuffer, string(p))
	if len(agentLogBuffer) > 200 {
		agentLogBuffer = agentLogBuffer[len(agentLogBuffer)-200:]
	}
	return len(p), nil
}

// processRecording handles a guacd recording after session destroy:
// - If S3 is configured: upload to S3, delete local file, notify core
// - If no S3: keep local file on PVC, notify core with storage_type=local
func processRecording(recordingPath, sessionID, userID, workspaceName, s3Endpoint, s3AccessKey, s3SecretKey, s3Bucket, s3Region, controlPlane, agentToken string) {
	if recordingPath == "" {
		return
	}

	// Check if recording file exists
	info, err := os.Stat(recordingPath)
	if err != nil {
		log.Printf("[Recording] No recording file for session %s: %v", sessionID, err)
		return
	}
	fileSize := info.Size()
	if fileSize == 0 {
		log.Printf("[Recording] Recording file empty for session %s, skipping", sessionID)
		os.Remove(recordingPath)
		return
	}

	storageType := "local"
	s3Key := ""

	// Upload to S3 if configured
	if s3Bucket != "" && s3Endpoint != "" && s3AccessKey != "" {
		log.Printf("[Recording] Uploading recording for session %s to S3 (%d bytes)", sessionID, fileSize)

		endpoint := s3Endpoint
		useSSL := true
		if strings.HasPrefix(endpoint, "https://") {
			endpoint = strings.TrimPrefix(endpoint, "https://")
		} else if strings.HasPrefix(endpoint, "http://") {
			endpoint = strings.TrimPrefix(endpoint, "http://")
			useSSL = false
		}

		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""),
			Secure: useSSL,
			Region: s3Region,
		})
		if err != nil {
			log.Printf("[Recording] Failed to create S3 client: %v — keeping local", err)
		} else {
			s3Key = fmt.Sprintf("recordings/%s/%s.guac", time.Now().Format("2006-01-02"), sessionID)
			f, err := os.Open(recordingPath)
			if err != nil {
				log.Printf("[Recording] Failed to open recording: %v", err)
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				_, putErr := client.PutObject(ctx, s3Bucket, s3Key, f, fileSize, minio.PutObjectOptions{
					ContentType: "application/octet-stream",
				})
				cancel()
				f.Close()
				if putErr != nil {
					log.Printf("[Recording] S3 upload failed: %v — keeping local", putErr)
					s3Key = ""
				} else {
					storageType = "s3"
					os.Remove(recordingPath)
					log.Printf("[Recording] S3 upload complete: %s (%d bytes)", s3Key, fileSize)
				}
			}
		}
	} else {
		log.Printf("[Recording] No S3 configured, keeping recording locally for session %s (%d bytes)", sessionID, fileSize)
	}

	// Notify core backend
	body, _ := json.Marshal(map[string]interface{}{
		"session_id":     sessionID,
		"s3_key":         s3Key,
		"file_size":      fileSize,
		"user_id":        userID,
		"workspace_name": workspaceName,
		"storage_type":   storageType,
	})
	req, err := http.NewRequest("POST", controlPlane+"/api/agent/recording-uploaded", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Recording] Failed to create notify request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", agentToken)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Recording] Failed to notify core: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("[Recording] Core notified for session %s (storage=%s, status=%d)", sessionID, storageType, resp.StatusCode)
}

func main() {
	// Config from env
	controlPlane := os.Getenv("OKLAVIER_CONTROL_PLANE") // https://oklavier-api.example.com
	agentToken := os.Getenv("OKLAVIER_AGENT_TOKEN")
	agentName := os.Getenv("OKLAVIER_AGENT_NAME")
	namespace := os.Getenv("OKLAVIER_NAMESPACE")
	listenAddr := os.Getenv("LISTEN_ADDR")
	region := os.Getenv("OKLAVIER_REGION")
	publicURL := os.Getenv("OKLAVIER_PUBLIC_URL")
	frontendURL := os.Getenv("OKLAVIER_FRONTEND_URL") // public URL for browser redirects (e.g. https://oklavier.example.com)
	if frontendURL == "" {
		frontendURL = controlPlane // fallback
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	guacdAddr := os.Getenv("GUACD_ADDRESS")
	if guacdAddr == "" {
		guacdAddr = "guacd:4822"
	}
	valkeyURL := os.Getenv("VALKEY_URL")
	if valkeyURL == "" {
		valkeyURL = "oklavier-valkey.oklavier.svc.cluster.local:6379"
	}

	// Capture logs
	log.SetOutput(io.MultiWriter(os.Stdout, &agentLogWriter{}))

	if controlPlane == "" || agentToken == "" {
		log.Fatal("OKLAVIER_CONTROL_PLANE and OKLAVIER_AGENT_TOKEN are required")
	}
	if namespace == "" {
		namespace = "default"
	}
	if listenAddr == "" {
		listenAddr = ":80"
	}
	if agentName == "" {
		hostname, _ := os.Hostname()
		agentName = hostname
	}
	if region == "" {
		region = "default"
	}

	// Init Valkey (Redis) client
	valkeyClient := redis.NewClient(&redis.Options{
		Addr: valkeyURL,
	})
	if err := valkeyClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Valkey at %s: %v", valkeyURL, err)
	}
	log.Printf("Valkey connected (%s)", valkeyURL)

	// Init K8s provisioner (in-cluster)
	prov, err := provisioner.New(namespace)
	if err != nil {
		log.Fatalf("Failed to init provisioner: %v", err)
	}
	log.Printf("Provisioner ready (namespace: %s)", namespace)

	// Init guacamole manager for server sessions (RDP/VNC via guacd) — state in Valkey
	guacManager := guacamole.NewManager(guacdAddr, valkeyClient)
	log.Printf("Guacamole manager ready (guacd: %s, valkey: %s)", guacdAddr, valkeyURL)

	// Start heartbeat to control plane
	hb := heartbeat.New(controlPlane, agentToken, agentName, region, namespace, publicURL, prov, guacManager)
	go hb.Start()

	// HTTP server for control plane commands
	app := fiber.New(fiber.Config{
		AppName:      "Oklavier Agent",
		ServerHeader: "oklavier-agent",
		BodyLimit:    10 * 1024 * 1024, // 10MB max
	})
	app.Use(logger.New(logger.Config{Format: "${time} | ${status} | ${latency} | ${method} | ${path}\n"}))
	// CORS: allow only the frontend URL and the agent's own domain
	allowedOrigins := frontendURL
	if publicURL != "" {
		allowedOrigins += ", https://" + publicURL
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST",
		AllowHeaders:     "Content-Type,X-Agent-Token,Authorization",
		AllowCredentials: false,
	}))

	// JWT session auth — verifies token from #token= fragment (passed as cookie or WS query param)
	verifySessionJWT := func(tokenStr, sessionID string) (userID string, role string, ok bool) {
		if jwtSecret == "" || tokenStr == "" {
			return "", "", false
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return "", "", false
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return "", "", false
		}
		// Verify session_id matches (prevent token reuse for other sessions)
		if sid, _ := claims["session_id"].(string); sid != sessionID {
			return "", "", false
		}
		uid, _ := claims["user_id"].(string)
		r, _ := claims["role"].(string)
		return uid, r, true
	}

	// Middleware: require valid session JWT (from Authorization header or query param)
	requireSessionAuth := func(c *fiber.Ctx) error {
		if jwtSecret == "" {
			return c.Next() // No JWT_SECRET configured, skip auth (dev mode)
		}
		sessionID := c.Params("sessionId")
		// Try Authorization: Bearer <token> header first
		tokenStr := ""
		if auth := c.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}
		if tokenStr == "" {
			tokenStr = c.Query("token") // fallback for WebSocket upgrades
		}
		if tokenStr == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
		}
		userID, role, valid := verifySessionJWT(tokenStr, sessionID)
		if !valid {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired token"})
		}
		c.Locals("user_id", userID)
		c.Locals("user_role", role)
		return c.Next()
	}

	// Health check (no auth needed)
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "agent": agentName, "region": region, "namespace": namespace})
	})

	// Prometheus-compatible metrics endpoint (no auth needed)
	app.Get("/metrics", func(c *fiber.Ctx) error {
		guacSessions := guacManager.SessionCount()
		podSessions := prov.CountActiveSessions()
		total := guacSessions + podSessions
		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return c.SendString(fmt.Sprintf(
			"# HELP oklavier_agent_sessions Number of active sessions\n"+
				"# TYPE oklavier_agent_sessions gauge\n"+
				"oklavier_agent_sessions{type=\"guacd\"} %d\n"+
				"oklavier_agent_sessions{type=\"container\"} %d\n"+
				"oklavier_agent_sessions{type=\"total\"} %d\n",
			guacSessions, podSessions, total))
	})

	// Destroy session from viewer (requires JWT auth)
	app.Post("/sessions/:sessionId/destroy", requireSessionAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")

		// Get user_id and recording info before destroying
		recPath, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg := guacManager.GetSessionInfo(sessionID)
		sess := guacManager.GetSession(sessionID)
		userID := ""
		if sess != nil {
			userID = sess.UserID
		}

		guacManager.DestroySession(sessionID)
		log.Printf("[Viewer] Session %s destroyed from viewer (user=%s)", sessionID, userID)

		// Upload recording in background
		if recPath != "" {
			go processRecording(recPath, sessionID, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg, controlPlane, agentToken)
		}

		// Notify core backend to clean up the DB
		go func() {
			body, _ := json.Marshal(map[string]string{"session_id": sessionID, "user_id": userID})
			req, err := http.NewRequest("POST", controlPlane+"/api/agent/destroy-session", bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Agent-Token", agentToken)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[Viewer] Failed to notify core: %v", err)
				return
			}
			resp.Body.Close()
			log.Printf("[Viewer] Core notified, session %s cleaned up (status=%d)", sessionID, resp.StatusCode)
		}()

		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Update session display settings (requires JWT auth)
	app.Post("/sessions/:sessionId/settings", requireSessionAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		var params map[string]string
		if err := c.BodyParser(&params); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		guacManager.UpdateSessionParams(sessionID, params)
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Screenshot upload from viewer (requires JWT auth)
	app.Post("/api/screenshot/:sessionId/upload", requireSessionAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		body := c.Body()
		if len(body) > 0 {
			guacManager.SetScreenshot(sessionID, body)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Unified Session Viewer — detects session type automatically
	app.Get("/sessions/:sessionId", func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		c.Set("Content-Type", "text/html")

		// All sessions go through guacd (server + container)
		if sess := guacManager.GetSession(sessionID); sess != nil {
			return c.SendString(guacViewerHTML(sessionID, agentName, frontendURL, sess.Protocol, sess.Lang, sess.DefaultSettings, sess.IsShadow))
		}

		// No session found — redirect to workspaces
		return c.Redirect(frontendURL + "/workspaces")
	})

	// Guacamole WebSocket tunnel (browser guacamole-common-js ↔ guacd)
	app.Use("/guac-ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/guac-ws/:sessionId", websocket.New(func(ws *websocket.Conn) {
		sessionID := ws.Params("sessionId")
		// Verify JWT from query param (WebSocket can't set Authorization header)
		tokenStr := ws.Query("token")
		if jwtSecret != "" {
			_, _, valid := verifySessionJWT(tokenStr, sessionID)
			if !valid {
				log.Printf("[GuacWS] Unauthorized WebSocket for session %s", sessionID)
				ws.Close()
				return
			}
		}
		clientW, _ := strconv.Atoi(ws.Query("w"))
		clientH, _ := strconv.Atoi(ws.Query("h"))
		guacdConn, err := guacManager.ConnectForTunnel(sessionID, clientW, clientH)
		if err != nil {
			log.Printf("[GuacWS] Failed to connect for session %s: %v", sessionID, err)
			ws.Close()
			return
		}
		tunnel := guacamole.NewTunnel(guacdConn)
		tunnel.OnScreenshot = func(data []byte) {
			guacManager.SetScreenshot(sessionID, data)
		}
		// Poll Valkey to detect session destruction (stateless kill mechanism)
		tunnel.SessionExists = func() bool {
			return guacManager.SessionExists(sessionID)
		}
		tunnel.HandleWebSocket(ws)
		guacdConn.Close()
	}))

	// Generic screenshot endpoint (public — works for both container and server sessions via guacd)
	app.Get("/api/screenshot/:sessionId", func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")

		data := guacManager.GetScreenshot(sessionID)
		if data != nil {
			c.Set("Content-Type", "image/png")
			return c.Send(data)
		}

		// No screenshot yet — return 1x1 transparent PNG
		c.Set("Content-Type", "image/png")
		return c.Send([]byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
			0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x00, 0x00, 0x02,
			0x00, 0x01, 0xE5, 0x27, 0xDE, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
			0x60, 0x82,
		})
	})

	// Download local recording (requires agent token auth)
	app.Get("/api/recordings/:sessionId/download", func(c *fiber.Ctx) error {
		token := c.Get("X-Agent-Token")
		if token != agentToken {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid agent token"})
		}
		sessionID := c.Params("sessionId")
		filePath := "/tmp/guac-recordings/" + sessionID
		if _, err := os.Stat(filePath); err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.guac\"", sessionID))
		return c.SendFile(filePath)
	})

	// List local recordings (requires agent token auth)
	app.Get("/api/recordings", func(c *fiber.Ctx) error {
		token := c.Get("X-Agent-Token")
		if token != agentToken {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid agent token"})
		}
		entries, err := os.ReadDir("/tmp/guac-recordings")
		if err != nil {
			return c.JSON(fiber.Map{"recordings": []string{}})
		}
		var files []fiber.Map
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, _ := e.Info()
			if info != nil {
				files = append(files, fiber.Map{"name": e.Name(), "size": info.Size(), "modified": info.ModTime()})
			}
		}
		if files == nil {
			files = []fiber.Map{}
		}
		return c.JSON(fiber.Map{"recordings": files})
	})

	// Auth middleware - verify agent token
	app.Use("/api", func(c *fiber.Ctx) error {
		token := c.Get("X-Agent-Token")
		if token != agentToken {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid agent token"})
		}
		return c.Next()
	})

	// API routes (called by control plane)
	api := app.Group("/api")
	api.Post("/create-session", prov.HandleCreateSession)
	api.Post("/destroy-session", func(c *fiber.Ctx) error {
		// Clean up guacd session before destroying the K8s pod
		var req struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(c.Body(), &req); err == nil && req.SessionID != "" {
			// Capture recording info before destroying the session
			recPath, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg := guacManager.GetSessionInfo(req.SessionID)
			guacManager.DestroySession(req.SessionID)
			log.Printf("[API] Destroyed guacd session for %s", req.SessionID)
			// Upload recording in background
			if recPath != "" {
				go processRecording(recPath, req.SessionID, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg, controlPlane, agentToken)
			}
		}
		// Destroy K8s pod + service + secrets
		return prov.HandleDestroySession(c)
	})

	// Server session endpoints (guacd-based)
	api.Post("/create-server-session", func(c *fiber.Ctx) error {
		var req guacamole.CreateServerSessionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if err := guacManager.CreateSession(req); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "connected", "session_id": req.SessionID})
	})
	api.Post("/shadow-session", func(c *fiber.Ctx) error {
		var req struct {
			OriginalSessionID string `json:"original_session_id"`
			ShadowSessionID   string `json:"shadow_session_id"`
			UserID            string `json:"user_id"`
			Lang              string `json:"lang"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		originalSess := guacManager.GetSession(req.OriginalSessionID)
		if originalSess == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Original session not found"})
		}
		lang := req.Lang
		if lang == "" {
			lang = originalSess.Lang
		}
		shadowReq := guacamole.CreateServerSessionRequest{
			SessionID:  req.ShadowSessionID,
			UserID:     req.UserID,
			Lang:       lang,
			Protocol:   originalSess.Protocol,
			Hostname:   originalSess.Hostname,
			Port:       originalSess.Port,
			Username:   originalSess.Params["username"],
			Password:   originalSess.Params["password"],
			Domain:     originalSess.Params["domain"],
			IgnoreCert: originalSess.Params["ignore-cert"] == "true",
			Security:   originalSess.Params["security"],
			Width:      1920,
			Height:     1080,
			ReadOnly:   true,
		}
		if err := guacManager.CreateSession(shadowReq); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		log.Printf("[ShadowSession] Created shadow %s for original %s (user=%s)", req.ShadowSessionID, req.OriginalSessionID, req.UserID)
		return c.JSON(fiber.Map{"status": "ok", "session_id": req.ShadowSessionID})
	})
	api.Post("/destroy-server-session", func(c *fiber.Ctx) error {
		var req struct {
			SessionID string `json:"session_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		// Capture recording info before destroying
		recPath, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg := guacManager.GetSessionInfo(req.SessionID)
		if err := guacManager.DestroySession(req.SessionID); err != nil {
			log.Printf("[GuacAPI] Destroy error: %v", err)
		}
		// Upload recording in background
		if recPath != "" {
			go processRecording(recPath, req.SessionID, recUserID, recWorkspace, s3Ep, s3AK, s3SK, s3Bkt, s3Rg, controlPlane, agentToken)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})
	api.Post("/session-status", prov.HandleSessionStatus)
	api.Post("/session-readiness", prov.HandleSessionReadiness)
	api.Get("/logs", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"logs": agentLogBuffer})
	})

	log.Printf("Oklavier Agent '%s' (region: %s) starting on %s", agentName, region, listenAddr)
	log.Printf("Control plane: %s", controlPlane)
	log.Fatal(app.Listen(listenAddr))
}

func sessionViewerHTML(sessionID, agentName, controlPlaneURL string) string {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Oklavier - Session</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0f1225; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, sans-serif; }
#video { width: 100vw; height: 100vh; object-fit: contain; cursor: none; background: #000; }
#canvas { display: none; width: 100vw; height: 100vh; object-fit: contain; cursor: none; background: #000; }
#overlay { position: fixed; inset: 0; display: flex; align-items: center; justify-content: center; background: #0f1225; z-index: 10; transition: opacity 0.5s; }
#overlay.hidden { opacity: 0; pointer-events: none; }
.spinner { width: 80px; height: 80px; border: 3px solid rgba(112,150,255,0.15); border-top-color: #7096ff; border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
#status { color: rgba(255,255,255,0.5); font-size: 14px; margin-top: 16px; text-align: center; }
#controls { position: fixed; top: 8px; right: 8px; z-index: 20; display: flex; gap: 4px; opacity: 0; transition: opacity 0.2s; }
#controls:hover { opacity: 1; }
.ctrl-btn { background: rgba(0,0,0,0.5); border: none; color: rgba(255,255,255,0.6); padding: 6px 10px; border-radius: 6px; cursor: pointer; font-size: 12px; }
.ctrl-btn:hover { color: #fff; background: rgba(0,0,0,0.7); }
#stats { position: fixed; top: 40px; right: 8px; background: rgba(0,0,0,0.8); border-radius: 8px; padding: 8px 12px; color: rgba(255,255,255,0.5); font-size: 11px; font-family: monospace; display: none; z-index: 20; }
#back { position: fixed; bottom: 12px; left: 12px; z-index: 20; background: rgba(0,0,0,0.5); border: 1px solid rgba(255,255,255,0.1); color: rgba(255,255,255,0.6); padding: 8px 16px; border-radius: 8px; cursor: pointer; font-size: 13px; text-decoration: none; opacity: 0; transition: opacity 0.2s; }
#back:hover { opacity: 1 !important; color: #fff; }
body:hover #back { opacity: 0.3; }
body:hover #controls { opacity: 0.3; }
</style>
</head>
<body>
<div id="overlay">
  <div style="text-align:center">
    <div class="spinner"></div>
    <p id="status">Connecting to workspace...</p>
  </div>
</div>

<video id="video" autoplay playsinline muted></video>
<canvas id="canvas"></canvas>

<div id="controls">
  <button class="ctrl-btn" onclick="toggleFullscreen()">&#x26F6;</button>
  <button class="ctrl-btn" onclick="toggleStats()">stats</button>
</div>

<div id="stats"></div>

<a id="back" href="javascript:void(0)" onclick="goBack()">&#8592; Back to workspaces</a>

<script>
var SESSION_ID = "SESSION_ID_PLACEHOLDER";
var AGENT_NAME = "AGENT_NAME_PLACEHOLDER";
var CONTROL_PLANE = "CONTROL_PLANE_PLACEHOLDER";
var BASE = window.location.origin;

var pc = null;
var ws = null;
var connected = false;
var cpuMode = false;
var canvasW = 1920;
var canvasH = 1080;

// Key mapping (browser key -> X11 keysym)
var KEY_MAP = {
  Backspace:0xff08,Tab:0xff09,Enter:0xff0d,Escape:0xff1b,Delete:0xffff,
  Home:0xff50,End:0xff57,PageUp:0xff55,PageDown:0xff56,
  ArrowLeft:0xff51,ArrowUp:0xff52,ArrowRight:0xff53,ArrowDown:0xff54,
  ShiftLeft:0xffe1,ShiftRight:0xffe2,ControlLeft:0xffe3,ControlRight:0xffe4,
  AltLeft:0xffe9,AltRight:0xffea,MetaLeft:0xffeb,MetaRight:0xffec,
  F1:0xffbe,F2:0xffbf,F3:0xffc0,F4:0xffc1,F5:0xffc2,F6:0xffc3,
  F7:0xffc4,F8:0xffc5,F9:0xffc6,F10:0xffc7,F11:0xffc8,F12:0xffc9,
  Space:0x20,Insert:0xff63,CapsLock:0xffe5
};

function setStatus(text) { document.getElementById('status').textContent = text; }

function startProxy() {
  setStatus('Starting proxy...');
  return fetch(BASE + '/proxy/connect', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      session_id: SESSION_ID,
      protocol: 'vnc',
      target_host: '',
      target_port: 5900,
      width: 1920,
      height: 1080
    })
  }).then(function(res) { return res.json(); })
    .then(function(data) {
      if (data.ok) return data;
      setStatus('Waiting for workspace... (' + (data.error || '') + ')');
      return null;
    })
    .catch(function() {
      setStatus('Connecting...');
      return null;
    });
}

function connectWebRTC() {
  setStatus('Establishing WebRTC...');

  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  });

  pc.ontrack = function(e) {
    console.log('[WebRTC] Got track:', e.track.kind);
    var video = document.getElementById('video');
    video.srcObject = e.streams[0];
    video.play().catch(function() {});
  };

  pc.oniceconnectionstatechange = function() {
    console.log('[WebRTC] ICE:', pc.iceConnectionState);
    if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
      connected = true;
      document.getElementById('overlay').classList.add('hidden');
    }
    if (pc.iceConnectionState === 'disconnected' || pc.iceConnectionState === 'failed') {
      connected = false;
      document.getElementById('overlay').classList.remove('hidden');
      setStatus('Connection lost. Reconnecting...');
      setTimeout(init, 3000);
    }
  };

  pc.addTransceiver('video', { direction: 'recvonly' });

  return pc.createOffer().then(function(offer) {
    return pc.setLocalDescription(offer).then(function() { return offer; });
  }).then(function(offer) {
    return fetch(BASE + '/proxy/webrtc/offer/' + SESSION_ID, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sdp: offer.sdp, type: offer.type })
    });
  }).then(function(res) { return res.json(); })
    .then(function(answer) {
      if (answer.error) {
        setStatus('WebRTC error: ' + answer.error);
        return;
      }
      return pc.setRemoteDescription(new RTCSessionDescription({ type: 'answer', sdp: answer.sdp }));
    }).then(function() {
      pc.onicecandidate = function(e) {
        if (e.candidate) {
          fetch(BASE + '/proxy/webrtc/ice/' + SESSION_ID, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(e.candidate.toJSON())
          }).catch(function() {});
        }
      };
    });
}

function connectWS() {
  var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + location.host + '/proxy/ws/' + SESSION_ID);
  ws.onopen = function() { console.log('[WS] Connected'); };
  ws.onmessage = function(e) {
    try {
      var msg = JSON.parse(e.data);
      if (msg.ch === 'control' && msg.type === 'pong') {
        latencyMs = Math.round(performance.now() - pingSent);
      }
      if (msg.ch === 'display') {
        var canvas = document.getElementById('canvas');
        var ctx = canvas.getContext('2d');
        if (msg.type === 'init') {
          // Switch to canvas mode (CPU mode)
          cpuMode = true;
          canvasW = msg.data.w;
          canvasH = msg.data.h;
          canvas.width = msg.data.w;
          canvas.height = msg.data.h;
          document.getElementById('video').style.display = 'none';
          canvas.style.display = 'block';
          connected = true;
          document.getElementById('overlay').classList.add('hidden');
        }
        if (msg.type === 'frame' || msg.type === 'rect') {
          var jpegData = msg.data.jpeg;
          var img = new Image();
          img.onload = function() {
            ctx.drawImage(img, msg.data.x, msg.data.y, msg.data.w, msg.data.h);
          };
          img.src = 'data:image/jpeg;base64,' + jpegData;
          trackCpuFrame(1, jpegData.length);
        }
      }
    } catch(ex) {}
  };
  ws.onclose = function() { console.log('[WS] Disconnected'); setTimeout(connectWS, 3000); };
  ws.onerror = function() { ws.close(); };
}

function sendInput(type, data) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ ch: 'input', type: type, data: data }));
  }
}

// Keyboard
document.addEventListener('keydown', function(e) {
  e.preventDefault();
  var key = KEY_MAP[e.code] || (e.key.length === 1 ? e.key.charCodeAt(0) : 0);
  if (key) sendInput('key', { key: key, down: true });
});
document.addEventListener('keyup', function(e) {
  e.preventDefault();
  var key = KEY_MAP[e.code] || (e.key.length === 1 ? e.key.charCodeAt(0) : 0);
  if (key) sendInput('key', { key: key, down: false });
});

// Mouse — bind to both video and canvas
var video = document.getElementById('video');
var canvasEl = document.getElementById('canvas');
function mousePos(e) {
  var el = cpuMode ? canvasEl : video;
  var r = el.getBoundingClientRect();
  var nw = cpuMode ? canvasW : (video.videoWidth || 1920);
  var nh = cpuMode ? canvasH : (video.videoHeight || 1080);
  return { x: Math.round((e.clientX - r.left) * nw / r.width), y: Math.round((e.clientY - r.top) * nh / r.height) };
}
function bindMouse(el) {
  el.addEventListener('mousemove', function(e) { var p = mousePos(e); sendInput('mouse', { x: p.x, y: p.y, buttons: e.buttons }); });
  el.addEventListener('mousedown', function(e) { e.preventDefault(); var p = mousePos(e); var b = 0; if(e.button===0)b=1;if(e.button===1)b=2;if(e.button===2)b=4; sendInput('mouse', { x: p.x, y: p.y, buttons: b }); });
  el.addEventListener('mouseup', function(e) { var p = mousePos(e); sendInput('mouse', { x: p.x, y: p.y, buttons: 0 }); });
  el.addEventListener('wheel', function(e) { e.preventDefault(); var p = mousePos(e); var b = e.deltaY < 0 ? 8 : 16; sendInput('mouse', { x: p.x, y: p.y, buttons: b }); setTimeout(function() { sendInput('mouse', { x: p.x, y: p.y, buttons: 0 }); }, 50); });
  el.addEventListener('contextmenu', function(e) { e.preventDefault(); });
}
bindMouse(video);
bindMouse(canvasEl);

// Clipboard paste
window.addEventListener('paste', function(e) {
  var text = e.clipboardData ? e.clipboardData.getData('text') : '';
  if (text && ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ ch: 'clipboard', type: 'set', data: { text: text } }));
  }
});

// Controls
function toggleFullscreen() {
  if (!document.fullscreenElement) document.documentElement.requestFullscreen();
  else document.exitFullscreen();
}
var showStats = false;
function toggleStats() { showStats = !showStats; document.getElementById('stats').style.display = showStats ? 'block' : 'none'; }

// Latency measurement via WebSocket ping/pong
var latencyMs = 0;
var pingSent = 0;
setInterval(function() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    pingSent = performance.now();
    ws.send(JSON.stringify({ ch: 'control', type: 'ping' }));
  }
}, 1000);

// Stats tracking
var prevBytes = 0;
var prevTime = performance.now();
var cpuFrameCount = 0;
var cpuRectCount = 0;
var cpuBytesTotal = 0;
var cpuLastFps = 0;
var cpuFpsCounter = 0;
var cpuFpsTime = performance.now();

// Count CPU mode frames
function trackCpuFrame(rectCount, bytes) {
  cpuFrameCount++;
  cpuRectCount += rectCount;
  cpuBytesTotal += bytes;
  cpuFpsCounter++;
  var now = performance.now();
  if (now - cpuFpsTime >= 1000) {
    cpuLastFps = cpuFpsCounter;
    cpuFpsCounter = 0;
    cpuFpsTime = now;
  }
}

// Stats polling — works for both GPU (WebRTC) and CPU (JPEG) modes
setInterval(function() {
  if (!connected && !cpuMode) return;
  var el = document.getElementById('stats');
  if (!el || el.style.display === 'none') return;

  var html = '<b style="color:#7096ff">Latency: ' + latencyMs + 'ms</b><br>';
  html += 'Mode: ' + (cpuMode ? '<span style="color:#f59e0b">CPU (JPEG)</span>' : '<span style="color:#10b981">GPU (WebRTC)</span>') + '<br>';
  html += 'GPU: ' + (cpuMode ? '<span style="color:#ef4444">No</span>' : '<span style="color:#10b981">Yes</span>') + '<br>';
  html += 'Protocol: VNC<br>';
  html += 'Resolution: ' + canvasW + 'x' + canvasH + '<br>';
  html += '<hr style="border-color:rgba(255,255,255,0.1);margin:4px 0">';

  if (cpuMode) {
    var now = performance.now();
    var elapsed = (now - prevTime) / 1000;
    var bitrate = elapsed > 0 ? ((cpuBytesTotal * 8 / elapsed / 1000).toFixed(0)) : '0';
    html += 'FPS: ' + cpuLastFps + '<br>';
    html += 'Bitrate: ~' + bitrate + ' kbps<br>';
    html += 'Codec: JPEG (dirty rects)<br>';
    html += 'Frames: ' + cpuFrameCount + '<br>';
    html += 'Rects sent: ' + cpuRectCount + '<br>';
    html += 'Data: ' + (cpuBytesTotal / 1024).toFixed(0) + ' KB<br>';
  } else if (pc) {
    pc.getStats().then(function(stats) {
      stats.forEach(function(r) {
        if (r.type === 'inbound-rtp' && r.kind === 'video') {
          var now2 = performance.now();
          var elapsed2 = (now2 - prevTime) / 1000;
          var bitrate2 = elapsed2 > 0 ? ((r.bytesReceived - prevBytes) * 8 / elapsed2 / 1000).toFixed(0) : '0';
          prevBytes = r.bytesReceived || 0;
          prevTime = now2;
          var jitter = r.jitter ? (r.jitter * 1000).toFixed(1) : '0';
          var codec = '';
          stats.forEach(function(c) { if (c.type === 'codec' && c.id === r.codecId) codec = c.mimeType; });
          html += 'FPS: ' + (r.framesPerSecond||0) + '<br>';
          html += 'Bitrate: ' + bitrate2 + ' kbps<br>';
          html += 'Codec: ' + (codec || 'H.264') + '<br>';
          html += 'Transport: UDP (SRTP)<br>';
          html += 'Jitter: ' + jitter + 'ms<br>';
          html += 'Packets lost: ' + (r.packetsLost||0) + '<br>';
          html += 'Frames: ' + (r.framesReceived||0) + ' / dropped: ' + (r.framesDropped||0);
          el.innerHTML = html;
        }
      });
    });
    return;
  }
  el.innerHTML = html;
}, 2000);

// Back button
function goBack() {
  if (CONTROL_PLANE && CONTROL_PLANE !== 'CONTROL_PLANE_PLACEHOLDER') {
    window.location.href = CONTROL_PLANE + '/workspaces';
  } else {
    window.history.back();
  }
}

// Main init
function init() {
  var attempts = 0;
  function tryProxy() {
    if (attempts >= 60) { setStatus('Failed to connect after 60 attempts'); return; }
    attempts++;
    startProxy().then(function(data) {
      if (data) {
        // Always connect WebSocket for control + input + clipboard
        connectWS();
        // In CPU mode, display comes via WebSocket — skip WebRTC
        // The display init message from the server will switch to canvas mode
        setStatus('Connecting WebRTC...');
        connectWebRTC().catch(function(e) {
          // WebRTC may fail in CPU mode (no video track) — that is OK,
          // display will be handled via WebSocket JPEG rects on the canvas
          console.log('[WebRTC] Setup error (may be CPU mode):', e.message);
        });
      } else {
        setTimeout(tryProxy, 2000);
      }
    });
  }
  tryProxy();
}

init();
</script>
</body>
</html>`
	html = strings.Replace(html, "SESSION_ID_PLACEHOLDER", sessionID, 1)
	html = strings.Replace(html, "AGENT_NAME_PLACEHOLDER", agentName, 1)
	html = strings.Replace(html, "CONTROL_PLANE_PLACEHOLDER", controlPlaneURL, 1)
	return html
}

func legacyViewerHTML(sessionID, agentName, controlPlaneURL string) string {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Oklavier - Session</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #000; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, sans-serif; }
#screen { width: 100vw; height: 100vh; }
#overlay { position: fixed; inset: 0; display: flex; align-items: center; justify-content: center; background: #0f1225; z-index: 50; transition: opacity 0.5s; }
#overlay.hidden { opacity: 0; pointer-events: none; }
.spinner { width: 80px; height: 80px; border: 3px solid rgba(112,150,255,0.15); border-top-color: #7096ff; border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
#status { color: rgba(255,255,255,0.5); font-size: 14px; margin-top: 16px; }
#toolbar { position: fixed; top: 0; right: 0; z-index: 40; display: flex; gap: 2px; padding: 4px; opacity: 0; transition: opacity 0.2s; }
#toolbar:hover { opacity: 1; }
body:hover #toolbar { opacity: 0.3; }
.tb { background: rgba(0,0,0,0.6); border: none; color: rgba(255,255,255,0.7); padding: 6px 10px; border-radius: 4px; cursor: pointer; font-size: 11px; }
.tb:hover { background: rgba(0,0,0,0.8); color: #fff; }
#stats { position: fixed; top: 36px; right: 4px; background: rgba(0,0,0,0.85); border-radius: 6px; padding: 8px 10px; color: rgba(255,255,255,0.5); font-size: 10px; font-family: monospace; display: none; z-index: 40; line-height: 1.6; }
#back { position: fixed; bottom: 8px; left: 8px; z-index: 40; background: rgba(0,0,0,0.5); border: 1px solid rgba(255,255,255,0.1); color: rgba(255,255,255,0.5); padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; text-decoration: none; opacity: 0; transition: opacity 0.2s; }
body:hover #back { opacity: 0.2; }
#back:hover { opacity: 1 !important; color: #fff; }
</style>
</head>
<body>
<div id="overlay">
  <div style="text-align:center">
    <div class="spinner"></div>
    <p id="status">Connecting to workspace...</p>
  </div>
</div>

<div id="screen"></div>

<div id="toolbar">
  <button class="tb" onclick="toggleFullscreen()">&#x26F6; Fullscreen</button>
  <button class="tb" onclick="sendCtrlAltDel()">Ctrl+Alt+Del</button>
  <button class="tb" onclick="toggleStats()">Stats</button>
</div>

<div id="stats"></div>

<a id="back" href="javascript:void(0)" onclick="goBack()">&#8592; Back</a>

<script type="module">
import RFB from '/novnc/core/rfb.js';

const SESSION_ID = 'SESSION_ID_PLACEHOLDER';
const CONTROL_PLANE = 'CONTROL_PLANE_PLACEHOLDER';

let rfb = null;
let connected = false;
let showStatsPanel = false;

function setStatus(text) {
  document.getElementById('status').textContent = text;
}

// Build WebSocket URL to agent's VNC proxy
// The agent proxies /vnc-ws/:sessionId/websockify → pod VNC
const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsUrl = proto + '//' + location.host + '/vnc-ws/' + SESSION_ID + '/websockify';

setStatus('Connecting to VNC...');

try {
  rfb = new RFB(
    document.getElementById('screen'),
    wsUrl,
    {
      credentials: { password: '' },
      wsProtocols: ['binary'],
    }
  );

  rfb.viewOnly = false;
  rfb.scaleViewport = true;
  rfb.resizeSession = true;
  rfb.showDotCursor = false;
  rfb.clipViewport = false;
  rfb.dragViewport = false;
  rfb.qualityLevel = 6;
  rfb.compressionLevel = 2;

  rfb.addEventListener('connect', function() {
    connected = true;
    document.getElementById('overlay').classList.add('hidden');
    console.log('[Oklavier] VNC connected via VNC');
  });

  rfb.addEventListener('disconnect', function(e) {
    connected = false;
    document.getElementById('overlay').classList.remove('hidden');
    if (e.detail.clean) {
      setStatus('Disconnected.');
    } else {
      setStatus('Connection lost. Reconnecting...');
      setTimeout(function() { location.reload(); }, 3000);
    }
  });

  rfb.addEventListener('desktopname', function(e) {
    document.title = 'Oklavier - ' + e.detail.name;
  });

} catch(e) {
  setStatus('Error: ' + e.message);
  console.error('[Oklavier] RFB init error:', e);
}

// Toolbar functions
window.toggleFullscreen = function() {
  if (!document.fullscreenElement) document.documentElement.requestFullscreen();
  else document.exitFullscreen();
};

window.sendCtrlAltDel = function() {
  if (rfb) rfb.sendCtrlAltDel();
};

window.toggleStats = function() {
  showStatsPanel = !showStatsPanel;
  document.getElementById('stats').style.display = showStatsPanel ? 'block' : 'none';
};

window.goBack = function() {
  if (CONTROL_PLANE && CONTROL_PLANE !== 'CONTROL_PLANE_PLACEHOLDER') {
    window.location.href = CONTROL_PLANE.replace(/\/api.*/, '') + '/workspaces';
  } else {
    window.history.back();
  }
};

// Stats
setInterval(function() {
  if (!showStatsPanel || !rfb) return;
  var el = document.getElementById('stats');
  el.innerHTML =
    '<b style="color:#7096ff">VNC</b><br>' +
    'Connected: ' + (connected ? '<span style="color:#10b981">Yes</span>' : '<span style="color:#ef4444">No</span>') + '<br>' +
    'Protocol: VNC (RFB)<br>' +
    'Transport: WebSocket<br>' +
    'Session: ' + SESSION_ID.substring(0, 8) + '...<br>' +
    'Agent: ' + 'AGENT_NAME_PLACEHOLDER';
}, 2000);
</script>
</body>
</html>`
	html = strings.Replace(html, "SESSION_ID_PLACEHOLDER", sessionID, -1)
	html = strings.Replace(html, "AGENT_NAME_PLACEHOLDER", agentName, -1)
	html = strings.Replace(html, "CONTROL_PLANE_PLACEHOLDER", controlPlaneURL, -1)
	return html
}
