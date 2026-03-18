package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// EngagementRepo handles database operations for the engagement / email-channel tables.
// It uses the DBConn interface so it works with both local SQLite and Turso.
type EngagementRepo struct {
	db db.DBConn
}

// NewEngagementRepo creates a new EngagementRepo with the given connection.
func NewEngagementRepo(conn db.DBConn) *EngagementRepo {
	return &EngagementRepo{db: conn}
}

// WithDB returns a new EngagementRepo using the provided tenant database connection.
// This is the standard tenant-routing pattern used throughout the codebase.
func (r *EngagementRepo) WithDB(conn db.DBConn) *EngagementRepo {
	if conn == nil {
		return r
	}
	return &EngagementRepo{db: conn}
}

// ========== Gmail OAuth Tokens ==========

// UpsertGmailOAuthToken inserts or updates a Gmail OAuth token record.
// Uses ON CONFLICT(org_id, user_id) DO UPDATE so it is safe to call on reconnect.
func (r *EngagementRepo) UpsertGmailOAuthToken(ctx context.Context, t *entity.GmailOAuthToken) error {
	query := `
		INSERT INTO gmail_oauth_tokens
		    (id, org_id, user_id, access_token_encrypted, refresh_token_encrypted,
		     token_expiry, gmail_address, dns_spf_valid, dns_dkim_valid, dns_dmarc_valid,
		     connected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(org_id, user_id) DO UPDATE SET
		    access_token_encrypted  = excluded.access_token_encrypted,
		    refresh_token_encrypted = excluded.refresh_token_encrypted,
		    token_expiry             = excluded.token_expiry,
		    gmail_address            = excluded.gmail_address,
		    dns_spf_valid            = excluded.dns_spf_valid,
		    dns_dkim_valid           = excluded.dns_dkim_valid,
		    dns_dmarc_valid          = excluded.dns_dmarc_valid,
		    connected_at             = excluded.connected_at,
		    updated_at               = CURRENT_TIMESTAMP
	`

	var tokenExpiry interface{}
	if t.TokenExpiry != nil {
		tokenExpiry = t.TokenExpiry.UTC().Format("2006-01-02T15:04:05Z")
	}

	var connectedAt interface{}
	if t.ConnectedAt != nil {
		connectedAt = t.ConnectedAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	_, err := r.db.ExecContext(ctx, query,
		t.ID, t.OrgID, t.UserID,
		t.AccessTokenEncrypted, t.RefreshTokenEncrypted,
		tokenExpiry, t.GmailAddress,
		t.DNSSPFValid, t.DNSDKIMValid, t.DNSDMARCValid,
		connectedAt,
	)
	return err
}

// GetGmailOAuthToken retrieves the Gmail OAuth token for a user within an org.
// Returns nil (not an error) when no record is found.
func (r *EngagementRepo) GetGmailOAuthToken(ctx context.Context, orgID, userID string) (*entity.GmailOAuthToken, error) {
	query := `
		SELECT id, org_id, user_id, access_token_encrypted, refresh_token_encrypted,
		       token_expiry, gmail_address, dns_spf_valid, dns_dkim_valid, dns_dmarc_valid,
		       connected_at, created_at, updated_at
		FROM gmail_oauth_tokens
		WHERE org_id = ? AND user_id = ?
	`
	row := r.db.QueryRowContext(ctx, query, orgID, userID)

	var t entity.GmailOAuthToken
	var tokenExpiry, connectedAt, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&t.ID, &t.OrgID, &t.UserID,
		&t.AccessTokenEncrypted, &t.RefreshTokenEncrypted,
		&tokenExpiry, &t.GmailAddress,
		&t.DNSSPFValid, &t.DNSDKIMValid, &t.DNSDMARCValid,
		&connectedAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error
	}
	if err != nil {
		return nil, err
	}

	if tokenExpiry.Valid {
		if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", tokenExpiry.String); parseErr == nil {
			t.TokenExpiry = &parsed
		}
	}
	if connectedAt.Valid {
		if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", connectedAt.String); parseErr == nil {
			t.ConnectedAt = &parsed
		}
	}

	return &t, nil
}

// DeleteGmailOAuthToken removes the Gmail OAuth token for a user within an org.
func (r *EngagementRepo) DeleteGmailOAuthToken(ctx context.Context, orgID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM gmail_oauth_tokens WHERE org_id = ? AND user_id = ?",
		orgID, userID,
	)
	return err
}

// ========== Email Templates ==========

// CreateEmailTemplate inserts a new email template record.
func (r *EngagementRepo) CreateEmailTemplate(ctx context.Context, tmpl *entity.EmailTemplate) error {
	query := `
		INSERT INTO email_templates
		    (id, org_id, name, subject, body_html, body_text, has_compliance_footer, created_by,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query,
		tmpl.ID, tmpl.OrgID, tmpl.Name, tmpl.Subject,
		tmpl.BodyHTML, tmpl.BodyText, tmpl.HasComplianceFooter, tmpl.CreatedBy,
	)
	return err
}

// GetEmailTemplate retrieves a single email template by org_id and id.
// Returns nil (not an error) when no record is found.
func (r *EngagementRepo) GetEmailTemplate(ctx context.Context, orgID, id string) (*entity.EmailTemplate, error) {
	query := `
		SELECT id, org_id, name, subject, body_html, body_text, has_compliance_footer,
		       created_by, created_at, updated_at
		FROM email_templates
		WHERE org_id = ? AND id = ?
	`
	row := r.db.QueryRowContext(ctx, query, orgID, id)

	var t entity.EmailTemplate
	var createdAt, updatedAt sql.NullString
	err := row.Scan(
		&t.ID, &t.OrgID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
		&t.HasComplianceFooter, &t.CreatedBy, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", createdAt.String); parseErr == nil {
			t.CreatedAt = parsed
		}
	}
	if updatedAt.Valid {
		if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", updatedAt.String); parseErr == nil {
			t.UpdatedAt = parsed
		}
	}

	return &t, nil
}

// GetEmailTemplateByID retrieves a single email template by ID within an org.
// Returns sql.ErrNoRows if not found.
// Deprecated: use GetEmailTemplate for consistent nil-not-error semantics.
func (r *EngagementRepo) GetEmailTemplateByID(ctx context.Context, orgID, templateID string) (*entity.EmailTemplate, error) {
	return r.GetEmailTemplate(ctx, orgID, templateID)
}

// ListEmailTemplates returns all email templates for an org, ordered by updated_at DESC.
func (r *EngagementRepo) ListEmailTemplates(ctx context.Context, orgID string) ([]*entity.EmailTemplate, error) {
	query := `
		SELECT id, org_id, name, subject, body_html, body_text, has_compliance_footer,
		       created_by, created_at, updated_at
		FROM email_templates
		WHERE org_id = ?
		ORDER BY updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*entity.EmailTemplate
	for rows.Next() {
		var t entity.EmailTemplate
		var createdAt, updatedAt sql.NullString
		if scanErr := rows.Scan(
			&t.ID, &t.OrgID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
			&t.HasComplianceFooter, &t.CreatedBy, &createdAt, &updatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		if createdAt.Valid {
			if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", createdAt.String); parseErr == nil {
				t.CreatedAt = parsed
			}
		}
		if updatedAt.Valid {
			if parsed, parseErr := time.Parse("2006-01-02T15:04:05Z", updatedAt.String); parseErr == nil {
				t.UpdatedAt = parsed
			}
		}
		templates = append(templates, &t)
	}
	return templates, rows.Err()
}

// UpdateEmailTemplate updates the mutable fields of an email template.
func (r *EngagementRepo) UpdateEmailTemplate(ctx context.Context, tmpl *entity.EmailTemplate) error {
	query := `
		UPDATE email_templates
		SET name = ?, subject = ?, body_html = ?, body_text = ?,
		    has_compliance_footer = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND org_id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		tmpl.Name, tmpl.Subject, tmpl.BodyHTML, tmpl.BodyText,
		tmpl.HasComplianceFooter, tmpl.ID, tmpl.OrgID,
	)
	return err
}

// DeleteEmailTemplate removes an email template by id + org_id.
func (r *EngagementRepo) DeleteEmailTemplate(ctx context.Context, orgID, id string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM email_templates WHERE id = ? AND org_id = ?",
		id, orgID,
	)
	return err
}
