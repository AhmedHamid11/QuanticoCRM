package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/google/uuid"
)

// TrackingRepo handles persistence for email tracking events and A/B tracking stats.
// All operations target the tenant database (per-org), not the master DB.
type TrackingRepo struct {
	db *sql.DB
}

// NewTrackingRepo creates a TrackingRepo for the given tenant DB connection.
func NewTrackingRepo(db *sql.DB) *TrackingRepo {
	return &TrackingRepo{db: db}
}

// WithDB returns a new TrackingRepo using the provided tenant database connection.
// This is the standard tenant-routing pattern used throughout the codebase.
func (r *TrackingRepo) WithDB(db *sql.DB) *TrackingRepo {
	if db == nil {
		return r
	}
	return &TrackingRepo{db: db}
}

// BatchInsertTrackingEvents inserts a batch of tracking events in a single transaction.
// Uses a prepared statement for efficiency. Events are immutable — no update path.
func (r *TrackingRepo) BatchInsertTrackingEvents(ctx context.Context, events []entity.TrackingEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO email_tracking_events
		    (id, org_id, enrollment_id, step_execution_id, event_type, link_url, metadata_json, occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		var linkURL interface{}
		if e.LinkURL != nil {
			linkURL = *e.LinkURL
		}
		var metaJSON interface{}
		if e.MetadataJSON != nil {
			metaJSON = *e.MetadataJSON
		}
		occurredAt := e.OccurredAt.UTC().Format(time.RFC3339)
		createdAt := e.CreatedAt.UTC().Format(time.RFC3339)
		if e.CreatedAt.IsZero() {
			createdAt = time.Now().UTC().Format(time.RFC3339)
		}

		if _, execErr := stmt.ExecContext(ctx,
			e.ID, e.OrgID, e.EnrollmentID, e.StepExecutionID,
			e.EventType, linkURL, metaJSON, occurredAt, createdAt,
		); execErr != nil {
			return execErr
		}
	}

	return tx.Commit()
}

// UpdateStepExecutionThreadID stores the Gmail thread ID on a step execution record.
// Used after a successful Gmail send to enable reply-thread polling.
func (r *TrackingRepo) UpdateStepExecutionThreadID(ctx context.Context, execID, threadID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_step_executions SET gmail_thread_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		threadID, execID,
	)
	return err
}

// SetDoNotEmail sets do_not_email = 1 on the given contact to prevent future sends.
// Used by BounceHandler on hard bounce.
func (r *TrackingRepo) SetDoNotEmail(ctx context.Context, orgID, contactID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE contacts SET do_not_email = 1 WHERE id = ? AND org_id = ?",
		contactID, orgID,
	)
	return err
}

// IncrementSoftBounceCount atomically increments soft_bounce_count for an enrollment.
// Returns the new count after increment.
func (r *TrackingRepo) IncrementSoftBounceCount(ctx context.Context, orgID, enrollmentID string) (int, error) {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_enrollments SET soft_bounce_count = soft_bounce_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND org_id = ?",
		enrollmentID, orgID,
	)
	if err != nil {
		return 0, err
	}
	var count int
	err = r.db.QueryRowContext(ctx,
		"SELECT soft_bounce_count FROM sequence_enrollments WHERE id = ? AND org_id = ?",
		enrollmentID, orgID,
	).Scan(&count)
	return count, err
}

// GetExecutionsForReplyCheck returns completed email step executions that have a
// gmail_thread_id, were executed in the last 7 days, and do not yet have a reply
// or bounce tracking event. Used by the reply/bounce polling job.
func (r *TrackingRepo) GetExecutionsForReplyCheck(ctx context.Context, orgID string) ([]*entity.StepExecution, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	query := `
		SELECT sse.id, sse.enrollment_id, sse.step_id, sse.org_id, sse.status,
		       sse.scheduled_at, sse.executed_at, sse.error_message, sse.gmail_thread_id,
		       sse.variant_id, sse.created_at, sse.updated_at
		FROM sequence_step_executions sse
		WHERE sse.org_id = ?
		  AND sse.status = 'completed'
		  AND sse.gmail_thread_id IS NOT NULL
		  AND sse.executed_at >= ?
		  AND NOT EXISTS (
		      SELECT 1 FROM email_tracking_events te
		      WHERE te.step_execution_id = sse.id
		        AND te.event_type IN ('reply', 'ooo', 'bounce')
		  )
		ORDER BY sse.executed_at ASC
		LIMIT 200
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("GetExecutionsForReplyCheck: %w", err)
	}
	defer rows.Close()

	var execs []*entity.StepExecution
	for rows.Next() {
		var exec entity.StepExecution
		var scheduledAt, executedAt, errorMsg, threadID, variantID, createdAt, updatedAt sql.NullString
		err := rows.Scan(
			&exec.ID, &exec.EnrollmentID, &exec.StepID, &exec.OrgID, &exec.Status,
			&scheduledAt, &executedAt, &errorMsg, &threadID, &variantID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		if scheduledAt.Valid {
			if t, parseErr := time.Parse("2006-01-02T15:04:05Z", scheduledAt.String); parseErr == nil {
				exec.ScheduledAt = &t
			}
		}
		if executedAt.Valid {
			if t, parseErr := time.Parse("2006-01-02T15:04:05Z", executedAt.String); parseErr == nil {
				exec.ExecutedAt = &t
			}
		}
		if errorMsg.Valid {
			exec.ErrorMessage = &errorMsg.String
		}
		if threadID.Valid {
			exec.GmailThreadID = &threadID.String
		}
		if variantID.Valid {
			exec.VariantID = &variantID.String
		}
		execs = append(execs, &exec)
	}
	return execs, rows.Err()
}

// GetEnrollmentContactID returns the contact_id and enrolled_by for a given enrollment.
// Used by the reply/bounce polling to look up the sender's OAuth credentials.
func (r *TrackingRepo) GetEnrollmentContactID(ctx context.Context, enrollmentID string) (contactID, enrolledBy string, err error) {
	err = r.db.QueryRowContext(ctx,
		"SELECT contact_id, enrolled_by FROM sequence_enrollments WHERE id = ?",
		enrollmentID,
	).Scan(&contactID, &enrolledBy)
	return contactID, enrolledBy, err
}

// ========== A/B Variant CRUD ==========

// ListABVariantsForStep returns all A/B test variants for a given step, ordered by variant_label.
func (r *TrackingRepo) ListABVariantsForStep(ctx context.Context, stepID string) ([]entity.ABTestVariant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, step_id, variant_label, subject_override, body_html_override,
		       traffic_pct, is_winner, created_at, updated_at
		FROM ab_test_variants
		WHERE step_id = ?
		ORDER BY variant_label
	`, stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variants []entity.ABTestVariant
	for rows.Next() {
		var v entity.ABTestVariant
		var createdAt, updatedAt string
		if err := rows.Scan(
			&v.ID, &v.StepID, &v.VariantLabel,
			&v.SubjectOverride, &v.BodyHTMLOverride,
			&v.TrafficPct, &v.IsWinner,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		v.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		v.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

// GetABVariant fetches a single A/B test variant by ID. Returns (nil, nil) when not found.
func (r *TrackingRepo) GetABVariant(ctx context.Context, variantID string) (*entity.ABTestVariant, error) {
	var v entity.ABTestVariant
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, step_id, variant_label, subject_override, body_html_override,
		       traffic_pct, is_winner, created_at, updated_at
		FROM ab_test_variants WHERE id = ?
	`, variantID).Scan(
		&v.ID, &v.StepID, &v.VariantLabel,
		&v.SubjectOverride, &v.BodyHTMLOverride,
		&v.TrafficPct, &v.IsWinner,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	v.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	return &v, nil
}

// CreateABVariant inserts a new A/B test variant.
func (r *TrackingRepo) CreateABVariant(ctx context.Context, v *entity.ABTestVariant) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ab_test_variants
		    (id, step_id, variant_label, subject_override, body_html_override,
		     traffic_pct, is_winner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, v.ID, v.StepID, v.VariantLabel, v.SubjectOverride, v.BodyHTMLOverride,
		v.TrafficPct, v.IsWinner)
	return err
}

// UpdateABVariant updates a variant's mutable fields (label, overrides, traffic_pct).
func (r *TrackingRepo) UpdateABVariant(ctx context.Context, v *entity.ABTestVariant) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ab_test_variants
		SET variant_label      = ?,
		    subject_override   = ?,
		    body_html_override = ?,
		    traffic_pct        = ?,
		    updated_at         = CURRENT_TIMESTAMP
		WHERE id = ?
	`, v.VariantLabel, v.SubjectOverride, v.BodyHTMLOverride, v.TrafficPct, v.ID)
	return err
}

// DeleteABVariant removes an A/B test variant by ID.
func (r *TrackingRepo) DeleteABVariant(ctx context.Context, variantID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ab_test_variants WHERE id = ?`, variantID)
	return err
}

// SetABWinner promotes a variant to winner within a transaction.
// Resets all variants for the step to 0%, then sets the winner to 100%.
func (r *TrackingRepo) SetABWinner(ctx context.Context, stepID, winnerVariantID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `
		UPDATE ab_test_variants
		SET is_winner = 0, traffic_pct = 0, updated_at = CURRENT_TIMESTAMP
		WHERE step_id = ?
	`, stepID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE ab_test_variants
		SET is_winner = 1, traffic_pct = 100, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, winnerVariantID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetABStatsForStep returns aggregated tracking stats for all variants of a step.
func (r *TrackingRepo) GetABStatsForStep(ctx context.Context, stepID string) ([]entity.ABTrackingStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.variant_id, s.org_id, s.sends, s.opens, s.clicks, s.replies, s.updated_at
		FROM ab_tracking_stats s
		WHERE s.variant_id IN (SELECT id FROM ab_test_variants WHERE step_id = ?)
	`, stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []entity.ABTrackingStats
	for rows.Next() {
		var s entity.ABTrackingStats
		var updatedAt string
		if err := rows.Scan(
			&s.ID, &s.VariantID, &s.OrgID,
			&s.Sends, &s.Opens, &s.Clicks, &s.Replies,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		s.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// UpsertABTrackingStats increments A/B test performance counters for a variant.
// Uses INSERT ON CONFLICT DO UPDATE to atomically increment counters.
func (r *TrackingRepo) UpsertABTrackingStats(ctx context.Context, id, variantID, orgID string, sends, opens, clicks, replies int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ab_tracking_stats (id, variant_id, org_id, sends, opens, clicks, replies, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(variant_id) DO UPDATE SET
		    sends   = sends + excluded.sends,
		    opens   = opens + excluded.opens,
		    clicks  = clicks + excluded.clicks,
		    replies = replies + excluded.replies,
		    updated_at = CURRENT_TIMESTAMP
	`, id, variantID, orgID, sends, opens, clicks, replies)
	return err
}

// OptOutContact inserts an entry into opt_out_list for the given contact and channel.
// Used by the unsubscribe endpoint.
func (r *TrackingRepo) OptOutContact(ctx context.Context, orgID, contactID, channel, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO opt_out_list (id, org_id, contact_id, channel, reason, opted_out_at, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT DO NOTHING
	`, generateID(), orgID, contactID, channel, reason)
	return err
}

// generateID generates a UUID for opt_out entries and similar records.
func generateID() string {
	return uuid.New().String()
}
