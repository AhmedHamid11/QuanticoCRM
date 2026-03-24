package repo

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// OrgFeaturesRepo handles database operations for per-org feature flags
type OrgFeaturesRepo struct {
	db db.DBConn
}

// NewOrgFeaturesRepo creates a new OrgFeaturesRepo
func NewOrgFeaturesRepo(conn db.DBConn) *OrgFeaturesRepo {
	return &OrgFeaturesRepo{db: conn}
}

// WithDB returns a new repo with a different DB connection (for multi-tenant)
func (r *OrgFeaturesRepo) WithDB(conn db.DBConn) *OrgFeaturesRepo {
	return &OrgFeaturesRepo{db: conn}
}

// ensureTable creates the org_features table if it does not exist.
// Returns true if the table was created (or already existed), false on error.
func (r *OrgFeaturesRepo) ensureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS org_features (
			feature_key TEXT PRIMARY KEY,
			enabled INTEGER NOT NULL DEFAULT 0,
			enabled_at TEXT,
			enabled_by TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			modified_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

// ListFeatures returns all known features with their enabled status from the DB.
// Features not present in the DB are returned as disabled.
func (r *OrgFeaturesRepo) ListFeatures(ctx context.Context, orgID string) ([]entity.FeatureStatus, error) {
	// Build a map of DB rows keyed by feature_key
	dbFeatures, err := r.loadDBFeatures(ctx)
	if err != nil {
		// If table missing, auto-create and return all-disabled
		if strings.Contains(err.Error(), "no such table") {
			if createErr := r.ensureTable(ctx); createErr != nil {
				return nil, createErr
			}
			dbFeatures = make(map[string]*entity.OrgFeature)
		} else {
			return nil, err
		}
	}

	// Merge known features with DB state
	result := make([]entity.FeatureStatus, 0, len(entity.KnownFeatures))
	for _, def := range entity.KnownFeatures {
		status := entity.FeatureStatus{
			FeatureDefinition: def,
			Enabled:           false,
		}
		if f, ok := dbFeatures[def.Key]; ok {
			status.Enabled = f.Enabled
			status.EnabledAt = f.EnabledAt
			status.EnabledBy = f.EnabledBy
		}
		result = append(result, status)
	}
	return result, nil
}

// SetFeature upserts a feature flag in the org_features table
func (r *OrgFeaturesRepo) SetFeature(ctx context.Context, featureKey string, enabled bool, enabledBy string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var enabledAt *string
	if enabled {
		enabledAt = &now
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO org_features (feature_key, enabled, enabled_at, enabled_by, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(feature_key) DO UPDATE SET
			enabled = excluded.enabled,
			enabled_at = CASE WHEN excluded.enabled = 1 THEN excluded.enabled_at ELSE NULL END,
			enabled_by = CASE WHEN excluded.enabled = 1 THEN excluded.enabled_by ELSE NULL END,
			modified_at = excluded.modified_at
	`, featureKey, boolToInt(enabled), enabledAt, enabledBy, now, now)

	if err != nil && strings.Contains(err.Error(), "no such table") {
		if createErr := r.ensureTable(ctx); createErr != nil {
			return createErr
		}
		// Retry after creating table
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO org_features (feature_key, enabled, enabled_at, enabled_by, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(feature_key) DO UPDATE SET
				enabled = excluded.enabled,
				enabled_at = CASE WHEN excluded.enabled = 1 THEN excluded.enabled_at ELSE NULL END,
				enabled_by = CASE WHEN excluded.enabled = 1 THEN excluded.enabled_by ELSE NULL END,
				modified_at = excluded.modified_at
		`, featureKey, boolToInt(enabled), enabledAt, enabledBy, now, now)
	}

	return err
}

// IsFeatureEnabled returns whether a specific feature is enabled for the org
func (r *OrgFeaturesRepo) IsFeatureEnabled(ctx context.Context, featureKey string) (bool, error) {
	var enabled int
	err := r.db.QueryRowContext(ctx,
		`SELECT enabled FROM org_features WHERE feature_key = ?`, featureKey).Scan(&enabled)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled == 1, nil
}

// GetEnabledFeatures returns a map of all known features and their enabled status.
// Used by the settings API to include feature flags in the response.
func (r *OrgFeaturesRepo) GetEnabledFeatures(ctx context.Context) (map[string]bool, error) {
	result := make(map[string]bool)

	// Initialize all known features as disabled
	for _, def := range entity.KnownFeatures {
		result[def.Key] = false
	}

	// Load DB state
	dbFeatures, err := r.loadDBFeatures(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			// Table doesn't exist yet — return all disabled
			return result, nil
		}
		return nil, err
	}

	// Override with DB values
	for key, f := range dbFeatures {
		result[key] = f.Enabled
	}

	return result, nil
}

// loadDBFeatures reads all rows from org_features into a map
func (r *OrgFeaturesRepo) loadDBFeatures(ctx context.Context) (map[string]*entity.OrgFeature, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT feature_key, enabled, enabled_at, enabled_by, created_at, modified_at FROM org_features`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	features := make(map[string]*entity.OrgFeature)
	for rows.Next() {
		var f entity.OrgFeature
		var enabled int
		if err := rows.Scan(&f.FeatureKey, &enabled, &f.EnabledAt, &f.EnabledBy, &f.CreatedAt, &f.ModifiedAt); err != nil {
			return nil, err
		}
		f.Enabled = enabled == 1
		features[f.FeatureKey] = &f
	}
	return features, rows.Err()
}

// boolToInt converts a bool to SQLite integer (0 or 1)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
