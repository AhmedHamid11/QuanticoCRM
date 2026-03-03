package repo

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// ListViewRepo handles list view database operations
type ListViewRepo struct {
	conn db.DBConn
}

// NewListViewRepo creates a new ListViewRepo
func NewListViewRepo(conn db.DBConn) *ListViewRepo {
	return &ListViewRepo{conn: conn}
}

// WithDB returns a new ListViewRepo using the specified database connection
// This is used for multi-tenant database routing
// Accepts db.DBConn interface for retry-enabled connections
func (r *ListViewRepo) WithDB(conn db.DBConn) *ListViewRepo {
	if conn == nil {
		return r
	}
	return &ListViewRepo{conn: conn}
}

// DB returns the current database connection
func (r *ListViewRepo) DB() db.DBConn {
	return r.conn
}

// List returns all list views for an entity
func (r *ListViewRepo) List(ctx context.Context, orgID, entityName string) ([]entity.ListView, error) {
	query := `
		SELECT id, org_id, entity_name, name, filter_query, columns, sort_by, sort_dir,
		       is_default, is_system, created_by_id, created_at, modified_at
		FROM list_views
		WHERE org_id = ? AND entity_name = ?
		ORDER BY is_default DESC, name ASC
	`

	rows, err := r.conn.QueryContext(ctx, query, orgID, entityName)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []entity.ListView{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var views []entity.ListView
	for rows.Next() {
		var v entity.ListView
		var createdAt, modifiedAt string
		var createdByID sql.NullString
		err := rows.Scan(
			&v.ID, &v.OrgID, &v.EntityName, &v.Name, &v.FilterQuery, &v.Columns,
			&v.SortBy, &v.SortDir, &v.IsDefault, &v.IsSystem, &createdByID,
			&createdAt, &modifiedAt,
		)
		if err != nil {
			return nil, err
		}
		v.CreatedByID = createdByID.String
		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		v.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		views = append(views, v)
	}

	return views, nil
}

// Get returns a list view by ID
func (r *ListViewRepo) Get(ctx context.Context, orgID, id string) (*entity.ListView, error) {
	query := `
		SELECT id, org_id, entity_name, name, filter_query, columns, sort_by, sort_dir,
		       is_default, is_system, created_by_id, created_at, modified_at
		FROM list_views
		WHERE org_id = ? AND id = ?
	`

	var v entity.ListView
	var createdAt, modifiedAt string
	var createdByID sql.NullString
	err := r.conn.QueryRowContext(ctx, query, orgID, id).Scan(
		&v.ID, &v.OrgID, &v.EntityName, &v.Name, &v.FilterQuery, &v.Columns,
		&v.SortBy, &v.SortDir, &v.IsDefault, &v.IsSystem, &createdByID,
		&createdAt, &modifiedAt,
	)
	if err != nil {
		return nil, err
	}
	v.CreatedByID = createdByID.String
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	v.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &v, nil
}

// GetDefault returns the default list view for an entity
func (r *ListViewRepo) GetDefault(ctx context.Context, orgID, entityName string) (*entity.ListView, error) {
	query := `
		SELECT id, org_id, entity_name, name, filter_query, columns, sort_by, sort_dir,
		       is_default, is_system, created_by_id, created_at, modified_at
		FROM list_views
		WHERE org_id = ? AND entity_name = ? AND is_default = 1
		LIMIT 1
	`

	var v entity.ListView
	var createdAt, modifiedAt string
	var createdByID sql.NullString
	err := r.conn.QueryRowContext(ctx, query, orgID, entityName).Scan(
		&v.ID, &v.OrgID, &v.EntityName, &v.Name, &v.FilterQuery, &v.Columns,
		&v.SortBy, &v.SortDir, &v.IsDefault, &v.IsSystem, &createdByID,
		&createdAt, &modifiedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	v.CreatedByID = createdByID.String
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	v.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	return &v, nil
}

// Create creates a new list view
func (r *ListViewRepo) Create(ctx context.Context, v *entity.ListView) error {
	v.ID = sfid.New("0Lv")
	now := time.Now().UTC()
	v.CreatedAt = now
	v.ModifiedAt = now

	// If this is default, unset other defaults first
	if v.IsDefault {
		_, err := r.conn.ExecContext(ctx,
			"UPDATE list_views SET is_default = 0 WHERE org_id = ? AND entity_name = ?",
			v.OrgID, v.EntityName)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO list_views (id, org_id, entity_name, name, filter_query, columns, sort_by, sort_dir,
		                        is_default, is_system, created_by_id, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.conn.ExecContext(ctx, query,
		v.ID, v.OrgID, v.EntityName, v.Name, v.FilterQuery, v.Columns,
		v.SortBy, v.SortDir, v.IsDefault, v.IsSystem, v.CreatedByID,
		v.CreatedAt.Format(time.RFC3339), v.ModifiedAt.Format(time.RFC3339),
	)
	return err
}

// Update updates a list view
func (r *ListViewRepo) Update(ctx context.Context, v *entity.ListView) error {
	v.ModifiedAt = time.Now().UTC()

	// If this is default, unset other defaults first
	if v.IsDefault {
		_, err := r.conn.ExecContext(ctx,
			"UPDATE list_views SET is_default = 0 WHERE org_id = ? AND entity_name = ? AND id != ?",
			v.OrgID, v.EntityName, v.ID)
		if err != nil {
			return err
		}
	}

	query := `
		UPDATE list_views
		SET name = ?, filter_query = ?, columns = ?, sort_by = ?, sort_dir = ?,
		    is_default = ?, modified_at = ?
		WHERE org_id = ? AND id = ?
	`

	_, err := r.conn.ExecContext(ctx, query,
		v.Name, v.FilterQuery, v.Columns, v.SortBy, v.SortDir,
		v.IsDefault, v.ModifiedAt.Format(time.RFC3339),
		v.OrgID, v.ID,
	)
	return err
}

// Delete deletes a list view
func (r *ListViewRepo) Delete(ctx context.Context, orgID, id string) error {
	_, err := r.conn.ExecContext(ctx, "DELETE FROM list_views WHERE org_id = ? AND id = ?", orgID, id)
	return err
}

// EnsureSchema ensures the list_views table has the correct columns
// This handles migration from old schema to new schema
func (r *ListViewRepo) EnsureSchema(ctx context.Context) {
	// Try to add missing columns - ignore errors if they already exist
	migrations := []string{
		`ALTER TABLE list_views ADD COLUMN filter_query TEXT DEFAULT ''`,
		`ALTER TABLE list_views ADD COLUMN sort_by TEXT DEFAULT ''`,
		`ALTER TABLE list_views ADD COLUMN sort_dir TEXT DEFAULT 'desc'`,
		`ALTER TABLE list_views ADD COLUMN created_by_id TEXT`,
	}
	for _, m := range migrations {
		r.conn.ExecContext(ctx, m)
	}
}
