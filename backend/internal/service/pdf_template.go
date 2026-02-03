package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// PdfTemplateService renders quote HTML from templates
type PdfTemplateService struct {
	quoteRepo       *repo.QuoteRepo
	pdfTemplateRepo *repo.PdfTemplateRepo
}

// NewPdfTemplateService creates a new PdfTemplateService
func NewPdfTemplateService(quoteRepo *repo.QuoteRepo, pdfTemplateRepo *repo.PdfTemplateRepo) *PdfTemplateService {
	return &PdfTemplateService{
		quoteRepo:       quoteRepo,
		pdfTemplateRepo: pdfTemplateRepo,
	}
}

// WithRepos returns a new PdfTemplateService with the given repos
// This is used to create a tenant-aware service for multi-tenant scenarios
func (s *PdfTemplateService) WithRepos(quoteRepo *repo.QuoteRepo, pdfTemplateRepo *repo.PdfTemplateRepo) *PdfTemplateService {
	return &PdfTemplateService{
		quoteRepo:       quoteRepo,
		pdfTemplateRepo: pdfTemplateRepo,
	}
}

// RenderQuoteHTML renders a quote as HTML using a template
func (s *PdfTemplateService) RenderQuoteHTML(ctx context.Context, orgID, quoteID, templateID string) (string, error) {
	// Fetch quote
	quote, err := s.quoteRepo.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch quote: %w", err)
	}
	if quote == nil {
		return "", fmt.Errorf("quote not found")
	}

	// Fetch template
	var tpl *entity.PdfTemplate
	if templateID != "" {
		tpl, err = s.pdfTemplateRepo.GetByID(ctx, orgID, templateID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch template: %w", err)
		}
	}
	if tpl == nil {
		tpl, _ = s.pdfTemplateRepo.GetDefaultByEntityType(ctx, orgID, "Quote")
	}

	// Build context
	pdfCtx := buildQuotePdfContext(quote, tpl)

	// Get base design HTML
	var htmlTemplate string
	if tpl != nil {
		htmlTemplate = getBaseDesignHTML(tpl.BaseDesign)
	} else {
		htmlTemplate = getBaseDesignHTML("professional")
	}

	// Build section/field enabled maps
	sectionEnabled := make(map[string]bool)
	fieldEnabled := make(map[string]map[string]bool)
	if pdfCtx.Sections != nil {
		for _, sec := range pdfCtx.Sections {
			sectionEnabled[sec.ID] = sec.Enabled
			fm := make(map[string]bool)
			for _, f := range sec.Fields {
				fm[f] = true
			}
			fieldEnabled[sec.ID] = fm
		}
	}

	// Parse and execute template
	funcMap := template.FuncMap{
		"sectionEnabled": func(sectionID string) bool {
			enabled, ok := sectionEnabled[sectionID]
			return ok && enabled
		},
		"fieldEnabled": func(sectionID, fieldName string) bool {
			sec, ok := fieldEnabled[sectionID]
			if !ok {
				return false
			}
			enabled, ok := sec[fieldName]
			return ok && enabled
		},
		"formatCurrency": func(amount float64, currency string) string {
			if currency == "" {
				currency = "USD"
			}
			symbol := "$"
			switch currency {
			case "EUR":
				symbol = "€"
			case "GBP":
				symbol = "£"
			}
			return fmt.Sprintf("%s%.2f", symbol, amount)
		},
		"formatDate": func(dateStr string) string {
			t, err := time.Parse(time.RFC3339, dateStr)
			if err != nil {
				t, err = time.Parse("2006-01-02", dateStr)
				if err != nil {
					return dateStr
				}
			}
			return t.Format("Jan 2, 2006")
		},
		"upper": strings.ToUpper,
		"hasLineItemField": func(fieldName string) bool {
			sec, ok := fieldEnabled["lineItems"]
			if !ok {
				return true // default show all
			}
			enabled, ok := sec[fieldName]
			return !ok || enabled
		},
	}

	tmpl, err := template.New("quote").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pdfCtx); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func buildQuotePdfContext(quote *entity.Quote, tpl *entity.PdfTemplate) *entity.QuotePdfContext {
	ctx := &entity.QuotePdfContext{
		Quote:     quote,
		LineItems: quote.LineItems,
	}

	if tpl != nil {
		ctx.Branding = tpl.Branding
		ctx.Sections = tpl.Sections
	} else {
		ctx.Branding = &entity.PdfBranding{
			PrimaryColor: "#2563eb",
			AccentColor:  "#1e40af",
			FontFamily:   "Helvetica, Arial, sans-serif",
		}
	}

	currency := quote.Currency
	if currency == "" {
		currency = "USD"
	}

	ctx.FormattedSubtotal = formatMoney(quote.Subtotal, currency)
	ctx.FormattedDiscountAmount = formatMoney(quote.DiscountAmount, currency)
	ctx.FormattedTaxAmount = formatMoney(quote.TaxAmount, currency)
	ctx.FormattedShippingAmount = formatMoney(quote.ShippingAmount, currency)
	ctx.FormattedGrandTotal = formatMoney(quote.GrandTotal, currency)

	if quote.ValidUntil != "" {
		if t, err := time.Parse("2006-01-02", quote.ValidUntil); err == nil {
			ctx.FormattedValidUntil = t.Format("Jan 2, 2006")
		} else if t, err := time.Parse(time.RFC3339, quote.ValidUntil); err == nil {
			ctx.FormattedValidUntil = t.Format("Jan 2, 2006")
		} else {
			ctx.FormattedValidUntil = quote.ValidUntil
		}
	}
	ctx.FormattedCreatedAt = quote.CreatedAt.Format("Jan 2, 2006")

	// Set conditional booleans
	ctx.HasDiscount = quote.DiscountAmount > 0 || quote.DiscountPercent > 0
	ctx.HasTax = quote.TaxAmount > 0 || quote.TaxPercent > 0
	ctx.HasShipping = quote.ShippingAmount > 0
	ctx.HasDescription = quote.Description != ""
	ctx.HasTerms = quote.Terms != ""
	ctx.HasNotes = quote.Notes != ""
	ctx.HasBillingAddress = quote.BillingAddressStreet != "" || quote.BillingAddressCity != ""
	ctx.HasShippingAddress = quote.ShippingAddressStreet != "" || quote.ShippingAddressCity != ""
	ctx.HasContactName = quote.ContactName != ""
	ctx.HasAccountName = quote.AccountName != ""
	ctx.HasValidUntil = quote.ValidUntil != ""
	ctx.HasLogo = ctx.Branding != nil && ctx.Branding.LogoURL != ""

	return ctx
}

func formatMoney(amount float64, currency string) string {
	symbol := "$"
	switch currency {
	case "EUR":
		symbol = "€"
	case "GBP":
		symbol = "£"
	}
	return fmt.Sprintf("%s%.2f", symbol, amount)
}
