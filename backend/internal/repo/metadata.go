package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// MetadataRepo handles database operations for entity/field metadata
type MetadataRepo struct {
	db db.DBConn
}

// NewMetadataRepo creates a new MetadataRepo
func NewMetadataRepo(dbConn db.DBConn) *MetadataRepo {
	return &MetadataRepo{db: dbConn}
}

// WithDB returns a new MetadataRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *MetadataRepo) WithDB(dbConn db.DBConn) *MetadataRepo {
	if dbConn == nil {
		return r
	}
	return &MetadataRepo{db: dbConn}
}

// WithRawDB returns a new MetadataRepo using a raw *sql.DB connection
// This is used for multi-tenant database routing with tenant databases
func (r *MetadataRepo) WithRawDB(rawDB *sql.DB) *MetadataRepo {
	if rawDB == nil {
		return r
	}
	return &MetadataRepo{db: rawDB}
}

// EnsureSchema ensures the metadata tables have all required columns
// This handles schema migrations for tenant databases that may be missing columns
func (r *MetadataRepo) EnsureSchema(ctx context.Context) error {
	// Check and add missing columns to entity_defs table
	entityColumnsToAdd := []struct {
		name       string
		definition string
	}{
		{"display_field", "TEXT DEFAULT 'name'"},
		{"search_fields", "TEXT DEFAULT '[\"name\"]'"},
	}

	for _, col := range entityColumnsToAdd {
		var count int
		err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('entity_defs') WHERE name = ?", col.name).Scan(&count)
		if err != nil {
			continue // Table might not exist yet
		}

		if count == 0 {
			_, err := r.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE entity_defs ADD COLUMN %s %s", col.name, col.definition))
			if err != nil {
				return fmt.Errorf("failed to add entity_defs.%s column: %w", col.name, err)
			}
		}
	}

	// Check and add missing columns to field_defs table
	columnsToAdd := []struct {
		name       string
		definition string
	}{
		{"max_length", "INTEGER"},
		{"link_type", "TEXT"},
		{"link_foreign_key", "TEXT"},
		{"link_display_field", "TEXT"},
		{"variant", "TEXT DEFAULT 'info'"},
		{"content", "TEXT DEFAULT ''"},
		{"rollup_query", "TEXT"},
		{"rollup_result_type", "TEXT"},
		{"rollup_decimal_places", "INTEGER DEFAULT 2"},
		{"default_to_today", "INTEGER DEFAULT 0"},
	}

	for _, col := range columnsToAdd {
		var count int
		err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('field_defs') WHERE name = ?", col.name).Scan(&count)
		if err != nil {
			continue // Table might not exist yet
		}

		if count == 0 {
			_, err := r.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE field_defs ADD COLUMN %s %s", col.name, col.definition))
			if err != nil {
				return fmt.Errorf("failed to add field_defs.%s column: %w", col.name, err)
			}
		}
	}
	return nil
}

// --- Entity Definitions ---

// ListEntities returns all entity definitions for an organization
func (r *MetadataRepo) ListEntities(ctx context.Context, orgID string) ([]entity.EntityDef, error) {
	query := `SELECT id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable,
	          has_stream, has_activities, COALESCE(display_field, 'name') as display_field,
	          COALESCE(search_fields, '["name"]') as search_fields, created_at, modified_at
	          FROM entity_defs WHERE org_id = ? AND name NOT LIKE '%__del' ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}
	defer rows.Close()

	var entities []entity.EntityDef
	for rows.Next() {
		var e entity.EntityDef
		var createdAt, modifiedAt string

		if err := rows.Scan(&e.ID, &e.OrgID, &e.Name, &e.Label, &e.LabelPlural, &e.Icon, &e.Color,
			&e.IsCustom, &e.IsCustomizable, &e.HasStream, &e.HasActivities,
			&e.DisplayField, &e.SearchFields, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}

		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		e.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		entities = append(entities, e)
	}

	if entities == nil {
		entities = []entity.EntityDef{}
	}

	return entities, nil
}

// GetEntity returns an entity definition by name for an organization
func (r *MetadataRepo) GetEntity(ctx context.Context, orgID, name string) (*entity.EntityDef, error) {
	query := `SELECT id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable,
	          has_stream, has_activities, COALESCE(display_field, 'name') as display_field,
	          COALESCE(search_fields, '["name"]') as search_fields, created_at, modified_at
	          FROM entity_defs WHERE org_id = ? AND name = ?`

	var e entity.EntityDef
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, name).Scan(
		&e.ID, &e.OrgID, &e.Name, &e.Label, &e.LabelPlural, &e.Icon, &e.Color,
		&e.IsCustom, &e.IsCustomizable, &e.HasStream, &e.HasActivities,
		&e.DisplayField, &e.SearchFields, &createdAt, &modifiedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	e.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &e, nil
}

// GetEntityByLowercaseName finds an entity by matching its name against a lowercase URL path
// This handles cases like "farmcustomers" -> "FarmCustomer"
func (r *MetadataRepo) GetEntityByLowercaseName(ctx context.Context, orgID, urlPath string) (string, error) {
	// Remove trailing 's' if present (e.g., "farmcustomers" -> "farmcustomer")
	searchName := strings.TrimSuffix(urlPath, "s")

	// Query using LOWER() to do case-insensitive match
	query := `SELECT name FROM entity_defs WHERE org_id = ? AND LOWER(name) = LOWER(?)`
	var name string
	err := r.db.QueryRowContext(ctx, query, orgID, searchName).Scan(&name)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get entity by lowercase name: %w", err)
	}

	return name, nil
}

// CreateEntity creates a new entity definition with default fields (id, name)
func (r *MetadataRepo) CreateEntity(ctx context.Context, orgID string, input entity.EntityDefCreateInput) (*entity.EntityDef, error) {
	// Set defaults
	name := input.Name
	label := input.Label
	labelPlural := input.LabelPlural
	if labelPlural == "" {
		labelPlural = util.Pluralize(label)
	}
	icon := input.Icon
	if icon == "" {
		icon = "folder"
	}
	color := input.Color
	if color == "" {
		color = "#6366f1"
	}

	now := time.Now().UTC()
	ent := &entity.EntityDef{
		ID:             sfid.NewEntity(),
		OrgID:          orgID,
		Name:           name,
		Label:          label,
		LabelPlural:    labelPlural,
		Icon:           icon,
		Color:          color,
		IsCustom:       true,
		IsCustomizable: true,
		HasStream:      input.HasStream,
		HasActivities:  input.HasActivities,
		DisplayField:   "name",
		SearchFields:   `["name"]`,
		CreatedAt:      now,
		ModifiedAt:     now,
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert entity definition
	query := `INSERT INTO entity_defs (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable,
	          has_stream, has_activities, display_field, search_fields, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, query,
		ent.ID, ent.OrgID, ent.Name, ent.Label, ent.LabelPlural, ent.Icon, ent.Color,
		ent.IsCustom, ent.IsCustomizable, ent.HasStream, ent.HasActivities,
		ent.DisplayField, ent.SearchFields,
		ent.CreatedAt.Format(time.RFC3339), ent.ModifiedAt.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	// Create default 'id' field (system field, read-only)
	idFieldID := sfid.NewFieldDef()
	_, err = tx.ExecContext(ctx, `INSERT INTO field_defs
		(id, org_id, entity_name, name, label, type, is_required, is_read_only, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, 'id', 'ID', 'varchar', 0, 1, 0, 0, ?, ?)`,
		idFieldID, orgID, name, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create id field: %w", err)
	}

	// Create default 'name' field (required)
	nameFieldID := sfid.NewFieldDef()
	_, err = tx.ExecContext(ctx, `INSERT INTO field_defs
		(id, org_id, entity_name, name, label, type, is_required, is_read_only, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, 'name', 'Name', 'varchar', 1, 0, 0, 1, ?, ?)`,
		nameFieldID, orgID, name, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create name field: %w", err)
	}

	// Create 'created_at' field (system, read-only)
	createdAtFieldID := sfid.NewFieldDef()
	_, err = tx.ExecContext(ctx, `INSERT INTO field_defs
		(id, org_id, entity_name, name, label, type, is_required, is_read_only, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, 'created_at', 'Created At', 'datetime', 0, 1, 0, 98, ?, ?)`,
		createdAtFieldID, orgID, name, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create created_at field: %w", err)
	}

	// Create 'modified_at' field (system, read-only)
	modifiedAtFieldID := sfid.NewFieldDef()
	_, err = tx.ExecContext(ctx, `INSERT INTO field_defs
		(id, org_id, entity_name, name, label, type, is_required, is_read_only, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, 'modified_at', 'Modified At', 'datetime', 0, 1, 0, 99, ?, ?)`,
		modifiedAtFieldID, orgID, name, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create modified_at field: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return ent, nil
}

// UpdateEntity updates an entity definition for an organization
func (r *MetadataRepo) UpdateEntity(ctx context.Context, orgID, name string, input entity.EntityDefUpdateInput) (*entity.EntityDef, error) {
	// Get existing entity
	existing, err := r.GetEntity(ctx, orgID, name)
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
	if input.LabelPlural != nil {
		existing.LabelPlural = *input.LabelPlural
	}
	if input.Icon != nil {
		existing.Icon = *input.Icon
	}
	if input.Color != nil {
		existing.Color = *input.Color
	}
	if input.HasStream != nil {
		existing.HasStream = *input.HasStream
	}
	if input.HasActivities != nil {
		existing.HasActivities = *input.HasActivities
	}

	existing.ModifiedAt = time.Now().UTC()

	query := `UPDATE entity_defs SET label = ?, label_plural = ?, icon = ?, color = ?,
	          has_stream = ?, has_activities = ?, modified_at = ?
	          WHERE org_id = ? AND name = ?`

	_, err = r.db.ExecContext(ctx, query,
		existing.Label, existing.LabelPlural, existing.Icon, existing.Color,
		existing.HasStream, existing.HasActivities, existing.ModifiedAt.Format(time.RFC3339),
		orgID, name)

	if err != nil {
		return nil, fmt.Errorf("failed to update entity: %w", err)
	}

	return existing, nil
}

// SoftDeleteEntity soft-deletes an entity by renaming it with __del suffix
// This preserves all metadata while hiding the entity from normal queries
func (r *MetadataRepo) SoftDeleteEntity(ctx context.Context, orgID, entityName string) error {
	// Get existing entity and verify it's custom
	existing, err := r.GetEntity(ctx, orgID, entityName)
	if err != nil {
		return err
	}
	if existing == nil {
		return sql.ErrNoRows
	}
	if !existing.IsCustom {
		return fmt.Errorf("Cannot delete system entity")
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// New name with __del suffix
	newName := entityName + "__del"

	// Update entity name
	_, err = tx.ExecContext(ctx, `UPDATE entity_defs SET name = ? WHERE org_id = ? AND name = ?`,
		newName, orgID, entityName)
	if err != nil {
		return fmt.Errorf("failed to rename entity: %w", err)
	}

	// Update field definitions to reference new entity name
	_, err = tx.ExecContext(ctx, `UPDATE field_defs SET entity_name = ? WHERE org_id = ? AND entity_name = ?`,
		newName, orgID, entityName)
	if err != nil {
		return fmt.Errorf("failed to update field entity names: %w", err)
	}

	// Update layout definitions to reference new entity name
	_, err = tx.ExecContext(ctx, `UPDATE layout_defs SET entity_name = ? WHERE org_id = ? AND entity_name = ?`,
		newName, orgID, entityName)
	if err != nil {
		return fmt.Errorf("failed to update layout entity names: %w", err)
	}

	// Hide navigation tab for this entity (if exists)
	_, err = tx.ExecContext(ctx, `UPDATE navigation_tabs SET is_visible = 0 WHERE org_id = ? AND entity_name = ?`,
		orgID, entityName)
	if err != nil {
		return fmt.Errorf("failed to hide navigation tab: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// --- Field Definitions ---

// ListFields returns all field definitions for an entity in an organization
func (r *MetadataRepo) ListFields(ctx context.Context, orgID, entityName string) ([]entity.FieldDef, error) {
	query := `SELECT id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited,
	          is_custom, default_value, options, max_length, min_value, max_value, pattern,
	          tooltip, link_entity, rollup_query, rollup_result_type, rollup_decimal_places,
	          default_to_today, COALESCE(variant, 'info') as variant, COALESCE(content, '') as content,
	          sort_order, created_at, modified_at
	          FROM field_defs WHERE org_id = ? AND entity_name = ? ORDER BY sort_order, name`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityName)
	if err != nil {
		return nil, fmt.Errorf("failed to list fields: %w", err)
	}
	defer rows.Close()

	var fields []entity.FieldDef
	for rows.Next() {
		var f entity.FieldDef
		var createdAt, modifiedAt string

		if err := rows.Scan(&f.ID, &f.OrgID, &f.EntityName, &f.Name, &f.Label, &f.Type,
			&f.IsRequired, &f.IsReadOnly, &f.IsAudited, &f.IsCustom,
			&f.DefaultValue, &f.Options, &f.MaxLength, &f.MinValue, &f.MaxValue,
			&f.Pattern, &f.Tooltip, &f.LinkEntity,
			&f.RollupQuery, &f.RollupResultType, &f.RollupDecimalPlaces,
			&f.DefaultToToday, &f.Variant, &f.Content,
			&f.SortOrder, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan field: %w", err)
		}

		f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		f.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		fields = append(fields, f)
	}

	if fields == nil {
		fields = []entity.FieldDef{}
	}

	return fields, nil
}

// GetField returns a field definition by entity and field name for an organization
func (r *MetadataRepo) GetField(ctx context.Context, orgID, entityName, fieldName string) (*entity.FieldDef, error) {
	query := `SELECT id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited,
	          is_custom, default_value, options, max_length, min_value, max_value, pattern,
	          tooltip, link_entity, rollup_query, rollup_result_type, rollup_decimal_places,
	          default_to_today, COALESCE(variant, 'info') as variant, COALESCE(content, '') as content,
	          sort_order, created_at, modified_at
	          FROM field_defs WHERE org_id = ? AND entity_name = ? AND name = ?`

	var f entity.FieldDef
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, entityName, fieldName).Scan(
		&f.ID, &f.OrgID, &f.EntityName, &f.Name, &f.Label, &f.Type,
		&f.IsRequired, &f.IsReadOnly, &f.IsAudited, &f.IsCustom,
		&f.DefaultValue, &f.Options, &f.MaxLength, &f.MinValue, &f.MaxValue,
		&f.Pattern, &f.Tooltip, &f.LinkEntity,
		&f.RollupQuery, &f.RollupResultType, &f.RollupDecimalPlaces,
		&f.DefaultToToday, &f.Variant, &f.Content,
		&f.SortOrder, &createdAt, &modifiedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get field: %w", err)
	}

	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	f.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &f, nil
}

// CreateField creates a new field definition for an organization
func (r *MetadataRepo) CreateField(ctx context.Context, orgID, entityName string, input entity.FieldDefCreateInput) (*entity.FieldDef, error) {
	// Get max sort order
	var maxOrder int
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) FROM field_defs WHERE org_id = ? AND entity_name = ?", orgID, entityName).Scan(&maxOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to get max sort order: %w", err)
	}

	field := &entity.FieldDef{
		ID:                  sfid.NewFieldDef(),
		OrgID:               orgID,
		EntityName:          entityName,
		Name:                input.Name,
		Label:               input.Label,
		Type:                input.Type,
		IsRequired:          input.IsRequired,
		IsReadOnly:          input.IsReadOnly,
		IsAudited:           input.IsAudited,
		IsCustom:            true, // All user-created fields are custom
		DefaultValue:        input.DefaultValue,
		Options:             input.Options,
		MaxLength:           input.MaxLength,
		MinValue:            input.MinValue,
		MaxValue:            input.MaxValue,
		Pattern:             input.Pattern,
		Tooltip:             input.Tooltip,
		LinkEntity:          input.LinkEntity,
		RollupQuery:         input.RollupQuery,
		RollupResultType:    input.RollupResultType,
		RollupDecimalPlaces: input.RollupDecimalPlaces,
		DefaultToToday:      input.DefaultToToday,
		Variant:             input.Variant,
		Content:             input.Content,
		SortOrder:           maxOrder + 1,
		CreatedAt:           time.Now().UTC(),
		ModifiedAt:          time.Now().UTC(),
	}

	query := `INSERT INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only,
	          is_audited, is_custom, default_value, options, max_length, min_value, max_value,
	          pattern, tooltip, link_entity, rollup_query, rollup_result_type, rollup_decimal_places,
	          default_to_today, variant, content, sort_order, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, query,
		field.ID, field.OrgID, field.EntityName, field.Name, field.Label, field.Type,
		field.IsRequired, field.IsReadOnly, field.IsAudited, field.IsCustom,
		field.DefaultValue, field.Options, field.MaxLength, field.MinValue, field.MaxValue,
		field.Pattern, field.Tooltip, field.LinkEntity,
		field.RollupQuery, field.RollupResultType, field.RollupDecimalPlaces,
		field.DefaultToToday, field.Variant, field.Content,
		field.SortOrder, field.CreatedAt.Format(time.RFC3339), field.ModifiedAt.Format(time.RFC3339))

	if err != nil {
		return nil, fmt.Errorf("failed to create field: %w", err)
	}

	return field, nil
}

// UpdateField updates a field definition for an organization
func (r *MetadataRepo) UpdateField(ctx context.Context, orgID, entityName, fieldName string, input entity.FieldDefUpdateInput) (*entity.FieldDef, error) {
	// Get existing field
	existing, err := r.GetField(ctx, orgID, entityName, fieldName)
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
	if input.IsRequired != nil {
		existing.IsRequired = *input.IsRequired
	}
	if input.IsReadOnly != nil {
		existing.IsReadOnly = *input.IsReadOnly
	}
	if input.IsAudited != nil {
		existing.IsAudited = *input.IsAudited
	}
	if input.DefaultValue != nil {
		existing.DefaultValue = input.DefaultValue
	}
	if input.Options != nil {
		existing.Options = input.Options
	}
	if input.MaxLength != nil {
		existing.MaxLength = input.MaxLength
	}
	if input.MinValue != nil {
		existing.MinValue = input.MinValue
	}
	if input.MaxValue != nil {
		existing.MaxValue = input.MaxValue
	}
	if input.Pattern != nil {
		existing.Pattern = input.Pattern
	}
	if input.Tooltip != nil {
		existing.Tooltip = input.Tooltip
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}
	if input.RollupQuery != nil {
		existing.RollupQuery = input.RollupQuery
	}
	if input.RollupResultType != nil {
		existing.RollupResultType = input.RollupResultType
	}
	if input.RollupDecimalPlaces != nil {
		existing.RollupDecimalPlaces = input.RollupDecimalPlaces
	}
	if input.DefaultToToday != nil {
		existing.DefaultToToday = *input.DefaultToToday
	}
	if input.Variant != nil {
		existing.Variant = input.Variant
	}
	if input.Content != nil {
		existing.Content = input.Content
	}

	existing.ModifiedAt = time.Now().UTC()

	query := `UPDATE field_defs SET label = ?, is_required = ?, is_read_only = ?, is_audited = ?,
	          default_value = ?, options = ?, max_length = ?, min_value = ?, max_value = ?,
	          pattern = ?, tooltip = ?, sort_order = ?, rollup_query = ?, rollup_result_type = ?,
	          rollup_decimal_places = ?, default_to_today = ?, variant = ?, content = ?, modified_at = ?
	          WHERE org_id = ? AND entity_name = ? AND name = ?`

	_, err = r.db.ExecContext(ctx, query,
		existing.Label, existing.IsRequired, existing.IsReadOnly, existing.IsAudited,
		existing.DefaultValue, existing.Options, existing.MaxLength, existing.MinValue, existing.MaxValue,
		existing.Pattern, existing.Tooltip, existing.SortOrder, existing.RollupQuery, existing.RollupResultType,
		existing.RollupDecimalPlaces, existing.DefaultToToday, existing.Variant, existing.Content, existing.ModifiedAt.Format(time.RFC3339),
		orgID, entityName, fieldName)

	if err != nil {
		return nil, fmt.Errorf("failed to update field: %w", err)
	}

	return existing, nil
}

// DeleteField deletes a field definition for an organization
func (r *MetadataRepo) DeleteField(ctx context.Context, orgID, entityName, fieldName string) error {
	// Check if field exists and is custom
	field, err := r.GetField(ctx, orgID, entityName, fieldName)
	if err != nil {
		return err
	}
	if field == nil {
		return sql.ErrNoRows
	}

	query := `DELETE FROM field_defs WHERE org_id = ? AND entity_name = ? AND name = ?`
	_, err = r.db.ExecContext(ctx, query, orgID, entityName, fieldName)
	if err != nil {
		return fmt.Errorf("failed to delete field: %w", err)
	}

	return nil
}

// ReorderFields updates the sort order of fields for an organization
func (r *MetadataRepo) ReorderFields(ctx context.Context, orgID, entityName string, fieldOrder []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, fieldName := range fieldOrder {
		_, err := tx.ExecContext(ctx, "UPDATE field_defs SET sort_order = ? WHERE org_id = ? AND entity_name = ? AND name = ?",
			i+1, orgID, entityName, fieldName)
		if err != nil {
			return fmt.Errorf("failed to update field order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// --- Layout Definitions ---

// GetLayout returns a layout definition for an organization
func (r *MetadataRepo) GetLayout(ctx context.Context, orgID, entityName, layoutType string) (*entity.LayoutDef, error) {
	query := `SELECT id, org_id, entity_name, layout_type, layout_data, created_at, modified_at
	          FROM layout_defs WHERE org_id = ? AND entity_name = ? AND layout_type = ?`

	var l entity.LayoutDef
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, entityName, layoutType).Scan(
		&l.ID, &l.OrgID, &l.EntityName, &l.LayoutType, &l.LayoutData, &createdAt, &modifiedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get layout: %w", err)
	}

	l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	l.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &l, nil
}

// OrgHasMetadata checks if an organization has any metadata provisioned
// Returns true if the org has at least one entity and one layout
func (r *MetadataRepo) OrgHasMetadata(ctx context.Context, orgID string) (bool, error) {
	// Check if org has at least one entity (e.g., Account)
	var entityCount int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_defs WHERE org_id = ?", orgID).Scan(&entityCount)
	if err != nil {
		return false, fmt.Errorf("failed to check entity count: %w", err)
	}

	if entityCount == 0 {
		return false, nil
	}

	// Check if org has at least one layout
	var layoutCount int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layout_defs WHERE org_id = ?", orgID).Scan(&layoutCount)
	if err != nil {
		return false, fmt.Errorf("failed to check layout count: %w", err)
	}

	return layoutCount > 0, nil
}

// SaveLayout creates or updates a layout definition for an organization
func (r *MetadataRepo) SaveLayout(ctx context.Context, orgID, entityName, layoutType, layoutData string) (*entity.LayoutDef, error) {
	existing, err := r.GetLayout(ctx, orgID, entityName, layoutType)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	if existing != nil {
		// Update
		query := `UPDATE layout_defs SET layout_data = ?, modified_at = ? WHERE org_id = ? AND entity_name = ? AND layout_type = ?`
		_, err = r.db.ExecContext(ctx, query, layoutData, now.Format(time.RFC3339), orgID, entityName, layoutType)
		if err != nil {
			return nil, fmt.Errorf("failed to update layout: %w", err)
		}
		existing.LayoutData = layoutData
		existing.ModifiedAt = now
		return existing, nil
	}

	// Create
	layout := &entity.LayoutDef{
		ID:         sfid.NewLayout(),
		OrgID:      orgID,
		EntityName: entityName,
		LayoutType: layoutType,
		LayoutData: layoutData,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	query := `INSERT INTO layout_defs (id, org_id, entity_name, layout_type, layout_data, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query, layout.ID, layout.OrgID, layout.EntityName, layout.LayoutType,
		layout.LayoutData, layout.CreatedAt.Format(time.RFC3339), layout.ModifiedAt.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create layout: %w", err)
	}

	return layout, nil
}
