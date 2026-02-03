package handler

import (
	"context"
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// QuoteHandler handles HTTP requests for quotes
type QuoteHandler struct {
	repo              *repo.QuoteRepo
	authRepo          *repo.AuthRepo
	tripwireService   TripwireServiceInterface
	validationService ValidationServiceInterface
	pdfTemplateRepo   *repo.PdfTemplateRepo
	pdfTemplateService PdfTemplateServiceInterface
	pdfRenderer       PdfRendererInterface
}

// PdfTemplateServiceInterface defines the interface for PDF template rendering
type PdfTemplateServiceInterface interface {
	RenderQuoteHTML(ctx context.Context, orgID, quoteID, templateID string) (string, error)
}

// PdfRendererInterface defines the interface for PDF rendering
type PdfRendererInterface interface {
	RenderPDF(ctx context.Context, html string, opts entity.PdfRenderOptions) ([]byte, error)
}

// NewQuoteHandler creates a new QuoteHandler
func NewQuoteHandler(
	repo *repo.QuoteRepo,
	authRepo *repo.AuthRepo,
	tripwireService TripwireServiceInterface,
	validationService ValidationServiceInterface,
) *QuoteHandler {
	return &QuoteHandler{
		repo:              repo,
		authRepo:          authRepo,
		tripwireService:   tripwireService,
		validationService: validationService,
	}
}

// SetPdfServices sets the PDF-related services (called after initialization)
func (h *QuoteHandler) SetPdfServices(
	pdfTemplateRepo *repo.PdfTemplateRepo,
	pdfTemplateService PdfTemplateServiceInterface,
	pdfRenderer PdfRendererInterface,
) {
	h.pdfTemplateRepo = pdfTemplateRepo
	h.pdfTemplateService = pdfTemplateService
	h.pdfRenderer = pdfRenderer
}

// getRepo returns the Quote repo using the tenant database from context
func (h *QuoteHandler) getRepo(c *fiber.Ctx) *repo.QuoteRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// resolveUserNames adds createdByName and modifiedByName to records
func (h *QuoteHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
	if h.authRepo == nil || len(records) == 0 {
		return
	}

	// Helper to extract user ID from interface{} (handles both string and *string)
	extractUserID := func(val interface{}) string {
		if val == nil {
			return ""
		}
		if s, ok := val.(string); ok {
			return s
		}
		if sp, ok := val.(*string); ok && sp != nil {
			return *sp
		}
		return ""
	}

	// Collect unique user IDs
	userIDSet := make(map[string]bool)
	for _, record := range records {
		if id := extractUserID(record["createdById"]); id != "" {
			userIDSet[id] = true
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			userIDSet[id] = true
		}
	}

	if len(userIDSet) == 0 {
		return
	}

	// Convert to slice
	userIDs := make([]string, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	// Lookup names
	userNames, err := h.authRepo.GetUserNamesByIDs(ctx, userIDs)
	if err != nil {
		log.Printf("WARNING: Failed to lookup user names: %v", err)
		return
	}

	// Apply names to records
	for _, record := range records {
		if id := extractUserID(record["createdById"]); id != "" {
			record["createdByName"] = userNames[id]
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			record["modifiedByName"] = userNames[id]
		}
	}
}

// getPdfTemplateRepo returns the PdfTemplate repo using the tenant database from context
func (h *QuoteHandler) getPdfTemplateRepo(c *fiber.Ctx) *repo.PdfTemplateRepo {
	if h.pdfTemplateRepo == nil {
		return nil
	}
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.pdfTemplateRepo.WithDB(tenantDB)
	}
	return h.pdfTemplateRepo
}

// List returns all quotes for the current organization
func (h *QuoteHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	params := entity.QuoteListParams{
		Search:     c.Query("search"),
		SortBy:     c.Query("sortBy"),
		SortDir:    c.Query("sortDir"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
		Filter:     c.Query("filter"),
		KnownTotal: c.QueryInt("knownTotal", 0),
	}

	result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Resolve user names for created_by and modified_by
	if h.authRepo != nil && len(result.Data) > 0 {
		userIDSet := make(map[string]bool)
		for _, quote := range result.Data {
			if quote.CreatedByID != nil && *quote.CreatedByID != "" {
				userIDSet[*quote.CreatedByID] = true
			}
			if quote.ModifiedByID != nil && *quote.ModifiedByID != "" {
				userIDSet[*quote.ModifiedByID] = true
			}
		}
		if len(userIDSet) > 0 {
			userIDs := make([]string, 0, len(userIDSet))
			for id := range userIDSet {
				userIDs = append(userIDs, id)
			}
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				for i := range result.Data {
					if result.Data[i].CreatedByID != nil {
						result.Data[i].CreatedByName = userNames[*result.Data[i].CreatedByID]
					}
					if result.Data[i].ModifiedByID != nil {
						result.Data[i].ModifiedByName = userNames[*result.Data[i].ModifiedByID]
					}
				}
			}
		}
	}

	return c.JSON(result)
}

// Get returns a single quote by ID
func (h *QuoteHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	quote, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if quote == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Quote not found",
		})
	}

	// Resolve user names for created_by and modified_by
	if h.authRepo != nil {
		userIDs := make([]string, 0, 2)
		if quote.CreatedByID != nil && *quote.CreatedByID != "" {
			userIDs = append(userIDs, *quote.CreatedByID)
		}
		if quote.ModifiedByID != nil && *quote.ModifiedByID != "" {
			userIDs = append(userIDs, *quote.ModifiedByID)
		}
		if len(userIDs) > 0 {
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				if quote.CreatedByID != nil {
					quote.CreatedByName = userNames[*quote.CreatedByID]
				}
				if quote.ModifiedByID != nil {
					quote.ModifiedByName = userNames[*quote.ModifiedByID]
				}
			}
		}
	}

	return c.JSON(quote)
}

// Create creates a new quote
func (h *QuoteHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.QuoteCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Quote", "", "CREATE", nil, newRecord)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	quote, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Fire tripwires for CREATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Quote", quote.ID, "CREATE", nil, StructToMap(quote))
	}

	return c.Status(fiber.StatusCreated).JSON(quote)
}

// Update updates an existing quote
func (h *QuoteHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.QuoteUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldQuote, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldQuote)
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Quote", id, "UPDATE", oldRecord, newRecord)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	quote, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if quote == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Quote not found",
		})
	}

	// Fire tripwires for UPDATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Quote", id, "UPDATE", oldRecord, StructToMap(quote))
	}

	return c.JSON(quote)
}

// Delete soft-deletes a quote
func (h *QuoteHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldQuote, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldQuote)
	}

	// Validate before delete
	if h.validationService != nil && oldRecord != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Quote", id, "DELETE", oldRecord, nil)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	err := h.getRepo(c).Delete(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Quote not found",
		})
	}

	// Fire tripwires for DELETE event
	if h.tripwireService != nil && oldRecord != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Quote", id, "DELETE", oldRecord, nil)
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// SaveLineItems replaces all line items for a quote
func (h *QuoteHandler) SaveLineItems(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	quoteID := c.Params("id")

	var items []entity.QuoteLineItemInput
	if err := c.BodyParser(&items); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	quote, err := h.getRepo(c).SaveLineItems(c.Context(), orgID, quoteID, items, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if quote == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Quote not found",
		})
	}

	return c.JSON(quote)
}

// GeneratePDF generates a PDF for a quote
func (h *QuoteHandler) GeneratePDF(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	quoteID := c.Params("id")
	templateID := c.Query("template")
	download := c.Query("download", "true") == "true"

	if h.pdfTemplateService == nil || h.pdfRenderer == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "PDF generation is not configured",
		})
	}

	// Get tenant-aware repos
	tenantQuoteRepo := h.getRepo(c)
	tenantPdfTemplateRepo := h.getPdfTemplateRepo(c)

	// Get tenant-aware PDF service by creating one with tenant repos
	var pdfService PdfTemplateServiceInterface
	if svc, ok := h.pdfTemplateService.(*service.PdfTemplateService); ok {
		pdfService = svc.WithRepos(tenantQuoteRepo, tenantPdfTemplateRepo)
	} else {
		pdfService = h.pdfTemplateService
	}

	// If no template specified, use default
	if templateID == "" && tenantPdfTemplateRepo != nil {
		tpl, _ := tenantPdfTemplateRepo.GetDefaultByEntityType(c.Context(), orgID, "Quote")
		if tpl != nil {
			templateID = tpl.ID
		}
	}

	// Render HTML
	log.Printf("[PDF] Starting HTML render for quote %s, template %s", quoteID, templateID)
	html, err := pdfService.RenderQuoteHTML(c.Context(), orgID, quoteID, templateID)
	if err != nil {
		log.Printf("[PDF] Failed to render HTML: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to render template: " + err.Error(),
		})
	}
	log.Printf("[PDF] HTML rendered, length: %d bytes", len(html))

	// Get template for page settings
	opts := entity.PdfRenderOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		MarginTop:   "10mm",
		MarginBottom: "10mm",
		MarginLeft:  "10mm",
		MarginRight: "10mm",
	}
	if templateID != "" && h.getPdfTemplateRepo(c) != nil {
		tpl, _ := h.getPdfTemplateRepo(c).GetByID(c.Context(), orgID, templateID)
		if tpl != nil {
			if tpl.PageSize != "" {
				opts.PageSize = tpl.PageSize
			}
			if tpl.Orientation != "" {
				opts.Orientation = tpl.Orientation
			}
			if tpl.Margins != "" {
				// margins stored as "top,bottom,left,right"
				parts := splitMargins(tpl.Margins)
				if len(parts) == 4 {
					opts.MarginTop = parts[0]
					opts.MarginBottom = parts[1]
					opts.MarginLeft = parts[2]
					opts.MarginRight = parts[3]
				}
			}
		}
	}

	// Render PDF
	log.Printf("[PDF] Starting PDF render with options: %+v", opts)
	pdfBytes, err := h.pdfRenderer.RenderPDF(c.Context(), html, opts)
	if err != nil {
		log.Printf("[PDF] Failed to render PDF: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate PDF: " + err.Error(),
		})
	}
	log.Printf("[PDF] PDF generated, size: %d bytes", len(pdfBytes))

	// Get quote for filename
	quote, _ := h.getRepo(c).GetByID(c.Context(), orgID, quoteID)
	filename := "quote.pdf"
	if quote != nil {
		filename = quote.QuoteNumber + ".pdf"
	}

	c.Set("Content-Type", "application/pdf")
	if download {
		c.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	} else {
		c.Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	}

	return c.Send(pdfBytes)
}

func splitMargins(margins string) []string {
	var parts []string
	current := ""
	for _, ch := range margins {
		if ch == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// RegisterRoutes registers quote routes on the Fiber app
func (h *QuoteHandler) RegisterRoutes(app fiber.Router) {
	quotes := app.Group("/quotes")
	quotes.Get("/", h.List)
	quotes.Get("/:id", h.Get)
	quotes.Post("/", h.Create)
	quotes.Put("/:id", h.Update)
	quotes.Patch("/:id", h.Update)
	quotes.Delete("/:id", h.Delete)
	quotes.Put("/:id/line-items", h.SaveLineItems)
	quotes.Get("/:id/pdf", h.GeneratePDF)
}
