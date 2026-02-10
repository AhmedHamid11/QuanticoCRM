package repo

import (
	"github.com/fastcrm/backend/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// QuoteRepo handles database operations for quotes
type QuoteRepo struct {
	db db.DBConn
}

// NewQuoteRepo creates a new QuoteRepo
func NewQuoteRepo(conn db.DBConn) *QuoteRepo {
	return &QuoteRepo{db: conn}
}

// WithDB returns a new QuoteRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *QuoteRepo) WithDB(conn db.DBConn) *QuoteRepo {
	if conn == nil {
		return r
	}
	return &QuoteRepo{db: conn}
}

// DB returns the current database connection
func (r *QuoteRepo) DB() db.DBConn {
	return r.db
}

// nextQuoteNumber generates the next quote number for an org (Q-YYYY-NNNN)
func (r *QuoteRepo) nextQuoteNumber(ctx context.Context, orgID string) (string, error) {
	year := time.Now().UTC().Year()
	prefix := fmt.Sprintf("Q-%d-", year)

	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM quotes WHERE org_id = ? AND quote_number LIKE ?`,
		orgID, prefix+"%",
	).Scan(&count)
	if err != nil {
		count = 0
	}

	return fmt.Sprintf("Q-%d-%04d", year, count+1), nil
}

// recalcTotals computes subtotal, tax, discount, and grand total from line items
func recalcTotals(items []entity.QuoteLineItem, discountPercent, discountAmount, taxPercent, shippingAmount float64) (subtotal, taxAmt, grandTotal float64) {
	for _, item := range items {
		subtotal += item.Total
	}
	subtotal = math.Round(subtotal*100) / 100

	// Apply quote-level discount
	discount := discountAmount
	if discountPercent > 0 {
		discount = math.Round(subtotal*discountPercent) / 100
	}

	afterDiscount := subtotal - discount

	// Apply tax
	taxAmt = 0
	if taxPercent > 0 {
		taxAmt = math.Round(afterDiscount*taxPercent) / 100
	}

	grandTotal = math.Round((afterDiscount+taxAmt+shippingAmount)*100) / 100
	return subtotal, taxAmt, grandTotal
}

// calcLineItemTotal computes a line item total
func calcLineItemTotal(qty, unitPrice, discountPct, discountAmt float64) float64 {
	lineTotal := qty * unitPrice
	if discountPct > 0 {
		lineTotal -= math.Round(lineTotal*discountPct) / 100
	} else if discountAmt > 0 {
		lineTotal -= discountAmt
	}
	return math.Round(lineTotal*100) / 100
}

// Create inserts a new quote with its line items
func (r *QuoteRepo) Create(ctx context.Context, orgID string, input entity.QuoteCreateInput, userID string) (*entity.Quote, error) {
	quoteNumber, err := r.nextQuoteNumber(ctx, orgID)
	if err != nil {
		return nil, err
	}

	status := input.Status
	if status == "" {
		status = "Draft"
	}
	currency := input.Currency
	if currency == "" {
		currency = "USD"
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	quote := &entity.Quote{
		ID:                        sfid.NewQuote(),
		OrgID:                     orgID,
		Name:                      input.Name,
		QuoteNumber:               quoteNumber,
		Status:                    status,
		AccountID:                 input.AccountID,
		AccountName:               input.AccountName,
		ContactID:                 input.ContactID,
		ContactName:               input.ContactName,
		ValidUntil:                input.ValidUntil,
		DiscountPercent:           input.DiscountPercent,
		DiscountAmount:            input.DiscountAmount,
		TaxPercent:                input.TaxPercent,
		ShippingAmount:            input.ShippingAmount,
		Currency:                  currency,
		BillingAddressStreet:      input.BillingAddressStreet,
		BillingAddressCity:        input.BillingAddressCity,
		BillingAddressState:       input.BillingAddressState,
		BillingAddressCountry:     input.BillingAddressCountry,
		BillingAddressPostalCode:  input.BillingAddressPostalCode,
		ShippingAddressStreet:     input.ShippingAddressStreet,
		ShippingAddressCity:       input.ShippingAddressCity,
		ShippingAddressState:      input.ShippingAddressState,
		ShippingAddressCountry:    input.ShippingAddressCountry,
		ShippingAddressPostalCode: input.ShippingAddressPostalCode,
		Description:               input.Description,
		Terms:                     input.Terms,
		Notes:                     input.Notes,
		AssignedUserID:            input.AssignedUserID,
		CreatedByID:               &userID,
		ModifiedByID:              &userID,
		CreatedAt:                 now,
		ModifiedAt:                now,
		CustomFields:              input.CustomFields,
	}

	// Build line items
	var lineItems []entity.QuoteLineItem
	for i, li := range input.LineItems {
		total := calcLineItemTotal(li.Quantity, li.UnitPrice, li.DiscountPercent, li.DiscountAmount)
		lineItems = append(lineItems, entity.QuoteLineItem{
			ID:              sfid.NewQuoteLineItem(),
			OrgID:           orgID,
			QuoteID:         quote.ID,
			Name:            li.Name,
			Description:     li.Description,
			SKU:             li.SKU,
			Quantity:        li.Quantity,
			UnitPrice:       li.UnitPrice,
			DiscountPercent: li.DiscountPercent,
			DiscountAmount:  li.DiscountAmount,
			TaxPercent:      li.TaxPercent,
			Total:           total,
			SortOrder:       i,
			CreatedAt:       nowStr,
			ModifiedAt:      nowStr,
		})
	}

	// Calculate totals
	quote.Subtotal, quote.TaxAmount, quote.GrandTotal = recalcTotals(
		lineItems, quote.DiscountPercent, quote.DiscountAmount, quote.TaxPercent, quote.ShippingAmount)

	// Serialize custom fields
	customFieldsJSON := "{}"
	if quote.CustomFields != nil {
		if b, err := json.Marshal(quote.CustomFields); err == nil {
			customFieldsJSON = string(b)
		}
	}

	// Insert quote
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO quotes (
			id, org_id, name, quote_number, status,
			account_id, account_name, contact_id, contact_name, valid_until,
			subtotal, discount_percent, discount_amount, tax_percent, tax_amount,
			shipping_amount, grand_total, currency,
			billing_address_street, billing_address_city, billing_address_state,
			billing_address_country, billing_address_postal_code,
			shipping_address_street, shipping_address_city, shipping_address_state,
			shipping_address_country, shipping_address_postal_code,
			description, terms, notes, assigned_user_id,
			created_by_id, modified_by_id, created_at, modified_at, deleted, custom_fields
		) VALUES (?,?,?,?,?, ?,?,?,?,?, ?,?,?,?,?, ?,?,?, ?,?,?, ?,?, ?,?,?, ?,?, ?,?,?,?, ?,?,?,?,?,?)
	`,
		quote.ID, quote.OrgID, quote.Name, quote.QuoteNumber, quote.Status,
		quote.AccountID, quote.AccountName, quote.ContactID, quote.ContactName, quote.ValidUntil,
		quote.Subtotal, quote.DiscountPercent, quote.DiscountAmount, quote.TaxPercent, quote.TaxAmount,
		quote.ShippingAmount, quote.GrandTotal, quote.Currency,
		quote.BillingAddressStreet, quote.BillingAddressCity, quote.BillingAddressState,
		quote.BillingAddressCountry, quote.BillingAddressPostalCode,
		quote.ShippingAddressStreet, quote.ShippingAddressCity, quote.ShippingAddressState,
		quote.ShippingAddressCountry, quote.ShippingAddressPostalCode,
		quote.Description, quote.Terms, quote.Notes, quote.AssignedUserID,
		quote.CreatedByID, quote.ModifiedByID, nowStr, nowStr, 0, customFieldsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create quote: %w", err)
	}

	// Insert line items
	for _, li := range lineItems {
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO quote_line_items (
				id, org_id, quote_id, name, description, sku,
				quantity, unit_price, discount_percent, discount_amount,
				tax_percent, total, sort_order, created_at, modified_at
			) VALUES (?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?)
		`,
			li.ID, li.OrgID, li.QuoteID, li.Name, li.Description, li.SKU,
			li.Quantity, li.UnitPrice, li.DiscountPercent, li.DiscountAmount,
			li.TaxPercent, li.Total, li.SortOrder, li.CreatedAt, li.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create line item: %w", err)
		}
	}

	quote.LineItems = lineItems
	return quote, nil
}

// GetByID retrieves a quote by ID with line items
func (r *QuoteRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Quote, error) {
	// Note: No user join - tenant DBs don't have users table
	// Use COALESCE for nullable string fields to avoid scan errors
	query := `
		SELECT q.id, q.org_id, COALESCE(q.name, ''), COALESCE(q.quote_number, ''), COALESCE(q.status, ''),
			q.account_id, COALESCE(q.account_name, ''), q.contact_id, COALESCE(q.contact_name, ''), COALESCE(q.valid_until, ''),
			COALESCE(q.subtotal, 0), COALESCE(q.discount_percent, 0), COALESCE(q.discount_amount, 0), COALESCE(q.tax_percent, 0), COALESCE(q.tax_amount, 0),
			COALESCE(q.shipping_amount, 0), COALESCE(q.grand_total, 0), COALESCE(q.currency, 'USD'),
			COALESCE(q.billing_address_street, ''), COALESCE(q.billing_address_city, ''), COALESCE(q.billing_address_state, ''),
			COALESCE(q.billing_address_country, ''), COALESCE(q.billing_address_postal_code, ''),
			COALESCE(q.shipping_address_street, ''), COALESCE(q.shipping_address_city, ''), COALESCE(q.shipping_address_state, ''),
			COALESCE(q.shipping_address_country, ''), COALESCE(q.shipping_address_postal_code, ''),
			COALESCE(q.description, ''), COALESCE(q.terms, ''), COALESCE(q.notes, ''), q.assigned_user_id,
			q.created_by_id, q.modified_by_id, q.created_at, q.modified_at, q.deleted,
			COALESCE(q.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		FROM quotes q
		WHERE q.id = ? AND q.org_id = ? AND q.deleted = 0
	`

	var q entity.Quote
	var createdAt, modifiedAt, customFieldsJSON string

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&q.ID, &q.OrgID, &q.Name, &q.QuoteNumber, &q.Status,
		&q.AccountID, &q.AccountName, &q.ContactID, &q.ContactName, &q.ValidUntil,
		&q.Subtotal, &q.DiscountPercent, &q.DiscountAmount, &q.TaxPercent, &q.TaxAmount,
		&q.ShippingAmount, &q.GrandTotal, &q.Currency,
		&q.BillingAddressStreet, &q.BillingAddressCity, &q.BillingAddressState,
		&q.BillingAddressCountry, &q.BillingAddressPostalCode,
		&q.ShippingAddressStreet, &q.ShippingAddressCity, &q.ShippingAddressState,
		&q.ShippingAddressCountry, &q.ShippingAddressPostalCode,
		&q.Description, &q.Terms, &q.Notes, &q.AssignedUserID,
		&q.CreatedByID, &q.ModifiedByID, &createdAt, &modifiedAt, &q.Deleted,
		&customFieldsJSON,
		&q.CreatedByName, &q.ModifiedByName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	// Try parsing as RFC3339 first, then fall back to SQLite format (YYYY-MM-DD HH:MM:SS)
	createdT, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		createdT, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	q.CreatedAt = createdT

	modifiedT, err := time.Parse(time.RFC3339, modifiedAt)
	if err != nil {
		modifiedT, _ = time.Parse("2006-01-02 15:04:05", modifiedAt)
	}
	q.ModifiedAt = modifiedT
	q.CustomFields = make(map[string]interface{})
	json.Unmarshal([]byte(customFieldsJSON), &q.CustomFields)

	// Fetch line items
	q.LineItems, _ = r.getLineItems(ctx, q.ID)

	return &q, nil
}

// getLineItems fetches all line items for a quote
func (r *QuoteRepo) getLineItems(ctx context.Context, quoteID string) ([]entity.QuoteLineItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, org_id, quote_id, name, description, sku,
			quantity, unit_price, discount_percent, discount_amount,
			tax_percent, total, sort_order, created_at, modified_at
		FROM quote_line_items
		WHERE quote_id = ?
		ORDER BY sort_order ASC
	`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []entity.QuoteLineItem
	for rows.Next() {
		var li entity.QuoteLineItem
		if err := rows.Scan(
			&li.ID, &li.OrgID, &li.QuoteID, &li.Name, &li.Description, &li.SKU,
			&li.Quantity, &li.UnitPrice, &li.DiscountPercent, &li.DiscountAmount,
			&li.TaxPercent, &li.Total, &li.SortOrder, &li.CreatedAt, &li.ModifiedAt,
		); err != nil {
			continue
		}
		items = append(items, li)
	}
	if items == nil {
		items = []entity.QuoteLineItem{}
	}
	return items, nil
}

// ListByOrg retrieves quotes with pagination and search
func (r *QuoteRepo) ListByOrg(ctx context.Context, orgID string, params entity.QuoteListParams) (*entity.QuoteListResponse, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "created_at"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	validSortCols := map[string]bool{
		"created_at": true, "modified_at": true, "name": true,
		"quote_number": true, "status": true, "grand_total": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "created_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Note: we don't join with users table since tenant DBs don't have it
	baseQuery := `FROM quotes q WHERE q.org_id = ? AND q.deleted = 0`
	args := []any{orgID}

	if params.Search != "" {
		baseQuery += ` AND (q.name LIKE ? OR q.quote_number LIKE ? OR q.account_name LIKE ?)`
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if params.Filter != "" {
		validColumns := map[string]bool{
			"name": true, "quote_number": true, "status": true,
			"account_id": true, "contact_id": true, "grand_total": true,
			"valid_until": true, "assigned_user_id": true,
			"created_at": true, "modified_at": true,
		}
		filterResult, err := util.ParseFilterWithColumns(params.Filter, validColumns, "q")
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		if filterResult != nil && filterResult.WhereClause != "" {
			baseQuery += " AND " + filterResult.WhereClause
			args = append(args, filterResult.Args...)
		}
	}

	var total int
	if params.KnownTotal > 0 {
		total = params.KnownTotal
	} else {
		countQuery := "SELECT COUNT(*) " + baseQuery
		if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return nil, fmt.Errorf("failed to count quotes: %w", err)
		}
	}

	// Note: CreatedByName and ModifiedByName are empty - user names should be stored when creating/updating
	// Use COALESCE for nullable string fields to avoid scan errors
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT q.id, q.org_id, COALESCE(q.name, ''), COALESCE(q.quote_number, ''), COALESCE(q.status, ''),
			q.account_id, COALESCE(q.account_name, ''), q.contact_id, COALESCE(q.contact_name, ''), COALESCE(q.valid_until, ''),
			COALESCE(q.subtotal, 0), COALESCE(q.discount_percent, 0), COALESCE(q.discount_amount, 0), COALESCE(q.tax_percent, 0), COALESCE(q.tax_amount, 0),
			COALESCE(q.shipping_amount, 0), COALESCE(q.grand_total, 0), COALESCE(q.currency, 'USD'),
			COALESCE(q.billing_address_street, ''), COALESCE(q.billing_address_city, ''), COALESCE(q.billing_address_state, ''),
			COALESCE(q.billing_address_country, ''), COALESCE(q.billing_address_postal_code, ''),
			COALESCE(q.shipping_address_street, ''), COALESCE(q.shipping_address_city, ''), COALESCE(q.shipping_address_state, ''),
			COALESCE(q.shipping_address_country, ''), COALESCE(q.shipping_address_postal_code, ''),
			COALESCE(q.description, ''), COALESCE(q.terms, ''), COALESCE(q.notes, ''), q.assigned_user_id,
			q.created_by_id, q.modified_by_id, q.created_at, q.modified_at, q.deleted,
			COALESCE(q.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		%s ORDER BY q.%s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list quotes: %w", err)
	}
	defer rows.Close()

	var quotes []entity.Quote
	for rows.Next() {
		var q entity.Quote
		var createdAt, modifiedAt, customFieldsJSON string

		if err := rows.Scan(
			&q.ID, &q.OrgID, &q.Name, &q.QuoteNumber, &q.Status,
			&q.AccountID, &q.AccountName, &q.ContactID, &q.ContactName, &q.ValidUntil,
			&q.Subtotal, &q.DiscountPercent, &q.DiscountAmount, &q.TaxPercent, &q.TaxAmount,
			&q.ShippingAmount, &q.GrandTotal, &q.Currency,
			&q.BillingAddressStreet, &q.BillingAddressCity, &q.BillingAddressState,
			&q.BillingAddressCountry, &q.BillingAddressPostalCode,
			&q.ShippingAddressStreet, &q.ShippingAddressCity, &q.ShippingAddressState,
			&q.ShippingAddressCountry, &q.ShippingAddressPostalCode,
			&q.Description, &q.Terms, &q.Notes, &q.AssignedUserID,
			&q.CreatedByID, &q.ModifiedByID, &createdAt, &modifiedAt, &q.Deleted,
			&customFieldsJSON,
			&q.CreatedByName, &q.ModifiedByName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan quote: %w", err)
		}

		// Try parsing as RFC3339 first, then fall back to SQLite format (YYYY-MM-DD HH:MM:SS)
		createdT, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			createdT, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		q.CreatedAt = createdT

		modifiedT, err := time.Parse(time.RFC3339, modifiedAt)
		if err != nil {
			modifiedT, _ = time.Parse("2006-01-02 15:04:05", modifiedAt)
		}
		q.ModifiedAt = modifiedT

		q.CustomFields = make(map[string]interface{})
		json.Unmarshal([]byte(customFieldsJSON), &q.CustomFields)
		quotes = append(quotes, q)
	}

	if quotes == nil {
		quotes = []entity.Quote{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.QuoteListResponse{
		Data:       quotes,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// Update updates an existing quote
func (r *QuoteRepo) Update(ctx context.Context, orgID, id string, input entity.QuoteUpdateInput, userID string) (*entity.Quote, error) {
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
	if input.Status != nil {
		existing.Status = *input.Status
	}
	if input.AccountID != nil {
		existing.AccountID = input.AccountID
	}
	if input.AccountName != nil {
		existing.AccountName = *input.AccountName
	}
	if input.ContactID != nil {
		existing.ContactID = input.ContactID
	}
	if input.ContactName != nil {
		existing.ContactName = *input.ContactName
	}
	if input.ValidUntil != nil {
		existing.ValidUntil = *input.ValidUntil
	}
	if input.DiscountPercent != nil {
		existing.DiscountPercent = *input.DiscountPercent
	}
	if input.DiscountAmount != nil {
		existing.DiscountAmount = *input.DiscountAmount
	}
	if input.TaxPercent != nil {
		existing.TaxPercent = *input.TaxPercent
	}
	if input.ShippingAmount != nil {
		existing.ShippingAmount = *input.ShippingAmount
	}
	if input.Currency != nil {
		existing.Currency = *input.Currency
	}
	if input.BillingAddressStreet != nil {
		existing.BillingAddressStreet = *input.BillingAddressStreet
	}
	if input.BillingAddressCity != nil {
		existing.BillingAddressCity = *input.BillingAddressCity
	}
	if input.BillingAddressState != nil {
		existing.BillingAddressState = *input.BillingAddressState
	}
	if input.BillingAddressCountry != nil {
		existing.BillingAddressCountry = *input.BillingAddressCountry
	}
	if input.BillingAddressPostalCode != nil {
		existing.BillingAddressPostalCode = *input.BillingAddressPostalCode
	}
	if input.ShippingAddressStreet != nil {
		existing.ShippingAddressStreet = *input.ShippingAddressStreet
	}
	if input.ShippingAddressCity != nil {
		existing.ShippingAddressCity = *input.ShippingAddressCity
	}
	if input.ShippingAddressState != nil {
		existing.ShippingAddressState = *input.ShippingAddressState
	}
	if input.ShippingAddressCountry != nil {
		existing.ShippingAddressCountry = *input.ShippingAddressCountry
	}
	if input.ShippingAddressPostalCode != nil {
		existing.ShippingAddressPostalCode = *input.ShippingAddressPostalCode
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.Terms != nil {
		existing.Terms = *input.Terms
	}
	if input.Notes != nil {
		existing.Notes = *input.Notes
	}
	if input.AssignedUserID != nil {
		existing.AssignedUserID = input.AssignedUserID
	}

	if input.CustomFields != nil {
		if existing.CustomFields == nil {
			existing.CustomFields = make(map[string]interface{})
		}
		for k, v := range input.CustomFields {
			existing.CustomFields[k] = v
		}
	}

	// If line items provided, replace them all
	if input.LineItems != nil {
		nowStr := time.Now().UTC().Format(time.RFC3339)
		// Delete existing
		r.db.ExecContext(ctx, `DELETE FROM quote_line_items WHERE quote_id = ?`, id)

		var lineItems []entity.QuoteLineItem
		for i, li := range input.LineItems {
			total := calcLineItemTotal(li.Quantity, li.UnitPrice, li.DiscountPercent, li.DiscountAmount)
			item := entity.QuoteLineItem{
				ID:              sfid.NewQuoteLineItem(),
				OrgID:           orgID,
				QuoteID:         id,
				Name:            li.Name,
				Description:     li.Description,
				SKU:             li.SKU,
				Quantity:        li.Quantity,
				UnitPrice:       li.UnitPrice,
				DiscountPercent: li.DiscountPercent,
				DiscountAmount:  li.DiscountAmount,
				TaxPercent:      li.TaxPercent,
				Total:           total,
				SortOrder:       i,
				CreatedAt:       nowStr,
				ModifiedAt:      nowStr,
			}
			lineItems = append(lineItems, item)

			_, err := r.db.ExecContext(ctx, `
				INSERT INTO quote_line_items (
					id, org_id, quote_id, name, description, sku,
					quantity, unit_price, discount_percent, discount_amount,
					tax_percent, total, sort_order, created_at, modified_at
				) VALUES (?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?)
			`,
				item.ID, item.OrgID, item.QuoteID, item.Name, item.Description, item.SKU,
				item.Quantity, item.UnitPrice, item.DiscountPercent, item.DiscountAmount,
				item.TaxPercent, item.Total, item.SortOrder, item.CreatedAt, item.ModifiedAt,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to save line item: %w", err)
			}
		}
		existing.LineItems = lineItems

		// Recalculate totals
		existing.Subtotal, existing.TaxAmount, existing.GrandTotal = recalcTotals(
			lineItems, existing.DiscountPercent, existing.DiscountAmount, existing.TaxPercent, existing.ShippingAmount)
	}

	existing.ModifiedByID = &userID
	existing.ModifiedAt = time.Now().UTC()

	customFieldsJSON := "{}"
	if existing.CustomFields != nil {
		if b, err := json.Marshal(existing.CustomFields); err == nil {
			customFieldsJSON = string(b)
		}
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE quotes SET
			name = ?, status = ?, account_id = ?, account_name = ?,
			contact_id = ?, contact_name = ?, valid_until = ?,
			subtotal = ?, discount_percent = ?, discount_amount = ?,
			tax_percent = ?, tax_amount = ?, shipping_amount = ?, grand_total = ?, currency = ?,
			billing_address_street = ?, billing_address_city = ?, billing_address_state = ?,
			billing_address_country = ?, billing_address_postal_code = ?,
			shipping_address_street = ?, shipping_address_city = ?, shipping_address_state = ?,
			shipping_address_country = ?, shipping_address_postal_code = ?,
			description = ?, terms = ?, notes = ?, assigned_user_id = ?,
			modified_by_id = ?, modified_at = ?, custom_fields = ?
		WHERE id = ? AND org_id = ? AND deleted = 0
	`,
		existing.Name, existing.Status, existing.AccountID, existing.AccountName,
		existing.ContactID, existing.ContactName, existing.ValidUntil,
		existing.Subtotal, existing.DiscountPercent, existing.DiscountAmount,
		existing.TaxPercent, existing.TaxAmount, existing.ShippingAmount, existing.GrandTotal, existing.Currency,
		existing.BillingAddressStreet, existing.BillingAddressCity, existing.BillingAddressState,
		existing.BillingAddressCountry, existing.BillingAddressPostalCode,
		existing.ShippingAddressStreet, existing.ShippingAddressCity, existing.ShippingAddressState,
		existing.ShippingAddressCountry, existing.ShippingAddressPostalCode,
		existing.Description, existing.Terms, existing.Notes, existing.AssignedUserID,
		existing.ModifiedByID, existing.ModifiedAt.Format(time.RFC3339), customFieldsJSON,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update quote: %w", err)
	}

	return existing, nil
}

// SaveLineItems replaces all line items for a quote and recalculates totals
func (r *QuoteRepo) SaveLineItems(ctx context.Context, orgID, quoteID string, items []entity.QuoteLineItemInput, userID string) (*entity.Quote, error) {
	existing, err := r.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	// Delete existing line items
	r.db.ExecContext(ctx, `DELETE FROM quote_line_items WHERE quote_id = ?`, quoteID)

	var lineItems []entity.QuoteLineItem
	for i, li := range items {
		total := calcLineItemTotal(li.Quantity, li.UnitPrice, li.DiscountPercent, li.DiscountAmount)
		item := entity.QuoteLineItem{
			ID:              sfid.NewQuoteLineItem(),
			OrgID:           orgID,
			QuoteID:         quoteID,
			Name:            li.Name,
			Description:     li.Description,
			SKU:             li.SKU,
			Quantity:        li.Quantity,
			UnitPrice:       li.UnitPrice,
			DiscountPercent: li.DiscountPercent,
			DiscountAmount:  li.DiscountAmount,
			TaxPercent:      li.TaxPercent,
			Total:           total,
			SortOrder:       i,
			CreatedAt:       nowStr,
			ModifiedAt:      nowStr,
		}
		lineItems = append(lineItems, item)

		_, err := r.db.ExecContext(ctx, `
			INSERT INTO quote_line_items (
				id, org_id, quote_id, name, description, sku,
				quantity, unit_price, discount_percent, discount_amount,
				tax_percent, total, sort_order, created_at, modified_at
			) VALUES (?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?)
		`,
			item.ID, item.OrgID, item.QuoteID, item.Name, item.Description, item.SKU,
			item.Quantity, item.UnitPrice, item.DiscountPercent, item.DiscountAmount,
			item.TaxPercent, item.Total, item.SortOrder, item.CreatedAt, item.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to save line item: %w", err)
		}
	}

	// Recalculate totals
	existing.Subtotal, existing.TaxAmount, existing.GrandTotal = recalcTotals(
		lineItems, existing.DiscountPercent, existing.DiscountAmount, existing.TaxPercent, existing.ShippingAmount)
	existing.LineItems = lineItems
	existing.ModifiedByID = &userID
	existing.ModifiedAt = time.Now().UTC()

	// Update quote totals
	_, err = r.db.ExecContext(ctx, `
		UPDATE quotes SET subtotal = ?, tax_amount = ?, grand_total = ?,
			modified_by_id = ?, modified_at = ?
		WHERE id = ? AND org_id = ? AND deleted = 0
	`,
		existing.Subtotal, existing.TaxAmount, existing.GrandTotal,
		existing.ModifiedByID, existing.ModifiedAt.Format(time.RFC3339),
		quoteID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update quote totals: %w", err)
	}

	return existing, nil
}

// Delete soft-deletes a quote
func (r *QuoteRepo) Delete(ctx context.Context, orgID, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE quotes SET deleted = 1, modified_at = ? WHERE id = ? AND org_id = ? AND deleted = 0`,
		time.Now().UTC().Format(time.RFC3339), id, orgID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete quote: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
