package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
)

// PlatformVersion represents a platform version record
type PlatformVersion struct {
	Version     string    `json:"version"`
	Description string    `json:"description"`
	ReleasedAt  time.Time `json:"releasedAt"`
}

// VersionRepo handles version-related database operations
type VersionRepo struct {
	db db.DBConn
}

// NewVersionRepo creates a new VersionRepo
func NewVersionRepo(dbConn db.DBConn) *VersionRepo {
	return &VersionRepo{db: dbConn}
}

// GetPlatformVersion returns the latest platform version
func (r *VersionRepo) GetPlatformVersion(ctx context.Context) (*PlatformVersion, error) {
	query := `
		SELECT version, description, released_at
		FROM platform_versions
		ORDER BY released_at DESC
		LIMIT 1
	`

	var pv PlatformVersion
	var releasedAtStr string

	err := r.db.QueryRowContext(ctx, query).Scan(&pv.Version, &pv.Description, &releasedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default version if no records exist
			return &PlatformVersion{
				Version:     "v0.1.0",
				Description: "Initial version",
				ReleasedAt:  time.Now(),
			}, nil
		}
		return nil, err
	}

	// Parse the released_at timestamp
	pv.ReleasedAt, _ = time.Parse(time.RFC3339, releasedAtStr)
	if pv.ReleasedAt.IsZero() {
		pv.ReleasedAt, _ = time.Parse("2006-01-02 15:04:05", releasedAtStr)
	}

	return &pv, nil
}

// GetOrgVersion returns the current version for an organization
func (r *VersionRepo) GetOrgVersion(ctx context.Context, orgID string) (string, error) {
	query := `SELECT current_version FROM organizations WHERE id = ?`

	var version sql.NullString
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "v0.1.0", nil // Default version
		}
		return "", err
	}

	if !version.Valid || version.String == "" {
		return "v0.1.0", nil
	}

	return version.String, nil
}

// GetVersionHistory returns all platform versions in descending order
func (r *VersionRepo) GetVersionHistory(ctx context.Context, limit int) ([]PlatformVersion, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT version, description, released_at
		FROM platform_versions
		ORDER BY released_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []PlatformVersion
	for rows.Next() {
		var pv PlatformVersion
		var releasedAtStr string
		if err := rows.Scan(&pv.Version, &pv.Description, &releasedAtStr); err != nil {
			return nil, err
		}
		pv.ReleasedAt, _ = time.Parse(time.RFC3339, releasedAtStr)
		if pv.ReleasedAt.IsZero() {
			pv.ReleasedAt, _ = time.Parse("2006-01-02 15:04:05", releasedAtStr)
		}
		versions = append(versions, pv)
	}

	return versions, rows.Err()
}
