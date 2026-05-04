package heartbeat

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"oklavier-agent/internal/guacamole"
	"oklavier-agent/internal/provisioner"
)

type Heartbeat struct {
	controlPlane string
	token        string
	agentName    string
	region       string
	namespace    string
	publicURL    string
	provisioner  *provisioner.Provisioner
	guacManager  *guacamole.Manager
}

const AgentVersion = "1.0.2"

type HeartbeatPayload struct {
	AgentName     string `json:"agent_name"`
	Region        string `json:"region"`
	Namespace     string `json:"namespace"`
	Status        string `json:"status"`
	Version       string `json:"version"`
	NodeCount     int    `json:"node_count"`
	CPUTotal      int    `json:"cpu_total"`
	MemoryTotalGB int    `json:"memory_total_gb"`
	CPUUsed       int    `json:"cpu_used"`
	MemoryUsedGB  int    `json:"memory_used_gb"`
	Sessions      int    `json:"active_sessions"`
	PublicURL     string `json:"public_url"`
	Timestamp     string `json:"timestamp"`
}

func New(controlPlane, token, agentName, region, namespace, publicURL string, prov *provisioner.Provisioner, guacMgr *guacamole.Manager) *Heartbeat {
	return &Heartbeat{
		controlPlane: controlPlane,
		token:        token,
		agentName:    agentName,
		publicURL:    publicURL,
		region:       region,
		namespace:    namespace,
		provisioner:  prov,
		guacManager:  guacMgr,
	}
}

func (h *Heartbeat) Start() {
	log.Printf("Heartbeat started (interval: 30s)")

	// Send first heartbeat immediately
	h.send()

	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		h.send()
	}
}

func (h *Heartbeat) send() {
	stats := h.provisioner.GetClusterStats()
	podSessions := h.provisioner.CountActiveSessions()
	guacSessions := 0
	if h.guacManager != nil {
		guacSessions = h.guacManager.SessionCount()
	}
	sessions := podSessions + guacSessions

	payload := HeartbeatPayload{
		AgentName:     h.agentName,
		Region:        h.region,
		Namespace:     h.namespace,
		Status:        "connected",
		Version:       AgentVersion,
		NodeCount:     stats.NodeCount,
		CPUTotal:      stats.CPUTotal,
		MemoryTotalGB: stats.MemoryTotalGB,
		CPUUsed:       stats.CPUUsed,
		MemoryUsedGB:  stats.MemoryUsedGB,
		Sessions:      sessions,
		PublicURL:     h.publicURL,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", h.controlPlane+"/api/agent/heartbeat", bytes.NewReader(body))
	if err != nil {
		log.Printf("Heartbeat error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", h.token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Heartbeat failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Heartbeat rejected: %d", resp.StatusCode)
	}
}
