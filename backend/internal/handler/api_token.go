package handler

import (
	"errors"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// APITokenHandler handles HTTP requests for API tokens
type APITokenHandler struct {
	apiTokenService *service.APITokenService
}

// NewAPITokenHandler creates a new APITokenHandler
func NewAPITokenHandler(apiTokenService *service.APITokenService) *APITokenHandler {
	return &APITokenHandler{apiTokenService: apiTokenService}
}

// Create generates a new API token
// POST /api-tokens
func (h *APITokenHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.APITokenCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token name is required",
		})
	}

	response, err := h.apiTokenService.Create(c.Context(), orgID, userID, input)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// List returns all tokens for the current organization
// GET /api-tokens
func (h *APITokenHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	tokens, err := h.apiTokenService.List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list tokens",
		})
	}

	return c.JSON(fiber.Map{
		"tokens": tokens,
	})
}

// Revoke deactivates a token (soft delete)
// POST /api-tokens/:id/revoke
func (h *APITokenHandler) Revoke(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tokenID := c.Params("id")

	if tokenID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token ID is required",
		})
	}

	if err := h.apiTokenService.Revoke(c.Context(), tokenID, orgID); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Token revoked successfully",
	})
}

// Delete permanently removes a token
// DELETE /api-tokens/:id
func (h *APITokenHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tokenID := c.Params("id")

	if tokenID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token ID is required",
		})
	}

	if err := h.apiTokenService.Delete(c.Context(), tokenID, orgID); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Token deleted successfully",
	})
}

// handleError converts service errors to appropriate HTTP responses
func (h *APITokenHandler) handleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrTokenNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token not found",
		})
	case errors.Is(err, service.ErrInvalidTokenName):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token name must be between 1 and 100 characters",
		})
	case errors.Is(err, service.ErrInvalidScope):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "An unexpected error occurred",
		})
	}
}
