package handler

import (
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// ValidationHandler handles HTTP requests for validation rules
type ValidationHandler struct {
	repo              *repo.ValidationRepo
	validationService *service.ValidationService
}

// NewValidationHandler creates a new ValidationHandler
func NewValidationHandler(repo *repo.ValidationRepo, validationService *service.ValidationService) *ValidationHandler {
	return &ValidationHandler{
		repo:              repo,
		validationService: validationService,
	}
}

// getRepo returns the repo with the correct tenant DB from context
func (h *ValidationHandler) getRepo(c *fiber.Ctx) *repo.ValidationRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// List returns all validation rules for an entity type
func (h *ValidationHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	params := entity.ValidationRuleListParams{
		Search:     c.Query("search"),
		EntityType: entityType,
		SortBy:     c.Query("sortBy"),
		SortDir:    c.Query("sortDir"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
	}

	// Handle enabled filter
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true" || enabledStr == "1"
		params.Enabled = &enabled
	}

	result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// Get returns a single validation rule by ID
func (h *ValidationHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	rule, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if rule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Validation rule not found",
		})
	}

	return c.JSON(rule)
}

// Create creates a new validation rule
func (h *ValidationHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityType := c.Params("entity")

	var input entity.ValidationRuleCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Override entity type from URL parameter
	input.EntityType = entityType

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}
	if len(input.Conditions) == 0 && len(input.Actions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "at least one condition or action is required",
		})
	}

	rule, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Invalidate cache for this entity type
	h.validationService.InvalidateCache(orgID, entityType)

	return c.Status(fiber.StatusCreated).JSON(rule)
}

// Update updates an existing validation rule
func (h *ValidationHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.ValidationRuleUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get existing rule to know its entity type for cache invalidation
	existing, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Validation rule not found",
		})
	}

	rule, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if rule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Validation rule not found",
		})
	}

	// Invalidate cache for both old and new entity types
	h.validationService.InvalidateCache(orgID, existing.EntityType)
	if input.EntityType != nil && *input.EntityType != existing.EntityType {
		h.validationService.InvalidateCache(orgID, *input.EntityType)
	}

	return c.JSON(rule)
}

// Delete deletes a validation rule
func (h *ValidationHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Get existing rule to know its entity type for cache invalidation
	existing, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Validation rule not found",
		})
	}

	if err := h.getRepo(c).Delete(c.Context(), orgID, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Invalidate cache
	h.validationService.InvalidateCache(orgID, existing.EntityType)

	return c.SendStatus(fiber.StatusNoContent)
}

// Toggle toggles the enabled state of a validation rule
func (h *ValidationHandler) Toggle(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	rule, err := h.getRepo(c).Toggle(c.Context(), orgID, id, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if rule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Validation rule not found",
		})
	}

	// Invalidate cache
	h.validationService.InvalidateCache(orgID, rule.EntityType)

	return c.JSON(rule)
}

// Test tests a validation rule against sample data
func (h *ValidationHandler) Test(c *fiber.Ctx) error {
	var input entity.TestValidationInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate operation
	if input.Operation != "CREATE" && input.Operation != "UPDATE" && input.Operation != "DELETE" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "operation must be CREATE, UPDATE, or DELETE",
		})
	}

	// Get entity type from URL if not in body
	if input.Rule.EntityType == "" {
		input.Rule.EntityType = c.Params("entity")
	}

	result := h.validationService.TestRule(&input.Rule, input.Operation, input.OldRecord, input.NewRecord)

	return c.JSON(result)
}

// RegisterRoutes registers validation rule routes
func (h *ValidationHandler) RegisterRoutes(app fiber.Router) {
	// Entity-specific validation rules
	rules := app.Group("/admin/entities/:entity/validation-rules")
	rules.Get("/", h.List)
	rules.Post("/", h.Create)
	rules.Get("/:id", h.Get)
	rules.Put("/:id", h.Update)
	rules.Patch("/:id", h.Update)
	rules.Delete("/:id", h.Delete)
	rules.Post("/:id/toggle", h.Toggle)
	rules.Post("/test", h.Test)
}
