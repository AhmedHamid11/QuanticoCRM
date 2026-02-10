package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// APITokenRepo handles database operations for API tokens
type APITokenRepo struct {
	db db.DBConn
}

// NewAPITokenRepo creates a new APITokenRepo
// Accepts db.DBConn interface which both *sql.DB and *db.TursoDB satisfy
func NewAPITokenRepo(conn db.DBConn) *APITokenRepo {
	return &APITokenRepo{db: conn}
}

// Create creates a new API token
func (r *APITokenRepo) Create(ctx context.Context, orgID, createdBy, name, tokenHash, tokenPrefix string, scopes []string, expiresAt *time.Time) (*entity.APIToken, error) {
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return nil, err
	}

	token := &entity.APIToken{
		ID:          sfid.NewAPIToken(),
		OrgID:       orgID,
		CreatedBy:   createdBy,
		Name:        name,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	query := `
		INSERT INTO api_tokens (id, org_id, created_by, name, token_hash, token_prefix, scopes, expires_at, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		token.ID, token.OrgID, token.CreatedBy, token.Name, token.TokenHash, token.TokenPrefix,
		string(scopesJSON), token.ExpiresAt, token.IsActive, token.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// GetByHash retrieves an active, non-expired token by its hash
func (r *APITokenRepo) GetByHash(ctx context.Context, tokenHash string) (*entity.APIToken, error) {
	token := &entity.APIToken{}
	var scopesJSON string
	var lastUsedAt, expiresAt, createdAt sql.NullString

	query := `
		SELECT id, org_id, created_by, name, token_hash, token_prefix, scopes, last_used_at, expires_at, is_active, created_at
		FROM api_tokens
		WHERE token_hash = ? AND is_active = 1 AND (expires_at IS NULL OR expires_at > ?)
	`

	err := r.db.QueryRowContext(ctx, query, tokenHash, time.Now().UTC()).Scan(
		&token.ID, &token.OrgID, &token.CreatedBy, &token.Name, &token.TokenHash, &token.TokenPrefix,
		&scopesJSON, &lastUsedAt, &expiresAt, &token.IsActive, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse scopes JSON
	if err := json.Unmarshal([]byte(scopesJSON), &token.Scopes); err != nil {
		return nil, err
	}

	token.LastUsedAt = parseTimestampPtr(lastUsedAt)
	token.ExpiresAt = parseTimestampPtr(expiresAt)
	token.CreatedAt = parseTimestamp(createdAt)

	return token, nil
}

// GetByID retrieves a token by ID
func (r *APITokenRepo) GetByID(ctx context.Context, id string) (*entity.APIToken, error) {
	token := &entity.APIToken{}
	var scopesJSON string
	var lastUsedAt, expiresAt, createdAt sql.NullString

	query := `
		SELECT id, org_id, created_by, name, token_hash, token_prefix, scopes, last_used_at, expires_at, is_active, created_at
		FROM api_tokens
		WHERE id = ?
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&token.ID, &token.OrgID, &token.CreatedBy, &token.Name, &token.TokenHash, &token.TokenPrefix,
		&scopesJSON, &lastUsedAt, &expiresAt, &token.IsActive, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse scopes JSON
	if err := json.Unmarshal([]byte(scopesJSON), &token.Scopes); err != nil {
		return nil, err
	}

	token.LastUsedAt = parseTimestampPtr(lastUsedAt)
	token.ExpiresAt = parseTimestampPtr(expiresAt)
	token.CreatedAt = parseTimestamp(createdAt)

	return token, nil
}

// ListByOrg retrieves all tokens for an organization
func (r *APITokenRepo) ListByOrg(ctx context.Context, orgID string) ([]entity.APITokenListItem, error) {
	query := `
		SELECT id, name, token_prefix, scopes, last_used_at, expires_at, is_active, created_at, created_by
		FROM api_tokens
		WHERE org_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []entity.APITokenListItem
	for rows.Next() {
		var t entity.APITokenListItem
		var scopesJSON string
		var lastUsedAt, expiresAt, createdAt sql.NullString

		if err := rows.Scan(
			&t.ID, &t.Name, &t.TokenPrefix, &scopesJSON, &lastUsedAt, &expiresAt, &t.IsActive, &createdAt, &t.CreatedBy,
		); err != nil {
			return nil, err
		}

		// Parse scopes JSON
		if err := json.Unmarshal([]byte(scopesJSON), &t.Scopes); err != nil {
			return nil, err
		}

		t.LastUsedAt = parseTimestampPtr(lastUsedAt)
		t.ExpiresAt = parseTimestampPtr(expiresAt)
		t.CreatedAt = parseTimestamp(createdAt)

		tokens = append(tokens, t)
	}

	return tokens, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *APITokenRepo) UpdateLastUsed(ctx context.Context, id string) error {
	query := `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id)
	return err
}

// Revoke deactivates a token
func (r *APITokenRepo) Revoke(ctx context.Context, id, orgID string) error {
	query := `UPDATE api_tokens SET is_active = 0 WHERE id = ? AND org_id = ?`
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

// Delete permanently removes a token
func (r *APITokenRepo) Delete(ctx context.Context, id, orgID string) error {
	query := `DELETE FROM api_tokens WHERE id = ? AND org_id = ?`
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

// CleanExpired removes expired tokens
func (r *APITokenRepo) CleanExpired(ctx context.Context) error {
	query := `DELETE FROM api_tokens WHERE expires_at IS NOT NULL AND expires_at < ?`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC())
	return err
}
