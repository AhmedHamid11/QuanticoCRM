package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// AccountRepo handles database operations for accounts
type AccountRepo struct {
	db db.DBConn
}

// NewAccountRepo creates a new AccountRepo
func NewAccountRepo(conn db.DBConn) *AccountRepo {
	return &AccountRepo{db: conn}
}

// WithDB returns a new AccountRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *AccountRepo) WithDB(conn db.DBConn) *AccountRepo {
	if conn == nil {
		return r
	}
	return &AccountRepo{db: conn}
}

// DB returns the current database connection
func (r *AccountRepo) DB() db.DBConn {
	return r.db
}

// Create inserts a new account into the database
func (r *AccountRepo) Create(ctx context.Context, orgID string, input entity.AccountCreateInput, userID string) (*entity.Account, error) {
	account := &entity.Account{
		ID:                        sfid.NewAccount(),
		OrgID:                     orgID,
		Name:                      input.Name,
		Website:                   input.Website,
		EmailAddress:              input.EmailAddress,
		PhoneNumber:               input.PhoneNumber,
		Type:                      input.Type,
		Industry:                  input.Industry,
		SicCode:                   input.SicCode,
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
		AssignedUserID:            input.AssignedUserID,
		CreatedByID:               &userID,
		ModifiedByID:              &userID,
		CreatedAt:                 time.Now().UTC(),
		ModifiedAt:                time.Now().UTC(),
		Deleted:                   false,
		CustomFields:              input.CustomFields,
	}

	// Serialize custom fields to JSON
	customFieldsJSON := "{}"
	if account.CustomFields != nil {
		if jsonBytes, err := json.Marshal(account.CustomFields); err == nil {
			customFieldsJSON = string(jsonBytes)
		}
	}

	query := `
		INSERT INTO accounts (
			id, org_id, name, website, email_address, phone_number,
			type, industry, sic_code,
			billing_address_street, billing_address_city, billing_address_state,
			billing_address_country, billing_address_postal_code,
			shipping_address_street, shipping_address_city, shipping_address_state,
			shipping_address_country, shipping_address_postal_code,
			description, assigned_user_id, created_by_id, modified_by_id,
			created_at, modified_at, deleted, custom_fields
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		account.ID, account.OrgID, account.Name, account.Website, account.EmailAddress, account.PhoneNumber,
		account.Type, account.Industry, account.SicCode,
		account.BillingAddressStreet, account.BillingAddressCity, account.BillingAddressState,
		account.BillingAddressCountry, account.BillingAddressPostalCode,
		account.ShippingAddressStreet, account.ShippingAddressCity, account.ShippingAddressState,
		account.ShippingAddressCountry, account.ShippingAddressPostalCode,
		account.Description, account.AssignedUserID, account.CreatedByID, account.ModifiedByID,
		account.CreatedAt.Format(time.RFC3339), account.ModifiedAt.Format(time.RFC3339), account.Deleted, customFieldsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	return account, nil
}

// GetByID retrieves an account by its ID
func (r *AccountRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Account, error) {
	// Note: No user join - tenant DBs don't have users table
	// Use COALESCE for nullable string fields to avoid scan errors
	query := `
		SELECT a.id, a.org_id, COALESCE(a.name, ''), COALESCE(a.website, ''), COALESCE(a.email_address, ''), COALESCE(a.phone_number, ''),
			COALESCE(a.type, ''), COALESCE(a.industry, ''), COALESCE(a.sic_code, ''),
			COALESCE(a.billing_address_street, ''), COALESCE(a.billing_address_city, ''), COALESCE(a.billing_address_state, ''),
			COALESCE(a.billing_address_country, ''), COALESCE(a.billing_address_postal_code, ''),
			COALESCE(a.shipping_address_street, ''), COALESCE(a.shipping_address_city, ''), COALESCE(a.shipping_address_state, ''),
			COALESCE(a.shipping_address_country, ''), COALESCE(a.shipping_address_postal_code, ''),
			COALESCE(a.description, ''), COALESCE(a.stage, ''), COALESCE(a.assigned_user_id, ''), COALESCE(a.created_by_id, ''), COALESCE(a.modified_by_id, ''),
			COALESCE(a.created_at, ''), COALESCE(a.modified_at, ''), COALESCE(a.deleted, 0), COALESCE(a.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		FROM accounts a
		WHERE a.id = ? AND a.org_id = ? AND a.deleted = 0
	`

	var account entity.Account
	var createdAt, modifiedAt, customFieldsJSON string

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&account.ID, &account.OrgID, &account.Name, &account.Website, &account.EmailAddress, &account.PhoneNumber,
		&account.Type, &account.Industry, &account.SicCode,
		&account.BillingAddressStreet, &account.BillingAddressCity, &account.BillingAddressState,
		&account.BillingAddressCountry, &account.BillingAddressPostalCode,
		&account.ShippingAddressStreet, &account.ShippingAddressCity, &account.ShippingAddressState,
		&account.ShippingAddressCountry, &account.ShippingAddressPostalCode,
		&account.Description, &account.Stage, &account.AssignedUserID, &account.CreatedByID, &account.ModifiedByID,
		&createdAt, &modifiedAt, &account.Deleted, &customFieldsJSON,
		&account.CreatedByName, &account.ModifiedByName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	account.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	account.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Parse custom fields
	account.CustomFields = make(map[string]interface{})
	json.Unmarshal([]byte(customFieldsJSON), &account.CustomFields)

	return &account, nil
}

// ListByOrg retrieves all accounts for an organization with pagination and search
func (r *AccountRepo) ListByOrg(ctx context.Context, orgID string, params entity.AccountListParams) (*entity.AccountListResponse, error) {
	// Set defaults
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

	// Validate sort column
	validSortCols := map[string]bool{
		"created_at": true, "modified_at": true, "name": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "created_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Build query - note: we don't join with users table since tenant DBs don't have it
	baseQuery := `FROM accounts a WHERE a.org_id = ? AND a.deleted = 0`
	args := []any{orgID}

	if params.Search != "" {
		baseQuery += ` AND (a.name LIKE ? OR a.email_address LIKE ? OR a.website LIKE ?)`
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// Apply filter if provided
	if params.Filter != "" {
		// Define valid columns for Account entity (using snake_case column names)
		validColumns := map[string]bool{
			"name": true, "website": true, "email_address": true, "phone_number": true,
			"type": true, "industry": true, "sic_code": true, "stage": true,
			"billing_address_street": true, "billing_address_city": true, "billing_address_state": true,
			"billing_address_country": true, "billing_address_postal_code": true,
			"shipping_address_street": true, "shipping_address_city": true, "shipping_address_state": true,
			"shipping_address_country": true, "shipping_address_postal_code": true,
			"description": true, "assigned_user_id": true, "created_at": true, "modified_at": true,
		}
		filterResult, err := util.ParseFilterWithColumns(params.Filter, validColumns, "a")
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		if filterResult != nil && filterResult.WhereClause != "" {
			baseQuery += " AND " + filterResult.WhereClause
			args = append(args, filterResult.Args...)
		}
	}

	// Skip COUNT(*) if the frontend already knows the total (saves row reads on Turso)
	var total int
	if params.KnownTotal > 0 {
		total = params.KnownTotal
	} else {
		countQuery := "SELECT COUNT(*) " + baseQuery
		if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return nil, fmt.Errorf("failed to count accounts: %w", err)
		}
	}

	// Query with pagination
	// Note: CreatedByName and ModifiedByName are empty - user names should be stored when creating/updating
	// Use COALESCE for nullable string fields to avoid scan errors
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT a.id, a.org_id, COALESCE(a.name, ''), COALESCE(a.website, ''), COALESCE(a.email_address, ''), COALESCE(a.phone_number, ''),
			COALESCE(a.type, ''), COALESCE(a.industry, ''), COALESCE(a.sic_code, ''),
			COALESCE(a.billing_address_street, ''), COALESCE(a.billing_address_city, ''), COALESCE(a.billing_address_state, ''),
			COALESCE(a.billing_address_country, ''), COALESCE(a.billing_address_postal_code, ''),
			COALESCE(a.shipping_address_street, ''), COALESCE(a.shipping_address_city, ''), COALESCE(a.shipping_address_state, ''),
			COALESCE(a.shipping_address_country, ''), COALESCE(a.shipping_address_postal_code, ''),
			COALESCE(a.description, ''), COALESCE(a.stage, ''), COALESCE(a.assigned_user_id, ''), COALESCE(a.created_by_id, ''), COALESCE(a.modified_by_id, ''),
			COALESCE(a.created_at, ''), COALESCE(a.modified_at, ''), COALESCE(a.deleted, 0), COALESCE(a.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		%s ORDER BY a.%s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []entity.Account
	for rows.Next() {
		var account entity.Account
		var createdAt, modifiedAt, customFieldsJSON string

		if err := rows.Scan(
			&account.ID, &account.OrgID, &account.Name, &account.Website, &account.EmailAddress, &account.PhoneNumber,
			&account.Type, &account.Industry, &account.SicCode,
			&account.BillingAddressStreet, &account.BillingAddressCity, &account.BillingAddressState,
			&account.BillingAddressCountry, &account.BillingAddressPostalCode,
			&account.ShippingAddressStreet, &account.ShippingAddressCity, &account.ShippingAddressState,
			&account.ShippingAddressCountry, &account.ShippingAddressPostalCode,
			&account.Description, &account.Stage, &account.AssignedUserID, &account.CreatedByID, &account.ModifiedByID,
			&createdAt, &modifiedAt, &account.Deleted, &customFieldsJSON,
			&account.CreatedByName, &account.ModifiedByName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}

		account.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		account.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		account.CustomFields = make(map[string]interface{})
		json.Unmarshal([]byte(customFieldsJSON), &account.CustomFields)
		accounts = append(accounts, account)
	}

	if accounts == nil {
		accounts = []entity.Account{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.AccountListResponse{
		Data:       accounts,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// Update updates an existing account
func (r *AccountRepo) Update(ctx context.Context, orgID, id string, input entity.AccountUpdateInput, userID string) (*entity.Account, error) {
	// First get the existing account
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Website != nil {
		existing.Website = *input.Website
	}
	if input.EmailAddress != nil {
		existing.EmailAddress = *input.EmailAddress
	}
	if input.PhoneNumber != nil {
		existing.PhoneNumber = *input.PhoneNumber
	}
	if input.Type != nil {
		existing.Type = *input.Type
	}
	if input.Industry != nil {
		existing.Industry = *input.Industry
	}
	if input.SicCode != nil {
		existing.SicCode = *input.SicCode
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
	if input.Stage != nil {
		existing.Stage = input.Stage
	}
	if input.AssignedUserID != nil {
		existing.AssignedUserID = input.AssignedUserID
	}

	// Merge custom fields
	if input.CustomFields != nil {
		if existing.CustomFields == nil {
			existing.CustomFields = make(map[string]interface{})
		}
		for k, v := range input.CustomFields {
			existing.CustomFields[k] = v
		}
	}

	existing.ModifiedByID = &userID
	existing.ModifiedAt = time.Now().UTC()

	// Serialize custom fields
	customFieldsJSON := "{}"
	if existing.CustomFields != nil {
		if jsonBytes, err := json.Marshal(existing.CustomFields); err == nil {
			customFieldsJSON = string(jsonBytes)
		}
	}

	query := `
		UPDATE accounts SET
			name = ?, website = ?, email_address = ?, phone_number = ?,
			type = ?, industry = ?, sic_code = ?,
			billing_address_street = ?, billing_address_city = ?, billing_address_state = ?,
			billing_address_country = ?, billing_address_postal_code = ?,
			shipping_address_street = ?, shipping_address_city = ?, shipping_address_state = ?,
			shipping_address_country = ?, shipping_address_postal_code = ?,
			description = ?, stage = ?, assigned_user_id = ?, modified_by_id = ?, modified_at = ?, custom_fields = ?
		WHERE id = ? AND org_id = ? AND deleted = 0
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Name, existing.Website, existing.EmailAddress, existing.PhoneNumber,
		existing.Type, existing.Industry, existing.SicCode,
		existing.BillingAddressStreet, existing.BillingAddressCity, existing.BillingAddressState,
		existing.BillingAddressCountry, existing.BillingAddressPostalCode,
		existing.ShippingAddressStreet, existing.ShippingAddressCity, existing.ShippingAddressState,
		existing.ShippingAddressCountry, existing.ShippingAddressPostalCode,
		existing.Description, existing.Stage, existing.AssignedUserID, existing.ModifiedByID, existing.ModifiedAt.Format(time.RFC3339), customFieldsJSON,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	return existing, nil
}

// Delete soft-deletes an account
func (r *AccountRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `UPDATE accounts SET deleted = 1, modified_at = ? WHERE id = ? AND org_id = ? AND deleted = 0`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339), id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
