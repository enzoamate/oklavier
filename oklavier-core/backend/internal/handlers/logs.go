package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/db"
)

// LogBuffer stores recent log lines in memory
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	maxSize int
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

var GlobalLogBuffer = &LogBuffer{maxSize: 500}

func (lb *LogBuffer) Add(source, level, message string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries = append(lb.entries, LogEntry{
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Source:    source,
		Level:     level,
		Message:   message,
	})
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[len(lb.entries)-lb.maxSize:]
	}
}

func (lb *LogBuffer) GetAll() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// LogWriter implements io.Writer to capture Go log output
type LogWriter struct {
	source string
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	GlobalLogBuffer.Add(lw.source, "INFO", msg)
	return len(p), nil
}

// InstallLogCapture redirects Go's log output to our buffer
func InstallLogCapture() {
	writer := &LogWriter{source: "oklavier-api"}
	log.SetOutput(io.MultiWriter(log.Writer(), writer))
}

type LogHandler struct {
	DB *db.DB
}

// GetLogs returns API logs + agent logs
func (h *LogHandler) GetLogs(c *fiber.Ctx) error {
	source := c.Query("source", "all") // "all", "api", or agent_id
	limit := c.QueryInt("limit", 100)

	apiLogs := GlobalLogBuffer.GetAll()

	// Filter by source
	var filtered []LogEntry
	if source == "all" || source == "api" {
		filtered = append(filtered, apiLogs...)
	}

	// If requesting agent logs, fetch from agents
	if source == "all" || (source != "api" && source != "all") {
		agentLogs := h.fetchAgentLogs(source)
		filtered = append(filtered, agentLogs...)
	}

	// Sort by timestamp desc and limit
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	// Get available sources (api + connected agents)
	sources := []map[string]string{{"id": "api", "name": "oklavier-api"}}
	rows, err := h.DB.Queryx("SELECT id, name FROM agent WHERE status = 'connected'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name string
			rows.Scan(&id, &name)
			sources = append(sources, map[string]string{"id": id, "name": name})
		}
	}

	return c.JSON(fiber.Map{
		"logs":    filtered,
		"sources": sources,
	})
}

func (h *LogHandler) fetchAgentLogs(agentID string) []LogEntry {
	var agents []struct {
		ID       string `db:"id"`
		Name     string `db:"name"`
		Endpoint string `db:"endpoint"`
		Token    string `db:"token"`
	}

	if agentID == "all" {
		h.DB.Select(&agents, "SELECT id, name, COALESCE(endpoint,'') as endpoint, token FROM agent WHERE status = 'connected'")
	} else {
		h.DB.Select(&agents, "SELECT id, name, COALESCE(endpoint,'') as endpoint, token FROM agent WHERE id = $1", agentID)
	}

	var logs []LogEntry
	client := &http.Client{Timeout: 5 * time.Second}

	for _, a := range agents {
		if a.Endpoint == "" {
			continue
		}
		url := fmt.Sprintf("%s/api/logs", a.Endpoint)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("X-Agent-Token", a.Token)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Parse agent JSON response {"logs": ["line1", "line2"]}
		var agentResp struct {
			Logs []string `json:"logs"`
		}
		if err := json.Unmarshal(body, &agentResp); err == nil {
			for _, line := range agentResp.Logs {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Try to extract timestamp from log line (format: 2026/03/23 20:34:08 ...)
				ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
				msg := line
				if len(line) > 20 && line[4] == '/' && line[7] == '/' {
					ts = strings.Replace(line[:19], "/", "-", -1)
					ts = strings.Replace(ts, " ", "T", 1) + "Z"
					msg = strings.TrimSpace(line[20:])
				}
				logs = append(logs, LogEntry{
					Timestamp: ts,
					Source:    a.Name,
					Level:     "INFO",
					Message:   msg,
				})
			}
		}
	}
	return logs
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
