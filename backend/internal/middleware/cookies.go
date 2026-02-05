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

// SetRefreshTokenCookie sets the refresh token in an HttpOnly, Secure cookie
// SECURITY: This cookie is:
// - HttpOnly: Cannot be accessed by JavaScript (XSS protection)
// - Secure: Only sent over HTTPS in production
// - SameSite=Strict: Prevents CSRF attacks
func SetRefreshTokenCookie(c *fiber.Ctx, token string, isProduction bool) {
	expiresAt := time.Now().Add(RefreshTokenMaxAge)
	c.Cookie(&fiber.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    token,
		Expires:  expiresAt,
		HTTPOnly: true,                   // CRITICAL: Not accessible to JavaScript
		Secure:   isProduction,           // Only HTTPS in production
		SameSite: "Strict",               // Strict CSRF protection
		Path:     "/api/v1/auth",         // Only sent to auth endpoints
	})
}

// GetRefreshTokenFromCookie extracts the refresh token from the HttpOnly cookie
func GetRefreshTokenFromCookie(c *fiber.Ctx) string {
	return c.Cookies(RefreshTokenCookieName)
}

// ClearRefreshTokenCookie removes the refresh token cookie by setting it to expire immediately
func ClearRefreshTokenCookie(c *fiber.Ctx, isProduction bool) {
	c.Cookie(&fiber.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // Expire immediately
		HTTPOnly: true,
		Secure:   isProduction,
		SameSite: "Strict",
		Path:     "/api/v1/auth",
	})
}
