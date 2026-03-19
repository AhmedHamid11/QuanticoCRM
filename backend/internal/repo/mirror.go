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
)

// MirrorRepo handles CRUD operations for mirrors in tenant databases
type MirrorRepo struct {
	db db.DBConn
}

// NewMirrorRepo creates a new MirrorRepo
func NewMirrorRepo(conn db.DBConn) *MirrorRepo {
	return &MirrorRepo{db: conn}
}

// Create creates a new mirror in the tenant database
func (r *MirrorRepo) Create(ctx context.Context, tenantDB db.DBConn, orgID string, input entity.MirrorCreateInput) (*entity.Mirror, error) {
	// Generate mirror ID
	mirrorID := sfid.NewMirror()

	// Set defaults
	unmappedFieldMode := input.UnmappedFieldMode
	if unmappedFieldMode == "" {
		unmappedFieldMode = entity.UnmappedFieldModeFlexible
	}

	rateLimit := 500
	if input.RateLimit != nil {
		rateLimit = *input.RateLimit
	}

	// Begin transaction
	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert mirror
	upsertModeInt := 0
	if input.UpsertMode {
		upsertModeInt = 1
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mirrors (id, org_id, name, target_entity, unique_key_field, unmapped_field_mode, rate_limit, is_active, upsert_mode, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)
	`, mirrorID, orgID, input.Name, input.TargetEntity, input.UniqueKeyField, unmappedFieldMode, rateLimit, upsertModeInt, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, fmt.Errorf("no such table: mirrors")
		}
		return nil, fmt.Errorf("insert mirror: %w", err)
	}

	// Insert source fields if provided
	sourceFields := []entity.MirrorSourceField{}
	if len(input.SourceFields) > 0 {
		for i, fieldInput := range input.SourceFields {
			fieldID := sfid.NewMirrorField()
			fieldType := fieldInput.FieldType
			if fieldType == "" {
				fieldType = "text"
			}

			// Handle map_field - use NULL if empty string
			var mapFieldArg interface{}
			if fieldInput.MapField != "" {
				mapFieldArg = fieldInput.MapField
			} else {
				mapFieldArg = nil
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO mirror_source_fields (id, mirror_id, field_name, field_type, is_required, description, map_field, sort_order, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, fieldID, mirrorID, fieldInput.FieldName, fieldType, fieldInput.IsRequired, fieldInput.Description, mapFieldArg, i, now.Format(time.RFC3339))
			if err != nil {
				return nil, fmt.Errorf("insert source field: %w", err)
			}

			var mapFieldPtr *string
			if fieldInput.MapField != "" {
				mapFieldPtr = &fieldInput.MapField
			}

			sourceFields = append(sourceFields, entity.MirrorSourceField{
				ID:          fieldID,
				MirrorID:    mirrorID,
				FieldName:   fieldInput.FieldName,
				FieldType:   fieldType,
				IsRequired:  fieldInput.IsRequired,
				Description: fieldInput.Description,
				MapField:    mapFieldPtr,
				SortOrder:   i,
				CreatedAt:   now,
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &entity.Mirror{
		ID:                mirrorID,
		OrgID:             orgID,
		Name:              input.Name,
		TargetEntity:      input.TargetEntity,
		UniqueKeyField:    input.UniqueKeyField,
		UnmappedFieldMode: unmappedFieldMode,
		RateLimit:         rateLimit,
		IsActive:          true,
		UpsertMode:        input.UpsertMode,
		SourceFields:      sourceFields,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// GetByID retrieves a mirror by ID from the tenant database
func (r *MirrorRepo) GetByID(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string) (*entity.Mirror, error) {
	var mirror entity.Mirror
	var createdAt, updatedAt string
	var isActiveInt, upsertModeInt int

	err := tenantDB.QueryRowContext(ctx, `
		SELECT id, org_id, name, target_entity, unique_key_field, unmapped_field_mode, rate_limit, is_active, upsert_mode, created_at, updated_at
		FROM mirrors
		WHERE id = ? AND org_id = ?
	`, mirrorID, orgID).Scan(
		&mirror.ID, &mirror.OrgID, &mirror.Name, &mirror.TargetEntity, &mirror.UniqueKeyField,
		&mirror.UnmappedFieldMode, &mirror.RateLimit, &isActiveInt, &upsertModeInt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get mirror: %w", err)
	}

	mirror.IsActive = isActiveInt == 1
	mirror.UpsertMode = upsertModeInt == 1
	mirror.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	mirror.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Fetch source fields
	sourceFields, err := r.getSourceFields(ctx, tenantDB, mirrorID)
	if err != nil {
		return nil, fmt.Errorf("get source fields: %w", err)
	}
	mirror.SourceFields = sourceFields

	return &mirror, nil
}

// getSourceFields retrieves all source fields for a mirror
func (r *MirrorRepo) getSourceFields(ctx context.Context, tenantDB db.DBConn, mirrorID string) ([]entity.MirrorSourceField, error) {
	rows, err := tenantDB.QueryContext(ctx, `
		SELECT id, mirror_id, field_name, field_type, is_required, description, map_field, sort_order, created_at
		FROM mirror_source_fields
		WHERE mirror_id = ?
		ORDER BY sort_order ASC
	`, mirrorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := []entity.MirrorSourceField{}
	for rows.Next() {
		var field entity.MirrorSourceField
		var isRequiredInt int
		var createdAt string
		var mapField sql.NullString

		err := rows.Scan(&field.ID, &field.MirrorID, &field.FieldName, &field.FieldType, &isRequiredInt, &field.Description, &mapField, &field.SortOrder, &createdAt)
		if err != nil {
			return nil, err
		}

		field.IsRequired = isRequiredInt == 1
		field.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

		// Convert sql.NullString to *string
		if mapField.Valid {
			field.MapField = &mapField.String
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// ListByOrg retrieves all mirrors for an organization
func (r *MirrorRepo) ListByOrg(ctx context.Context, tenantDB db.DBConn, orgID string) ([]*entity.Mirror, error) {
	rows, err := tenantDB.QueryContext(ctx, `
		SELECT id, org_id, name, target_entity, unique_key_field, unmapped_field_mode, rate_limit, is_active, upsert_mode, created_at, updated_at
		FROM mirrors
		WHERE org_id = ?
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, fmt.Errorf("no such table: mirrors")
		}
		return nil, fmt.Errorf("list mirrors: %w", err)
	}
	defer rows.Close()

	mirrors := []*entity.Mirror{}
	for rows.Next() {
		var mirror entity.Mirror
		var createdAt, updatedAt string
		var isActiveInt, upsertModeInt int

		err := rows.Scan(
			&mirror.ID, &mirror.OrgID, &mirror.Name, &mirror.TargetEntity, &mirror.UniqueKeyField,
			&mirror.UnmappedFieldMode, &mirror.RateLimit, &isActiveInt, &upsertModeInt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		mirror.IsActive = isActiveInt == 1
		mirror.UpsertMode = upsertModeInt == 1
		mirror.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		mirror.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		// Fetch source fields for each mirror
		sourceFields, err := r.getSourceFields(ctx, tenantDB, mirror.ID)
		if err != nil {
			return nil, fmt.Errorf("get source fields for mirror %s: %w", mirror.ID, err)
		}
		mirror.SourceFields = sourceFields

		mirrors = append(mirrors, &mirror)
	}

	return mirrors, nil
}

// Update updates a mirror in the tenant database
func (r *MirrorRepo) Update(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string, input entity.MirrorUpdateInput) (*entity.Mirror, error) {
	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}

	if input.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *input.Name)
	}
	if input.TargetEntity != nil {
		setClauses = append(setClauses, "target_entity = ?")
		args = append(args, *input.TargetEntity)
	}
	if input.UniqueKeyField != nil {
		setClauses = append(setClauses, "unique_key_field = ?")
		args = append(args, *input.UniqueKeyField)
	}
	if input.UnmappedFieldMode != nil {
		setClauses = append(setClauses, "unmapped_field_mode = ?")
		args = append(args, *input.UnmappedFieldMode)
	}
	if input.RateLimit != nil {
		setClauses = append(setClauses, "rate_limit = ?")
		args = append(args, *input.RateLimit)
	}
	if input.IsActive != nil {
		isActiveInt := 0
		if *input.IsActive {
			isActiveInt = 1
		}
		setClauses = append(setClauses, "is_active = ?")
		args = append(args, isActiveInt)
	}
	if input.UpsertMode != nil {
		upsertModeInt := 0
		if *input.UpsertMode {
			upsertModeInt = 1
		}
		setClauses = append(setClauses, "upsert_mode = ?")
		args = append(args, upsertModeInt)
	}

	// Always update updated_at
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339))

	// Add WHERE clause args
	args = append(args, mirrorID, orgID)

	// Begin transaction
	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update mirror if there are changes
	if len(setClauses) > 1 { // > 1 because updated_at is always included
		query := fmt.Sprintf("UPDATE mirrors SET %s WHERE id = ? AND org_id = ?", strings.Join(setClauses, ", "))
		_, err = tx.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("update mirror: %w", err)
		}
	}

	// Handle source fields replacement if provided
	if input.SourceFields != nil {
		// Delete existing source fields
		_, err = tx.ExecContext(ctx, "DELETE FROM mirror_source_fields WHERE mirror_id = ?", mirrorID)
		if err != nil {
			return nil, fmt.Errorf("delete existing source fields: %w", err)
		}

		// Insert new source fields
		now := time.Now().UTC()
		for i, fieldInput := range *input.SourceFields {
			fieldID := sfid.NewMirrorField()
			fieldType := fieldInput.FieldType
			if fieldType == "" {
				fieldType = "text"
			}

			// Handle map_field - use NULL if empty string
			var mapFieldArg interface{}
			if fieldInput.MapField != "" {
				mapFieldArg = fieldInput.MapField
			} else {
				mapFieldArg = nil
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO mirror_source_fields (id, mirror_id, field_name, field_type, is_required, description, map_field, sort_order, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, fieldID, mirrorID, fieldInput.FieldName, fieldType, fieldInput.IsRequired, fieldInput.Description, mapFieldArg, i, now.Format(time.RFC3339))
			if err != nil {
				return nil, fmt.Errorf("insert source field: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Fetch and return updated mirror
	return r.GetByID(ctx, tenantDB, orgID, mirrorID)
}

// Delete deletes a mirror from the tenant database
func (r *MirrorRepo) Delete(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string) error {
	result, err := tenantDB.ExecContext(ctx, "DELETE FROM mirrors WHERE id = ? AND org_id = ?", mirrorID, orgID)
	if err != nil {
		return fmt.Errorf("delete mirror: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("mirror not found")
	}

	return nil
}

// GetActiveByID retrieves an active mirror by ID from the tenant database
func (r *MirrorRepo) GetActiveByID(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string) (*entity.Mirror, error) {
	var mirror entity.Mirror
	var createdAt, updatedAt string
	var isActiveInt, upsertModeInt int

	err := tenantDB.QueryRowContext(ctx, `
		SELECT id, org_id, name, target_entity, unique_key_field, unmapped_field_mode, rate_limit, is_active, upsert_mode, created_at, updated_at
		FROM mirrors
		WHERE id = ? AND org_id = ? AND is_active = 1
	`, mirrorID, orgID).Scan(
		&mirror.ID, &mirror.OrgID, &mirror.Name, &mirror.TargetEntity, &mirror.UniqueKeyField,
		&mirror.UnmappedFieldMode, &mirror.RateLimit, &isActiveInt, &upsertModeInt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active mirror: %w", err)
	}

	mirror.IsActive = isActiveInt == 1
	mirror.UpsertMode = upsertModeInt == 1
	mirror.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	mirror.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Fetch source fields
	sourceFields, err := r.getSourceFields(ctx, tenantDB, mirrorID)
	if err != nil {
		return nil, fmt.Errorf("get source fields: %w", err)
	}
	mirror.SourceFields = sourceFields

	return &mirror, nil
}
