package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/db"
	"oklavier-api/internal/models"
)

type AuthHandler struct {
	DB *db.DB
}

func (h *AuthHandler) LoginSettings(c *fiber.Ctx) error {
	return c.JSON(models.LoginSettings{
		LoginLogo:          "/img/logo.svg",
		LoginCaption:       "Oklavier — Espaces de travail virtuels",
		HeaderLogo:         "/img/headerlogo.svg",
		HTMLTitle:          "Oklavier",
		FaviconLogo:        "/img/favicon.png",
		UsernameInputLabel: "Email",
		NoticeTitle:        "Notice",
		SAML:               models.SAMLConfig{Configs: []models.SAMLProvider{}},
		OIDC:               models.OIDCConfig{Configs: []models.OIDCProvider{}},
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error_message": "Invalid request"})
	}

	user, err := h.DB.GetUserByUsername(req.Username)
	if err != nil {
		return c.Status(403).JSON(fiber.Map{"error_message": "Invalid username or password"})
	}

	if user.Locked {
		return c.Status(403).JSON(fiber.Map{"error_message": "Account is locked"})
	}

if !auth.CheckPasswordSHA256(req.Password, user.Salt, user.PasswordHash) {
		// Increment failed attempts
		user.FailedPWAttempts++
		locked := user.FailedPWAttempts >= 5
		h.DB.UpdateUserLock(user.UserID.String(), locked, user.FailedPWAttempts)
		return c.Status(403).JSON(fiber.Map{"error_message": "Invalid username or password"})
	}

	// Reset failed attempts
	h.DB.UpdateUserLock(user.UserID.String(), false, 0)

	authorizations := []int{100}
	isAdmin := false
	var role string
	if err := h.DB.Get(&role, `SELECT COALESCE(role,'user') FROM "user" WHERE id = $1`, user.UserID.String()); err == nil && role == "admin" {
		authorizations = append(authorizations, 200)
		isAdmin = true
	}

	sessionTokenID := uuid.New().String()
	token, err := auth.GenerateToken(sessionTokenID, authorizations, 24)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error_message": "Failed to generate token"})
	}

	// Set cookies
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Lax",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "username",
		Value:    user.Username,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: false,
		Secure:   false,
		SameSite: "Lax",
	})

	return c.JSON(models.LoginResponse{
		Token:           token,
		UserID:          user.UserID.String(),
		Username:        user.Username,
		IsAdmin:         isAdmin,
		AuthorizedViews: authorizations,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "username",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: false,
	})
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AuthHandler) GetUser(c *fiber.Ctx) error {
	username := c.Cookies("username")
	if username == "" {
		return c.Status(401).JSON(fiber.Map{"error_message": "Not authenticated"})
	}

	user, err := h.DB.GetUserByUsername(username)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error_message": "User not found"})
	}

	return c.JSON(fiber.Map{"user": user})
}
