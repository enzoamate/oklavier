package middleware

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"oklavier-api/internal/auth"
)

// AuthRequired validates requests by checking:
// 1. Bearer JWT access token (primary)
// 2. Basic Auth (for direct API access / automation)
func AuthRequired(db *sqlx.DB, blacklist *auth.TokenBlacklist) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		// 1. Bearer JWT
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

		// 2. Basic Auth
		if strings.HasPrefix(authHeader, "Basic ") {
			user, err := validateBasicAuth(db, authHeader)
			if err == nil {
				c.Locals("user_id", user.ID)
				c.Locals("user_email", user.Email)
				c.Locals("user_role", user.Role)
				return c.Next()
			}
		}

		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
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

// validateBasicAuth checks email:password against account tables
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
	`, email).Scan(&user.ID, &user.Email, &user.Role, &hashedPassword)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return nil, fiber.ErrUnauthorized
	}

	return &user, nil
}
