package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// RelatedListRepo handles database operations for related list configurations
type RelatedListRepo struct {
	dbConn       db.DBConn
	metadataRepo *MetadataRepo
}

// NewRelatedListRepo creates a new RelatedListRepo
func NewRelatedListRepo(dbConn db.DBConn, metadataRepo *MetadataRepo) *RelatedListRepo {
	return &RelatedListRepo{dbConn: dbConn, metadataRepo: metadataRepo}
}

// WithDB returns a new RelatedListRepo using the specified database connection
// This is used for multi-tenant database routing with DBConn interface
func (r *RelatedListRepo) WithDB(dbConn db.DBConn) *RelatedListRepo {
	if dbConn == nil {
		return r
	}
	return &RelatedListRepo{dbConn: dbConn, metadataRepo: r.metadataRepo}
}

// WithRawDB returns a new RelatedListRepo using a raw *sql.DB connection
// This is used for multi-tenant database routing with tenant databases
func (r *RelatedListRepo) WithRawDB(rawDB *sql.DB) *RelatedListRepo {
	if rawDB == nil {
		return r
	}
	return &RelatedListRepo{dbConn: rawDB, metadataRepo: r.metadataRepo}
}

// DB returns the current database connection (for compatibility)
func (r *RelatedListRepo) DB() db.DBConn {
	return r.dbConn
}

// EnsureSchema ensures the related_list_configs table has all required columns
// This handles schema migrations for tenant databases that may be missing columns
func (r *RelatedListRepo) EnsureSchema(ctx context.Context) error {
	// First check if the table exists at all
	var tableExists int
	if err := r.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='related_list_configs'").Scan(&tableExists); err != nil {
		return fmt.Errorf("failed to check related_list_configs table existence: %w", err)
	}
	if tableExists == 0 {
		return nil // Table doesn't exist yet - will be created by migrations
	}

	// Check if is_multi_lookup column exists
	var count int
	err := r.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('related_list_configs') WHERE name = 'is_multi_lookup'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check is_multi_lookup column: %w", err)
	}

	if count == 0 {
		// Add the missing column
		_, err := r.dbConn.ExecContext(ctx, "ALTER TABLE related_list_configs ADD COLUMN is_multi_lookup INTEGER DEFAULT 0")
		if err != nil {
			return fmt.Errorf("failed to add is_multi_lookup column: %w", err)
		}
	}

	// Check if edit_in_list column exists
	err = r.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('related_list_configs') WHERE name = 'edit_in_list'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check edit_in_list column: %w", err)
	}

	if count == 0 {
		// Add the missing column
		_, err := r.dbConn.ExecContext(ctx, "ALTER TABLE related_list_configs ADD COLUMN edit_in_list INTEGER DEFAULT 0")
		if err != nil {
			return fmt.Errorf("failed to add edit_in_list column: %w", err)
		}
	}

	return nil
}

// ListByEntity returns all related list configs for an entity type
func (r *RelatedListRepo) ListByEntity(ctx context.Context, orgID, entityType string) ([]entity.RelatedListConfig, error) {
	query := `SELECT id, org_id, entity_type, related_entity, lookup_field, label,
	          enabled, is_multi_lookup, edit_in_list, display_fields, sort_order, default_sort, default_sort_dir, page_size,
	          created_at, modified_at
	          FROM related_list_configs
	          WHERE org_id = ? AND entity_type = ?
	          ORDER BY sort_order, label`

	rows, err := r.dbConn.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list related list configs: %w", err)
	}
	defer rows.Close()

	var configs []entity.RelatedListConfig
	for rows.Next() {
		var c entity.RelatedListConfig
		var createdAt, modifiedAt string
		var defaultSort, defaultSortDir sql.NullString

		if err := rows.Scan(&c.ID, &c.OrgID, &c.EntityType, &c.RelatedEntity, &c.LookupField,
			&c.Label, &c.Enabled, &c.IsMultiLookup, &c.EditInList, &c.DisplayFieldsRaw, &c.SortOrder, &defaultSort, &defaultSortDir,
			&c.PageSize, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan related list config: %w", err)
		}

		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		if defaultSort.Valid {
			c.DefaultSort = defaultSort.String
		}
		if defaultSortDir.Valid {
			c.DefaultSortDir = defaultSortDir.String
		}

		// Parse display fields from JSON
		c.DisplayFields = []entity.FieldConfig{}
		if c.DisplayFieldsRaw != "" {
			json.Unmarshal([]byte(c.DisplayFieldsRaw), &c.DisplayFields)
		}

		configs = append(configs, c)
	}

	if configs == nil {
		configs = []entity.RelatedListConfig{}
	}

	return configs, nil
}

// ListEnabledByEntity returns only enabled related list configs for an entity type
func (r *RelatedListRepo) ListEnabledByEntity(ctx context.Context, orgID, entityType string) ([]entity.RelatedListConfig, error) {
	query := `SELECT id, org_id, entity_type, related_entity, lookup_field, label,
	          enabled, is_multi_lookup, edit_in_list, display_fields, sort_order, default_sort, default_sort_dir, page_size,
	          created_at, modified_at
	          FROM related_list_configs
	          WHERE org_id = ? AND entity_type = ? AND enabled = 1
	          ORDER BY sort_order, label`

	rows, err := r.dbConn.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled related list configs: %w", err)
	}
	defer rows.Close()

	var configs []entity.RelatedListConfig
	for rows.Next() {
		var c entity.RelatedListConfig
		var createdAt, modifiedAt string
		var defaultSort, defaultSortDir sql.NullString

		if err := rows.Scan(&c.ID, &c.OrgID, &c.EntityType, &c.RelatedEntity, &c.LookupField,
			&c.Label, &c.Enabled, &c.IsMultiLookup, &c.EditInList, &c.DisplayFieldsRaw, &c.SortOrder, &defaultSort, &defaultSortDir,
			&c.PageSize, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan related list config: %w", err)
		}

		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		if defaultSort.Valid {
			c.DefaultSort = defaultSort.String
		}
		if defaultSortDir.Valid {
			c.DefaultSortDir = defaultSortDir.String
		}

		// Parse display fields from JSON
		c.DisplayFields = []entity.FieldConfig{}
		if c.DisplayFieldsRaw != "" {
			json.Unmarshal([]byte(c.DisplayFieldsRaw), &c.DisplayFields)
		}

		configs = append(configs, c)
	}

	if configs == nil {
		configs = []entity.RelatedListConfig{}
	}

	return configs, nil
}

// GetByID returns a related list config by ID
func (r *RelatedListRepo) GetByID(ctx context.Context, orgID, id string) (*entity.RelatedListConfig, error) {
	query := `SELECT id, org_id, entity_type, related_entity, lookup_field, label,
	          enabled, is_multi_lookup, edit_in_list, display_fields, sort_order, default_sort, default_sort_dir, page_size,
	          created_at, modified_at
	          FROM related_list_configs
	          WHERE org_id = ? AND id = ?`

	var c entity.RelatedListConfig
	var createdAt, modifiedAt string
	var defaultSort, defaultSortDir sql.NullString

	err := r.dbConn.QueryRowContext(ctx, query, orgID, id).Scan(
		&c.ID, &c.OrgID, &c.EntityType, &c.RelatedEntity, &c.LookupField,
		&c.Label, &c.Enabled, &c.IsMultiLookup, &c.EditInList, &c.DisplayFieldsRaw, &c.SortOrder, &defaultSort, &defaultSortDir,
		&c.PageSize, &createdAt, &modifiedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get related list config: %w", err)
	}

	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
	if defaultSort.Valid {
		c.DefaultSort = defaultSort.String
	}
	if defaultSortDir.Valid {
		c.DefaultSortDir = defaultSortDir.String
	}

	// Parse display fields from JSON
	c.DisplayFields = []entity.FieldConfig{}
	if c.DisplayFieldsRaw != "" {
		json.Unmarshal([]byte(c.DisplayFieldsRaw), &c.DisplayFields)
	}

	return &c, nil
}

// Create creates a new related list config
func (r *RelatedListRepo) Create(ctx context.Context, orgID, entityType string, input entity.RelatedListConfigCreateInput) (*entity.RelatedListConfig, error) {
	// Get max sort order
	var maxOrder int
	err := r.dbConn.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), 0) FROM related_list_configs WHERE org_id = ? AND entity_type = ?",
		orgID, entityType).Scan(&maxOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to get max sort order: %w", err)
	}

	// Set defaults
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 5
	}
	defaultSortDir := input.DefaultSortDir
	if defaultSortDir == "" {
		defaultSortDir = "desc"
	}
	sortOrder := input.SortOrder
	if sortOrder == 0 {
		sortOrder = maxOrder + 1
	}

	// Auto-populate display fields if not provided
	displayFields := input.DisplayFields
	if displayFields == nil || len(displayFields) == 0 {
		displayFields = r.getDefaultDisplayFields(ctx, orgID, input.RelatedEntity)
	}

	// Serialize display fields
	displayFieldsJSON := "[]"
	if displayFields != nil && len(displayFields) > 0 {
		if jsonBytes, err := json.Marshal(displayFields); err == nil {
			displayFieldsJSON = string(jsonBytes)
		}
	}

	config := &entity.RelatedListConfig{
		ID:             sfid.NewRelatedList(),
		OrgID:          orgID,
		EntityType:     entityType,
		RelatedEntity:  input.RelatedEntity,
		LookupField:    input.LookupField,
		Label:          input.Label,
		Enabled:        input.Enabled,
		IsMultiLookup:  input.IsMultiLookup,
		EditInList:     input.EditInList,
		DisplayFields:  displayFields,
		SortOrder:      sortOrder,
		DefaultSort:    input.DefaultSort,
		DefaultSortDir: defaultSortDir,
		PageSize:       pageSize,
		CreatedAt:      time.Now().UTC(),
		ModifiedAt:     time.Now().UTC(),
	}

	query := `INSERT INTO related_list_configs (id, org_id, entity_type, related_entity, lookup_field,
	          label, enabled, is_multi_lookup, edit_in_list, display_fields, sort_order, default_sort, default_sort_dir, page_size,
	          created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.dbConn.ExecContext(ctx, query,
		config.ID, config.OrgID, config.EntityType, config.RelatedEntity, config.LookupField,
		config.Label, config.Enabled, config.IsMultiLookup, config.EditInList, displayFieldsJSON, config.SortOrder, config.DefaultSort,
		config.DefaultSortDir, config.PageSize,
		config.CreatedAt.Format(time.RFC3339), config.ModifiedAt.Format(time.RFC3339))

	if err != nil {
		return nil, fmt.Errorf("failed to create related list config: %w", err)
	}

	return config, nil
}

// Update updates a related list config
func (r *RelatedListRepo) Update(ctx context.Context, orgID, id string, input entity.RelatedListConfigUpdateInput) (*entity.RelatedListConfig, error) {
	// Get existing
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.Label != nil {
		existing.Label = *input.Label
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.EditInList != nil {
		existing.EditInList = *input.EditInList
	}
	if input.DisplayFields != nil {
		existing.DisplayFields = input.DisplayFields
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}
	if input.DefaultSort != nil {
		existing.DefaultSort = *input.DefaultSort
	}
	if input.DefaultSortDir != nil {
		existing.DefaultSortDir = *input.DefaultSortDir
	}
	if input.PageSize != nil {
		existing.PageSize = *input.PageSize
	}

	existing.ModifiedAt = time.Now().UTC()

	// Serialize display fields
	displayFieldsJSON := "[]"
	if existing.DisplayFields != nil && len(existing.DisplayFields) > 0 {
		if jsonBytes, err := json.Marshal(existing.DisplayFields); err == nil {
			displayFieldsJSON = string(jsonBytes)
		}
	}

	query := `UPDATE related_list_configs SET
	          label = ?, enabled = ?, edit_in_list = ?, display_fields = ?, sort_order = ?,
	          default_sort = ?, default_sort_dir = ?, page_size = ?, modified_at = ?
	          WHERE org_id = ? AND id = ?`

	_, err = r.dbConn.ExecContext(ctx, query,
		existing.Label, existing.Enabled, existing.EditInList, displayFieldsJSON, existing.SortOrder,
		existing.DefaultSort, existing.DefaultSortDir, existing.PageSize,
		existing.ModifiedAt.Format(time.RFC3339),
		orgID, id)

	if err != nil {
		return nil, fmt.Errorf("failed to update related list config: %w", err)
	}

	return existing, nil
}

// Delete deletes a related list config
func (r *RelatedListRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM related_list_configs WHERE org_id = ? AND id = ?`

	result, err := r.dbConn.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete related list config: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// BulkSave saves all related list configs for an entity (replace all)
func (r *RelatedListRepo) BulkSave(ctx context.Context, orgID, entityType string, configs []entity.RelatedListConfigCreateInput) ([]entity.RelatedListConfig, error) {
	tx, err := r.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing configs for this entity
	_, err = tx.ExecContext(ctx, "DELETE FROM related_list_configs WHERE org_id = ? AND entity_type = ?", orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to delete existing configs: %w", err)
	}

	// Insert new configs
	var results []entity.RelatedListConfig
	for i, input := range configs {
		// Set defaults
		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 5
		}
		defaultSortDir := input.DefaultSortDir
		if defaultSortDir == "" {
			defaultSortDir = "desc"
		}

		// Auto-populate display fields if not provided
		displayFields := input.DisplayFields
		if displayFields == nil || len(displayFields) == 0 {
			displayFields = r.getDefaultDisplayFields(ctx, orgID, input.RelatedEntity)
		}

		// Serialize display fields
		displayFieldsJSON := "[]"
		if displayFields != nil && len(displayFields) > 0 {
			if jsonBytes, err := json.Marshal(displayFields); err == nil {
				displayFieldsJSON = string(jsonBytes)
			}
		}

		config := entity.RelatedListConfig{
			ID:             sfid.NewRelatedList(),
			OrgID:          orgID,
			EntityType:     entityType,
			RelatedEntity:  input.RelatedEntity,
			LookupField:    input.LookupField,
			Label:          input.Label,
			Enabled:        input.Enabled,
			IsMultiLookup:  input.IsMultiLookup,
			EditInList:     input.EditInList,
			DisplayFields:  displayFields,
			SortOrder:      i + 1, // Use position as sort order
			DefaultSort:    input.DefaultSort,
			DefaultSortDir: defaultSortDir,
			PageSize:       pageSize,
			CreatedAt:      time.Now().UTC(),
			ModifiedAt:     time.Now().UTC(),
		}

		query := `INSERT INTO related_list_configs (id, org_id, entity_type, related_entity, lookup_field,
		          label, enabled, is_multi_lookup, edit_in_list, display_fields, sort_order, default_sort, default_sort_dir, page_size,
		          created_at, modified_at)
		          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = tx.ExecContext(ctx, query,
			config.ID, config.OrgID, config.EntityType, config.RelatedEntity, config.LookupField,
			config.Label, config.Enabled, config.IsMultiLookup, config.EditInList, displayFieldsJSON, config.SortOrder, config.DefaultSort,
			config.DefaultSortDir, config.PageSize,
			config.CreatedAt.Format(time.RFC3339), config.ModifiedAt.Format(time.RFC3339))

		if err != nil {
			return nil, fmt.Errorf("failed to insert related list config: %w", err)
		}

		results = append(results, config)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return results, nil
}

// DiscoverRelatedLists finds all possible related lists for an entity by scanning lookup fields
func (r *RelatedListRepo) DiscoverRelatedLists(ctx context.Context, orgID, targetEntity string) ([]entity.PossibleRelatedList, error) {
	// Find all lookup fields (type = 'link' or 'linkMultiple') that point to the target entity
	// Filter by org_id to only show relationships for the current org
	query := `SELECT entity_name, name, label, type
	          FROM field_defs
	          WHERE org_id = ? AND (type = 'link' OR type = 'linkMultiple') AND link_entity = ?
	          ORDER BY entity_name, name`

	rows, err := r.dbConn.QueryContext(ctx, query, orgID, targetEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to discover related lists: %w", err)
	}
	defer rows.Close()

	var results []entity.PossibleRelatedList
	for rows.Next() {
		var entityName, fieldName, fieldLabel, fieldType string
		if err := rows.Scan(&entityName, &fieldName, &fieldLabel, &fieldType); err != nil {
			return nil, fmt.Errorf("failed to scan field: %w", err)
		}

		results = append(results, entity.PossibleRelatedList{
			RelatedEntity:   entityName,
			LookupField:     fieldName,
			SuggestedLabel:  pluralize(entityName),
			FieldLabel:      fieldLabel,
			IsMultiLookup:   fieldType == "linkMultiple",
		})
	}

	// Check if Task entity exists and add it as a possible related list
	// Task uses polymorphic parent_type/parent_id, so it can relate to any entity
	var taskExists int
	err = r.dbConn.QueryRowContext(ctx, "SELECT 1 FROM entity_defs WHERE name = 'Task' LIMIT 1").Scan(&taskExists)
	if err == nil && taskExists == 1 {
		// Add Task as a possible related list for this entity
		results = append(results, entity.PossibleRelatedList{
			RelatedEntity:  "Task",
			LookupField:    "parentId", // Virtual field - actual filter uses parent_type + parent_id
			SuggestedLabel: "Tasks",
			FieldLabel:     "Related To",
		})
	}

	if results == nil {
		results = []entity.PossibleRelatedList{}
	}

	return results, nil
}

// getDefaultDisplayFields returns default display fields for an entity
func (r *RelatedListRepo) getDefaultDisplayFields(ctx context.Context, orgID, entityName string) []entity.FieldConfig {
	// Define default fields for known entities
	knownDefaults := map[string][]entity.FieldConfig{
		"Task": {
			{Field: "subject", Label: "Subject", Position: 1},
			{Field: "status", Label: "Status", Position: 2},
			{Field: "priority", Label: "Priority", Position: 3},
			{Field: "dueDate", Label: "Due Date", Position: 4},
		},
		"Contact": {
			{Field: "firstName", Label: "First Name", Position: 1},
			{Field: "lastName", Label: "Last Name", Position: 2},
			{Field: "emailAddress", Label: "Email", Position: 3},
			{Field: "phoneNumber", Label: "Phone", Position: 4},
		},
		"Account": {
			{Field: "name", Label: "Name", Position: 1},
			{Field: "industry", Label: "Industry", Position: 2},
			{Field: "phoneNumber", Label: "Phone", Position: 3},
		},
	}

	// Check if we have known defaults for this entity
	if defaults, ok := knownDefaults[entityName]; ok {
		return defaults
	}

	// For unknown entities, try to get fields from metadata and pick reasonable defaults
	if r.metadataRepo != nil {
		fields, err := r.metadataRepo.ListFields(ctx, orgID, entityName)
		if err == nil && len(fields) > 0 {
			var displayFields []entity.FieldConfig
			position := 1

			// Prioritize 'name' field first if it exists
			for _, f := range fields {
				if f.Name == "name" {
					displayFields = append(displayFields, entity.FieldConfig{
						Field:    f.Name,
						Label:    f.Label,
						Position: position,
					})
					position++
					break
				}
			}

			// Add other non-system fields (up to 4 total)
			systemFields := map[string]bool{
				"id": true, "orgId": true, "createdAt": true, "modifiedAt": true,
				"deleted": true, "customFields": true, "name": true,
			}
			for _, f := range fields {
				if position > 4 {
					break
				}
				if !systemFields[f.Name] && f.Type != "link" {
					displayFields = append(displayFields, entity.FieldConfig{
						Field:    f.Name,
						Label:    f.Label,
						Position: position,
					})
					position++
				}
			}

			if len(displayFields) > 0 {
				return displayFields
			}
		}
	}

	// Fallback: just show name and createdAt
	return []entity.FieldConfig{
		{Field: "name", Label: "Name", Position: 1},
		{Field: "createdAt", Label: "Created", Position: 2},
	}
}

// pluralize adds an 's' to the end of a word (simple pluralization)
func pluralize(word string) string {
	if word == "" {
		return ""
	}
	lastChar := word[len(word)-1]
	switch lastChar {
	case 's', 'x', 'z':
		return word + "es"
	case 'y':
		if len(word) > 1 {
			secondLastChar := word[len(word)-2]
			if secondLastChar != 'a' && secondLastChar != 'e' && secondLastChar != 'i' && secondLastChar != 'o' && secondLastChar != 'u' {
				return word[:len(word)-1] + "ies"
			}
		}
		return word + "s"
	default:
		return word + "s"
	}
}
