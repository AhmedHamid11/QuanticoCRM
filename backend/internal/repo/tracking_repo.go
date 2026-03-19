package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/entity"
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
