package handler

import (
	"fmt"
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SalesforceHandler handles HTTP requests for Salesforce integration
type SalesforceHandler struct {
	oauthService     *service.SalesforceOAuthService
	deliveryService  *service.SFDeliveryService
	rateLimitService *service.RateLimitService
	repo             *repo.SalesforceRepo
}

// NewSalesforceHandler creates a new SalesforceHandler
func NewSalesforceHandler(
	oauthService *service.SalesforceOAuthService,
	deliveryService *service.SFDeliveryService,
	rateLimitService *service.RateLimitService,
	repo *repo.SalesforceRepo,
) *SalesforceHandler {
	return &SalesforceHandler{
		oauthService:     oauthService,
		deliveryService:  deliveryService,
		rateLimitService: rateLimitService,
		repo:             repo,
	}
}

// SaveConfig saves Salesforce Connected App credentials
// POST /salesforce/config
func (h *SalesforceHandler) SaveConfig(c *fiber.Ctx) error {
	log.Println("[DEBUG] SaveConfig handler called")

	orgIDRaw := c.Locals("orgID")
	if orgIDRaw == nil {
		log.Println("[ERROR] orgID not set in context locals")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Organization context not set",
		})
	}

	orgID, ok := orgIDRaw.(string)
	if !ok {
		log.Printf("[ERROR] orgID has wrong type: %T", orgIDRaw)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization context type",
		})
	}
	log.Printf("[DEBUG] SaveConfig processing for org: %s", orgID)

	var config entity.SFSyncConfig
	if err := c.BodyParser(&config); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if config.ClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "clientId is required",
		})
	}
	if config.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "clientSecret is required",
		})
	}
	if config.RedirectURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "redirectUrl is required",
		})
	}

	// Save config (encrypts client secret)
	if err := h.oauthService.SaveConfig(c.Context(), orgID, config); err != nil {
		log.Printf("Failed to save Salesforce config for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	return c.JSON(fiber.Map{
		"status": "configured",
	})
}

// GetConfig retrieves Salesforce connection configuration (without sensitive data)
// GET /salesforce/config
func (h *SalesforceHandler) GetConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	conn, err := h.oauthService.GetConfig(c.Context(), orgID)
	if err != nil {
		log.Printf("Failed to get Salesforce config for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get configuration",
		})
	}

	// No connection exists
	if conn == nil {
		return c.JSON(fiber.Map{
			"configured": false,
		})
	}

	// Get connection status
	status, err := h.oauthService.GetConnectionStatus(c.Context(), orgID)
	if err != nil {
		log.Printf("Failed to get connection status for org %s: %v", orgID, err)
		status = "unknown"
	}

	// Return config WITHOUT sensitive data (no encrypted tokens, no client secret)
	response := fiber.Map{
		"configured":  true,
		"clientId":    conn.ClientID,
		"redirectUrl": conn.RedirectURL,
		"instanceUrl": conn.InstanceURL,
		"isEnabled":   conn.IsEnabled,
		"status":      status,
	}

	if conn.ConnectedAt != nil {
		response["connectedAt"] = conn.ConnectedAt
	}

	return c.JSON(response)
}

// InitiateOAuth generates the Salesforce authorization URL
// POST /salesforce/oauth/authorize
func (h *SalesforceHandler) InitiateOAuth(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	authURL, err := h.oauthService.GetAuthorizationURL(c.Context(), orgID)
	if err != nil {
		log.Printf("Failed to generate OAuth URL for org %s: %v", orgID, err)

		// Check specific error types for better messages
		if err == service.ErrNoConnection {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Salesforce not configured. Please save your Connected App credentials first.",
			})
		}
		if err == service.ErrMissingEncryptionKey {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Encryption key not configured. Please contact support.",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate authorization URL",
		})
	}

	return c.JSON(fiber.Map{
		"authUrl": authURL,
	})
}

// OAuthCallback handles the OAuth callback from Salesforce
// GET /salesforce/oauth/callback
func (h *SalesforceHandler) OAuthCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Redirect("/admin/integrations/salesforce?status=error&message=Missing authorization code")
	}
	if state == "" {
		return c.Redirect("/admin/integrations/salesforce?status=error&message=Missing state parameter")
	}

	// Exchange code for tokens
	orgID, err := h.oauthService.HandleCallback(c.Context(), code, state)
	if err != nil {
		log.Printf("OAuth callback failed: %v", err)

		// Check specific error types
		if err == service.ErrInvalidState {
			return c.Redirect("/admin/integrations/salesforce?status=error&message=Invalid state parameter (CSRF check failed)")
		}

		return c.Redirect("/admin/integrations/salesforce?status=error&message=Failed to connect to Salesforce")
	}

	log.Printf("OAuth callback successful for org: %s", orgID)
	return c.Redirect("/admin/integrations/salesforce?status=connected")
}

// GetStatus returns the current connection status
// GET /salesforce/status
func (h *SalesforceHandler) GetStatus(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	status, err := h.oauthService.GetConnectionStatus(c.Context(), orgID)
	if err != nil {
		log.Printf("Failed to get connection status for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get status",
		})
	}

	return c.JSON(fiber.Map{
		"status": status,
	})
}

// Disconnect clears tokens and disconnects from Salesforce
// POST /salesforce/disconnect
func (h *SalesforceHandler) Disconnect(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	if err := h.oauthService.DisconnectOrg(c.Context(), orgID); err != nil {
		log.Printf("Failed to disconnect org %s: %v", orgID, err)

		if err == service.ErrNoConnection {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Salesforce not configured",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to disconnect",
		})
	}

	return c.JSON(fiber.Map{
		"status": "disconnected",
	})
}

// ToggleSync enables or disables Salesforce sync
// PUT /salesforce/toggle
func (h *SalesforceHandler) ToggleSync(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.repo.SetEnabled(c.Context(), orgID, input.Enabled); err != nil {
		log.Printf("Failed to toggle sync for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update sync status",
		})
	}

	return c.JSON(fiber.Map{
		"isEnabled": input.Enabled,
	})
}

// QueueMergeInstructions queues merge instructions for delivery to Salesforce
// POST /salesforce/queue
func (h *SalesforceHandler) QueueMergeInstructions(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input struct {
		Instructions []service.MergeInstructionInput `json:"instructions"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.Instructions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "instructions array is required",
		})
	}

	// Queue instructions for delivery
	jobID, err := h.deliveryService.QueueMergeInstructions(c.Context(), orgID, input.Instructions)
	if err != nil {
		// Check for QuotaExceededError
		if quotaErr, ok := err.(*entity.QuotaExceededError); ok {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":     quotaErr.Message,
				"usage":     quotaErr.Usage,
				"threshold": quotaErr.Threshold,
				"hint":      "API quota threshold reached. Use manual trigger with force flag to override.",
			})
		}

		log.Printf("Failed to queue merge instructions for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to queue instructions: %v", err),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobId":   jobID,
		"status":  "pending",
		"message": "Merge instructions queued for delivery",
	})
}

// ListJobs returns sync job history for the org
// GET /salesforce/jobs
func (h *SalesforceHandler) ListJobs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	// Parse query params
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	if limit < 1 || limit > 100 {
		limit = 20
	}

	jobs, total, err := h.deliveryService.ListJobs(c.Context(), orgID, limit, offset)
	if err != nil {
		log.Printf("Failed to list jobs for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list jobs",
		})
	}

	return c.JSON(fiber.Map{
		"jobs":  jobs,
		"total": total,
	})
}

// GetJobStatus returns the status of a specific sync job
// GET /salesforce/jobs/:jobId
func (h *SalesforceHandler) GetJobStatus(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	jobID := c.Params("jobId")

	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "jobId is required",
		})
	}

	job, err := h.deliveryService.GetJobStatus(c.Context(), orgID, jobID)
	if err != nil {
		log.Printf("Failed to get job status for job %s: %v", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get job status",
		})
	}

	return c.JSON(job)
}

// RetryJob retries a failed sync job
// POST /salesforce/jobs/:jobId/retry
func (h *SalesforceHandler) RetryJob(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	jobID := c.Params("jobId")

	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "jobId is required",
		})
	}

	if err := h.deliveryService.RetryJob(c.Context(), orgID, jobID); err != nil {
		log.Printf("Failed to retry job %s: %v", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to retry job: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"status": "retrying",
	})
}

// ManualTrigger manually triggers merge instruction delivery with optional force flag
// POST /salesforce/trigger
func (h *SalesforceHandler) ManualTrigger(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input struct {
		Instructions []service.MergeInstructionInput `json:"instructions"`
		Force        bool                            `json:"force"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.Instructions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "instructions array is required",
		})
	}

	jobID, err := h.deliveryService.QueueMergeInstructionsWithOptions(
		c.Context(),
		orgID,
		input.Instructions,
		service.DeliveryOptions{
			Force:       input.Force,
			TriggerType: entity.SyncTriggerManual,
		},
	)

	if err != nil {
		// Check for QuotaExceededError specifically
		if quotaErr, ok := err.(*entity.QuotaExceededError); ok {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":     quotaErr.Message,
				"usage":     quotaErr.Usage,
				"threshold": quotaErr.Threshold,
				"hint":      "Use {\"force\": true} to override rate limiting",
			})
		}

		log.Printf("Failed to manually trigger merge instructions for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to trigger instructions: %v", err),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobId":   jobID,
		"status":  "pending",
		"message": "Merge instructions manually triggered for delivery",
	})
}

// GetQuota returns current API usage quota status
// GET /salesforce/quota
func (h *SalesforceHandler) GetQuota(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	quota, err := h.rateLimitService.GetQuotaStatus(c.Context(), orgID)
	if err != nil {
		log.Printf("Failed to get quota for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get quota status",
		})
	}

	return c.JSON(quota)
}

// RegisterRoutes registers all Salesforce routes
func (h *SalesforceHandler) RegisterRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Salesforce routes")
	sf := router.Group("/salesforce")

	// Configuration and OAuth endpoints (admin-protected from Plan 02)
	sf.Post("/config", h.SaveConfig)
	log.Println("[STARTUP] Registered POST /salesforce/config")
	sf.Get("/config", h.GetConfig)
	log.Println("[STARTUP] Registered GET /salesforce/config")
	sf.Post("/oauth/authorize", h.InitiateOAuth)
	sf.Get("/status", h.GetStatus)
	sf.Post("/disconnect", h.Disconnect)
	sf.Put("/toggle", h.ToggleSync)

	// Delivery endpoints (admin-protected from Plan 04)
	sf.Post("/queue", h.QueueMergeInstructions)
	sf.Get("/jobs", h.ListJobs)
	sf.Get("/jobs/:jobId", h.GetJobStatus)
	sf.Post("/jobs/:jobId/retry", h.RetryJob)
	sf.Post("/trigger", h.ManualTrigger)
	sf.Get("/quota", h.GetQuota)
}

// RegisterCallbackRoute registers the OAuth callback route (public, no auth required)
func (h *SalesforceHandler) RegisterCallbackRoute(router fiber.Router) {
	router.Get("/salesforce/oauth/callback", h.OAuthCallback)
}
