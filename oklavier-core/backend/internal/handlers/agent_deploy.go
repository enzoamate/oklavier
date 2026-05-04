package handlers

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"

	"github.com/gofiber/fiber/v2"
)

// sanitizeName allows only alphanumeric, dash, underscore
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (h *AgentHandler) DeployAgent(c *fiber.Ctx) error {
	var req struct {
		AgentID      string `json:"agent_id"`
		Token        string `json:"token"`
		Name         string `json:"name"`
		Region       string `json:"region"`
		Namespace    string `json:"namespace"`
		Mode         string `json:"mode"`
		Kubeconfig   string `json:"kubeconfig"`
		ControlPlane string `json:"control_plane"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	ns := req.Namespace
	if ns == "" {
		ns = "oklavier"
	}

	// Validate inputs to prevent injection
	if !safeNameRe.MatchString(req.Name) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid agent name (alphanumeric, dash, underscore only)"})
	}
	if !safeNameRe.MatchString(ns) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid namespace (alphanumeric, dash, underscore only)"})
	}
	if !safeNameRe.MatchString(req.Region) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid region (alphanumeric, dash, underscore only)"})
	}

	// Build helm install command
	args := []string{
		"upgrade", "--install", fmt.Sprintf("oklavier-agent-%s", req.Name),
		"/chart/oklavier-agent",
		"--namespace", ns,
		"--create-namespace",
		"--set", fmt.Sprintf("agent.name=%s", req.Name),
		"--set", fmt.Sprintf("agent.region=%s", req.Region),
		"--set", fmt.Sprintf("agent.token=%s", req.Token),
		"--set", fmt.Sprintf("controlPlane.url=%s", req.ControlPlane),
		"--set", fmt.Sprintf("workspaceNamespace=%s", ns),
		"--wait",
		"--timeout", "120s",
	}

	if req.Mode == "remote" && req.Kubeconfig != "" {
		// Write kubeconfig to temp file
		tmpFile := fmt.Sprintf("/tmp/kubeconfig-%s", req.AgentID)
		if err := writeFile(tmpFile, req.Kubeconfig); err != nil {
			return c.JSON(fiber.Map{"error": "Failed to write kubeconfig"})
		}
		args = append(args, "--kubeconfig", tmpFile)
	}

	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Helm deploy failed: %s\n%s", err, string(output))
		return c.JSON(fiber.Map{"error": fmt.Sprintf("Helm deploy failed: %s", string(output))})
	}

	// Update agent endpoint in DB
	endpoint := fmt.Sprintf("http://oklavier-agent.%s.svc.cluster.local:4444", ns)
	h.DB.Exec("UPDATE agent SET endpoint = $1 WHERE id = $2", endpoint, req.AgentID)

	log.Printf("Agent '%s' deployed via Helm in namespace '%s'", req.Name, ns)
	return c.JSON(fiber.Map{"ok": true})
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
