package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/google/uuid"
)

// warmupLevels defines the daily send limit at each warmup level.
// Each level lasts 7 days before advancing to the next.
var warmupLevels = []int{5, 10, 20, 40, 75, 100, 150}

// WarmupScheduler manages the Gmail account warmup process.
// It tracks daily send counts, enforces per-account limits, advances warmup
// levels weekly, and resets daily counters at midnight UTC.
type WarmupScheduler struct {
	// getDB returns the tenant DB for the given orgID. Used for warmup queries.
	getDB func(ctx context.Context, orgID string) (*sql.DB, error)
}

// NewWarmupScheduler creates a WarmupScheduler.
// getDB is a function that returns the tenant database for a given orgID.
func NewWarmupScheduler(getDB func(ctx context.Context, orgID string) (*sql.DB, error)) *WarmupScheduler {
	return &WarmupScheduler{getDB: getDB}
}

// InitWarmupForAccount creates a warmup session for a newly connected Gmail account.
// Idempotent — if a session already exists for this user/org, it is a no-op.
func (ws *WarmupScheduler) InitWarmupForAccount(ctx context.Context, orgID, userID, email string, db *sql.DB) error {
	// Check if a session already exists
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM email_warmup_sessions WHERE org_id = ? AND user_id = ?",
		orgID, userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("warmup init: check existing: %w", err)
	}
	if count > 0 {
		return nil // Already initialized
	}

	now := time.Now().UTC()
	session := &entity.WarmupSession{
		ID:                uuid.New().String(),
		OrgID:             orgID,
		UserID:            userID,
		GmailAccountEmail: email,
		DailyLimit:        warmupLevels[0], // Start at 5/day
		CurrentDailyCount: 0,
		Status:            entity.WarmupStatusActive,
		StartedAt:         now,
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO email_warmup_sessions
		    (id, org_id, user_id, gmail_account_email, daily_limit, current_daily_count, status, started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`,
		session.ID, session.OrgID, session.UserID, session.GmailAccountEmail,
		session.DailyLimit, session.CurrentDailyCount, session.Status,
		session.StartedAt.Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return fmt.Errorf("warmup init: insert session: %w", err)
	}

	log.Printf("[WarmupScheduler] Initialized warmup session for user %s in org %s (limit=%d/day)", userID, orgID, session.DailyLimit)
	return nil
}

// CheckAndIncrementDailyCount checks if a user is within their warmup limit and increments
// the count if allowed. Returns allowed=true if the send can proceed.
// If the user has no warmup session or the session is completed, returns allowed=true (no limit).
func (ws *WarmupScheduler) CheckAndIncrementDailyCount(ctx context.Context, orgID, userID string, db *sql.DB) (allowed bool, err error) {
	var sessionID string
	var dailyLimit, currentCount int
	var status string

	err = db.QueryRowContext(ctx, `
		SELECT id, daily_limit, current_daily_count, status
		FROM email_warmup_sessions
		WHERE org_id = ? AND user_id = ?
	`, orgID, userID).Scan(&sessionID, &dailyLimit, &currentCount, &status)

	if err == sql.ErrNoRows {
		// No warmup session — allow the send (legacy accounts or pre-warmup)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("warmup check: query session: %w", err)
	}

	// Completed warmup — no limit
	if status != entity.WarmupStatusActive {
		return true, nil
	}

	// Limit reached
	if currentCount >= dailyLimit {
		return false, nil
	}

	// Increment count
	_, err = db.ExecContext(ctx,
		"UPDATE email_warmup_sessions SET current_daily_count = current_daily_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("warmup check: increment count: %w", err)
	}

	return true, nil
}

// AdvanceWarmupLevels recalculates the daily_limit for all active warmup sessions in an org
// based on how many days have elapsed since started_at. Runs once per day.
func (ws *WarmupScheduler) AdvanceWarmupLevels(ctx context.Context, orgID string, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, started_at, daily_limit, status
		FROM email_warmup_sessions
		WHERE org_id = ? AND status = 'active'
	`, orgID)
	if err != nil {
		return fmt.Errorf("warmup advance: query sessions: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var sessionID, startedAtStr, status string
		var currentLimit int
		if scanErr := rows.Scan(&sessionID, &startedAtStr, &currentLimit, &status); scanErr != nil {
			log.Printf("[WarmupScheduler] scan error for org %s: %v", orgID, scanErr)
			continue
		}

		startedAt, parseErr := time.Parse("2006-01-02T15:04:05Z", startedAtStr)
		if parseErr != nil {
			// Try alternative format
			startedAt, parseErr = time.Parse("2006-01-02 15:04:05", startedAtStr)
			if parseErr != nil {
				log.Printf("[WarmupScheduler] failed to parse started_at %q for session %s: %v", startedAtStr, sessionID, parseErr)
				continue
			}
		}

		daysElapsed := int(now.Sub(startedAt).Hours() / 24)
		levelIndex := daysElapsed / 7

		var newLimit int
		var newStatus string
		if levelIndex >= len(warmupLevels) {
			// Warmup complete
			newLimit = warmupLevels[len(warmupLevels)-1]
			newStatus = entity.WarmupStatusCompleted
		} else {
			newLimit = warmupLevels[levelIndex]
			newStatus = entity.WarmupStatusActive
		}

		if newLimit != currentLimit || newStatus != status {
			_, updateErr := db.ExecContext(ctx,
				"UPDATE email_warmup_sessions SET daily_limit = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
				newLimit, newStatus, sessionID,
			)
			if updateErr != nil {
				log.Printf("[WarmupScheduler] update level failed for session %s: %v", sessionID, updateErr)
			} else {
				log.Printf("[WarmupScheduler] Session %s: days=%d level=%d limit=%d status=%s", sessionID, daysElapsed, levelIndex, newLimit, newStatus)
			}
		}
	}
	return rows.Err()
}

// ResetDailyCounts resets current_daily_count to 0 for all active warmup sessions in an org.
// This runs at midnight UTC each day.
func (ws *WarmupScheduler) ResetDailyCounts(ctx context.Context, orgID string, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		"UPDATE email_warmup_sessions SET current_daily_count = 0, updated_at = CURRENT_TIMESTAMP WHERE org_id = ? AND status = 'active'",
		orgID,
	)
	if err != nil {
		return fmt.Errorf("warmup reset: reset counts for org %s: %w", orgID, err)
	}
	return nil
}

// RunDailyMaintenance runs ResetDailyCounts and AdvanceWarmupLevels for an org.
// Designed to be called by a gocron daily job.
func (ws *WarmupScheduler) RunDailyMaintenance(ctx context.Context, orgID string) {
	db, err := ws.getDB(ctx, orgID)
	if err != nil {
		log.Printf("[WarmupScheduler] failed to get tenant DB for org %s: %v", orgID, err)
		return
	}

	if err := ws.ResetDailyCounts(ctx, orgID, db); err != nil {
		log.Printf("[WarmupScheduler] ResetDailyCounts failed for org %s: %v", orgID, err)
	}
	if err := ws.AdvanceWarmupLevels(ctx, orgID, db); err != nil {
		log.Printf("[WarmupScheduler] AdvanceWarmupLevels failed for org %s: %v", orgID, err)
	}
	log.Printf("[WarmupScheduler] Daily maintenance complete for org %s", orgID)
}
