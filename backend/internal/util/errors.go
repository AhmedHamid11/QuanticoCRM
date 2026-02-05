package util

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Error categories for classification
const (
	ErrCategoryDatabase   = "database"
	ErrCategoryValidation = "validation"
	ErrCategoryPermission = "permission"
	ErrCategoryAuth       = "auth"
	ErrCategoryInternal   = "internal"
	ErrCategoryNotFound   = "not_found"
	ErrCategoryExternal   = "external"
)

// APIError is a structured error that can be returned as JSON
type APIError struct {
	StatusCode int    `json:"-"`
	Err        string `json:"error"`
	Message    string `json:"message,omitempty"`
	Category   string `json:"category,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Err
}

// Unwrap allows error unwrapping
func (e *APIError) Unwrap() error {
	return nil
}

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

// NewAPIError creates a new APIError with proper sanitization based on environment
// It logs the error and returns a sanitized response for production
func NewAPIError(c *fiber.Ctx, statusCode int, err error, category string) error {
	requestID := GenerateErrorID()

	// Always log the full error for debugging
	log.Printf("[ERROR %s] [%s] %v", requestID, category, err)

	apiErr := &APIError{
		StatusCode: statusCode,
		Err:        GetCategoryMessage(category),
		Category:   category,
		RequestID:  requestID,
	}

	// In development, include full error details
	if !IsProduction() {
		apiErr.Details = err.Error()
	}

	return apiErr
}

// NewAPIErrorWithMessage creates an APIError with a custom message
// Useful when you want to show a specific message to the user
func NewAPIErrorWithMessage(c *fiber.Ctx, statusCode int, message string, category string) error {
	requestID := GenerateErrorID()

	// Log the error
	log.Printf("[ERROR %s] [%s] %s", requestID, category, message)

	return &APIError{
		StatusCode: statusCode,
		Err:        message, // Use the provided message directly
		Message:    message,
		Category:   category,
		RequestID:  requestID,
	}
}

// ClassifyError categorizes an error based on its content
func ClassifyError(err error) string {
	if err == nil {
		return ErrCategoryInternal
	}

	errStr := strings.ToLower(err.Error())

	// Database errors
	if strings.Contains(errStr, "sql") ||
		strings.Contains(errStr, "database") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "constraint") ||
		strings.Contains(errStr, "duplicate") ||
		strings.Contains(errStr, "turso") {
		return ErrCategoryDatabase
	}

	// Permission errors
	if strings.Contains(errStr, "permission") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "not allowed") ||
		strings.Contains(errStr, "unauthorized") {
		return ErrCategoryPermission
	}

	// Auth errors
	if strings.Contains(errStr, "auth") ||
		strings.Contains(errStr, "token") ||
		strings.Contains(errStr, "jwt") ||
		strings.Contains(errStr, "credential") {
		return ErrCategoryAuth
	}

	// Not found errors
	if strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "no rows") ||
		strings.Contains(errStr, "does not exist") {
		return ErrCategoryNotFound
	}

	// Validation errors
	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "required") ||
		strings.Contains(errStr, "must be") ||
		strings.Contains(errStr, "validation") {
		return ErrCategoryValidation
	}

	// External service errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "external") ||
		strings.Contains(errStr, "api") {
		return ErrCategoryExternal
	}

	return ErrCategoryInternal
}

// GetCategoryMessage returns a user-friendly message for an error category
func GetCategoryMessage(category string) string {
	switch category {
	case ErrCategoryDatabase:
		return "A database error occurred. Please try again later."
	case ErrCategoryValidation:
		return "Invalid request. Please check your input."
	case ErrCategoryPermission:
		return "You don't have permission to perform this action."
	case ErrCategoryAuth:
		return "Authentication error. Please log in again."
	case ErrCategoryNotFound:
		return "The requested resource was not found."
	case ErrCategoryExternal:
		return "An external service error occurred. Please try again later."
	default:
		return "An internal error occurred. Please try again later."
	}
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
