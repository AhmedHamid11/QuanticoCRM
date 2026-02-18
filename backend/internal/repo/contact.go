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

// ContactRepo handles database operations for contacts
type ContactRepo struct {
	db db.DBConn
}

// NewContactRepo creates a new ContactRepo
func NewContactRepo(conn db.DBConn) *ContactRepo {
	return &ContactRepo{db: conn}
}

// WithDB returns a new ContactRepo using the specified database connection
// This is used for multi-tenant database routing
// Accepts db.DBConn interface for retry-enabled connections
func (r *ContactRepo) WithDB(conn db.DBConn) *ContactRepo {
	if conn == nil {
		return r
	}
	return &ContactRepo{db: conn}
}

// DB returns the current database connection
func (r *ContactRepo) DB() db.DBConn {
	return r.db
}

// Create inserts a new contact into the database
func (r *ContactRepo) Create(ctx context.Context, orgID string, input entity.ContactCreateInput, userID string) (*entity.Contact, error) {
	contact := &entity.Contact{
		ID:              sfid.NewContact(),
		OrgID:           orgID,
		SalutationName:  input.SalutationName,
		FirstName:       input.FirstName,
		LastName:        input.LastName,
		EmailAddress:    input.EmailAddress,
		PhoneNumber:     input.PhoneNumber,
		PhoneNumberType: input.PhoneNumberType,
		DoNotCall:       input.DoNotCall,
		Description:     input.Description,
		AddressStreet:   input.AddressStreet,
		AddressCity:     input.AddressCity,
		AddressState:    input.AddressState,
		AddressCountry:  input.AddressCountry,
		AddressPostal:   input.AddressPostal,
		AccountID:       input.AccountID,
		AccountName:     input.AccountName,
		AssignedUserID:  input.AssignedUserID,
		CreatedByID:     &userID,
		ModifiedByID:    &userID,
		CreatedAt:       time.Now().UTC(),
		ModifiedAt:      time.Now().UTC(),
		Deleted:         false,
		CustomFields:    input.CustomFields,
	}

	if contact.PhoneNumberType == "" {
		contact.PhoneNumberType = "Mobile"
	}

	// Serialize custom fields to JSON
	customFieldsJSON := "{}"
	if contact.CustomFields != nil {
		if jsonBytes, err := json.Marshal(contact.CustomFields); err == nil {
			customFieldsJSON = string(jsonBytes)
		}
	}

	query := `
		INSERT INTO contacts (
			id, org_id, salutation_name, first_name, last_name,
			email_address, phone_number, phone_number_type, do_not_call,
			description, address_street, address_city, address_state,
			address_country, address_postal_code, account_id, account_name, assigned_user_id,
			created_by_id, modified_by_id, created_at, modified_at, deleted, custom_fields
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		contact.ID, contact.OrgID, contact.SalutationName, contact.FirstName, contact.LastName,
		contact.EmailAddress, contact.PhoneNumber, contact.PhoneNumberType, contact.DoNotCall,
		contact.Description, contact.AddressStreet, contact.AddressCity, contact.AddressState,
		contact.AddressCountry, contact.AddressPostal, contact.AccountID, contact.AccountName, contact.AssignedUserID,
		contact.CreatedByID, contact.ModifiedByID, contact.CreatedAt.Format(time.RFC3339), contact.ModifiedAt.Format(time.RFC3339), contact.Deleted, customFieldsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	return contact, nil
}

// GetByID retrieves a contact by its ID
func (r *ContactRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Contact, error) {
	// Note: No user join - tenant DBs don't have users table
	// LEFT JOIN accounts to resolve account display name dynamically
	query := `
		SELECT c.id, c.org_id, COALESCE(c.salutation_name, ''), COALESCE(c.first_name, ''), COALESCE(c.last_name, ''),
			COALESCE(c.email_address, ''), COALESCE(c.phone_number, ''), COALESCE(c.phone_number_type, ''), COALESCE(c.do_not_call, 0),
			COALESCE(c.description, ''), COALESCE(c.address_street, ''), COALESCE(c.address_city, ''), COALESCE(c.address_state, ''),
			COALESCE(c.address_country, ''), COALESCE(c.address_postal_code, ''), COALESCE(c.account_id, ''), COALESCE(a.name, c.account_name, ''), COALESCE(c.assigned_user_id, ''),
			COALESCE(c.created_by_id, ''), COALESCE(c.modified_by_id, ''), c.created_at, c.modified_at, COALESCE(c.deleted, 0),
			COALESCE(c.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		FROM contacts c
		LEFT JOIN accounts a ON a.id = c.account_id AND a.org_id = c.org_id AND a.deleted = 0
		WHERE c.id = ? AND c.org_id = ? AND c.deleted = 0
	`

	var contact entity.Contact
	var createdAt, modifiedAt, customFieldsJSON string

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&contact.ID, &contact.OrgID, &contact.SalutationName, &contact.FirstName, &contact.LastName,
		&contact.EmailAddress, &contact.PhoneNumber, &contact.PhoneNumberType, &contact.DoNotCall,
		&contact.Description, &contact.AddressStreet, &contact.AddressCity, &contact.AddressState,
		&contact.AddressCountry, &contact.AddressPostal, &contact.AccountID, &contact.AccountName, &contact.AssignedUserID,
		&contact.CreatedByID, &contact.ModifiedByID, &createdAt, &modifiedAt, &contact.Deleted,
		&customFieldsJSON,
		&contact.CreatedByName, &contact.ModifiedByName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}

	contact.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	contact.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Parse custom fields
	contact.CustomFields = make(map[string]interface{})
	json.Unmarshal([]byte(customFieldsJSON), &contact.CustomFields)

	return &contact, nil
}

// ListByOrg retrieves all contacts for an organization with pagination and search
func (r *ContactRepo) ListByOrg(ctx context.Context, orgID string, params entity.ContactListParams) (*entity.ContactListResponse, error) {
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
		"created_at": true, "modified_at": true, "first_name": true,
		"last_name": true, "email_address": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "created_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Build query - LEFT JOIN accounts to resolve account display name dynamically
	baseQuery := `FROM contacts c LEFT JOIN accounts a ON a.id = c.account_id AND a.org_id = c.org_id AND a.deleted = 0 WHERE c.org_id = ? AND c.deleted = 0`
	args := []any{orgID}

	if params.Search != "" {
		baseQuery += ` AND (c.first_name LIKE ? OR c.last_name LIKE ? OR c.email_address LIKE ?)`
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// Apply filter if provided
	if params.Filter != "" {
		validColumns := map[string]bool{
			"salutation_name": true, "first_name": true, "last_name": true, "email_address": true,
			"phone_number": true, "mobile_phone": true, "do_not_call": true, "title": true,
			"department": true, "account_id": true, "description": true, "lead_source": true,
			"address_street": true, "address_city": true, "address_state": true,
			"address_country": true, "address_postal_code": true, "assigned_user_id": true,
			"created_at": true, "modified_at": true,
		}
		filterResult, err := util.ParseFilterWithColumns(params.Filter, validColumns, "c")
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
		// Use db.QueryRowScan for retry-enabled count query
		if err := db.QueryRowScan(ctx, r.db, []interface{}{&total}, countQuery, args...); err != nil {
			return nil, fmt.Errorf("failed to count contacts: %w", err)
		}
	}

	// Query with pagination
	// Note: CreatedByName and ModifiedByName are empty - user names should be stored when creating/updating
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT c.id, c.org_id, COALESCE(c.salutation_name, ''), COALESCE(c.first_name, ''), COALESCE(c.last_name, ''),
			COALESCE(c.email_address, ''), COALESCE(c.phone_number, ''), COALESCE(c.phone_number_type, ''), COALESCE(c.do_not_call, 0),
			COALESCE(c.description, ''), COALESCE(c.address_street, ''), COALESCE(c.address_city, ''), COALESCE(c.address_state, ''),
			COALESCE(c.address_country, ''), COALESCE(c.address_postal_code, ''), COALESCE(c.account_id, ''), COALESCE(a.name, c.account_name, ''), COALESCE(c.assigned_user_id, ''),
			COALESCE(c.created_by_id, ''), COALESCE(c.modified_by_id, ''), c.created_at, c.modified_at, COALESCE(c.deleted, 0),
			COALESCE(c.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		%s ORDER BY c.%s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []entity.Contact
	for rows.Next() {
		var contact entity.Contact
		var createdAt, modifiedAt, customFieldsJSON string

		if err := rows.Scan(
			&contact.ID, &contact.OrgID, &contact.SalutationName, &contact.FirstName, &contact.LastName,
			&contact.EmailAddress, &contact.PhoneNumber, &contact.PhoneNumberType, &contact.DoNotCall,
			&contact.Description, &contact.AddressStreet, &contact.AddressCity, &contact.AddressState,
			&contact.AddressCountry, &contact.AddressPostal, &contact.AccountID, &contact.AccountName, &contact.AssignedUserID,
			&contact.CreatedByID, &contact.ModifiedByID, &createdAt, &modifiedAt, &contact.Deleted,
			&customFieldsJSON,
			&contact.CreatedByName, &contact.ModifiedByName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}

		contact.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		contact.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		contact.CustomFields = make(map[string]interface{})
		json.Unmarshal([]byte(customFieldsJSON), &contact.CustomFields)
		contacts = append(contacts, contact)
	}

	if contacts == nil {
		contacts = []entity.Contact{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.ContactListResponse{
		Data:       contacts,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// Update updates an existing contact
func (r *ContactRepo) Update(ctx context.Context, orgID, id string, input entity.ContactUpdateInput, userID string) (*entity.Contact, error) {
	// First get the existing contact
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.SalutationName != nil {
		existing.SalutationName = *input.SalutationName
	}
	if input.FirstName != nil {
		existing.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		existing.LastName = *input.LastName
	}
	if input.EmailAddress != nil {
		existing.EmailAddress = *input.EmailAddress
	}
	if input.PhoneNumber != nil {
		existing.PhoneNumber = *input.PhoneNumber
	}
	if input.PhoneNumberType != nil {
		existing.PhoneNumberType = *input.PhoneNumberType
	}
	if input.DoNotCall != nil {
		existing.DoNotCall = *input.DoNotCall
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.AddressStreet != nil {
		existing.AddressStreet = *input.AddressStreet
	}
	if input.AddressCity != nil {
		existing.AddressCity = *input.AddressCity
	}
	if input.AddressState != nil {
		existing.AddressState = *input.AddressState
	}
	if input.AddressCountry != nil {
		existing.AddressCountry = *input.AddressCountry
	}
	if input.AddressPostal != nil {
		existing.AddressPostal = *input.AddressPostal
	}
	if input.AccountID != nil {
		existing.AccountID = input.AccountID
	}
	if input.AccountName != nil {
		existing.AccountName = *input.AccountName
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
		UPDATE contacts SET
			salutation_name = ?, first_name = ?, last_name = ?,
			email_address = ?, phone_number = ?, phone_number_type = ?,
			do_not_call = ?, description = ?, address_street = ?,
			address_city = ?, address_state = ?, address_country = ?,
			address_postal_code = ?, account_id = ?, account_name = ?, assigned_user_id = ?,
			modified_by_id = ?, modified_at = ?, custom_fields = ?
		WHERE id = ? AND org_id = ? AND deleted = 0
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.SalutationName, existing.FirstName, existing.LastName,
		existing.EmailAddress, existing.PhoneNumber, existing.PhoneNumberType,
		existing.DoNotCall, existing.Description, existing.AddressStreet,
		existing.AddressCity, existing.AddressState, existing.AddressCountry,
		existing.AddressPostal, existing.AccountID, existing.AccountName, existing.AssignedUserID,
		existing.ModifiedByID, existing.ModifiedAt.Format(time.RFC3339), customFieldsJSON,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}

	return existing, nil
}

// Delete soft-deletes a contact
func (r *ContactRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `UPDATE contacts SET deleted = 1, modified_at = ? WHERE id = ? AND org_id = ? AND deleted = 0`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339), id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
