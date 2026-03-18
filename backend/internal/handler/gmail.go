package handler

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GmailHandler handles HTTP requests for Gmail OAuth, connection management,
// email template CRUD, and sending test emails via the Gmail API.
type GmailHandler struct {
	gmailService   *service.GmailOAuthService
	gmailProvider  *service.GmailProvider
	templateEngine *service.TemplateEngine
	engagementRepo *repo.EngagementRepo
	contactRepo    *repo.ContactRepo
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

// SetGmailProvider injects the GmailProvider for send-test functionality.
func (h *GmailHandler) SetGmailProvider(p *service.GmailProvider) {
	h.gmailProvider = p
}

// SetTemplateEngine injects the TemplateEngine for variable substitution.
func (h *GmailHandler) SetTemplateEngine(e *service.TemplateEngine) {
	h.templateEngine = e
}

// SetContactRepo injects the ContactRepo for resolving contactId in preview/send-test.
func (h *GmailHandler) SetContactRepo(r *repo.ContactRepo) {
	h.contactRepo = r
}

// RegisterRoutes registers authenticated Gmail routes under the provided router group.
// These routes require OrgAdmin role (registered under adminProtected in main.go).
func (h *GmailHandler) RegisterRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Gmail routes")
	g := router.Group("/gmail")

	// OAuth management
	g.Get("/status", h.GetStatus)
	g.Get("/connect", h.GetConnectURL)
	g.Delete("/disconnect", h.Disconnect)
	g.Post("/send-test", h.SendTestEmail)

	// Email template CRUD
	log.Println("[STARTUP] Registering email template routes")
	g.Get("/email-templates", h.ListEmailTemplates)
	g.Post("/email-templates", h.CreateEmailTemplate)
	g.Get("/email-templates/:id", h.GetEmailTemplate)
	g.Put("/email-templates/:id", h.UpdateEmailTemplate)
	g.Delete("/email-templates/:id", h.DeleteEmailTemplate)
	g.Post("/email-templates/:id/preview", h.PreviewEmailTemplate)
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

// SendTestEmail renders an email template (optionally against a contact) and
// sends it via the Gmail API to verify the integration.
// POST /gmail/send-test
//
// Request body:
//
//	{
//	  "templateId": "...",
//	  "toEmail": "...",
//	  "contactId": "..." (optional)
//	}
func (h *GmailHandler) SendTestEmail(c *fiber.Ctx) error {
	orgID, userID := getGmailContext(c)

	var req struct {
		TemplateID string `json:"templateId"`
		ToEmail    string `json:"toEmail"`
		ContactID  string `json:"contactId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if req.TemplateID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "templateId is required",
		})
	}
	if req.ToEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "toEmail is required",
		})
	}
	if h.gmailProvider == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Gmail send is not configured on this server",
		})
	}

	tenantDB := middleware.GetTenantDBConn(c)
	tenantEngagementRepo := h.engagementRepo.WithDB(tenantDB)

	// Fetch the email template
	tmpl, err := tenantEngagementRepo.GetEmailTemplate(c.Context(), orgID, req.TemplateID)
	if err != nil {
		log.Printf("[Gmail] SendTestEmail: failed to fetch template %s for org %s: %v", req.TemplateID, orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch email template",
		})
	}
	if tmpl == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Email template not found",
		})
	}

	subject := tmpl.Subject
	bodyHTML := tmpl.BodyHTML

	// Optionally render template variables against a contact record
	if h.templateEngine != nil && req.ContactID != "" && h.contactRepo != nil {
		tenantContactRepo := h.contactRepo.WithDB(tenantDB)
		contact, contactErr := tenantContactRepo.GetByID(c.Context(), orgID, req.ContactID)
		if contactErr == nil && contact != nil {
			vars := map[string]string{
				"first_name":   contact.FirstName,
				"last_name":    contact.LastName,
				"full_name":    contact.Name(),
				"email":        contact.EmailAddress,
				"phone":        contact.PhoneNumber,
				"account_name": contact.AccountName,
				"city":         contact.AddressCity,
				"state":        contact.AddressState,
				"country":      contact.AddressCountry,
			}
			subject, bodyHTML = h.templateEngine.RenderTemplate(tmpl, vars)
		}
	} else if h.templateEngine != nil {
		// No contactId: render with empty vars (tokens remain unresolved)
		subject, bodyHTML = h.templateEngine.RenderTemplate(tmpl, map[string]string{})
	}

	// Get the Gmail address for the From header
	svc := service.NewGmailOAuthService(tenantEngagementRepo, h.gmailService.GetEncryptionKey())
	status, statusErr := svc.GetConnectionStatus(c.Context(), orgID, userID)
	fromEmail := ""
	if statusErr == nil && status.Connected {
		fromEmail = status.GmailAddress
	}
	if fromEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Gmail not connected — connect your Gmail account before sending test emails",
		})
	}

	// Build a provider scoped to the tenant DB
	tenantProvider := service.NewGmailProvider(svc)

	if err := tenantProvider.Send(c.Context(), orgID, userID, fromEmail, req.ToEmail, subject, bodyHTML); err != nil {
		log.Printf("[Gmail] SendTestEmail: send failed for org %s user %s: %v", orgID, userID, err)

		errMsg := fmt.Sprintf("Failed to send test email: %s", err.Error())
		if errors.Is(err, service.ErrGmailNotConnected) || errors.Is(err, service.ErrGmailNoTokens) {
			errMsg = "Gmail not connected — connect your Gmail account first"
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": errMsg,
		})
	}

	log.Printf("[Gmail] SendTestEmail: sent to %s for org %s user %s", req.ToEmail, orgID, userID)
	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Test email sent to %s", req.ToEmail),
	})
}

// ========== Email Template CRUD Handlers ==========

// ListEmailTemplates returns all email templates for the calling org.
// GET /gmail/email-templates
func (h *GmailHandler) ListEmailTemplates(c *fiber.Ctx) error {
	orgID, _ := getGmailContext(c)
	tenantDB := middleware.GetTenantDBConn(c)

	templates, err := h.engagementRepo.WithDB(tenantDB).ListEmailTemplates(c.Context(), orgID)
	if err != nil {
		log.Printf("[Gmail] ListEmailTemplates error for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list email templates",
		})
	}
	if templates == nil {
		templates = []*entity.EmailTemplate{}
	}
	return c.JSON(templates)
}

// CreateEmailTemplate creates a new email template.
// POST /gmail/email-templates
func (h *GmailHandler) CreateEmailTemplate(c *fiber.Ctx) error {
	orgID, userID := getGmailContext(c)
	tenantDB := middleware.GetTenantDBConn(c)

	var input struct {
		Name                string `json:"name"`
		Subject             string `json:"subject"`
		BodyHTML            string `json:"bodyHtml"`
		BodyText            string `json:"bodyText"`
		HasComplianceFooter int    `json:"hasComplianceFooter"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	now := time.Now().UTC()
	tmpl := &entity.EmailTemplate{
		ID:                  uuid.New().String(),
		OrgID:               orgID,
		Name:                input.Name,
		Subject:             input.Subject,
		BodyHTML:            input.BodyHTML,
		BodyText:            input.BodyText,
		HasComplianceFooter: input.HasComplianceFooter,
		CreatedBy:           userID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := h.engagementRepo.WithDB(tenantDB).CreateEmailTemplate(c.Context(), tmpl); err != nil {
		log.Printf("[Gmail] CreateEmailTemplate error for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create email template"})
	}
	return c.Status(fiber.StatusCreated).JSON(tmpl)
}

// GetEmailTemplate retrieves a single email template by ID.
// GET /gmail/email-templates/:id
func (h *GmailHandler) GetEmailTemplate(c *fiber.Ctx) error {
	orgID, _ := getGmailContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	tmpl, err := h.engagementRepo.WithDB(tenantDB).GetEmailTemplate(c.Context(), orgID, id)
	if err != nil {
		log.Printf("[Gmail] GetEmailTemplate error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch email template"})
	}
	if tmpl == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email template not found"})
	}
	return c.JSON(tmpl)
}

// UpdateEmailTemplate updates an existing email template.
// PUT /gmail/email-templates/:id
func (h *GmailHandler) UpdateEmailTemplate(c *fiber.Ctx) error {
	orgID, _ := getGmailContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	existing, err := h.engagementRepo.WithDB(tenantDB).GetEmailTemplate(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch email template"})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email template not found"})
	}

	var input struct {
		Name                string `json:"name"`
		Subject             string `json:"subject"`
		BodyHTML            string `json:"bodyHtml"`
		BodyText            string `json:"bodyText"`
		HasComplianceFooter int    `json:"hasComplianceFooter"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if input.Name != "" {
		existing.Name = input.Name
	}
	if input.Subject != "" {
		existing.Subject = input.Subject
	}
	existing.BodyHTML = input.BodyHTML
	existing.BodyText = input.BodyText
	existing.HasComplianceFooter = input.HasComplianceFooter

	if err := h.engagementRepo.WithDB(tenantDB).UpdateEmailTemplate(c.Context(), existing); err != nil {
		log.Printf("[Gmail] UpdateEmailTemplate error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update email template"})
	}
	return c.JSON(existing)
}

// DeleteEmailTemplate removes an email template.
// DELETE /gmail/email-templates/:id
func (h *GmailHandler) DeleteEmailTemplate(c *fiber.Ctx) error {
	orgID, _ := getGmailContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	if err := h.engagementRepo.WithDB(tenantDB).DeleteEmailTemplate(c.Context(), orgID, id); err != nil {
		log.Printf("[Gmail] DeleteEmailTemplate error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete email template"})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}

// PreviewEmailTemplate renders a template against an optional contact (or sample data)
// and returns the rendered subject and bodyHtml.
// POST /gmail/email-templates/:id/preview
//
// Request body (one of):
//
//	{ "contactId": "..." }                              — fetches real contact from DB
//	{ "sampleData": { "first_name": "Jane", ... } }    — uses provided data directly
func (h *GmailHandler) PreviewEmailTemplate(c *fiber.Ctx) error {
	orgID, _ := getGmailContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	var req struct {
		ContactID  string            `json:"contactId"`
		SampleData map[string]string `json:"sampleData"`
	}
	_ = c.BodyParser(&req)

	tmpl, err := h.engagementRepo.WithDB(tenantDB).GetEmailTemplate(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch email template"})
	}
	if tmpl == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email template not found"})
	}

	engine := h.templateEngine
	if engine == nil {
		engine = service.NewTemplateEngine()
	}

	var vars map[string]string

	if req.ContactID != "" && h.contactRepo != nil {
		contact, contactErr := h.contactRepo.WithDB(tenantDB).GetByID(c.Context(), orgID, req.ContactID)
		if contactErr == nil && contact != nil {
			contactMap := map[string]interface{}{
				"first_name":   contact.FirstName,
				"last_name":    contact.LastName,
				"email":        contact.EmailAddress,
				"phone":        contact.PhoneNumber,
				"account_name": contact.AccountName,
				"city":         contact.AddressCity,
				"state":        contact.AddressState,
				"country":      contact.AddressCountry,
			}
			vars = engine.ContactToTemplateVars(contactMap)
		}
	} else if req.SampleData != nil {
		vars = req.SampleData
	}

	if vars == nil {
		vars = map[string]string{}
	}

	subject, bodyHTML := engine.RenderTemplate(tmpl, vars)
	return c.JSON(fiber.Map{
		"subject":  subject,
		"bodyHtml": bodyHTML,
	})
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
	orgID, _, err := h.gmailService.DecodeStateForCallback(state)
	if err != nil {
		return "", nil, err
	}

	if h.dbManager.IsLocalMode() {
		return orgID, h.dbManager.GetMasterDB(), nil
	}

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
