package middleware

import (
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// AuditAuthorizationFailures creates middleware that logs 403 responses
// This middleware captures all authorization failures (403 Forbidden responses)
// and records them in the audit log for security monitoring and forensic analysis.
//
// SECURITY: Authorization failures may indicate:
// - Attack attempts (privilege escalation)
// - Misconfigured permissions
// - User confusion about access rights
//
// The middleware:
// - Executes the handler first (non-blocking)
// - Checks the response status code
// - If 403, logs the event asynchronously (fire-and-forget)
// - Does not impact response time or user experience
func AuditAuthorizationFailures(auditLogger *service.AuditLogger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Execute the next handler first
		err := c.Next()

		// Check if response was 403 Forbidden
		if c.Response().StatusCode() == fiber.StatusForbidden {
			// Extract context values (may not exist if auth failed early)
			userID, _ := c.Locals("userID").(string)
			email, _ := c.Locals("email").(string)
			orgID, _ := c.Locals("orgID").(string)

			// Log asynchronously to not block response
			// Fire-and-forget pattern ensures audit logging never impacts user experience
			go auditLogger.LogAuthorizationDenied(
				c.Context(),
				userID,
				email,
				orgID,
				c.Path(),
				c.Method(),
				c.IP(),
				c.Get("User-Agent"),
			)
		}

		return err
	}
}
