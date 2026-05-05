package middleware

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"oklavier-api/internal/auth"
)

// accessCookieName must match the value in handlers/auth_handlers.go.
const accessCookieName = "oklavier_access"

// AuthRequired validates requests via:
//  1. The httpOnly access cookie (oklavier_access) — primary, browser-driven.
//  2. Authorization: Bearer <jwt> — fallback for automation / API clients.
//
// Basic Auth was removed from this path; use AutomationAuthRequired on a
// dedicated /api/automation/* group if needed.
func AuthRequired(db *sqlx.DB, blacklist *auth.TokenBlacklist) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Cookies(accessCookieName)
		if tokenStr == "" {
			if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		if tokenStr == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
		}
		claims, err := auth.ValidateAccessToken(tokenStr)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
		}
		if blacklist != nil && claims.ID != "" && blacklist.IsBlacklisted(claims.ID) {
			return c.Status(401).JSON(fiber.Map{"error": "Token revoked"})
		}
		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		return c.Next()
	}
}

// AutomationAuthRequired allows Basic Auth for API automation. Mount this
// only on a dedicated /api/automation/* group, not on browser-facing routes.
// It enforces banned / locked_until and is suitable for server-to-server clients.
func AutomationAuthRequired(db *sqlx.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Basic ") {
			return c.Status(401).JSON(fiber.Map{"error": "Basic auth required"})
		}
		user, err := validateBasicAuth(db, authHeader)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
		}
		c.Locals("user_id", user.ID)
		c.Locals("user_email", user.Email)
		c.Locals("user_role", user.Role)
		return c.Next()
	}
}

// AdminRequired checks that the authenticated user has admin role
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("user_role").(string)
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
		}
		return c.Next()
	}
}

// AgentTokenRequired validates agent-to-API communication
func AgentTokenRequired(db *sqlx.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Get("X-Agent-Token")
		// Never accept tokens from query strings (prevents leaking in logs/referrer)
		if token == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Agent token required"})
		}
		var agentID string
		if err := db.Get(&agentID, `SELECT id FROM agent WHERE token = $1`, token); err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid agent token"})
		}
		c.Locals("agent_id", agentID)
		return c.Next()
	}
}

type sessionUser struct {
	ID    string `db:"id"`
	Email string `db:"email"`
	Role  string `db:"role"`
}

// validateBasicAuth checks email:password against account tables.
// SECURITY: enforces banned + locked_until on the basic-auth path; without these,
// banned users retained API access via Authorization: Basic ...
func validateBasicAuth(db *sqlx.DB, authHeader string) (*sessionUser, error) {
	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, fiber.ErrUnauthorized
	}
	email, password := parts[0], parts[1]

	var user sessionUser
	var hashedPassword string
	err = db.QueryRow(`
		SELECT u.id, u.email, COALESCE(u.role, 'user') as role, a.password
		FROM "user" u
		JOIN "account" a ON a."userId" = u.id AND a."providerId" = 'credential'
		WHERE u.email = $1
		  AND COALESCE(u.banned, false) = false
		  AND (u."banExpires" IS NULL OR u."banExpires" < NOW())
	`, email).Scan(&user.ID, &user.Email, &user.Role, &hashedPassword)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return nil, fiber.ErrUnauthorized
	}

	return &user, nil
}
