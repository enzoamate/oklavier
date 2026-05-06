package handlers

import (
	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/db"
	"oklavier-api/internal/models"
)

// AuthHandler exposes only LoginSettings now. The legacy Login/Logout/GetUser
// handlers were removed: they used SHA-256-only password verification, issued
// tokens via auth.GenerateToken (whose ValidateToken was vulnerable to
// alg-confusion), and stored the session in a non-httpOnly cookie. Use
// AuthHandlers (auth_handlers.go) for the modern bcrypt + httpOnly-cookie path.
type AuthHandler struct {
	DB *db.DB
}

// LoginSettings returns public branding/i18n hints for the login screen.
// Public endpoint (no auth required).
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
