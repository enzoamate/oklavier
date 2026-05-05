package handlers

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

// safeNameRe allows only alphanumeric, dash, underscore.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// safeTokenRe allows alphanumeric + `-_=` for typical token formats.
var safeTokenRe = regexp.MustCompile(`^[A-Za-z0-9_=\-]+$`)

// safeHTTPSURLRe validates `https://host[:port][/path]`.
var safeHTTPSURLRe = regexp.MustCompile(`^https?://[A-Za-z0-9._\-]+(:\d{1,5})?(/[A-Za-z0-9._\-/?=&%]*)?$`)

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

	// SECURITY: validate every value that ends up in argv to helm or in a
	// filesystem path. The previous version validated Name/Namespace/Region
	// but NOT AgentID (used as a `/tmp/kubeconfig-<id>` filename — path
	// traversal) and NOT Token / ControlPlane (passed to `--set` — Helm
	// `--set` parses commas/braces/brackets specially, allowing chart-value
	// override injection like `agent.token=x,agent.image=evil/img`).
	if !safeNameRe.MatchString(req.AgentID) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid agent_id"})
	}
	if !safeNameRe.MatchString(req.Name) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid agent name (alphanumeric, dash, underscore only)"})
	}
	if !safeNameRe.MatchString(ns) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid namespace (alphanumeric, dash, underscore only)"})
	}
	if !safeNameRe.MatchString(req.Region) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid region (alphanumeric, dash, underscore only)"})
	}
	if req.Token != "" && !safeTokenRe.MatchString(req.Token) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid token format"})
	}
	if req.ControlPlane != "" && !safeHTTPSURLRe.MatchString(req.ControlPlane) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid control_plane URL"})
	}

	// Build helm install command
	args := []string{
		"upgrade", "--install", fmt.Sprintf("oklavier-agent-%s", req.Name),
		"/chart/oklavier-agent",
		"--namespace", ns,
		"--create-namespace",
		"--set", fmt.Sprintf("agent.name=%s", req.Name),
		"--set", fmt.Sprintf("agent.region=%s", req.Region),
		"--set-string", fmt.Sprintf("agent.token=%s", req.Token),
		"--set-string", fmt.Sprintf("controlPlane.url=%s", req.ControlPlane),
		"--set", fmt.Sprintf("workspaceNamespace=%s", ns),
		"--wait",
		"--timeout", "120s",
	}

	if req.Mode == "remote" && req.Kubeconfig != "" {
		// SECURITY: parse the kubeconfig and reject any user that uses an
		// `exec` auth provider — these run an arbitrary binary inside the
		// core pod (kubectl/helm RCE). Also reject any user.token-file or
		// auth-provider config that points outside the kubeconfig itself.
		if err := validateKubeconfig(req.Kubeconfig); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "kubeconfig rejected: " + err.Error()})
		}

		// SECURITY: use os.CreateTemp so the filename has 6 random chars; never
		// interpolate user input. Defer-remove the file so the credential
		// doesn't sit on disk after the call.
		tmp, err := os.CreateTemp("/tmp", "kubeconfig-*.yaml")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create temp file"})
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, werr := tmp.WriteString(req.Kubeconfig); werr != nil {
			tmp.Close()
			return c.Status(500).JSON(fiber.Map{"error": "Failed to write kubeconfig"})
		}
		tmp.Close()
		_ = os.Chmod(tmpName, 0600)
		args = append(args, "--kubeconfig", tmpName)
	}

	cmd := exec.Command("helm", args...)
	// Defense in depth: disable kubectl exec-plugin auth in the spawned helm
	// process even if a malicious kubeconfig slipped through validation.
	cmd.Env = append(os.Environ(),
		"KUBECTL_DISABLE_EXEC_PLUGIN=1",
		"KUBECACHEDIR=/tmp/.kube-cache",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Helm deploy failed: %s\n%s", err, string(output))
		// SECURITY: previously this returned 200 OK on helm failure (silent fail).
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Helm deploy failed: %s", string(output))})
	}

	// Update agent endpoint in DB only after helm succeeds.
	endpoint := fmt.Sprintf("http://oklavier-agent.%s.svc.cluster.local:4444", ns)
	h.DB.Exec("UPDATE agent SET endpoint = $1 WHERE id = $2", endpoint, req.AgentID)

	log.Printf("Agent '%s' deployed via Helm in namespace '%s'", req.Name, ns)
	return c.JSON(fiber.Map{"ok": true})
}

// validateKubeconfig parses a kubeconfig and rejects exec-plugin authentication,
// which is a well-known kubectl/helm RCE primitive (the cluster-side `users[].user.exec.command`
// is executed by the local helm/kubectl process when it tries to authenticate).
func validateKubeconfig(raw string) error {
	if len(raw) > 256*1024 {
		return fmt.Errorf("kubeconfig too large")
	}
	var doc struct {
		Users []struct {
			Name string                 `yaml:"name"`
			User map[string]interface{} `yaml:"user"`
		} `yaml:"users"`
		Clusters []struct {
			Name    string                 `yaml:"name"`
			Cluster map[string]interface{} `yaml:"cluster"`
		} `yaml:"clusters"`
	}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return fmt.Errorf("invalid yaml")
	}
	for _, u := range doc.Users {
		for k := range u.User {
			lk := strings.ToLower(k)
			if lk == "exec" {
				return fmt.Errorf("exec auth providers are not allowed")
			}
			if lk == "auth-provider" {
				return fmt.Errorf("auth-provider entries are not allowed")
			}
			if lk == "tokenfile" || lk == "token-file" {
				return fmt.Errorf("tokenFile entries are not allowed")
			}
		}
	}
	for _, cl := range doc.Clusters {
		for k := range cl.Cluster {
			if strings.ToLower(k) == "tls-server-name" {
				continue
			}
		}
	}
	return nil
}
