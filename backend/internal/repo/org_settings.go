package repo

import (
	"context"
	"database/sql"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// OrgSettingsRepo handles database operations for org settings
type OrgSettingsRepo struct {
	db db.DBConn
}

// NewOrgSettingsRepo creates a new OrgSettingsRepo
func NewOrgSettingsRepo(conn db.DBConn) *OrgSettingsRepo {
	return &OrgSettingsRepo{db: conn}
}

// WithDB returns a new repo with a different DB connection (for multi-tenant)
func (r *OrgSettingsRepo) WithDB(conn db.DBConn) *OrgSettingsRepo {
	return &OrgSettingsRepo{db: conn}
}

// Get retrieves org settings, creating default if not exists
func (r *OrgSettingsRepo) Get(ctx context.Context, orgID string) (*entity.OrgSettings, error) {
	var settings entity.OrgSettings
	err := r.db.QueryRowContext(ctx,
		`SELECT org_id, home_page, COALESCE(settings_json, '{}')
		 FROM org_settings WHERE org_id = ?`, orgID).Scan(
		&settings.OrgID,
		&settings.HomePage,
		&settings.SettingsJSON,
	)

	if err == sql.ErrNoRows {
		// Create default settings
		settings = entity.OrgSettings{
			OrgID:    orgID,
			HomePage: "/",
		}
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO org_settings (org_id, home_page) VALUES (?, ?)`,
			orgID, settings.HomePage)
		if err != nil {
			return nil, err
		}
		return &settings, nil
	}

	if err != nil {
		return nil, err
	}

	return &settings, nil
}

// UpdateHomePage updates the homepage setting for an org
func (r *OrgSettingsRepo) UpdateHomePage(ctx context.Context, orgID string, homePage string) (*entity.OrgSettings, error) {
	// Upsert the setting
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO org_settings (org_id, home_page, modified_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(org_id) DO UPDATE SET
		 home_page = excluded.home_page,
		 modified_at = datetime('now')`,
		orgID, homePage)

	if err != nil {
		return nil, err
	}

	return r.Get(ctx, orgID)
}
