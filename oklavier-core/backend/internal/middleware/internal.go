package middleware

import (
	"crypto/subtle"
	"os"

	"github.com/gofiber/fiber/v2"
)

// InternalOnly validates that the request comes from the Next.js proxy
// by checking the X-Oklavier-Internal header against the shared secret.
// If no secret is configured, this middleware is skipped (development mode).
func InternalOnly() fiber.Handler {
	secret := os.Getenv("OKLAVIER_INTERNAL_SECRET")
	return func(c *fiber.Ctx) error {
		if secret == "" {
			return c.Next() // No secret configured, skip (dev mode)
		}

		header := c.Get("X-Oklavier-Internal")
		if subtle.ConstantTimeCompare([]byte(header), []byte(secret)) == 1 {
			c.Locals("internal_proxy", true)
		}

		return c.Next()
	}
}
