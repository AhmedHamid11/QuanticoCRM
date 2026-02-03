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

// CustomPageRepo handles database operations for custom pages
type CustomPageRepo struct {
	db db.DBConn
}

// NewCustomPageRepo creates a new CustomPageRepo
func NewCustomPageRepo(conn db.DBConn) *CustomPageRepo {
	return &CustomPageRepo{db: conn}
}

// WithDB returns a new CustomPageRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *CustomPageRepo) WithDB(conn db.DBConn) *CustomPageRepo {
	if conn == nil {
		return r
	}
	return &CustomPageRepo{db: conn}
}

// DB returns the current database connection
func (r *CustomPageRepo) DB() db.DBConn {
	return r.db
}

// List retrieves all custom pages for an org
func (r *CustomPageRepo) List(ctx context.Context, orgID string) ([]entity.CustomPageListItem, error) {
	query := `
		SELECT id, slug, title, description, icon, is_enabled, is_public, sort_order, modified_at
		FROM custom_pages
		WHERE org_id = ?
		ORDER BY sort_order ASC, title ASC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list custom pages: %w", err)
	}
	defer rows.Close()

	var pages []entity.CustomPageListItem
	for rows.Next() {
		var page entity.CustomPageListItem
		var modifiedAt string
		var description sql.NullString

		if err := rows.Scan(
			&page.ID, &page.Slug, &page.Title, &description,
			&page.Icon, &page.IsEnabled, &page.IsPublic, &page.SortOrder, &modifiedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan custom page: %w", err)
		}

		if description.Valid {
			page.Description = description.String
		}
		page.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		pages = append(pages, page)
	}

	if pages == nil {
		pages = []entity.CustomPageListItem{}
	}

	return pages, nil
}

// ListEnabled retrieves only enabled custom pages for an org (for non-admin users)
func (r *CustomPageRepo) ListEnabled(ctx context.Context, orgID string, includeAdminOnly bool) ([]entity.CustomPageListItem, error) {
	var query string
	if includeAdminOnly {
		query = `
			SELECT id, slug, title, description, icon, is_enabled, is_public, sort_order, modified_at
			FROM custom_pages
			WHERE org_id = ? AND is_enabled = 1
			ORDER BY sort_order ASC, title ASC
		`
	} else {
		query = `
			SELECT id, slug, title, description, icon, is_enabled, is_public, sort_order, modified_at
			FROM custom_pages
			WHERE org_id = ? AND is_enabled = 1 AND is_public = 1
			ORDER BY sort_order ASC, title ASC
		`
	}

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled custom pages: %w", err)
	}
	defer rows.Close()

	var pages []entity.CustomPageListItem
	for rows.Next() {
		var page entity.CustomPageListItem
		var modifiedAt string
		var description sql.NullString

		if err := rows.Scan(
			&page.ID, &page.Slug, &page.Title, &description,
			&page.Icon, &page.IsEnabled, &page.IsPublic, &page.SortOrder, &modifiedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan custom page: %w", err)
		}

		if description.Valid {
			page.Description = description.String
		}
		page.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		pages = append(pages, page)
	}

	if pages == nil {
		pages = []entity.CustomPageListItem{}
	}

	return pages, nil
}

// GetByID retrieves a custom page by ID
func (r *CustomPageRepo) GetByID(ctx context.Context, orgID, id string) (*entity.CustomPage, error) {
	query := `
		SELECT id, org_id, slug, title, description, icon, is_enabled, is_public, layout, components,
			   sort_order, created_at, modified_at, created_by, modified_by
		FROM custom_pages
		WHERE org_id = ? AND id = ?
	`

	var page entity.CustomPage
	var componentsJSON string
	var description, createdBy, modifiedBy sql.NullString
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, id).Scan(
		&page.ID, &page.OrgID, &page.Slug, &page.Title, &description, &page.Icon,
		&page.IsEnabled, &page.IsPublic, &page.Layout, &componentsJSON,
		&page.SortOrder, &createdAt, &modifiedAt, &createdBy, &modifiedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get custom page: %w", err)
	}

	if description.Valid {
		page.Description = description.String
	}
	if createdBy.Valid {
		page.CreatedBy = &createdBy.String
	}
	if modifiedBy.Valid {
		page.ModifiedBy = &modifiedBy.String
	}

	page.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	page.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Parse components JSON
	if err := json.Unmarshal([]byte(componentsJSON), &page.Components); err != nil {
		page.Components = []entity.PageComponent{}
	}

	return &page, nil
}

// GetBySlug retrieves a custom page by slug
func (r *CustomPageRepo) GetBySlug(ctx context.Context, orgID, slug string) (*entity.CustomPage, error) {
	query := `
		SELECT id, org_id, slug, title, description, icon, is_enabled, is_public, layout, components,
			   sort_order, created_at, modified_at, created_by, modified_by
		FROM custom_pages
		WHERE org_id = ? AND slug = ?
	`

	var page entity.CustomPage
	var componentsJSON string
	var description, createdBy, modifiedBy sql.NullString
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, slug).Scan(
		&page.ID, &page.OrgID, &page.Slug, &page.Title, &description, &page.Icon,
		&page.IsEnabled, &page.IsPublic, &page.Layout, &componentsJSON,
		&page.SortOrder, &createdAt, &modifiedAt, &createdBy, &modifiedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get custom page by slug: %w", err)
	}

	if description.Valid {
		page.Description = description.String
	}
	if createdBy.Valid {
		page.CreatedBy = &createdBy.String
	}
	if modifiedBy.Valid {
		page.ModifiedBy = &modifiedBy.String
	}

	page.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	page.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Parse components JSON
	if err := json.Unmarshal([]byte(componentsJSON), &page.Components); err != nil {
		page.Components = []entity.PageComponent{}
	}

	return &page, nil
}

// Create creates a new custom page
func (r *CustomPageRepo) Create(ctx context.Context, orgID string, input entity.CustomPageCreateInput, userID string) (*entity.CustomPage, error) {
	// Get max sort order if not provided
	sortOrder := input.SortOrder
	if sortOrder == 0 {
		var maxOrder int
		err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) FROM custom_pages WHERE org_id = ?", orgID).Scan(&maxOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to get max sort order: %w", err)
		}
		sortOrder = maxOrder + 1
	}

	// Default layout
	layout := input.Layout
	if layout == "" {
		layout = "single"
	}

	// Default icon
	icon := input.Icon
	if icon == "" {
		icon = "file"
	}

	// Serialize components
	components := input.Components
	if components == nil {
		components = []entity.PageComponent{}
	}
	componentsJSON, err := json.Marshal(components)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize components: %w", err)
	}

	page := &entity.CustomPage{
		ID:          sfid.NewCustomPage(),
		OrgID:       orgID,
		Slug:        input.Slug,
		Title:       input.Title,
		Description: input.Description,
		Icon:        icon,
		IsEnabled:   input.IsEnabled,
		IsPublic:    input.IsPublic,
		Layout:      layout,
		Components:  components,
		SortOrder:   sortOrder,
		CreatedAt:   time.Now().UTC(),
		ModifiedAt:  time.Now().UTC(),
		CreatedBy:   &userID,
		ModifiedBy:  &userID,
	}

	query := `
		INSERT INTO custom_pages (id, org_id, slug, title, description, icon, is_enabled, is_public, layout, components, sort_order, created_at, modified_at, created_by, modified_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		page.ID, page.OrgID, page.Slug, page.Title, page.Description, page.Icon,
		page.IsEnabled, page.IsPublic, page.Layout, string(componentsJSON),
		page.SortOrder, page.CreatedAt.Format(time.RFC3339), page.ModifiedAt.Format(time.RFC3339),
		page.CreatedBy, page.ModifiedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom page: %w", err)
	}

	return page, nil
}

// Update updates an existing custom page
func (r *CustomPageRepo) Update(ctx context.Context, orgID, id string, input entity.CustomPageUpdateInput, userID string) (*entity.CustomPage, error) {
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.Slug != nil {
		existing.Slug = *input.Slug
	}
	if input.Title != nil {
		existing.Title = *input.Title
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.Icon != nil {
		existing.Icon = *input.Icon
	}
	if input.IsEnabled != nil {
		existing.IsEnabled = *input.IsEnabled
	}
	if input.IsPublic != nil {
		existing.IsPublic = *input.IsPublic
	}
	if input.Layout != nil {
		existing.Layout = *input.Layout
	}
	if input.Components != nil {
		existing.Components = *input.Components
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}

	existing.ModifiedAt = time.Now().UTC()
	existing.ModifiedBy = &userID

	// Serialize components
	componentsJSON, err := json.Marshal(existing.Components)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize components: %w", err)
	}

	query := `
		UPDATE custom_pages SET
			slug = ?, title = ?, description = ?, icon = ?, is_enabled = ?, is_public = ?,
			layout = ?, components = ?, sort_order = ?, modified_at = ?, modified_by = ?
		WHERE org_id = ? AND id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Slug, existing.Title, existing.Description, existing.Icon,
		existing.IsEnabled, existing.IsPublic, existing.Layout, string(componentsJSON),
		existing.SortOrder, existing.ModifiedAt.Format(time.RFC3339), existing.ModifiedBy,
		orgID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update custom page: %w", err)
	}

	return existing, nil
}

// Delete deletes a custom page
func (r *CustomPageRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM custom_pages WHERE org_id = ? AND id = ?`
	result, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete custom page: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Reorder updates the sort order of pages
func (r *CustomPageRepo) Reorder(ctx context.Context, orgID string, pageIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, id := range pageIDs {
		_, err := tx.ExecContext(ctx,
			"UPDATE custom_pages SET sort_order = ?, modified_at = ? WHERE org_id = ? AND id = ?",
			i+1, time.Now().UTC().Format(time.RFC3339), orgID, id,
		)
		if err != nil {
			return fmt.Errorf("failed to update page order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SlugExists checks if a slug already exists for an org (excluding a specific page ID)
func (r *CustomPageRepo) SlugExists(ctx context.Context, orgID, slug, excludeID string) (bool, error) {
	var count int
	var err error

	if excludeID == "" {
		err = r.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM custom_pages WHERE org_id = ? AND slug = ?",
			orgID, slug,
		).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM custom_pages WHERE org_id = ? AND slug = ? AND id != ?",
			orgID, slug, excludeID,
		).Scan(&count)
	}

	if err != nil {
		return false, fmt.Errorf("failed to check slug existence: %w", err)
	}

	return count > 0, nil
}
