package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// NavigationRepo handles database operations for navigation tabs
type NavigationRepo struct {
	db db.DBConn
}

// NewNavigationRepo creates a new NavigationRepo
func NewNavigationRepo(dbConn db.DBConn) *NavigationRepo {
	return &NavigationRepo{db: dbConn}
}

// WithDB returns a new NavigationRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *NavigationRepo) WithDB(dbConn db.DBConn) *NavigationRepo {
	if dbConn == nil {
		return r
	}
	return &NavigationRepo{db: dbConn}
}

// WithRawDB returns a new NavigationRepo using a raw *sql.DB connection
// This is used for multi-tenant database routing with tenant databases
func (r *NavigationRepo) WithRawDB(rawDB *sql.DB) *NavigationRepo {
	if rawDB == nil {
		return r
	}
	return &NavigationRepo{db: rawDB}
}

// List retrieves all navigation tabs for an org ordered by sort_order
func (r *NavigationRepo) List(ctx context.Context, orgID string) ([]entity.NavigationTab, error) {
	query := `
		SELECT id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at
		FROM navigation_tabs
		WHERE org_id = ?
		ORDER BY sort_order ASC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list navigation tabs: %w", err)
	}
	defer rows.Close()

	var tabs []entity.NavigationTab
	for rows.Next() {
		var tab entity.NavigationTab
		var createdAt, modifiedAt string

		if err := rows.Scan(
			&tab.ID, &tab.OrgID, &tab.Label, &tab.Href, &tab.Icon, &tab.EntityName,
			&tab.SortOrder, &tab.IsVisible, &tab.IsSystem, &createdAt, &modifiedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan navigation tab: %w", err)
		}

		tab.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tab.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		tabs = append(tabs, tab)
	}

	if tabs == nil {
		tabs = []entity.NavigationTab{}
	}

	return tabs, nil
}

// ListVisible retrieves only visible navigation tabs for an org
func (r *NavigationRepo) ListVisible(ctx context.Context, orgID string) ([]entity.NavigationTab, error) {
	query := `
		SELECT id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at
		FROM navigation_tabs
		WHERE org_id = ? AND is_visible = 1
		ORDER BY sort_order ASC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list visible navigation tabs: %w", err)
	}
	defer rows.Close()

	var tabs []entity.NavigationTab
	for rows.Next() {
		var tab entity.NavigationTab
		var createdAt, modifiedAt string

		if err := rows.Scan(
			&tab.ID, &tab.OrgID, &tab.Label, &tab.Href, &tab.Icon, &tab.EntityName,
			&tab.SortOrder, &tab.IsVisible, &tab.IsSystem, &createdAt, &modifiedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan navigation tab: %w", err)
		}

		tab.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tab.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		tabs = append(tabs, tab)
	}

	if tabs == nil {
		tabs = []entity.NavigationTab{}
	}

	return tabs, nil
}

// GetByID retrieves a navigation tab by ID for an org
func (r *NavigationRepo) GetByID(ctx context.Context, orgID, id string) (*entity.NavigationTab, error) {
	query := `
		SELECT id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at
		FROM navigation_tabs
		WHERE org_id = ? AND id = ?
	`

	var tab entity.NavigationTab
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, id).Scan(
		&tab.ID, &tab.OrgID, &tab.Label, &tab.Href, &tab.Icon, &tab.EntityName,
		&tab.SortOrder, &tab.IsVisible, &tab.IsSystem, &createdAt, &modifiedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get navigation tab: %w", err)
	}

	tab.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	tab.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &tab, nil
}

// GetByHref retrieves a navigation tab by href for an org
func (r *NavigationRepo) GetByHref(ctx context.Context, orgID, href string) (*entity.NavigationTab, error) {
	query := `
		SELECT id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at
		FROM navigation_tabs
		WHERE org_id = ? AND href = ?
	`

	var tab entity.NavigationTab
	var createdAt, modifiedAt string

	err := r.db.QueryRowContext(ctx, query, orgID, href).Scan(
		&tab.ID, &tab.OrgID, &tab.Label, &tab.Href, &tab.Icon, &tab.EntityName,
		&tab.SortOrder, &tab.IsVisible, &tab.IsSystem, &createdAt, &modifiedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get navigation tab by href: %w", err)
	}

	tab.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	tab.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &tab, nil
}

// Create creates a new navigation tab for an org
func (r *NavigationRepo) Create(ctx context.Context, orgID string, input entity.NavigationTabCreateInput) (*entity.NavigationTab, error) {
	// Get max sort order if not provided
	sortOrder := input.SortOrder
	if sortOrder == 0 {
		var maxOrder int
		err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) FROM navigation_tabs WHERE org_id = ?", orgID).Scan(&maxOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to get max sort order: %w", err)
		}
		sortOrder = maxOrder + 1
	}

	tab := &entity.NavigationTab{
		ID:         sfid.New("Nav"),
		OrgID:      orgID,
		Label:      input.Label,
		Href:       input.Href,
		Icon:       input.Icon,
		EntityName: input.EntityName,
		SortOrder:  sortOrder,
		IsVisible:  input.IsVisible,
		IsSystem:   false,
		CreatedAt:  time.Now().UTC(),
		ModifiedAt: time.Now().UTC(),
	}

	query := `
		INSERT INTO navigation_tabs (id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		tab.ID, tab.OrgID, tab.Label, tab.Href, tab.Icon, tab.EntityName,
		tab.SortOrder, tab.IsVisible, tab.IsSystem,
		tab.CreatedAt.Format(time.RFC3339), tab.ModifiedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create navigation tab: %w", err)
	}

	return tab, nil
}

// Update updates an existing navigation tab for an org
func (r *NavigationRepo) Update(ctx context.Context, orgID, id string, input entity.NavigationTabUpdateInput) (*entity.NavigationTab, error) {
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
	if input.Href != nil && !existing.IsSystem {
		// System tabs cannot change href
		existing.Href = *input.Href
	}
	if input.Icon != nil {
		existing.Icon = *input.Icon
	}
	if input.EntityName != nil {
		existing.EntityName = input.EntityName
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}
	if input.IsVisible != nil {
		existing.IsVisible = *input.IsVisible
	}

	existing.ModifiedAt = time.Now().UTC()

	query := `
		UPDATE navigation_tabs SET
			label = ?, href = ?, icon = ?, entity_name = ?, sort_order = ?, is_visible = ?, modified_at = ?
		WHERE org_id = ? AND id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Label, existing.Href, existing.Icon, existing.EntityName, existing.SortOrder, existing.IsVisible,
		existing.ModifiedAt.Format(time.RFC3339), orgID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update navigation tab: %w", err)
	}

	return existing, nil
}

// Delete deletes a navigation tab for an org
func (r *NavigationRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM navigation_tabs WHERE org_id = ? AND id = ?`
	result, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete navigation tab: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Reorder updates the sort order of tabs for an org
func (r *NavigationRepo) Reorder(ctx context.Context, orgID string, tabIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, id := range tabIDs {
		_, err := tx.ExecContext(ctx,
			"UPDATE navigation_tabs SET sort_order = ?, modified_at = ? WHERE org_id = ? AND id = ?",
			i+1, time.Now().UTC().Format(time.RFC3339), orgID, id,
		)
		if err != nil {
			return fmt.Errorf("failed to update tab order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
