package repo

import (
	"context"
	"database/sql"
	"fmt"

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
		`SELECT org_id, home_page, idle_timeout_minutes, absolute_timeout_minutes, COALESCE(settings_json, '{}')
		 FROM org_settings WHERE org_id = ?`, orgID).Scan(
		&settings.OrgID,
		&settings.HomePage,
		&settings.IdleTimeoutMinutes,
		&settings.AbsoluteTimeoutMinutes,
		&settings.SettingsJSON,
	)

	if err == sql.ErrNoRows {
		// Create default settings
		settings = entity.OrgSettings{
			OrgID:                  orgID,
			HomePage:               "/",
			IdleTimeoutMinutes:     entity.DefaultIdleTimeout,
			AbsoluteTimeoutMinutes: entity.DefaultAbsoluteTimeout,
		}
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO org_settings (org_id, home_page, idle_timeout_minutes, absolute_timeout_minutes) VALUES (?, ?, ?, ?)`,
			orgID, settings.HomePage, settings.IdleTimeoutMinutes, settings.AbsoluteTimeoutMinutes)
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

// Update updates org settings (including session timeouts)
func (r *OrgSettingsRepo) Update(ctx context.Context, orgID string, input *entity.OrgSettingsUpdateInput) (*entity.OrgSettings, error) {
	// Get current settings first
	current, err := r.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if input.HomePage != nil {
		current.HomePage = *input.HomePage
	}
	if input.IdleTimeoutMinutes != nil {
		current.IdleTimeoutMinutes = *input.IdleTimeoutMinutes
	}
	if input.AbsoluteTimeoutMinutes != nil {
		current.AbsoluteTimeoutMinutes = *input.AbsoluteTimeoutMinutes
	}

	// Upsert the settings
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO org_settings (org_id, home_page, idle_timeout_minutes, absolute_timeout_minutes, modified_at)
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(org_id) DO UPDATE SET
		 home_page = excluded.home_page,
		 idle_timeout_minutes = excluded.idle_timeout_minutes,
		 absolute_timeout_minutes = excluded.absolute_timeout_minutes,
		 modified_at = datetime('now')`,
		orgID, current.HomePage, current.IdleTimeoutMinutes, current.AbsoluteTimeoutMinutes)

	if err != nil {
		return nil, err
	}

	return r.Get(ctx, orgID)
}

// ValidateSessionTimeouts validates that session timeout values are within acceptable bounds
func ValidateSessionTimeouts(idle, absolute int) error {
	if idle < entity.MinIdleTimeout || idle > entity.MaxIdleTimeout {
		return fmt.Errorf("idle timeout must be between %d and %d minutes", entity.MinIdleTimeout, entity.MaxIdleTimeout)
	}
	if absolute < entity.MinAbsoluteTimeout || absolute > entity.MaxAbsoluteTimeout {
		return fmt.Errorf("absolute timeout must be between %d and %d minutes (8-72 hours)", entity.MinAbsoluteTimeout, entity.MaxAbsoluteTimeout)
	}
	return nil
}
