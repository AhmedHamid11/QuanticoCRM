package handler

import (
	"database/sql"
	"errors"
	"log"
	"os"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SchedulingHandler handles HTTP requests for the scheduling feature
type SchedulingHandler struct {
	schedulingService *service.SchedulingService
	gcalService       *service.GoogleCalendarService
	schedulingRepo    *repo.SchedulingRepo
	dbManager         *db.Manager
	authRepo          *repo.AuthRepo
}

// NewSchedulingHandler creates a new SchedulingHandler
func NewSchedulingHandler(
	schedulingService *service.SchedulingService,
	gcalService *service.GoogleCalendarService,
	schedulingRepo *repo.SchedulingRepo,
	dbManager *db.Manager,
	authRepo *repo.AuthRepo,
) *SchedulingHandler {
	return &SchedulingHandler{
		schedulingService: schedulingService,
		gcalService:       gcalService,
		schedulingRepo:    schedulingRepo,
		dbManager:         dbManager,
		authRepo:          authRepo,
	}
}

// RegisterRoutes registers authenticated scheduling routes
func (h *SchedulingHandler) RegisterRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Scheduling routes")
	sched := router.Group("/scheduling")

	// Scheduling page CRUD
	sched.Get("/pages", h.ListPages)
	sched.Post("/pages", h.CreatePage)
	sched.Put("/pages/:id", h.UpdatePage)
	sched.Delete("/pages/:id", h.DeletePage)
	sched.Get("/pages/:id/bookings", h.ListBookings)

	// Google Calendar OAuth management
	sched.Get("/google/status", h.GetGoogleStatus)
	sched.Get("/google/connect", h.GetGoogleConnectURL)
	sched.Delete("/google/disconnect", h.DisconnectGoogle)
}

// RegisterPublicRoutes registers public (no auth) scheduling routes
func (h *SchedulingHandler) RegisterPublicRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering Scheduling public routes")

	// Google OAuth callback from Google
	router.Get("/scheduling/google/callback", h.GoogleCallback)

	// Public booking endpoints
	router.Get("/public/scheduling/:slug", h.GetPublicPage)
	router.Get("/public/scheduling/:slug/slots", h.GetAvailableSlots)
	router.Post("/public/scheduling/:slug/book", h.BookSlot)
}

// ========== Authenticated Handlers ==========

// ListPages returns the user's scheduling pages
// GET /scheduling/pages
func (h *SchedulingHandler) ListPages(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)

	tenantDB := middleware.GetTenantDBConn(c)
	r := h.schedulingRepo.WithDB(tenantDB)

	pages, err := r.ListPagesByUser(c.Context(), orgID, userID)
	if err != nil {
		log.Printf("[Scheduling] ListPages error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list scheduling pages",
		})
	}

	// Enrich pages with parsed availability for the response
	result := make([]fiber.Map, 0, len(pages))
	for _, page := range pages {
		result = append(result, pageToMap(page))
	}

	return c.JSON(fiber.Map{"pages": result})
}

// CreatePage creates a new scheduling page
// POST /scheduling/pages
func (h *SchedulingHandler) CreatePage(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)

	var input entity.SchedulingPageCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "title is required",
		})
	}
	if input.Slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "slug is required",
		})
	}

	// Use tenant DB for the service
	tenantDB := middleware.GetTenantDBConn(c)
	svc := service.NewSchedulingService(h.schedulingRepo.WithDB(tenantDB), h.gcalService)

	page, err := svc.CreatePage(c.Context(), orgID, userID, input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidSlug) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, service.ErrSlugTaken) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		log.Printf("[Scheduling] CreatePage error for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create scheduling page"})
	}

	return c.Status(fiber.StatusCreated).JSON(pageToMap(page))
}

// UpdatePage updates a scheduling page
// PUT /scheduling/pages/:id
func (h *SchedulingHandler) UpdatePage(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)
	pageID := c.Params("id")

	var input entity.SchedulingPageUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantDB := middleware.GetTenantDBConn(c)
	svc := service.NewSchedulingService(h.schedulingRepo.WithDB(tenantDB), h.gcalService)

	page, err := svc.UpdatePage(c.Context(), orgID, userID, pageID, input)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		if errors.Is(err, service.ErrInvalidSlug) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, service.ErrSlugTaken) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		log.Printf("[Scheduling] UpdatePage error for org %s page %s: %v", orgID, pageID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update scheduling page"})
	}

	return c.JSON(pageToMap(page))
}

// DeletePage deletes a scheduling page
// DELETE /scheduling/pages/:id
func (h *SchedulingHandler) DeletePage(c *fiber.Ctx) error {
	orgID, _ := getSchedulingContext(c)
	pageID := c.Params("id")

	tenantDB := middleware.GetTenantDBConn(c)
	r := h.schedulingRepo.WithDB(tenantDB)

	if err := r.DeletePage(c.Context(), pageID, orgID); err != nil {
		log.Printf("[Scheduling] DeletePage error for org %s page %s: %v", orgID, pageID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete scheduling page"})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ListBookings returns bookings for a scheduling page
// GET /scheduling/pages/:id/bookings
func (h *SchedulingHandler) ListBookings(c *fiber.Ctx) error {
	pageID := c.Params("id")

	tenantDB := middleware.GetTenantDBConn(c)
	r := h.schedulingRepo.WithDB(tenantDB)

	bookings, err := r.ListBookingsByPage(c.Context(), pageID)
	if err != nil {
		log.Printf("[Scheduling] ListBookings error for page %s: %v", pageID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list bookings"})
	}

	return c.JSON(fiber.Map{"bookings": bookings})
}

// GetGoogleStatus returns the Google Calendar connection status
// GET /scheduling/google/status
func (h *SchedulingHandler) GetGoogleStatus(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)

	tenantDB := middleware.GetTenantDBConn(c)
	gcalSvc := service.NewGoogleCalendarService(h.schedulingRepo.WithDB(tenantDB), h.gcalService.GetEncryptionKey())

	status, err := gcalSvc.GetConnectionStatus(c.Context(), orgID, userID)
	if err != nil {
		log.Printf("[Scheduling] GetGoogleStatus error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get Google Calendar status"})
	}

	return c.JSON(fiber.Map{"status": status})
}

// GetGoogleConnectURL returns the Google OAuth authorization URL
// GET /scheduling/google/connect
func (h *SchedulingHandler) GetGoogleConnectURL(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)

	redirectBase := getRedirectBase(c)

	authURL, err := h.gcalService.GetAuthorizationURL(c.Context(), orgID, userID, redirectBase)
	if err != nil {
		log.Printf("[Scheduling] GetGoogleConnectURL error for org %s user %s: %v", orgID, userID, err)
		if errors.Is(err, service.ErrGoogleMissingClientCfg) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Google Calendar integration is not configured on this server",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate authorization URL"})
	}

	return c.JSON(fiber.Map{"authUrl": authURL})
}

// DisconnectGoogle disconnects Google Calendar
// DELETE /scheduling/google/disconnect
func (h *SchedulingHandler) DisconnectGoogle(c *fiber.Ctx) error {
	orgID, userID := getSchedulingContext(c)

	tenantDB := middleware.GetTenantDBConn(c)
	gcalSvc := service.NewGoogleCalendarService(h.schedulingRepo.WithDB(tenantDB), h.gcalService.GetEncryptionKey())

	if err := gcalSvc.Disconnect(c.Context(), orgID, userID); err != nil {
		log.Printf("[Scheduling] DisconnectGoogle error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to disconnect Google Calendar"})
	}

	return c.JSON(fiber.Map{"status": "disconnected"})
}

// ========== Public Handlers ==========

// GoogleCallback handles the OAuth callback from Google
// GET /scheduling/google/callback
func (h *SchedulingHandler) GoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Redirect("/settings/scheduling?error=missing_code")
	}
	if state == "" {
		return c.Redirect("/settings/scheduling?error=missing_state")
	}

	redirectBase := getRedirectBase(c)

	orgID, _, err := h.gcalService.HandleCallback(c.Context(), code, state, redirectBase)
	if err != nil {
		log.Printf("[Scheduling] GoogleCallback error: %v", err)
		if errors.Is(err, service.ErrGoogleInvalidState) {
			return c.Redirect("/settings/scheduling?error=invalid_state")
		}
		return c.Redirect("/settings/scheduling?error=connection_failed")
	}

	log.Printf("[Scheduling] Google Calendar connected for org %s", orgID)

	// Redirect to frontend settings page
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = ""
	}
	return c.Redirect(frontendURL + "/settings/scheduling?connected=true")
}

// GetPublicPage returns public-safe scheduling page info
// GET /public/scheduling/:slug
func (h *SchedulingHandler) GetPublicPage(c *fiber.Ctx) error {
	slug := c.Params("slug")

	svc := service.NewSchedulingService(h.schedulingRepo, h.gcalService)

	pageView, err := svc.GetPublicPageInfo(c.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) || errors.Is(err, service.ErrPageInactive) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		log.Printf("[Scheduling] GetPublicPage error for slug %s: %v", slug, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get scheduling page"})
	}

	return c.JSON(pageView)
}

// GetAvailableSlots returns available time slots for a date
// GET /public/scheduling/:slug/slots?date=2026-02-27
func (h *SchedulingHandler) GetAvailableSlots(c *fiber.Ctx) error {
	slug := c.Params("slug")
	dateStr := c.Query("date")

	if dateStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "date query parameter is required (YYYY-MM-DD)"})
	}

	tenantDB, err := h.resolveTenantDBForSlug(c, slug)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		log.Printf("[Scheduling] GetAvailableSlots: failed to resolve tenant for slug %s: %v", slug, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to resolve scheduling page"})
	}

	svc := service.NewSchedulingService(h.schedulingRepo.WithDB(tenantDB), h.gcalService)

	slots, err := svc.GetAvailableSlots(c.Context(), slug, dateStr)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) || errors.Is(err, service.ErrPageInactive) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		log.Printf("[Scheduling] GetAvailableSlots error for slug %s date %s: %v", slug, dateStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get available slots"})
	}

	return c.JSON(fiber.Map{"slots": slots})
}

// BookSlot creates a booking for an available time slot
// POST /public/scheduling/:slug/book
func (h *SchedulingHandler) BookSlot(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var input entity.BookingCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate required fields
	if input.GuestName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "guestName is required"})
	}
	if input.GuestEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "guestEmail is required"})
	}
	if input.StartTime == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "startTime is required"})
	}

	tenantDB, err := h.resolveTenantDBForSlug(c, slug)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		log.Printf("[Scheduling] BookSlot: failed to resolve tenant for slug %s: %v", slug, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to resolve scheduling page"})
	}

	svc := service.NewSchedulingService(h.schedulingRepo.WithDB(tenantDB), h.gcalService)

	booking, err := svc.BookSlot(c.Context(), slug, input)
	if err != nil {
		if errors.Is(err, service.ErrPageNotFound) || errors.Is(err, service.ErrPageInactive) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scheduling page not found"})
		}
		if errors.Is(err, service.ErrSlotNotAvailable) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "This time slot is no longer available"})
		}
		log.Printf("[Scheduling] BookSlot error for slug %s: %v", slug, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create booking"})
	}

	return c.Status(fiber.StatusCreated).JSON(booking)
}

// ========== Helpers ==========

// getSchedulingContext extracts orgID and userID from Fiber context
func getSchedulingContext(c *fiber.Ctx) (string, string) {
	orgID, _ := c.Locals("orgID").(string)
	userID, _ := c.Locals("userID").(string)
	return orgID, userID
}

// getRedirectBase determines the redirect base URL for OAuth callbacks
func getRedirectBase(c *fiber.Ctx) string {
	// Check env var first
	if base := os.Getenv("API_BASE_URL"); base != "" {
		return base
	}
	// Infer from request
	scheme := "https"
	if c.Protocol() == "http" {
		scheme = "http"
	}
	return scheme + "://" + c.Hostname()
}

// resolveTenantDBForSlug finds the tenant DB for a given scheduling page slug
// First queries master DB to find the page (works in local mode where all data is in one DB)
// Then resolves the correct tenant DB based on org_id
func (h *SchedulingHandler) resolveTenantDBForSlug(c *fiber.Ctx, slug string) (db.DBConn, error) {
	// In local mode, the master DB contains all tenant tables — just use it directly
	if h.dbManager.IsLocalMode() {
		masterDB := h.dbManager.GetMasterDB()
		return masterDB, nil
	}

	// Production (Turso): we need to find the org for this slug
	// Query the master DB scheduling repo to find the page
	page, err := h.schedulingRepo.GetPageBySlug(c.Context(), slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrPageNotFound
		}
		return nil, err
	}

	// Resolve tenant DB for the page's org
	org, err := h.authRepo.GetOrganizationByID(c.Context(), page.OrgID)
	if err != nil {
		return nil, err
	}

	if org.DatabaseURL == "" {
		// Fall back to master DB
		return h.dbManager.GetMasterDB(), nil
	}

	tenantDB, err := h.dbManager.GetTenantDBConn(c.Context(), page.OrgID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		return nil, err
	}

	return tenantDB, nil
}

// pageToMap converts a SchedulingPage to a JSON-friendly map including parsed availability
func pageToMap(page *entity.SchedulingPage) fiber.Map {
	return fiber.Map{
		"id":              page.ID,
		"slug":            page.Slug,
		"title":           page.Title,
		"description":     page.Description,
		"durationMinutes": page.DurationMinutes,
		"timezone":        page.Timezone,
		"isActive":        page.IsActive,
		"bufferMinutes":   page.BufferMinutes,
		"maxDaysAhead":    page.MaxDaysAhead,
		"createdAt":       page.CreatedAt,
		"updatedAt":       page.UpdatedAt,
	}
}
