package repo

import (
	"github.com/fastcrm/backend/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// PdfTemplateRepo handles database operations for PDF templates
type PdfTemplateRepo struct {
	db db.DBConn
}

// NewPdfTemplateRepo creates a new PdfTemplateRepo
func NewPdfTemplateRepo(conn db.DBConn) *PdfTemplateRepo {
	return &PdfTemplateRepo{db: conn}
}

// WithDB returns a new PdfTemplateRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *PdfTemplateRepo) WithDB(conn db.DBConn) *PdfTemplateRepo {
	if conn == nil {
		return r
	}
	return &PdfTemplateRepo{db: conn}
}

// DB returns the current database connection
func (r *PdfTemplateRepo) DB() db.DBConn {
	return r.db
}

func scanTemplate(row interface{ Scan(dest ...any) error }) (*entity.PdfTemplate, error) {
	var t entity.PdfTemplate
	err := row.Scan(
		&t.ID, &t.OrgID, &t.Name, &t.EntityType,
		&t.IsDefault, &t.IsSystem, &t.BaseDesign,
		&t.BrandingRaw, &t.SectionsRaw,
		&t.PageSize, &t.Orientation, &t.Margins,
		&t.CreatedAt, &t.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize branding
	t.Branding = &entity.PdfBranding{}
	json.Unmarshal([]byte(t.BrandingRaw), t.Branding)

	// Deserialize sections
	t.Sections = []entity.SectionConfig{}
	json.Unmarshal([]byte(t.SectionsRaw), &t.Sections)

	return &t, nil
}

const templateColumns = `id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at`

// Create inserts a new PDF template
func (r *PdfTemplateRepo) Create(ctx context.Context, orgID string, input entity.PdfTemplateCreateInput) (*entity.PdfTemplate, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	entityType := input.EntityType
	if entityType == "" {
		entityType = "Quote"
	}
	baseDesign := input.BaseDesign
	if baseDesign == "" {
		baseDesign = "professional"
	}
	pageSize := input.PageSize
	if pageSize == "" {
		pageSize = "A4"
	}
	orientation := input.Orientation
	if orientation == "" {
		orientation = "portrait"
	}
	margins := input.Margins
	if margins == "" {
		margins = "10mm,10mm,10mm,10mm"
	}

	brandingJSON := "{}"
	if input.Branding != nil {
		if b, err := json.Marshal(input.Branding); err == nil {
			brandingJSON = string(b)
		}
	}

	sectionsJSON := "[]"
	if input.Sections != nil {
		if b, err := json.Marshal(input.Sections); err == nil {
			sectionsJSON = string(b)
		}
	} else {
		// Use default sections for the base design
		sectionsJSON = defaultSectionsJSON()
	}

	id := sfid.NewPdfTemplate()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pdf_templates (id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at)
		VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, orgID, input.Name, entityType, baseDesign, brandingJSON, sectionsJSON, pageSize, orientation, margins, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create pdf template: %w", err)
	}

	return r.GetByID(ctx, orgID, id)
}

func defaultSectionsJSON() string {
	sections := []entity.SectionConfig{
		{ID: "header", Label: "Header", Enabled: true, Fields: []string{"companyLogo", "companyName", "quoteNumber", "quoteDate", "validUntil", "status"}},
		{ID: "billTo", Label: "Bill To", Enabled: true, Fields: []string{"contactName", "accountName", "billingAddressStreet", "billingAddressCity", "billingAddressState", "billingAddressPostalCode", "billingAddressCountry"}},
		{ID: "shipTo", Label: "Ship To", Enabled: false, Fields: []string{"shippingAddressStreet", "shippingAddressCity", "shippingAddressState", "shippingAddressPostalCode", "shippingAddressCountry"}},
		{ID: "lineItems", Label: "Line Items", Enabled: true, Fields: []string{"name", "description", "sku", "quantity", "unitPrice", "discount", "total"}},
		{ID: "totals", Label: "Totals", Enabled: true, Fields: []string{"subtotal", "discountAmount", "taxAmount", "shippingAmount", "grandTotal"}},
		{ID: "description", Label: "Description", Enabled: false, Fields: []string{"description"}},
		{ID: "terms", Label: "Terms & Conditions", Enabled: true, Fields: []string{"terms"}},
		{ID: "notes", Label: "Notes", Enabled: true, Fields: []string{"notes"}},
		{ID: "footer", Label: "Footer", Enabled: true, Fields: []string{"companyName", "generatedDate"}},
	}
	b, _ := json.Marshal(sections)
	return string(b)
}

// GetByID retrieves a template by ID
func (r *PdfTemplateRepo) GetByID(ctx context.Context, orgID, id string) (*entity.PdfTemplate, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+templateColumns+` FROM pdf_templates WHERE id = ? AND org_id = ?`, id, orgID)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pdf template: %w", err)
	}
	return t, nil
}

// GetDefaultByEntityType retrieves the default template for an entity type
func (r *PdfTemplateRepo) GetDefaultByEntityType(ctx context.Context, orgID, entityType string) (*entity.PdfTemplate, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+templateColumns+` FROM pdf_templates WHERE org_id = ? AND entity_type = ? AND is_default = 1 LIMIT 1`,
		orgID, entityType)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		// Fall back to any template
		row = r.db.QueryRowContext(ctx,
			`SELECT `+templateColumns+` FROM pdf_templates WHERE org_id = ? AND entity_type = ? ORDER BY created_at ASC LIMIT 1`,
			orgID, entityType)
		t, err = scanTemplate(row)
		if err == sql.ErrNoRows {
			return nil, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get default pdf template: %w", err)
	}
	return t, nil
}

// ListByEntityType lists templates for an entity type
func (r *PdfTemplateRepo) ListByEntityType(ctx context.Context, orgID, entityType string) ([]entity.PdfTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+templateColumns+` FROM pdf_templates WHERE org_id = ? AND entity_type = ? ORDER BY is_default DESC, name ASC`,
		orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list pdf templates: %w", err)
	}
	defer rows.Close()

	var templates []entity.PdfTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			continue
		}
		templates = append(templates, *t)
	}
	if templates == nil {
		templates = []entity.PdfTemplate{}
	}
	return templates, nil
}

// Update modifies a template
func (r *PdfTemplateRepo) Update(ctx context.Context, orgID, id string, input entity.PdfTemplateUpdateInput) (*entity.PdfTemplate, error) {
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.BaseDesign != nil {
		existing.BaseDesign = *input.BaseDesign
	}
	if input.Branding != nil {
		existing.Branding = input.Branding
	}
	if input.Sections != nil {
		existing.Sections = input.Sections
	}
	if input.PageSize != nil {
		existing.PageSize = *input.PageSize
	}
	if input.Orientation != nil {
		existing.Orientation = *input.Orientation
	}
	if input.Margins != nil {
		existing.Margins = *input.Margins
	}

	brandingJSON := "{}"
	if existing.Branding != nil {
		if b, err := json.Marshal(existing.Branding); err == nil {
			brandingJSON = string(b)
		}
	}
	sectionsJSON := "[]"
	if existing.Sections != nil {
		if b, err := json.Marshal(existing.Sections); err == nil {
			sectionsJSON = string(b)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.ExecContext(ctx, `
		UPDATE pdf_templates SET name = ?, base_design = ?, branding = ?, sections = ?,
			page_size = ?, orientation = ?, margins = ?, modified_at = ?
		WHERE id = ? AND org_id = ?
	`, existing.Name, existing.BaseDesign, brandingJSON, sectionsJSON,
		existing.PageSize, existing.Orientation, existing.Margins, now,
		id, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to update pdf template: %w", err)
	}

	return r.GetByID(ctx, orgID, id)
}

// SetDefault sets a template as the default for its entity type
func (r *PdfTemplateRepo) SetDefault(ctx context.Context, orgID, id string) error {
	t, err := r.GetByID(ctx, orgID, id)
	if err != nil || t == nil {
		return fmt.Errorf("template not found")
	}

	// Unset current defaults for this entity type
	r.db.ExecContext(ctx, `UPDATE pdf_templates SET is_default = 0 WHERE org_id = ? AND entity_type = ?`, orgID, t.EntityType)

	// Set new default
	_, err = r.db.ExecContext(ctx, `UPDATE pdf_templates SET is_default = 1 WHERE id = ? AND org_id = ?`, id, orgID)
	return err
}

// Delete removes a template (not system templates)
func (r *PdfTemplateRepo) Delete(ctx context.Context, orgID, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM pdf_templates WHERE id = ? AND org_id = ? AND is_system = 0`, id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete pdf template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
