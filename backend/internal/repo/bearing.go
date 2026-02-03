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

// BearingRepo handles database operations for bearing configurations
type BearingRepo struct {
	db           db.DBConn
	metadataRepo *MetadataRepo
}

// NewBearingRepo creates a new BearingRepo
func NewBearingRepo(dbConn db.DBConn, metadataRepo *MetadataRepo) *BearingRepo {
	return &BearingRepo{db: dbConn, metadataRepo: metadataRepo}
}

// WithDB returns a new BearingRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *BearingRepo) WithDB(dbConn db.DBConn) *BearingRepo {
	if dbConn == nil {
		return r
	}
	// Also update metadataRepo with new connection so field lookups use the correct DB
	var newMetadataRepo *MetadataRepo
	if r.metadataRepo != nil {
		newMetadataRepo = r.metadataRepo.WithDB(dbConn)
	}
	return &BearingRepo{db: dbConn, metadataRepo: newMetadataRepo}
}

// WithRawDB returns a new BearingRepo using a raw *sql.DB connection
func (r *BearingRepo) WithRawDB(rawDB *sql.DB) *BearingRepo {
	if rawDB == nil {
		return r
	}
	return &BearingRepo{db: rawDB, metadataRepo: r.metadataRepo}
}

// ListByEntity returns all bearing configs for an entity type
func (r *BearingRepo) ListByEntity(ctx context.Context, orgID, entityType string) ([]entity.BearingConfig, error) {
	query := `SELECT id, org_id, entity_type, name, source_picklist, display_order,
	          active, confirm_backward, allow_updates, created_at, modified_at
	          FROM bearing_configs
	          WHERE org_id = ? AND entity_type = ?
	          ORDER BY display_order, name`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list bearing configs: %w", err)
	}
	defer rows.Close()

	var configs []entity.BearingConfig
	for rows.Next() {
		var c entity.BearingConfig
		var createdAt, modifiedAt string
		var allowUpdates sql.NullBool

		if err := rows.Scan(&c.ID, &c.OrgID, &c.EntityType, &c.Name, &c.SourcePicklist,
			&c.DisplayOrder, &c.Active, &c.ConfirmBackward, &allowUpdates, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bearing config: %w", err)
		}

		c.AllowUpdates = !allowUpdates.Valid || allowUpdates.Bool // Default to true if NULL
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		configs = append(configs, c)
	}

	if configs == nil {
		configs = []entity.BearingConfig{}
	}

	return configs, nil
}

// ListActiveByEntity returns only active bearing configs for an entity type
func (r *BearingRepo) ListActiveByEntity(ctx context.Context, orgID, entityType string) ([]entity.BearingConfig, error) {
	query := `SELECT id, org_id, entity_type, name, source_picklist, display_order,
	          active, confirm_backward, allow_updates, created_at, modified_at
	          FROM bearing_configs
	          WHERE org_id = ? AND entity_type = ? AND active = 1
	          ORDER BY display_order, name`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list active bearing configs: %w", err)
	}
	defer rows.Close()

	var configs []entity.BearingConfig
	for rows.Next() {
		var c entity.BearingConfig
		var createdAt, modifiedAt string
		var allowUpdates sql.NullBool

		if err := rows.Scan(&c.ID, &c.OrgID, &c.EntityType, &c.Name, &c.SourcePicklist,
			&c.DisplayOrder, &c.Active, &c.ConfirmBackward, &allowUpdates, &createdAt, &modifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bearing config: %w", err)
		}

		c.AllowUpdates = !allowUpdates.Valid || allowUpdates.Bool // Default to true if NULL
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		configs = append(configs, c)
	}

	if configs == nil {
		configs = []entity.BearingConfig{}
	}

	return configs, nil
}

// GetByID returns a bearing config by ID
func (r *BearingRepo) GetByID(ctx context.Context, orgID, id string) (*entity.BearingConfig, error) {
	query := `SELECT id, org_id, entity_type, name, source_picklist, display_order,
	          active, confirm_backward, allow_updates, created_at, modified_at
	          FROM bearing_configs
	          WHERE org_id = ? AND id = ?`

	var c entity.BearingConfig
	var createdAt, modifiedAt string
	var allowUpdates sql.NullBool

	err := r.db.QueryRowContext(ctx, query, orgID, id).Scan(
		&c.ID, &c.OrgID, &c.EntityType, &c.Name, &c.SourcePicklist,
		&c.DisplayOrder, &c.Active, &c.ConfirmBackward, &allowUpdates, &createdAt, &modifiedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bearing config: %w", err)
	}

	c.AllowUpdates = !allowUpdates.Valid || allowUpdates.Bool // Default to true if NULL
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &c, nil
}

// Create creates a new bearing config
func (r *BearingRepo) Create(ctx context.Context, orgID, entityType string, input entity.BearingConfigCreateInput) (*entity.BearingConfig, error) {
	// Check if we already have 12 bearings for this entity
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM bearing_configs WHERE org_id = ? AND entity_type = ?",
		orgID, entityType).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count bearings: %w", err)
	}
	if count >= 12 {
		return nil, fmt.Errorf("maximum of 12 bearings per entity reached")
	}

	// Get max display order
	var maxOrder int
	err = r.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(display_order), 0) FROM bearing_configs WHERE org_id = ? AND entity_type = ?",
		orgID, entityType).Scan(&maxOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to get max display order: %w", err)
	}

	// Set defaults
	displayOrder := input.DisplayOrder
	if displayOrder == 0 {
		displayOrder = maxOrder + 1
	}

	config := &entity.BearingConfig{
		ID:              sfid.NewBearing(),
		OrgID:           orgID,
		EntityType:      entityType,
		Name:            input.Name,
		SourcePicklist:  input.SourcePicklist,
		DisplayOrder:    displayOrder,
		Active:          input.Active,
		ConfirmBackward: input.ConfirmBackward,
		AllowUpdates:    input.AllowUpdates,
		CreatedAt:       time.Now().UTC(),
		ModifiedAt:      time.Now().UTC(),
	}

	query := `INSERT INTO bearing_configs (id, org_id, entity_type, name, source_picklist,
	          display_order, active, confirm_backward, allow_updates, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, query,
		config.ID, config.OrgID, config.EntityType, config.Name, config.SourcePicklist,
		config.DisplayOrder, config.Active, config.ConfirmBackward, config.AllowUpdates,
		config.CreatedAt.Format(time.RFC3339), config.ModifiedAt.Format(time.RFC3339))

	if err != nil {
		return nil, fmt.Errorf("failed to create bearing config: %w", err)
	}

	return config, nil
}

// Update updates a bearing config
func (r *BearingRepo) Update(ctx context.Context, orgID, id string, input entity.BearingConfigUpdateInput) (*entity.BearingConfig, error) {
	// Get existing
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
	if input.SourcePicklist != nil {
		existing.SourcePicklist = *input.SourcePicklist
	}
	if input.DisplayOrder != nil {
		existing.DisplayOrder = *input.DisplayOrder
	}
	if input.Active != nil {
		existing.Active = *input.Active
	}
	if input.ConfirmBackward != nil {
		existing.ConfirmBackward = *input.ConfirmBackward
	}
	if input.AllowUpdates != nil {
		existing.AllowUpdates = *input.AllowUpdates
	}

	existing.ModifiedAt = time.Now().UTC()

	query := `UPDATE bearing_configs SET
	          name = ?, source_picklist = ?, display_order = ?,
	          active = ?, confirm_backward = ?, allow_updates = ?, modified_at = ?
	          WHERE org_id = ? AND id = ?`

	_, err = r.db.ExecContext(ctx, query,
		existing.Name, existing.SourcePicklist, existing.DisplayOrder,
		existing.Active, existing.ConfirmBackward, existing.AllowUpdates,
		existing.ModifiedAt.Format(time.RFC3339),
		orgID, id)

	if err != nil {
		return nil, fmt.Errorf("failed to update bearing config: %w", err)
	}

	return existing, nil
}

// Delete deletes a bearing config
func (r *BearingRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM bearing_configs WHERE org_id = ? AND id = ?`

	result, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete bearing config: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetPicklistFields returns all enum fields for an entity (fields that can be used as bearing source)
func (r *BearingRepo) GetPicklistFields(ctx context.Context, orgID, entityType string) ([]entity.FieldDef, error) {
	if r.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repo not initialized")
	}

	fields, err := r.metadataRepo.ListFields(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list fields: %w", err)
	}

	// Filter to only enum/picklist fields
	var picklistFields []entity.FieldDef
	for _, f := range fields {
		if f.Type == entity.FieldTypeEnum {
			picklistFields = append(picklistFields, f)
		}
	}

	if picklistFields == nil {
		picklistFields = []entity.FieldDef{}
	}

	return picklistFields, nil
}

// GetBearingWithStages returns a bearing config with resolved picklist stages
func (r *BearingRepo) GetBearingWithStages(ctx context.Context, orgID string, config entity.BearingConfig) (*entity.BearingWithStages, error) {
	if r.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repo not initialized")
	}

	// Get the field definition to get the options
	field, err := r.metadataRepo.GetField(ctx, orgID, config.EntityType, config.SourcePicklist)
	if err != nil {
		return nil, fmt.Errorf("failed to get field: %w", err)
	}
	if field == nil {
		return nil, fmt.Errorf("source picklist field not found: %s", config.SourcePicklist)
	}

	// Parse the options
	var stages []entity.PicklistOption
	if field.Options != nil && *field.Options != "" {
		var options []string
		if err := json.Unmarshal([]byte(*field.Options), &options); err != nil {
			return nil, fmt.Errorf("failed to parse options: %w", err)
		}
		for i, opt := range options {
			stages = append(stages, entity.PicklistOption{
				Value: opt,
				Label: opt, // The stored value is the label
				Order: i,
			})
		}
	}

	if stages == nil {
		stages = []entity.PicklistOption{}
	}

	return &entity.BearingWithStages{
		BearingConfig: config,
		Stages:        stages,
	}, nil
}

// ListActiveWithStages returns active bearing configs with resolved stages for an entity
func (r *BearingRepo) ListActiveWithStages(ctx context.Context, orgID, entityType string) ([]entity.BearingWithStages, error) {
	configs, err := r.ListActiveByEntity(ctx, orgID, entityType)
	if err != nil {
		return nil, err
	}

	var result []entity.BearingWithStages
	for _, config := range configs {
		withStages, err := r.GetBearingWithStages(ctx, orgID, config)
		if err != nil {
			// Log but continue - don't fail the whole request
			continue
		}
		result = append(result, *withStages)
	}

	if result == nil {
		result = []entity.BearingWithStages{}
	}

	return result, nil
}
