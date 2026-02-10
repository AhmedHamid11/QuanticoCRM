package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

type PendingAlertRepo struct {
	db db.DBConn
}

func NewPendingAlertRepo(conn db.DBConn) *PendingAlertRepo {
	return &PendingAlertRepo{db: conn}
}

func (r *PendingAlertRepo) WithDB(conn db.DBConn) *PendingAlertRepo {
	return &PendingAlertRepo{db: conn}
}

// Upsert creates or updates a pending alert for a record
// Uses INSERT OR REPLACE to handle rapid edits without duplicates
func (r *PendingAlertRepo) Upsert(ctx context.Context, alert *entity.PendingDuplicateAlert) error {
	// Marshal matches to JSON
	matchesJSON, err := json.Marshal(alert.Matches)
	if err != nil {
		return err
	}

	// Marshal merge display fields to JSON (may be nil/empty)
	var mergeDisplayFieldsJSON *string
	if len(alert.MergeDisplayFields) > 0 {
		jsonBytes, err := json.Marshal(alert.MergeDisplayFields)
		if err != nil {
			return err
		}
		jsonStr := string(jsonBytes)
		mergeDisplayFieldsJSON = &jsonStr
	}

	// Generate ID if not set
	if alert.ID == "" {
		alert.ID = sfid.New("alrt")
	}

	// Convert bool to int for SQLite
	isBlockModeInt := 0
	if alert.IsBlockMode {
		isBlockModeInt = 1
	}

	query := `
		INSERT OR REPLACE INTO pending_duplicate_alerts
		(id, org_id, entity_type, record_id, matches_json, total_match_count,
		 highest_confidence, is_block_mode, merge_display_fields, detected_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		alert.ID, alert.OrgID, alert.EntityType, alert.RecordID,
		string(matchesJSON), alert.TotalMatchCount, alert.HighestConfidence,
		isBlockModeInt, mergeDisplayFieldsJSON, alert.DetectedAt.Format(time.RFC3339), alert.Status)

	return err
}

// GetPendingByRecord gets the pending alert for a specific record
func (r *PendingAlertRepo) GetPendingByRecord(ctx context.Context, orgID, entityType, recordID string) (*entity.PendingDuplicateAlert, error) {
	query := `
		SELECT id, org_id, entity_type, record_id, matches_json, total_match_count,
		       highest_confidence, is_block_mode, merge_display_fields, detected_at, status,
		       resolved_at, resolved_by_id, override_text
		FROM pending_duplicate_alerts
		WHERE org_id = ? AND entity_type = ? AND record_id = ? AND status = 'pending'
		LIMIT 1
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No pending alert
	}

	var alert entity.PendingDuplicateAlert
	var detectedAt string
	var resolvedAt *string
	var isBlockModeInt int
	var resolvedByID, overrideText, mergeDisplayFieldsJSON *string

	err = rows.Scan(
		&alert.ID, &alert.OrgID, &alert.EntityType, &alert.RecordID,
		&alert.MatchesJSON, &alert.TotalMatchCount, &alert.HighestConfidence,
		&isBlockModeInt, &mergeDisplayFieldsJSON, &detectedAt, &alert.Status,
		&resolvedAt, &resolvedByID, &overrideText,
	)
	if err != nil {
		return nil, err
	}

	// Convert int to bool for IsBlockMode
	alert.IsBlockMode = isBlockModeInt == 1

	// Parse detected_at
	alert.DetectedAt, _ = time.Parse(time.RFC3339, detectedAt)

	// Parse resolved_at if set
	if resolvedAt != nil && *resolvedAt != "" {
		t, _ := time.Parse(time.RFC3339, *resolvedAt)
		alert.ResolvedAt = &t
	}
	alert.ResolvedByID = resolvedByID
	alert.OverrideText = overrideText

	// Unmarshal matches
	if alert.MatchesJSON != "" {
		json.Unmarshal([]byte(alert.MatchesJSON), &alert.Matches)
	}

	// Unmarshal merge display fields
	if mergeDisplayFieldsJSON != nil && *mergeDisplayFieldsJSON != "" {
		json.Unmarshal([]byte(*mergeDisplayFieldsJSON), &alert.MergeDisplayFields)
	}

	return &alert, nil
}

// Resolve updates an alert's status to resolved
func (r *PendingAlertRepo) Resolve(ctx context.Context, orgID, entityType, recordID, status, userID string, overrideText string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var overridePtr *string
	if overrideText != "" {
		overridePtr = &overrideText
	}

	// Delete any previously resolved alert with the same target status to avoid UNIQUE constraint
	// violation on (org_id, entity_type, record_id, status)
	deleteQuery := `
		DELETE FROM pending_duplicate_alerts
		WHERE org_id = ? AND entity_type = ? AND record_id = ? AND status = ?
	`
	r.db.ExecContext(ctx, deleteQuery, orgID, entityType, recordID, status)

	query := `
		UPDATE pending_duplicate_alerts
		SET status = ?, resolved_at = ?, resolved_by_id = ?, override_text = ?
		WHERE org_id = ? AND entity_type = ? AND record_id = ? AND status = 'pending'
	`

	_, err := r.db.ExecContext(ctx, query, status, now, userID, overridePtr, orgID, entityType, recordID)
	return err
}

// DeleteOldResolved deletes resolved alerts older than specified days
func (r *PendingAlertRepo) DeleteOldResolved(ctx context.Context, orgID string, olderThanDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays).Format(time.RFC3339)

	query := `
		DELETE FROM pending_duplicate_alerts
		WHERE org_id = ? AND status != 'pending' AND resolved_at < ?
	`

	result, err := r.db.ExecContext(ctx, query, orgID, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// ListAllPending returns all pending alerts for an org with optional entity type filter and pagination
func (r *PendingAlertRepo) ListAllPending(ctx context.Context, orgID string, entityType string, limit, offset int) ([]entity.PendingDuplicateAlert, int, error) {
	// Build WHERE clause with optional entity type filter
	whereClause := "org_id = ? AND status = 'pending'"
	args := []interface{}{orgID}

	if entityType != "" {
		whereClause += " AND entity_type = ?"
		args = append(args, entityType)
	}

	// Get total count for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM pending_duplicate_alerts WHERE %s", whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := fmt.Sprintf(`
		SELECT id, org_id, entity_type, record_id, matches_json, total_match_count,
		       highest_confidence, is_block_mode, merge_display_fields, detected_at, status,
		       resolved_at, resolved_by_id, override_text
		FROM pending_duplicate_alerts
		WHERE %s
		ORDER BY highest_confidence DESC, detected_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []entity.PendingDuplicateAlert
	for rows.Next() {
		var alert entity.PendingDuplicateAlert
		var detectedAt string
		var resolvedAt *string
		var isBlockModeInt int
		var resolvedByID, overrideText, mergeDisplayFieldsJSON *string

		err = rows.Scan(
			&alert.ID, &alert.OrgID, &alert.EntityType, &alert.RecordID,
			&alert.MatchesJSON, &alert.TotalMatchCount, &alert.HighestConfidence,
			&isBlockModeInt, &mergeDisplayFieldsJSON, &detectedAt, &alert.Status,
			&resolvedAt, &resolvedByID, &overrideText,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert int to bool for IsBlockMode
		alert.IsBlockMode = isBlockModeInt == 1

		// Parse detected_at
		alert.DetectedAt, _ = time.Parse(time.RFC3339, detectedAt)

		// Parse resolved_at if set
		if resolvedAt != nil && *resolvedAt != "" {
			t, _ := time.Parse(time.RFC3339, *resolvedAt)
			alert.ResolvedAt = &t
		}
		alert.ResolvedByID = resolvedByID
		alert.OverrideText = overrideText

		// Unmarshal matches
		if alert.MatchesJSON != "" {
			json.Unmarshal([]byte(alert.MatchesJSON), &alert.Matches)
		}

		// Unmarshal merge display fields
		if mergeDisplayFieldsJSON != nil && *mergeDisplayFieldsJSON != "" {
			json.Unmarshal([]byte(*mergeDisplayFieldsJSON), &alert.MergeDisplayFields)
		}

		alerts = append(alerts, alert)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}
