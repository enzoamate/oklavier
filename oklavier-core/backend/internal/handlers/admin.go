package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"oklavier-api/internal/cache"
	"oklavier-api/internal/db"
	"oklavier-api/internal/middleware"
	"oklavier-api/internal/models"
)

type AdminHandler struct {
	DB    *db.DB
	Cache *cache.Cache
}

func maskPassword(s string) string {
	if s == "" {
		return ""
	}
	return "••••••"
}

// jsonOrEmpty returns the raw JSON bytes if the string is valid JSON, otherwise returns "{}".
func jsonOrEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	if json.Valid([]byte(s)) {
		return s
	}
	return "{}"
}

// Workspaces CRUD
func (h *AdminHandler) GetWorkspaces(c *fiber.Ctx) error {
	var req struct {
		Page    int    `json:"page"`
		PerPage int    `json:"per_page"`
		Search  string `json:"search"`
	}
	c.BodyParser(&req)

	// Backward compat: no pagination params = return all
	if req.PerPage == 0 {
		req.PerPage = 1000
	}
	if req.Page == 0 {
		req.Page = 1
	}

	workspaces, total, err := h.DB.GetWorkspacesPaginated(req.Page, req.PerPage, req.Search)
	if err != nil {
		log.Printf("GetWorkspaces error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "DB error"})
	}
	// Enrich with group names
	result := make([]fiber.Map, len(workspaces))
	for i, w := range workspaces {
		groups, _ := h.DB.GetWorkspaceGroups(w.ID)
		groupNames := make([]string, len(groups))
		for j, g := range groups {
			groupNames[j] = g.Name
		}
		result[i] = fiber.Map{
			"id": w.ID, "name": w.Name, "friendly_name": w.FriendlyName,
			"description": w.Description, "image_src": w.ImageSrc,
			"docker_image": w.DockerImage, "cores": w.Cores, "memory": w.Memory,
			"category": w.Category, "enabled": w.Enabled, "groups": groupNames,
			"docker_registry": w.DockerRegistry, "docker_user": w.DockerUser, "docker_password": maskPassword(w.DockerPassword),
			"session_time_limit": w.SessionTimeLimit, "gpu_count": w.GPUCount, "uncompressed_size_mb": w.UncompressedSizeMB,
			"shm_size":          w.SHMSize,
			"restrict_to_agent": w.RestrictToAgent, "restrict_to_region": w.RestrictToRegion,
			"run_config": w.RunConfig, "exec_config": w.ExecConfig, "volume_mappings": w.VolumeMappings,
			"categories": w.Categories, "notes": w.Notes,
			"persistent": w.Persistent, "persistent_size": w.PersistentSize,
			"workspace_type":  w.WorkspaceType,
			"server_hostname": w.ServerHostname, "server_port": w.ServerPort, "server_protocol": w.ServerProtocol,
			"server_username": w.ServerUsername, "server_password": maskPassword(w.ServerPassword), "server_domain": w.ServerDomain,
			"server_ignore_cert": w.ServerIgnoreCert, "server_security": w.ServerSecurity, "server_auth_mode": w.ServerAuthMode,
			"server_allow_remember": w.ServerAllowRemember, "server_default_settings": w.ServerDefaultSettings,
			"record_sessions": w.RecordSessions,
		}
	}
	return c.JSON(fiber.Map{"workspaces": result, "total": total, "page": req.Page, "per_page": req.PerPage})
}

func (h *AdminHandler) CreateWorkspace(c *fiber.Ctx) error {
	var body struct {
		Name                  string   `json:"name"`
		FriendlyName          string   `json:"friendly_name" validate:"required,min=1,max=100"`
		Description           string   `json:"description"`
		ImageSrc              string   `json:"image_src"`
		DockerImage           string   `json:"docker_image"`
		Cores                 float64  `json:"cores" validate:"gte=0"`
		Memory                int64    `json:"memory" validate:"gte=0"`
		Category              string   `json:"category"`
		DockerRegistry        string   `json:"docker_registry"`
		DockerUser            string   `json:"docker_user"`
		DockerPassword        string   `json:"docker_password"`
		SessionTimeLimit      int      `json:"session_time_limit" validate:"gte=0"`
		GPUCount              int      `json:"gpu_count" validate:"gte=0"`
		UncompressedSizeMB    int      `json:"uncompressed_size_mb" validate:"gte=0"`
		SHMSize               string   `json:"shm_size"`
		RestrictToAgent       string   `json:"restrict_to_agent"`
		RestrictToRegion      string   `json:"restrict_to_region"`
		RunConfig             string   `json:"run_config"`
		ExecConfig            string   `json:"exec_config"`
		VolumeMappings        string   `json:"volume_mappings"`
		Categories            []string `json:"categories"`
		Notes                 string   `json:"notes"`
		Persistent            bool     `json:"persistent"`
		PersistentSize        string   `json:"persistent_size"`
		WorkspaceType         string   `json:"workspace_type"`
		ServerHostname        string   `json:"server_hostname"`
		ServerPort            int      `json:"server_port" validate:"gte=0,lte=65535"`
		ServerProtocol        string   `json:"server_protocol"`
		ServerUsername        string   `json:"server_username"`
		ServerPassword        string   `json:"server_password"`
		ServerDomain          string   `json:"server_domain"`
		ServerIgnoreCert      bool     `json:"server_ignore_cert"`
		ServerSecurity        string   `json:"server_security"`
		ServerAuthMode        string   `json:"server_auth_mode"`
		ServerAllowRemember   bool     `json:"server_allow_remember"`
		ServerDefaultSettings string   `json:"server_default_settings"`
		RecordSessions        bool     `json:"record_sessions"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if err := middleware.Validate.Struct(body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	desc := body.Description
	var notes *string
	if body.Notes != "" {
		notes = &body.Notes
	}
	w := &models.Workspace{
		Name: body.Name, FriendlyName: body.FriendlyName, Description: &desc,
		ImageSrc: body.ImageSrc, DockerImage: body.DockerImage,
		Cores: body.Cores, Memory: body.Memory, Category: body.Category,
		DockerRegistry: body.DockerRegistry, DockerUser: body.DockerUser, DockerPassword: body.DockerPassword,
		SessionTimeLimit: body.SessionTimeLimit, GPUCount: body.GPUCount, UncompressedSizeMB: body.UncompressedSizeMB,
		SHMSize:         body.SHMSize,
		RestrictToAgent: body.RestrictToAgent, RestrictToRegion: body.RestrictToRegion,
		RunConfig: jsonOrEmpty(body.RunConfig), ExecConfig: jsonOrEmpty(body.ExecConfig), VolumeMappings: jsonOrEmpty(body.VolumeMappings),
		Categories: pq.StringArray(body.Categories), Notes: notes,
		Persistent: body.Persistent, PersistentSize: body.PersistentSize,
		WorkspaceType:  body.WorkspaceType,
		ServerHostname: body.ServerHostname, ServerPort: body.ServerPort, ServerProtocol: body.ServerProtocol,
		ServerUsername: body.ServerUsername, ServerPassword: body.ServerPassword, ServerDomain: body.ServerDomain,
		ServerIgnoreCert: body.ServerIgnoreCert, ServerSecurity: body.ServerSecurity, ServerAuthMode: body.ServerAuthMode,
		ServerAllowRemember: body.ServerAllowRemember, ServerDefaultSettings: jsonOrEmpty(body.ServerDefaultSettings),
		RecordSessions: body.RecordSessions,
	}
	err := h.DB.CreateWorkspace(w)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create"})
	}
	if h.Cache != nil {
		h.Cache.Invalidate("workspaces")
	}
	h.DB.LogAudit(c.Locals("user_id").(string), "", "create", "workspace", "", body.FriendlyName, c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) UpdateWorkspace(c *fiber.Ctx) error {
	var body struct {
		ID                    string   `json:"id" validate:"required"`
		Name                  string   `json:"name"`
		FriendlyName          string   `json:"friendly_name" validate:"required,min=1,max=100"`
		Description           string   `json:"description"`
		ImageSrc              string   `json:"image_src"`
		DockerImage           string   `json:"docker_image"`
		Cores                 float64  `json:"cores" validate:"gte=0"`
		Memory                int64    `json:"memory" validate:"gte=0"`
		Category              string   `json:"category"`
		DockerRegistry        string   `json:"docker_registry"`
		DockerUser            string   `json:"docker_user"`
		DockerPassword        string   `json:"docker_password"`
		SessionTimeLimit      int      `json:"session_time_limit" validate:"gte=0"`
		GPUCount              int      `json:"gpu_count" validate:"gte=0"`
		UncompressedSizeMB    int      `json:"uncompressed_size_mb" validate:"gte=0"`
		SHMSize               string   `json:"shm_size"`
		RestrictToAgent       string   `json:"restrict_to_agent"`
		RestrictToRegion      string   `json:"restrict_to_region"`
		RunConfig             string   `json:"run_config"`
		ExecConfig            string   `json:"exec_config"`
		VolumeMappings        string   `json:"volume_mappings"`
		Categories            []string `json:"categories"`
		Notes                 string   `json:"notes"`
		Persistent            bool     `json:"persistent"`
		PersistentSize        string   `json:"persistent_size"`
		WorkspaceType         string   `json:"workspace_type"`
		ServerHostname        string   `json:"server_hostname"`
		ServerPort            int      `json:"server_port" validate:"gte=0,lte=65535"`
		ServerProtocol        string   `json:"server_protocol"`
		ServerUsername        string   `json:"server_username"`
		ServerPassword        string   `json:"server_password"`
		ServerDomain          string   `json:"server_domain"`
		ServerIgnoreCert      bool     `json:"server_ignore_cert"`
		ServerSecurity        string   `json:"server_security"`
		ServerAuthMode        string   `json:"server_auth_mode"`
		ServerAllowRemember   bool     `json:"server_allow_remember"`
		ServerDefaultSettings string   `json:"server_default_settings"`
		RecordSessions        bool     `json:"record_sessions"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if err := middleware.Validate.Struct(body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	desc := body.Description
	var notes *string
	if body.Notes != "" {
		notes = &body.Notes
	}
	// Preserve existing passwords if masked value sent (admin didn't change them)
	dockerPw := body.DockerPassword
	serverPw := body.ServerPassword
	if dockerPw == "••••••" || serverPw == "••••••" {
		existing, _ := h.DB.GetWorkspace(body.ID)
		if existing != nil {
			if dockerPw == "••••••" {
				dockerPw = existing.DockerPassword
			}
			if serverPw == "••••••" {
				serverPw = existing.ServerPassword
			}
		}
	}
	w := &models.Workspace{
		ID: body.ID, Name: body.Name, FriendlyName: body.FriendlyName, Description: &desc,
		ImageSrc: body.ImageSrc, DockerImage: body.DockerImage,
		Cores: body.Cores, Memory: body.Memory, Category: body.Category,
		DockerRegistry: body.DockerRegistry, DockerUser: body.DockerUser, DockerPassword: dockerPw,
		SessionTimeLimit: body.SessionTimeLimit, GPUCount: body.GPUCount, UncompressedSizeMB: body.UncompressedSizeMB,
		SHMSize:         body.SHMSize,
		RestrictToAgent: body.RestrictToAgent, RestrictToRegion: body.RestrictToRegion,
		RunConfig: jsonOrEmpty(body.RunConfig), ExecConfig: jsonOrEmpty(body.ExecConfig), VolumeMappings: jsonOrEmpty(body.VolumeMappings),
		Categories: pq.StringArray(body.Categories), Notes: notes,
		Persistent: body.Persistent, PersistentSize: body.PersistentSize,
		WorkspaceType:  body.WorkspaceType,
		ServerHostname: body.ServerHostname, ServerPort: body.ServerPort, ServerProtocol: body.ServerProtocol,
		ServerUsername: body.ServerUsername, ServerPassword: serverPw, ServerDomain: body.ServerDomain,
		ServerIgnoreCert: body.ServerIgnoreCert, ServerSecurity: body.ServerSecurity, ServerAuthMode: body.ServerAuthMode,
		ServerAllowRemember: body.ServerAllowRemember, ServerDefaultSettings: jsonOrEmpty(body.ServerDefaultSettings),
		RecordSessions: body.RecordSessions,
	}
	err := h.DB.UpdateWorkspace(w)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update"})
	}
	if h.Cache != nil {
		h.Cache.Invalidate("workspaces")
	}
	h.DB.LogAudit(c.Locals("user_id").(string), "", "update", "workspace", body.ID, "", c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) DeleteWorkspace(c *fiber.Ctx) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.BodyParser(&req); err != nil {
		log.Printf("DeleteWorkspace: parse error: %v body: %s", err, string(c.Body()))
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	log.Printf("DeleteWorkspace: id=%s", req.ID)
	if req.ID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id required"})
	}
	if err := h.DB.RemoveWorkspace(req.ID); err != nil {
		log.Printf("DeleteWorkspace: RemoveWorkspace error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workspace"})
	}
	if h.Cache != nil {
		h.Cache.Invalidate("workspaces")
	}
	h.DB.LogAudit(c.Locals("user_id").(string), "", "delete", "workspace", req.ID, "", c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) ToggleWorkspace(c *fiber.Ctx) error {
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	h.DB.ToggleWorkspace(req.ID, req.Enabled)
	if h.Cache != nil {
		h.Cache.Invalidate("workspaces")
	}
	h.DB.LogAudit(c.Locals("user_id").(string), "", "toggle", "workspace", req.ID, fmt.Sprintf("enabled=%v", req.Enabled), c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

// Sessions
func (h *AdminHandler) GetActiveSessions(c *fiber.Ctx) error {
	var req struct {
		Page    int    `json:"page"`
		PerPage int    `json:"per_page"`
		Search  string `json:"search"`
		Cursor  string `json:"cursor"`
	}
	c.BodyParser(&req)

	// Cursor-based pagination takes priority when cursor is provided
	if req.Cursor != "" {
		if req.PerPage == 0 {
			req.PerPage = 25
		}
		sessions, nextCursor, err := h.DB.GetActiveSessionsCursor(req.Cursor, req.PerPage, req.Search)
		if err != nil {
			log.Printf("GetActiveSessions cursor error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "DB error"})
		}
		resp := fiber.Map{"sessions": sessions, "per_page": req.PerPage}
		if nextCursor != "" {
			resp["next_cursor"] = nextCursor
		}
		return c.JSON(resp)
	}

	// Backward compat: offset-based pagination
	if req.PerPage == 0 {
		req.PerPage = 1000
	}
	if req.Page == 0 {
		req.Page = 1
	}

	sessions, total, err := h.DB.GetActiveSessionsPaginated(req.Page, req.PerPage, req.Search)
	if err != nil {
		log.Printf("GetActiveSessions error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "DB error"})
	}
	return c.JSON(fiber.Map{"sessions": sessions, "total": total, "page": req.Page, "per_page": req.PerPage})
}

// Legacy compat
func (h *AdminHandler) GetServers(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"servers": []interface{}{}})
}

func (h *AdminHandler) GetImages(c *fiber.Ctx) error {
	return h.GetWorkspaces(c)
}

func (h *AdminHandler) GetSessions(c *fiber.Ctx) error {
	return h.GetActiveSessions(c)
}
