package handler

import (
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// PdfTemplateHandler handles HTTP requests for PDF templates
type PdfTemplateHandler struct {
	repo *repo.PdfTemplateRepo
}

// NewPdfTemplateHandler creates a new PdfTemplateHandler
func NewPdfTemplateHandler(repo *repo.PdfTemplateRepo) *PdfTemplateHandler {
	return &PdfTemplateHandler{repo: repo}
}

// getRepo returns the repo with the correct tenant DB from context
func (h *PdfTemplateHandler) getRepo(c *fiber.Ctx) *repo.PdfTemplateRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// List returns all PDF templates for an entity type
func (h *PdfTemplateHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Query("entityType", "Quote")

	templates, err := h.getRepo(c).ListByEntityType(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(templates)
}

// Get returns a single PDF template
func (h *PdfTemplateHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	tpl, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tpl == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Template not found"})
	}

	return c.JSON(tpl)
}

// Create creates a new PDF template
func (h *PdfTemplateHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.PdfTemplateCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	tpl, err := h.getRepo(c).Create(c.Context(), orgID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(tpl)
}

// Update modifies a PDF template
func (h *PdfTemplateHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	var input entity.PdfTemplateUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	tpl, err := h.getRepo(c).Update(c.Context(), orgID, id, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if tpl == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Template not found"})
	}

	return c.JSON(tpl)
}

// Delete removes a PDF template
func (h *PdfTemplateHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	err := h.getRepo(c).Delete(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Template not found or is a system template"})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// SetDefault sets a template as default for its entity type
func (h *PdfTemplateHandler) SetDefault(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	err := h.getRepo(c).SetDefault(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

// AvailableFields returns all possible fields grouped by section
func (h *PdfTemplateHandler) AvailableFields(c *fiber.Ctx) error {
	entityType := c.Query("entityType", "Quote")

	if entityType != "Quote" {
		return c.JSON([]entity.AvailableField{})
	}

	fields := []entity.AvailableField{
		// Header
		{Name: "companyLogo", Label: "Company Logo", Section: "header"},
		{Name: "companyName", Label: "Company Name", Section: "header"},
		{Name: "quoteNumber", Label: "Quote Number", Section: "header"},
		{Name: "quoteDate", Label: "Quote Date", Section: "header"},
		{Name: "validUntil", Label: "Valid Until", Section: "header"},
		{Name: "status", Label: "Status", Section: "header"},
		// Bill To
		{Name: "contactName", Label: "Contact Name", Section: "billTo"},
		{Name: "accountName", Label: "Account Name", Section: "billTo"},
		{Name: "billingAddressStreet", Label: "Street", Section: "billTo"},
		{Name: "billingAddressCity", Label: "City", Section: "billTo"},
		{Name: "billingAddressState", Label: "State", Section: "billTo"},
		{Name: "billingAddressPostalCode", Label: "Postal Code", Section: "billTo"},
		{Name: "billingAddressCountry", Label: "Country", Section: "billTo"},
		// Ship To
		{Name: "shippingAddressStreet", Label: "Street", Section: "shipTo"},
		{Name: "shippingAddressCity", Label: "City", Section: "shipTo"},
		{Name: "shippingAddressState", Label: "State", Section: "shipTo"},
		{Name: "shippingAddressPostalCode", Label: "Postal Code", Section: "shipTo"},
		{Name: "shippingAddressCountry", Label: "Country", Section: "shipTo"},
		// Line Items
		{Name: "name", Label: "Item Name", Section: "lineItems"},
		{Name: "description", Label: "Description", Section: "lineItems"},
		{Name: "sku", Label: "SKU", Section: "lineItems"},
		{Name: "quantity", Label: "Quantity", Section: "lineItems"},
		{Name: "unitPrice", Label: "Unit Price", Section: "lineItems"},
		{Name: "discount", Label: "Discount", Section: "lineItems"},
		{Name: "total", Label: "Total", Section: "lineItems"},
		// Totals
		{Name: "subtotal", Label: "Subtotal", Section: "totals"},
		{Name: "discountAmount", Label: "Discount Amount", Section: "totals"},
		{Name: "taxAmount", Label: "Tax Amount", Section: "totals"},
		{Name: "shippingAmount", Label: "Shipping Amount", Section: "totals"},
		{Name: "grandTotal", Label: "Grand Total", Section: "totals"},
		// Description
		{Name: "description", Label: "Description", Section: "description"},
		// Terms
		{Name: "terms", Label: "Terms & Conditions", Section: "terms"},
		// Notes
		{Name: "notes", Label: "Notes", Section: "notes"},
		// Footer
		{Name: "companyName", Label: "Company Name", Section: "footer"},
		{Name: "generatedDate", Label: "Generated Date", Section: "footer"},
	}

	return c.JSON(fields)
}

// PreviewHTML returns rendered HTML for preview
func (h *PdfTemplateHandler) PreviewHTML(c *fiber.Ctx) error {
	// This will be handled by the quote handler's GeneratePDF with download=false
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Use GET /quotes/:id/pdf?template=TEMPLATE_ID&download=false for preview",
	})
}

// RegisterRoutes registers admin routes for PDF template management
func (h *PdfTemplateHandler) RegisterRoutes(app fiber.Router) {
	templates := app.Group("/pdf-templates")
	templates.Get("", h.List)
	templates.Get("/available-fields", h.AvailableFields)
	templates.Get("/:id", h.Get)
	templates.Post("", h.Create)
	templates.Put("/:id", h.Update)
	templates.Delete("/:id", h.Delete)
	templates.Post("/:id/set-default", h.SetDefault)
}

// RegisterPublicRoutes registers read-only routes accessible to all authenticated users
func (h *PdfTemplateHandler) RegisterPublicRoutes(app fiber.Router) {
	templates := app.Group("/pdf-templates")
	templates.Get("/list", h.List)
}
