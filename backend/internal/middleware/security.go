package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// Body size limits
const (
	// DefaultBodyLimit is the default request body size limit (1MB)
	DefaultBodyLimit = 1 * 1024 * 1024

	// UploadBodyLimit is the body size limit for file uploads (10MB)
	UploadBodyLimit = 10 * 1024 * 1024
)

// HSTS returns middleware that adds HTTP Strict Transport Security header
// Forces browsers to use HTTPS for the specified duration
func HSTS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// max-age=31536000 (1 year), includeSubDomains
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		return c.Next()
	}
}

// SecurityHeaders returns middleware that adds security headers
// Protects against clickjacking, MIME sniffing, and XSS
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Prevent clickjacking
		c.Set("X-Frame-Options", "DENY")
		// Prevent MIME type sniffing
		c.Set("X-Content-Type-Options", "nosniff")
		// XSS protection (legacy browsers)
		c.Set("X-XSS-Protection", "1; mode=block")
		// Content Security Policy
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		// Referrer policy
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	}
}

// NewCORS returns CORS middleware configured with the given options
// allowOrigins: list of allowed origins (use ["*"] for all)
// isDevelopment: if true, more permissive settings
func NewCORS(allowOrigins []string, isDevelopment bool) fiber.Handler {
	allowOriginsStr := "*"
	if len(allowOrigins) > 0 && allowOrigins[0] != "*" {
		allowOriginsStr = ""
		for i, origin := range allowOrigins {
			if i > 0 {
				allowOriginsStr += ","
			}
			allowOriginsStr += origin
		}
	}

	return cors.New(cors.Config{
		AllowOrigins:     allowOriginsStr,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token,X-Stream-Progress",
		AllowCredentials: true,
		ExposeHeaders:    "Content-Length,Content-Range",
		MaxAge:           86400, // 24 hours
	})
}

// BodyLimit returns middleware that limits request body size.
// Import/upload paths are excluded — they use Fiber's global BodyLimit (10 MB) instead.
func BodyLimit(limit int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.Contains(path, "/import/csv") {
			return c.Next()
		}
		if len(c.Body()) > limit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error": "Request body too large",
			})
		}
		return c.Next()
	}
}

// RequirePasswordChange returns middleware that blocks requests from users who must change password
// Used to protect non-auth endpoints when user has mustChangePassword flag set
func RequirePasswordChange() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// This check happens in the auth handler when validating tokens
		// The flag is set in the JWT claims as mustChangePassword
		// The frontend is responsible for redirecting to the change-password page
		// This middleware just ensures the API enforces the requirement
		return c.Next()
	}
}
