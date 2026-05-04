package middleware

import (
	"oklavier-api/internal/db"

	"github.com/gofiber/fiber/v2"
)

// AuditFailedRequests logs 401/403 API responses to the audit log.
func AuditFailedRequests(database *db.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		if status == 401 || status == 403 {
			userID, _ := c.Locals("user_id").(string)
			path := c.Path()
			method := c.Method()
			ip := c.IP()
			action := "unauthorized"
			if status == 403 {
				action = "forbidden"
			}
			database.LogAudit(userID, "", action, "api", "", method+" "+path, ip)
		}
		return err
	}
}
