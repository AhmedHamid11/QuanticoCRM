package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// IngestAPIKeyRepo handles database operations for ingest API keys
type IngestAPIKeyRepo struct {
	db db.DBConn
}

// NewIngestAPIKeyRepo creates a new IngestAPIKeyRepo
// Accepts db.DBConn interface which both *sql.DB and *db.TursoDB satisfy
func NewIngestAPIKeyRepo(conn db.DBConn) *IngestAPIKeyRepo {
	return &IngestAPIKeyRepo{db: conn}
}

// Create creates a new ingest API key
func (r *IngestAPIKeyRepo) Create(ctx context.Context, orgID, createdBy, name, keyHash, keyPrefix string, rateLimit int) (*entity.IngestAPIKey, error) {
	key := &entity.IngestAPIKey{
		ID:        sfid.NewIngestKey(),
		OrgID:     orgID,
		CreatedBy: createdBy,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		IsActive:  true,
		RateLimit: rateLimit,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	query := `
		INSERT INTO ingest_api_keys (id, org_id, created_by, name, key_hash, key_prefix, is_active, rate_limit, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		key.ID, key.OrgID, key.CreatedBy, key.Name, key.KeyHash, key.KeyPrefix,
		key.IsActive, key.RateLimit, key.CreatedAt, key.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// GetByHash retrieves an active key by its hash
func (r *IngestAPIKeyRepo) GetByHash(ctx context.Context, keyHash string) (*entity.IngestAPIKey, error) {
	key := &entity.IngestAPIKey{}
	var createdAt, updatedAt sql.NullString

	query := `
		SELECT id, org_id, created_by, name, key_hash, key_prefix, is_active, rate_limit, created_at, updated_at
		FROM ingest_api_keys
		WHERE key_hash = ? AND is_active = 1
	`

	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID, &key.OrgID, &key.CreatedBy, &key.Name, &key.KeyHash, &key.KeyPrefix,
		&key.IsActive, &key.RateLimit, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	key.CreatedAt = parseTimestamp(createdAt)
	key.UpdatedAt = parseTimestamp(updatedAt)

	return key, nil
}

// ListByOrg retrieves all keys for an organization
func (r *IngestAPIKeyRepo) ListByOrg(ctx context.Context, orgID string) ([]*entity.IngestAPIKey, error) {
	query := `
		SELECT id, org_id, created_by, name, key_hash, key_prefix, is_active, rate_limit, created_at, updated_at
		FROM ingest_api_keys
		WHERE org_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*entity.IngestAPIKey
	for rows.Next() {
		key := &entity.IngestAPIKey{}
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(
			&key.ID, &key.OrgID, &key.CreatedBy, &key.Name, &key.KeyHash, &key.KeyPrefix,
			&key.IsActive, &key.RateLimit, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		key.CreatedAt = parseTimestamp(createdAt)
		key.UpdatedAt = parseTimestamp(updatedAt)

		keys = append(keys, key)
	}

	if keys == nil {
		keys = []*entity.IngestAPIKey{}
	}

	return keys, nil
}

// Deactivate deactivates a key (keeps it in DB for audit trail)
func (r *IngestAPIKeyRepo) Deactivate(ctx context.Context, id, orgID string) error {
	query := `UPDATE ingest_api_keys SET is_active = 0, updated_at = ? WHERE id = ? AND org_id = ?`
	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id, orgID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Delete permanently removes a key
func (r *IngestAPIKeyRepo) Delete(ctx context.Context, id, orgID string) error {
	query := `DELETE FROM ingest_api_keys WHERE id = ? AND org_id = ?`
	result, err := r.db.ExecContext(ctx, query, id, orgID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
