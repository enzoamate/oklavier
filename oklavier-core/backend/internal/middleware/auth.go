package middleware

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"oklavier-api/internal/auth"
)

// AuthRequired validates requests by checking the Bearer JWT access token.
//
// SECURITY: Basic Auth was removed from this path. Browsers cache Basic
// credentials and replay them automatically, opening CSRF surface on every
// authenticated route. Basic Auth also bypassed banned/locked_until checks.
// Use AutomationAuthRequired (separate group) if Basic Auth is needed for
// API automation; that path enforces the ban/lock checks.
func AuthRequired(db *sqlx.DB, blacklist *auth.TokenBlacklist) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		// Bearer JWT only.
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.ValidateAccessToken(tokenStr)
			if err == nil {
				// Check blacklist
				if blacklist != nil && claims.ID != "" && blacklist.IsBlacklisted(claims.ID) {
					return c.Status(401).JSON(fiber.Map{"error": "Token revoked"})
				}
				c.Locals("user_id", claims.UserID)
				c.Locals("user_email", claims.Email)
				c.Locals("user_role", claims.Role)
				return c.Next()
			}
		}

		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
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
