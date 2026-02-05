package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	// RefreshTokenCookieName is the name of the HttpOnly cookie storing the refresh token
	RefreshTokenCookieName = "refresh_token"

	// RefreshTokenMaxAge is the maximum age of the refresh token cookie (7 days)
	RefreshTokenMaxAge = 7 * 24 * time.Hour
)

// CookieConfig holds cookie configuration
type CookieConfig struct {
	IsProduction bool
	Domain       string // e.g., ".quanticocrm.com" - leading dot for subdomain sharing
}

// SetRefreshTokenCookie sets the refresh token in an HttpOnly, Secure cookie
// SECURITY: This cookie is:
// - HttpOnly: Cannot be accessed by JavaScript (XSS protection)
// - Secure: Always true in production (required for SameSite=None)
// - SameSite=None: Required for cross-origin requests (Vercel frontend → Railway backend)
// - CSRF protection is handled via X-CSRF-Token header for mutating requests
func SetRefreshTokenCookie(c *fiber.Ctx, token string, cfg CookieConfig) {
	expiresAt := time.Now().Add(RefreshTokenMaxAge)

	// SameSite=None requires Secure=true
	// In production, we're always HTTPS; in dev, browsers may accept without Secure
	sameSite := "None"
	secure := true
	if !cfg.IsProduction {
		// In development, use Lax to avoid issues with localhost
		sameSite = "Lax"
		secure = false
	}

	cookie := &fiber.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    token,
		Expires:  expiresAt,
		HTTPOnly: true,     // CRITICAL: Not accessible to JavaScript
		Secure:   secure,   // Required for SameSite=None
		SameSite: sameSite, // None for cross-site, Lax for dev
		Path:     "/api/v1/auth", // Only sent to auth endpoints
	}

	// Set domain if configured (enables cross-subdomain cookies)
	if cfg.Domain != "" {
		cookie.Domain = cfg.Domain
	}

	c.Cookie(cookie)
}

// GetRefreshTokenFromCookie extracts the refresh token from the HttpOnly cookie
func GetRefreshTokenFromCookie(c *fiber.Ctx) string {
	return c.Cookies(RefreshTokenCookieName)
}

// ClearRefreshTokenCookie removes the refresh token cookie by setting it to expire immediately
func ClearRefreshTokenCookie(c *fiber.Ctx, cfg CookieConfig) {
	// Match the same SameSite settings as SetRefreshTokenCookie
	sameSite := "None"
	secure := true
	if !cfg.IsProduction {
		sameSite = "Lax"
		secure = false
	}

	cookie := &fiber.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // Expire immediately
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Path:     "/api/v1/auth",
	}

	// Set domain if configured (must match SetRefreshTokenCookie)
	if cfg.Domain != "" {
		cookie.Domain = cfg.Domain
	}

	c.Cookie(cookie)
}
