package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/db"
)

// JSONB is a custom type for scanning PostgreSQL JSONB columns into Go maps.
type JSONB map[string]interface{}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONB.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

type AuthMethodHandler struct {
	DB *db.DB
}

type AuthMethod struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Type      string    `db:"type" json:"type"`
	Enabled   bool      `db:"enabled" json:"enabled"`
	Config    JSONB     `db:"config" json:"config"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (h *AuthMethodHandler) List(c *fiber.Ctx) error {
	var methods []AuthMethod
	err := h.DB.Select(&methods, `SELECT id, name, type, enabled, config, created_at FROM auth_method ORDER BY created_at`)
	if err != nil {
		log.Printf("AuthMethod list error: %v", err)
		return c.JSON(fiber.Map{"methods": []interface{}{}})
	}
	return c.JSON(fiber.Map{"methods": methods})
}

func (h *AuthMethodHandler) Create(c *fiber.Ctx) error {
	var req struct {
		Type    string                 `json:"type"`
		Config  map[string]interface{} `json:"config"`
		Name    string                 `json:"name"`
		Enabled bool                   `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	name := req.Name
	if name == "" {
		if dn, ok := req.Config["display_name"].(string); ok && dn != "" {
			name = dn
		} else {
			name = req.Type
		}
	}

	configJSON, _ := json.Marshal(req.Config)

	_, err := h.DB.Exec(
		`INSERT INTO auth_method (type, name, config) VALUES ($1, $2, $3)`,
		req.Type, name, string(configJSON),
	)
	if err != nil {
		log.Printf("AuthMethod create error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create auth method"})
	}

	h.DB.LogAudit(c.Locals("user_id").(string), "", "create", "auth_method", "", name, c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthMethodHandler) Update(c *fiber.Ctx) error {
	var req struct {
		ID     string                 `json:"id"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	name := ""
	if dn, ok := req.Config["display_name"].(string); ok {
		name = dn
	}
	configJSON, _ := json.Marshal(req.Config)

	_, err := h.DB.Exec(`UPDATE auth_method SET config = $1, name = $2 WHERE id = $3`, string(configJSON), name, req.ID)
	if err != nil {
		log.Printf("AuthMethod update error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update"})
	}

	h.DB.LogAudit(c.Locals("user_id").(string), "", "update", "auth_method", req.ID, name, c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthMethodHandler) Delete(c *fiber.Ctx) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.ID == "credentials" {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot delete credentials"})
	}

	h.DB.Exec(`DELETE FROM auth_method WHERE id = $1`, req.ID)
	h.DB.LogAudit(c.Locals("user_id").(string), "", "delete", "auth_method", req.ID, "", c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthMethodHandler) Toggle(c *fiber.Ctx) error {
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	h.DB.Exec(`UPDATE auth_method SET enabled = $1 WHERE id = $2`, req.Enabled, req.ID)
	h.DB.LogAudit(c.Locals("user_id").(string), "", "toggle", "auth_method", req.ID, "", c.IP())
	return c.JSON(fiber.Map{"ok": true})
}
