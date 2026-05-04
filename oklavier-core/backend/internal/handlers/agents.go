package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/db"
)

type AgentHandler struct {
	DB *db.DB
}

// Heartbeat received from an agent
func (h *AgentHandler) Heartbeat(c *fiber.Ctx) error {
	token := c.Get("X-Agent-Token")
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Missing token"})
	}

	var payload struct {
		AgentName     string `json:"agent_name"`
		Region        string `json:"region"`
		Namespace     string `json:"namespace"`
		Status        string `json:"status"`
		NodeCount     int    `json:"node_count"`
		CPUTotal      int    `json:"cpu_total"`
		MemoryTotalGB int    `json:"memory_total_gb"`
		CPUUsed       int    `json:"cpu_used"`
		MemoryUsedGB  int    `json:"memory_used_gb"`
		Sessions      int    `json:"active_sessions"`
		PublicURL     string `json:"public_url"`
		Version       string `json:"version"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Find agent by token and update
	result, err := h.DB.Exec(`
		UPDATE agent SET
			status = 'connected', total_nodes = $2, total_cpu = $3, total_memory = $4,
			active_sessions = $5, last_heartbeat = NOW(),
			name = COALESCE(NULLIF($6, ''), name),
			region = COALESCE(NULLIF($7, ''), region),
			namespace = COALESCE(NULLIF($8, ''), namespace),
			public_url = COALESCE(NULLIF($9, ''), public_url),
			version = COALESCE(NULLIF($10, ''), version)
		WHERE token = $1`,
		token, payload.NodeCount, fmt.Sprintf("%d", payload.CPUTotal), fmt.Sprintf("%d GB", payload.MemoryTotalGB),
		payload.Sessions,
		payload.AgentName, payload.Region, payload.Namespace, payload.PublicURL, payload.Version)

	if err != nil {
		log.Printf("Heartbeat DB error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "DB error"})
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// Admin: list agents (paginated)
func (h *AgentHandler) ListAgents(c *fiber.Ctx) error {
	var req struct {
		Page    int    `json:"page"`
		PerPage int    `json:"per_page"`
		Search  string `json:"search"`
	}
	c.BodyParser(&req)

	if req.PerPage == 0 {
		req.PerPage = 1000
	}
	if req.Page == 0 {
		req.Page = 1
	}
	offset := (req.Page - 1) * req.PerPage

	// Count total
	var total int
	if req.Search != "" {
		h.DB.Get(&total, `SELECT COUNT(*) FROM agent WHERE name ILIKE $1 OR region ILIKE $1`, "%"+req.Search+"%")
	} else {
		h.DB.Get(&total, `SELECT COUNT(*) FROM agent`)
	}

	// Query with pagination
	var query string
	var args []interface{}
	baseSelect := `SELECT id, name, token, region, namespace, COALESCE(endpoint,'') as endpoint, COALESCE(public_url,'') as public_url, status,
		COALESCE(total_nodes,0) as total_nodes, COALESCE(total_cpu,'0') as total_cpu, COALESCE(total_memory,'0') as total_memory,
		COALESCE(active_sessions,0) as active_sessions, COALESCE(version,'') as version, last_heartbeat, created_at FROM agent`

	if req.Search != "" {
		query = fmt.Sprintf(`%s WHERE name ILIKE $1 OR region ILIKE $1 ORDER BY created_at LIMIT %d OFFSET %d`, baseSelect, req.PerPage, offset)
		args = append(args, "%"+req.Search+"%")
	} else {
		query = fmt.Sprintf(`%s ORDER BY created_at LIMIT %d OFFSET %d`, baseSelect, req.PerPage, offset)
	}

	rows, err := h.DB.Queryx(query, args...)
	if err != nil {
		log.Printf("GetAgents error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "DB error"})
	}
	defer rows.Close()

	var agents []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		rows.MapScan(row)
		// Convert []byte to string for JSON
		for k, v := range row {
			if b, ok := v.([]byte); ok {
				row[k] = string(b)
			}
		}
		agents = append(agents, row)
	}
	if agents == nil {
		agents = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"agents": agents, "total": total, "page": req.Page, "per_page": req.PerPage})
}

// Admin: create agent (generates token)
func (h *AgentHandler) CreateAgent(c *fiber.Ctx) error {
	var req struct {
		Name      string `json:"name"`
		Region    string `json:"region"`
		Namespace string `json:"namespace"`
		PublicURL string `json:"public_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var agentID, token string
	err := h.DB.QueryRow(`INSERT INTO agent (name, region, namespace, public_url) VALUES ($1, $2, $3, $4)
		RETURNING id, token`, req.Name, req.Region, req.Namespace, req.PublicURL).Scan(&agentID, &token)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create agent"})
	}

	return c.JSON(fiber.Map{"id": agentID, "token": token, "name": req.Name})
}

// Admin: delete agent
func (h *AgentHandler) DeleteAgent(c *fiber.Ctx) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if _, err := h.DB.Exec("DELETE FROM agent WHERE id = $1", req.ID); err != nil {
		log.Printf("DeleteAgent error: %v", err)
	}
	return c.JSON(fiber.Map{"ok": true})
}

// Admin: generate deploy manifest for an agent
func (h *AgentHandler) GetDeployManifest(c *fiber.Ctx) error {
	var req struct {
		ID        string `json:"id"`
		Namespace string `json:"namespace"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var name, token, region string
	err := h.DB.QueryRow("SELECT name, token, region FROM agent WHERE id = $1", req.ID).Scan(&name, &token, &region)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	ns := req.Namespace
	if ns == "" {
		ns = "oklavier"
	}

	controlPlane := c.Get("X-Forwarded-Host", c.Hostname())
	manifest := fmt.Sprintf(`---
apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: oklavier-agent
  namespace: %s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: oklavier-agent
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "nodes"]
    verbs: ["get", "list", "watch", "create", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oklavier-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: oklavier-agent
subjects:
  - kind: ServiceAccount
    name: oklavier-agent
    namespace: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oklavier-agent
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: oklavier-agent
  template:
    metadata:
      labels:
        app: oklavier-agent
    spec:
      serviceAccountName: oklavier-agent
      containers:
        - name: agent
          image: ghcr.io/enzoamate/oklavier-agent:latest
          env:
            - name: OKLAVIER_CONTROL_PLANE
              value: "https://%s"
            - name: OKLAVIER_AGENT_TOKEN
              value: "%s"
            - name: OKLAVIER_AGENT_NAME
              value: "%s"
            - name: OKLAVIER_REGION
              value: "%s"
            - name: OKLAVIER_NAMESPACE
              value: "%s"
          ports:
            - containerPort: 4444
---
apiVersion: v1
kind: Service
metadata:
  name: oklavier-agent
  namespace: %s
spec:
  selector:
    app: oklavier-agent
  ports:
    - port: 4444
      targetPort: 4444
`, ns, ns, ns, ns, controlPlane, token, name, region, ns, ns)

	return c.JSON(fiber.Map{"manifest": manifest, "token": token})
}

// ProxyToAgent forwards a request to the appropriate agent with retry and exponential backoff.
func (h *AgentHandler) ProxyToAgent(agentID string, path string, body []byte) ([]byte, error) {
	const maxAttempts = 3

	var endpoint, token string
	err := h.DB.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", agentID).Scan(&endpoint, &token)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	if endpoint == "" {
		return nil, fmt.Errorf("agent has no endpoint")
	}

	url := fmt.Sprintf("%s%s", strings.TrimRight(endpoint, "/"), path)

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s
			log.Printf("ProxyToAgent: retry %d/%d after %v for %s", attempt+1, maxAttempts, delay, path)
			time.Sleep(delay)
		}

		req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", token)

		client := &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("ProxyToAgent: attempt %d failed (network): %v", attempt+1, err)
			continue // retry on network error
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("agent returned %d", resp.StatusCode)
			log.Printf("ProxyToAgent: attempt %d failed (status %d): %s", attempt+1, resp.StatusCode, string(respBody))
			continue // retry on 5xx
		}

		if resp.StatusCode >= 400 {
			log.Printf("ProxyToAgent: agent returned %d: %s", resp.StatusCode, string(respBody))
			return respBody, fmt.Errorf("agent error %d", resp.StatusCode)
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("ProxyToAgent failed after %d attempts: %w", maxAttempts, lastErr)
}
