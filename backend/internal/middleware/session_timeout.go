package middleware

import (
	"context"
	"log"

	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SessionTimeoutConfig holds configuration for session timeout middleware
type SessionTimeoutConfig struct {
	AuthService        *service.AuthService
	SkipActivityUpdate []string // Paths to skip activity updates
}

// NewSessionTimeoutMiddleware creates session timeout middleware that tracks activity
// and returns a fiber.Handler
func NewSessionTimeoutMiddleware(config SessionTimeoutConfig) fiber.Handler {
	skipPaths := make(map[string]bool)
	for _, path := range config.SkipActivityUpdate {
		skipPaths[path] = true
	}

	return func(c *fiber.Ctx) error {
		// Get session ID from context (set by auth middleware)
		sessionID, ok := c.Locals("sessionID").(string)
		if !ok || sessionID == "" {
			// No session ID in context, skip activity update
			return c.Next()
		}

		// Check if this path should skip activity update
		if skipPaths[c.Path()] {
			return c.Next()
		}

		// Update activity timestamp (non-blocking)
		if config.AuthService != nil {
			go func(ctx context.Context, sid string) {
				if err := config.AuthService.UpdateSessionActivity(ctx, sid); err != nil {
					log.Printf("[SessionTimeout] Failed to update activity: %v", err)
				}
			}(c.Context(), sessionID)
		}

		return c.Next()
	}
}
