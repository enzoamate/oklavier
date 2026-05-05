package middleware

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// internalOnlyPublicPrefixes are paths that legitimately bypass the Next.js
// proxy and have their own authentication scheme (or are intentionally public).
// They are exempt from the X-Oklavier-Internal check.
var internalOnlyPublicPrefixes = []string{
	"/health",
	"/metrics",
	"/api/health",
	"/api/agent/", // X-Agent-Token authenticated; called by remote agents
	"/proxy/ws/",  // WebSocket proxy; auth handled by agent
}

// InternalOnly validates that the request comes from the Next.js proxy
// by checking the X-Oklavier-Internal header against the shared secret.
//
// SECURITY: previously this middleware only set a Locals flag and always
// fell through to c.Next() — providing zero defense. Now it returns 403
// when the header is missing or mismatched (and a secret is configured).
//
// Public endpoints that legitimately bypass the Next.js proxy (health,
// metrics, OIDC callback, agent-token routes, etc.) must be registered
// BEFORE this middleware in the route tree.
//
// If no secret is configured, this middleware is skipped (development mode).
func InternalOnly() fiber.Handler {
	secret := os.Getenv("OKLAVIER_INTERNAL_SECRET")
	return func(c *fiber.Ctx) error {
		if secret == "" {
			return c.Next() // No secret configured, skip (dev mode)
		}

		// Public/agent paths bypass this gate (they have their own auth).
		path := c.Path()
		for _, p := range internalOnlyPublicPrefixes {
			if strings.HasPrefix(path, p) {
				return c.Next()
			}
		}

		header := c.Get("X-Oklavier-Internal")
		if subtle.ConstantTimeCompare([]byte(header), []byte(secret)) != 1 {
			return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
		}
		c.Locals("internal_proxy", true)
		return c.Next()
	}
}
