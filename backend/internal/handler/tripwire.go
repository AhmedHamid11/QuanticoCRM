package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// TripwireHandler handles HTTP requests for tripwires
type TripwireHandler struct {
	tripwireRepo *repo.TripwireRepo
}

// NewTripwireHandler creates a new TripwireHandler
func NewTripwireHandler(tripwireRepo *repo.TripwireRepo) *TripwireHandler {
	return &TripwireHandler{
		tripwireRepo: tripwireRepo,
	}
}

// RegisterRoutes registers tripwire routes
func (h *TripwireHandler) RegisterRoutes(app fiber.Router) {
	tw := app.Group("/tripwires")
	tw.Get("/", h.List)
	tw.Post("/", h.Create)
	tw.Get("/:id", h.Get)
	tw.Put("/:id", h.Update)
	tw.Delete("/:id", h.Delete)
	tw.Post("/:id/toggle", h.Toggle)
	tw.Post("/:id/test", h.Test)
	tw.Get("/:id/logs", h.ListLogs)

	// Webhook settings
	app.Get("/settings/webhooks", h.GetWebhookSettings)
	app.Put("/settings/webhooks", h.SaveWebhookSettings)
}

// List returns all tripwires for the organization
func (h *TripwireHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	params := entity.TripwireListParams{
		Search:     c.Query("search"),
		EntityType: c.Query("entityType"),
		SortBy:     c.Query("sortBy", "created_at"),
		SortDir:    c.Query("sortDir", "desc"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
	}

	// Handle enabled filter
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true" || enabledStr == "1"
		params.Enabled = &enabled
	}

	result, err := h.tripwireRepo.ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// Create creates a new tripwire
func (h *TripwireHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.TripwireCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name is required"})
	}
	if input.EntityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Entity type is required"})
	}
	if input.EndpointURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Endpoint URL is required"})
	}
	if len(input.Conditions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "At least one condition is required"})
	}

	// SECURITY: Validate webhook URL to prevent SSRF attacks
	if err := util.IsAllowedWebhookURL(input.EndpointURL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook URL: " + err.Error(),
		})
	}

	tripwire, err := h.tripwireRepo.Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(tripwire)
}

// Get returns a single tripwire by ID
func (h *TripwireHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	tripwire, err := h.tripwireRepo.GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tripwire == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tripwire not found"})
	}

	return c.JSON(tripwire)
}

// Update updates an existing tripwire
func (h *TripwireHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.TripwireUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// SECURITY: Validate webhook URL if being updated
	if input.EndpointURL != nil && *input.EndpointURL != "" {
		if err := util.IsAllowedWebhookURL(*input.EndpointURL); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid webhook URL: " + err.Error(),
			})
		}
	}

	tripwire, err := h.tripwireRepo.Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tripwire == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tripwire not found"})
	}

	return c.JSON(tripwire)
}

// Delete deletes a tripwire
func (h *TripwireHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	err := h.tripwireRepo.Delete(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Toggle toggles the enabled state of a tripwire
func (h *TripwireHandler) Toggle(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	tripwire, err := h.tripwireRepo.Toggle(c.Context(), orgID, id, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tripwire == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tripwire not found"})
	}

	return c.JSON(tripwire)
}

// Test sends a test payload to the tripwire's endpoint URL
func (h *TripwireHandler) Test(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Get the tripwire
	tripwire, err := h.tripwireRepo.GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tripwire == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tripwire not found"})
	}

	// Get webhook settings for auth
	settings, err := h.tripwireRepo.GetWebhookSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Create test payload
	testPayload := map[string]interface{}{
		"tripwireId":   tripwire.ID,
		"tripwireName": tripwire.Name,
		"event":        "test",
		"entityType":   tripwire.EntityType,
		"recordId":     "test-record-id",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"test":         true,
		"data": map[string]interface{}{
			"message": "This is a test payload from Quantico to verify webhook connectivity.",
		},
	}

	payloadBytes, err := json.Marshal(testPayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create test payload"})
	}

	// Create HTTP request
	timeout := 10000 // Default 10 seconds
	if settings != nil && settings.TimeoutMs > 0 {
		timeout = settings.TimeoutMs
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	req, err := http.NewRequest("POST", tripwire.EndpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid endpoint URL: " + err.Error(),
		})
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FastCRM-Webhook/1.0")
	req.Header.Set("X-FastCRM-Event", "test")

	// Add authentication headers based on settings
	if settings != nil {
		switch settings.AuthType {
		case entity.WebhookAuthAPIKey:
			if settings.APIKey != nil {
				req.Header.Set("X-API-Key", *settings.APIKey)
			}
		case entity.WebhookAuthBearer:
			if settings.BearerToken != nil {
				req.Header.Set("Authorization", "Bearer "+*settings.BearerToken)
			}
		case entity.WebhookAuthCustomHeader:
			if settings.CustomHeaderName != nil && settings.CustomHeaderValue != nil {
				req.Header.Set(*settings.CustomHeaderName, *settings.CustomHeaderValue)
			}
		}
	}

	// Send the request
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return c.JSON(fiber.Map{
			"success":    false,
			"error":      err.Error(),
			"durationMs": duration,
		})
	}
	defer resp.Body.Close()

	// Read response body (limit to 1KB for safety)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	responseBody := string(bodyBytes)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return c.JSON(fiber.Map{
		"success":      success,
		"statusCode":   resp.StatusCode,
		"statusText":   resp.Status,
		"durationMs":   duration,
		"responseBody": responseBody,
	})
}

// ListLogs returns execution logs for a tripwire
func (h *TripwireHandler) ListLogs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tripwireID := c.Params("id")

	params := entity.TripwireLogListParams{
		TripwireID: tripwireID,
		Status:     c.Query("status"),
		EventType:  c.Query("eventType"),
		SortBy:     c.Query("sortBy", "executed_at"),
		SortDir:    c.Query("sortDir", "desc"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
	}

	result, err := h.tripwireRepo.ListLogs(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// GetWebhookSettings returns the webhook settings for the organization
func (h *TripwireHandler) GetWebhookSettings(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	settings, err := h.tripwireRepo.GetWebhookSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(settings)
}

// SaveWebhookSettings saves the webhook settings for the organization
func (h *TripwireHandler) SaveWebhookSettings(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.OrgWebhookSettingsInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	settings, err := h.tripwireRepo.SaveWebhookSettings(c.Context(), orgID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(settings)
}
