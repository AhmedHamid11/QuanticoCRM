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

		// Skip CSRF for safe methods, API tokens, JWT auth, and pre-auth endpoints
		Next: func(c *fiber.Ctx) bool {
			method := c.Method()
			path := c.Path()

			// Safe methods don't need CSRF
			if method == "GET" || method == "HEAD" || method == "OPTIONS" || method == "TRACE" {
				return true
			}

			// Bearer tokens are exempt from CSRF:
			// - API tokens (fcr_ prefix) are not browser-based
			// - JWT tokens must be explicitly set by JavaScript (can't be forged by CSRF)
			// CSRF protection is only needed for cookie-based auth where browsers auto-send credentials
			auth := c.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				return true
			}

			// Pre-authentication endpoints are exempt (no session to protect yet)
			// Platform admin endpoints (impersonate) are also exempt - they require
			// strong authentication (platform admin JWT) which provides sufficient protection
			authExemptPaths := []string{
				"/api/v1/auth/login",
				"/api/v1/auth/register",
				"/api/v1/auth/forgot-password",
				"/api/v1/auth/reset-password",
				"/api/v1/auth/refresh",
				"/api/v1/auth/impersonate",
				"/api/v1/auth/stop-impersonate",
			}
			for _, exempt := range authExemptPaths {
				if path == exempt {
					return true
				}
			}

			return false
		},
	})
}
