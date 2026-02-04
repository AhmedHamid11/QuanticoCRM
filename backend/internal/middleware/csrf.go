package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/utils"
)

type CSRFConfig struct {
	IsProduction bool
}

// NewCSRFMiddleware returns CSRF middleware configured for FastCRM
func NewCSRFMiddleware(config CSRFConfig) fiber.Handler {
	return csrf.New(csrf.Config{
		// Token delivery: Cookie + Header (double-submit pattern)
		KeyLookup:      "header:X-CSRF-Token",
		CookieName:     "csrf_token",
		CookiePath:     "/",
		CookieSecure:   config.IsProduction,
		CookieHTTPOnly: false, // JS must read cookie to set header
		CookieSameSite: "Strict",

		// Token lifecycle - 1 hour, rotates with each request
		Expiration:   1 * time.Hour,
		KeyGenerator: utils.UUIDv4,

		// Error handling - structured JSON response
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF validation failed",
				"code":  "CSRF_INVALID",
			})
		},

		// Skip CSRF for safe methods and API tokens
		Next: func(c *fiber.Ctx) bool {
			method := c.Method()

			// Safe methods don't need CSRF
			if method == "GET" || method == "HEAD" || method == "OPTIONS" || method == "TRACE" {
				return true
			}

			// API tokens (fcr_ prefix) are exempt - they're not browser-based
			auth := c.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer fcr_") {
				return true
			}

			return false
		},
	})
}
