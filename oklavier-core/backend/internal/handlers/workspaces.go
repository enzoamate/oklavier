package handlers

import (
	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/db"
)

type WorkspaceHandler struct {
	DB *db.DB
}

func (h *WorkspaceHandler) GetUserImages(c *fiber.Ctx) error {
	images, err := h.DB.GetImages()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to fetch images"})
	}
	return c.JSON(fiber.Map{"images": images})
}

func (h *WorkspaceHandler) GetUserSessions(c *fiber.Ctx) error {
	username := c.Cookies("username")
	if username == "" {
		return c.Status(401).JSON(fiber.Map{"error_message": "Not authenticated"})
	}

	user, err := h.DB.GetUserByUsername(username)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error_message": "User not found"})
	}

	sessions, err := h.DB.GetSessionsByUser(user.UserID.String())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to fetch sessions"})
	}

	// Enrich sessions with image info
	type SessionWithImage struct {
		Session interface{} `json:"session"`
		Image   interface{} `json:"image"`
	}

	var enriched []interface{}
	for _, s := range sessions {
		img, _ := h.DB.GetImageByID(s.ImageID.String())
		enriched = append(enriched, fiber.Map{
			"session_id":         s.SessionID,
			"operational_status": s.OperationalStatus,
			"container_id":       s.ContainerID,
			"container_ip":       s.ContainerIP,
			"start_date":         s.StartDate,
			"expiration_date":    s.ExpirationDate,
			"keepalive_date":     s.KeepaliveDate,
			"image":              img,
		})
	}

	if enriched == nil {
		enriched = []interface{}{}
	}

	return c.JSON(fiber.Map{"sessions": enriched})
}
