package util

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
)

// IsProduction returns true if running in production environment
func IsProduction() bool {
	env := os.Getenv("ENVIRONMENT")
	return env == "production" || env == "prod"
}

// GenerateErrorID creates a unique error ID for correlation
func GenerateErrorID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SafeErrorResponse returns an appropriate error response based on environment
// SECURITY: In production, internal errors are sanitized to prevent information disclosure
func SafeErrorResponse(c *fiber.Ctx, statusCode int, err error, publicMessage string) error {
	if IsProduction() {
		errorID := GenerateErrorID()
		// Log the full error with correlation ID for debugging
		log.Printf("[ERROR %s] %v", errorID, err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   publicMessage,
			"errorId": errorID,
		})
	}

	// In development, return full error details
	return c.Status(statusCode).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// SafeInternalError returns a generic internal error message in production
func SafeInternalError(c *fiber.Ctx, err error) error {
	return SafeErrorResponse(c, fiber.StatusInternalServerError, err, "An internal error occurred")
}

// SafeBadRequest returns the error in dev, generic message in production
func SafeBadRequest(c *fiber.Ctx, err error) error {
	return SafeErrorResponse(c, fiber.StatusBadRequest, err, "Invalid request")
}
