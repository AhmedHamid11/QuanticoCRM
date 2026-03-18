package handler

import (
	"errors"
	"log"
	"os"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// GmailHandler handles HTTP requests for Gmail OAuth and connection management.
type GmailHandler struct {
	gmailService   *service.GmailOAuthService
	engagementRepo *repo.EngagementRepo
	dbManager      *db.Manager
	authRepo       *repo.AuthRepo
}

// NewGmailHandler creates a new GmailHandler.
func NewGmailHandler(
	gmailService *service.GmailOAuthService,
	engagementRepo *repo.EngagementRepo,
	dbManager *db.Manager,
	authRepo *repo.AuthRepo,
) *GmailHandler {
	return &GmailHandler{
		gmailService:   gmailService,
		engagementRepo: engagementRepo,
		dbManager:      dbManager,
		authRepo:       authRepo,
	}
}

// RegisterRoutes registers authenticated Gmail routes under the provided router group.
// These routes require OrgAdmin role (registered under adminProtected in main.go).
func (h *GmailHandler) RegisterRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Gmail routes")
	g := router.Group("/gmail")

	g.Get("/status", h.GetStatus)
	g.Get("/connect", h.GetConnectURL)
	g.Delete("/disconnect", h.Disconnect)
}

// RegisterPublicRoutes registers the OAuth callback route on the public api group.
// The callback from Google is unauthenticated — security is provided by the CSRF state token.
func (h *GmailHandler) RegisterPublicRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Gmail public routes")
	router.Get("/gmail/callback", h.OAuthCallback)
}

// ========== Authenticated Handlers ==========

// GetStatus returns the Gmail connection status for the calling user.
// GET /gmail/status
func (h *GmailHandler) GetStatus(c *fiber.Ctx) error {
	orgID, userID := getGmailContext(c)

	tenantDB := middleware.GetTenantDBConn(c)
	svc := service.NewGmailOAuthService(h.engagementRepo.WithDB(tenantDB), h.gmailService.GetEncryptionKey())

	status, err := svc.GetConnectionStatus(c.Context(), orgID, userID)
	if err != nil {
		log.Printf("[Gmail] GetStatus error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get Gmail connection status",
		})
	}

	return c.JSON(status)
}

// GetConnectURL returns the Google OAuth authorization URL.
// GET /gmail/connect
func (h *GmailHandler) GetConnectURL(c *fiber.Ctx) error {
	orgID, userID := getGmailContext(c)

	redirectBase := getRedirectBase(c)

	authURL, err := h.gmailService.GetAuthorizationURL(c.Context(), orgID, userID, redirectBase)
	if err != nil {
		log.Printf("[Gmail] GetConnectURL error for org %s user %s: %v", orgID, userID, err)
		if errors.Is(err, service.ErrGmailMissingClientCfg) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Gmail integration is not configured on this server",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate authorization URL",
		})
	}

	return c.JSON(fiber.Map{"authUrl": authURL})
}

// Disconnect removes the Gmail OAuth token for the calling user.
// DELETE /gmail/disconnect
func (h *GmailHandler) Disconnect(c *fiber.Ctx) error {
	orgID, userID := getGmailContext(c)

	tenantDB := middleware.GetTenantDBConn(c)
	svc := service.NewGmailOAuthService(h.engagementRepo.WithDB(tenantDB), h.gmailService.GetEncryptionKey())

	if err := svc.Disconnect(c.Context(), orgID, userID); err != nil {
		log.Printf("[Gmail] Disconnect error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to disconnect Gmail",
		})
	}

	return c.JSON(fiber.Map{"status": "disconnected"})
}

// ========== Public Handler ==========

// OAuthCallback handles the OAuth callback from Google.
// GET /gmail/callback
// The state token embeds orgID + userID, allowing us to resolve the correct tenant DB.
func (h *GmailHandler) OAuthCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	frontendURL := os.Getenv("FRONTEND_URL")

	if code == "" {
		return c.Redirect(frontendURL + "/admin/integrations/gmail?error=missing_code")
	}
	if state == "" {
		return c.Redirect(frontendURL + "/admin/integrations/gmail?error=missing_state")
	}

	redirectBase := getRedirectBase(c)

	// Resolve the tenant DB for the org encoded in the state token.
	// We must do this before calling HandleCallback so the service can persist tokens.
	orgID, tenantDB, err := h.resolveTenantDBFromState(c, state)
	if err != nil {
		log.Printf("[Gmail] OAuthCallback: failed to resolve tenant DB: %v", err)
		return c.Redirect(frontendURL + "/admin/integrations/gmail?error=tenant_resolution_failed")
	}

	svc := service.NewGmailOAuthService(h.engagementRepo.WithDB(tenantDB), h.gmailService.GetEncryptionKey())

	_, _, err = svc.HandleCallback(c.Context(), code, state, redirectBase)
	if err != nil {
		log.Printf("[Gmail] OAuthCallback error for org %s: %v", orgID, err)
		if errors.Is(err, service.ErrGmailInvalidState) {
			return c.Redirect(frontendURL + "/admin/integrations/gmail?error=invalid_state")
		}
		return c.Redirect(frontendURL + "/admin/integrations/gmail?error=connection_failed")
	}

	log.Printf("[Gmail] OAuth callback completed for org %s", orgID)
	return c.Redirect(frontendURL + "/admin/integrations/gmail?connected=true")
}

// ========== Helpers ==========

// getGmailContext extracts orgID and userID from the Fiber context (set by auth middleware).
func getGmailContext(c *fiber.Ctx) (string, string) {
	orgID, _ := c.Locals("orgID").(string)
	userID, _ := c.Locals("userID").(string)
	return orgID, userID
}

// resolveTenantDBFromState decodes the OAuth state token to extract the orgID,
// then looks up the correct tenant DB connection for that org.
// This is necessary for the public callback route, which has no auth middleware.
func (h *GmailHandler) resolveTenantDBFromState(c *fiber.Ctx, state string) (string, db.DBConn, error) {
	// Decode the state to get orgID (we just need orgID here)
	orgID, _, err := h.gmailService.DecodeStateForCallback(state)
	if err != nil {
		return "", nil, err
	}

	// Local mode: all data is in the shared master DB
	if h.dbManager.IsLocalMode() {
		return orgID, h.dbManager.GetMasterDB(), nil
	}

	// Production: resolve the org's dedicated Turso database
	org, err := h.authRepo.GetOrganizationByID(c.Context(), orgID)
	if err != nil {
		return "", nil, err
	}

	if org.DatabaseURL == "" {
		return orgID, h.dbManager.GetMasterDB(), nil
	}

	tenantDB, err := h.dbManager.GetTenantDBConn(c.Context(), orgID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		return "", nil, err
	}

	return orgID, tenantDB, nil
}
