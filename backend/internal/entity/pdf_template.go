package entity

// PdfTemplate represents a PDF template configuration
type PdfTemplate struct {
	ID          string `json:"id" db:"id"`
	OrgID       string `json:"orgId" db:"org_id"`
	Name        string `json:"name" db:"name"`
	EntityType  string `json:"entityType" db:"entity_type"`
	IsDefault   bool   `json:"isDefault" db:"is_default"`
	IsSystem    bool   `json:"isSystem" db:"is_system"`
	BaseDesign  string `json:"baseDesign" db:"base_design"`
	BrandingRaw string `json:"-" db:"branding"`
	SectionsRaw string `json:"-" db:"sections"`
	Branding    *PdfBranding    `json:"branding" db:"-"`
	Sections    []SectionConfig `json:"sections" db:"-"`
	PageSize    string `json:"pageSize" db:"page_size"`
	Orientation string `json:"orientation" db:"orientation"`
	Margins     string `json:"margins" db:"margins"`
	CreatedAt   string `json:"createdAt" db:"created_at"`
	ModifiedAt  string `json:"modifiedAt" db:"modified_at"`
}

// PdfBranding holds branding configuration for PDF templates
type PdfBranding struct {
	LogoURL      string `json:"logoUrl"`
	CompanyName  string `json:"companyName"`
	PrimaryColor string `json:"primaryColor"`
	AccentColor  string `json:"accentColor"`
	FontFamily   string `json:"fontFamily"`
}

// SectionConfig defines a section in the PDF template
type SectionConfig struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Enabled bool     `json:"enabled"`
	Fields  []string `json:"fields"`
}

// PdfTemplateCreateInput represents input for creating a template
type PdfTemplateCreateInput struct {
	Name        string          `json:"name"`
	EntityType  string          `json:"entityType"`
	BaseDesign  string          `json:"baseDesign"`
	Branding    *PdfBranding    `json:"branding"`
	Sections    []SectionConfig `json:"sections"`
	PageSize    string          `json:"pageSize"`
	Orientation string          `json:"orientation"`
	Margins     string          `json:"margins"`
}

// PdfTemplateUpdateInput represents input for updating a template
type PdfTemplateUpdateInput struct {
	Name        *string         `json:"name"`
	BaseDesign  *string         `json:"baseDesign"`
	Branding    *PdfBranding    `json:"branding"`
	Sections    []SectionConfig `json:"sections,omitempty"`
	PageSize    *string         `json:"pageSize"`
	Orientation *string         `json:"orientation"`
	Margins     *string         `json:"margins"`
}

// QuotePdfContext holds all data needed to render a quote PDF
type QuotePdfContext struct {
	Quote       *Quote
	LineItems   []QuoteLineItem
	Branding    *PdfBranding
	Sections    []SectionConfig

	// Pre-computed display values
	FormattedSubtotal       string
	FormattedDiscountAmount string
	FormattedTaxAmount      string
	FormattedShippingAmount string
	FormattedGrandTotal     string
	FormattedValidUntil     string
	FormattedCreatedAt      string

	// Conditional rendering booleans
	HasDiscount        bool
	HasTax             bool
	HasShipping        bool
	HasDescription     bool
	HasTerms           bool
	HasNotes           bool
	HasBillingAddress  bool
	HasShippingAddress bool
	HasContactName     bool
	HasAccountName     bool
	HasValidUntil      bool
	HasLogo            bool
}

// PdfRenderOptions holds PDF rendering options
type PdfRenderOptions struct {
	PageSize    string
	Orientation string
	MarginTop   string
	MarginBottom string
	MarginLeft  string
	MarginRight string
}

// AvailableField describes a field available for a PDF section
type AvailableField struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Section string `json:"section"`
}
