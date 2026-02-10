package handler

import (
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SalesforceHandler handles HTTP requests for Salesforce integration
type SalesforceHandler struct {
	oauthService *service.SalesforceOAuthService
	repo         *repo.SalesforceRepo
}

// NewSalesforceHandler creates a new SalesforceHandler
func NewSalesforceHandler(oauthService *service.SalesforceOAuthService, repo *repo.SalesforceRepo) *SalesforceHandler {
	return &SalesforceHandler{
		oauthService: oauthService,
		repo:         repo,
	}
}

// SaveConfig saves Salesforce Connected App credentials
// POST /salesforce/config
func (h *SalesforceHandler) SaveConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

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

// RegisterRoutes registers all Salesforce routes
func (h *SalesforceHandler) RegisterRoutes(router fiber.Router) {
	sf := router.Group("/salesforce")

	// Configuration and OAuth endpoints (admin-protected)
	sf.Post("/config", h.SaveConfig)
	sf.Get("/config", h.GetConfig)
	sf.Post("/oauth/authorize", h.InitiateOAuth)
	sf.Get("/status", h.GetStatus)
	sf.Post("/disconnect", h.Disconnect)
	sf.Put("/toggle", h.ToggleSync)
}

// RegisterCallbackRoute registers the OAuth callback route (public, no auth required)
func (h *SalesforceHandler) RegisterCallbackRoute(router fiber.Router) {
	router.Get("/salesforce/oauth/callback", h.OAuthCallback)
}
