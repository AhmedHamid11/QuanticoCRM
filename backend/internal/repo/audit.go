package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// AuditRepo handles database operations for audit logs
type AuditRepo struct {
	db db.DBConn
}

// NewAuditRepo creates a new AuditRepo
func NewAuditRepo(conn db.DBConn) *AuditRepo {
	return &AuditRepo{db: conn}
}

// WithDB returns a new repo with a different DB connection (for multi-tenant)
func (r *AuditRepo) WithDB(conn db.DBConn) *AuditRepo {
	return &AuditRepo{db: conn}
}

// GetLastEntryHash retrieves the hash of the most recent audit entry for an org
// Returns "GENESIS" if this is the first entry in the chain
func (r *AuditRepo) GetLastEntryHash(ctx context.Context, orgID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx,
		`SELECT entry_hash FROM audit_logs
		 WHERE org_id = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
	).Scan(&hash)

	if err == sql.ErrNoRows {
		// First entry in org's chain
		return "GENESIS", nil
	}

	if err != nil {
		return "", fmt.Errorf("failed to get last entry hash: %w", err)
	}

	return hash, nil
}

// Create inserts a new audit log entry with hash chain linking
func (r *AuditRepo) Create(ctx context.Context, entry *entity.AuditLogEntry) error {
	// Generate ID
	entry.ID = sfid.NewAuditLog()

	// Get previous hash for chain
	prevHash, err := r.GetLastEntryHash(ctx, entry.OrgID)
	if err != nil {
		return fmt.Errorf("failed to get previous hash: %w", err)
	}
	entry.PrevHash = prevHash

	// Compute entry hash
	entry.EntryHash = entry.ComputeEntryHash()

	// Convert success bool to int for SQLite
	successInt := 0
	if entry.Success {
		successInt = 1
	}

	// Insert into database
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (
			id, org_id, event_type, actor_id, actor_email,
			target_id, target_type, ip_address, user_agent,
			details, success, error_msg, prev_hash, entry_hash, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.OrgID,
		entry.EventType,
		entry.ActorID,
		entry.ActorEmail,
		entry.TargetID,
		entry.TargetType,
		entry.IPAddress,
		entry.UserAgent,
		entry.Details,
		successInt,
		entry.ErrorMsg,
		entry.PrevHash,
		entry.EntryHash,
		entry.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// List retrieves audit logs for an org with optional filters
func (r *AuditRepo) List(ctx context.Context, orgID string, filters *entity.AuditLogFilters) (*entity.AuditLogListResponse, error) {
	if filters == nil {
		filters = &entity.AuditLogFilters{
			Page:     1,
			PageSize: 50,
		}
	}

	// Set defaults
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 || filters.PageSize > 100 {
		filters.PageSize = 50
	}

	// Build WHERE clause
	whereClauses := []string{"org_id = ?"}
	args := []interface{}{orgID}

	// Filter by event types
	if len(filters.EventTypes) > 0 {
		placeholders := make([]string, len(filters.EventTypes))
		for i, eventType := range filters.EventTypes {
			placeholders[i] = "?"
			args = append(args, eventType)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("event_type IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Filter by actor
	if filters.ActorID != "" {
		whereClauses = append(whereClauses, "actor_id = ?")
		args = append(args, filters.ActorID)
	}

	// Filter by date range
	if filters.DateFrom != nil {
		whereClauses = append(whereClauses, "created_at >= ?")
		args = append(args, filters.DateFrom.Format("2006-01-02 15:04:05"))
	}
	if filters.DateTo != nil {
		whereClauses = append(whereClauses, "created_at <= ?")
		args = append(args, filters.DateTo.Format("2006-01-02 15:04:05"))
	}

	whereClause := strings.Join(whereClauses, " AND ")

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	query := fmt.Sprintf(`
		SELECT id, org_id, event_type, actor_id, actor_email,
		       target_id, target_type, ip_address, user_agent,
		       details, success, error_msg, prev_hash, entry_hash, created_at
		FROM audit_logs
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, filters.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []entity.AuditLogEntry
	for rows.Next() {
		var entry entity.AuditLogEntry
		var successInt int

		err := rows.Scan(
			&entry.ID,
			&entry.OrgID,
			&entry.EventType,
			&entry.ActorID,
			&entry.ActorEmail,
			&entry.TargetID,
			&entry.TargetType,
			&entry.IPAddress,
			&entry.UserAgent,
			&entry.Details,
			&successInt,
			&entry.ErrorMsg,
			&entry.PrevHash,
			&entry.EntryHash,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		entry.Success = successInt == 1
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	hasMore := offset+filters.PageSize < total

	return &entity.AuditLogListResponse{
		Data:     entries,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
		HasMore:  hasMore,
	}, nil
}

// VerifyChainIntegrity verifies the hash chain for an org
// Returns verification result with any errors found
func (r *AuditRepo) VerifyChainIntegrity(ctx context.Context, orgID string) (*entity.ChainVerificationResult, error) {
	result := &entity.ChainVerificationResult{
		Valid:  true,
		Errors: []string{},
	}

	// Get all entries ordered by creation time (oldest first for chain verification)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, event_type, actor_id, actor_email,
		        target_id, target_type, ip_address, user_agent,
		        details, success, error_msg, prev_hash, entry_hash, created_at
		 FROM audit_logs
		 WHERE org_id = ?
		 ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []entity.AuditLogEntry
	for rows.Next() {
		var entry entity.AuditLogEntry
		var successInt int

		err := rows.Scan(
			&entry.ID,
			&entry.OrgID,
			&entry.EventType,
			&entry.ActorID,
			&entry.ActorEmail,
			&entry.TargetID,
			&entry.TargetType,
			&entry.IPAddress,
			&entry.UserAgent,
			&entry.Details,
			&successInt,
			&entry.ErrorMsg,
			&entry.PrevHash,
			&entry.EntryHash,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		entry.Success = successInt == 1
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if len(entries) == 0 {
		// No entries to verify
		return result, nil
	}

	result.FirstEntryID = entries[0].ID
	result.LastEntryID = entries[len(entries)-1].ID

	// Verify each entry in the chain
	expectedPrevHash := "GENESIS"
	for i, entry := range entries {
		result.EntriesVerified++

		// Check prev_hash matches expected
		if entry.PrevHash != expectedPrevHash {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Entry %s (index %d): prev_hash mismatch. Expected %s, got %s",
					entry.ID, i, expectedPrevHash, entry.PrevHash))
			if result.FirstBrokenEntry == "" {
				result.FirstBrokenEntry = entry.ID
			}
		}

		// Verify entry_hash is correctly computed
		computedHash := entry.ComputeEntryHash()
		if entry.EntryHash != computedHash {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Entry %s (index %d): hash computation mismatch. Stored %s, computed %s",
					entry.ID, i, entry.EntryHash, computedHash))
			if result.FirstBrokenEntry == "" {
				result.FirstBrokenEntry = entry.ID
			}
		}

		// Next entry should have this entry's hash as prev_hash
		expectedPrevHash = entry.EntryHash
	}

	return result, nil
}

// ConvertEventToEntry converts an AuditEvent to an AuditLogEntry for persistence
func ConvertEventToEntry(event entity.AuditEvent) (*entity.AuditLogEntry, error) {
	// Convert details map to JSON string
	detailsJSON := ""
	if event.Details != nil && len(event.Details) > 0 {
		detailsBytes, err := json.Marshal(event.Details)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal details: %w", err)
		}
		detailsJSON = string(detailsBytes)
	}

	return &entity.AuditLogEntry{
		OrgID:      event.OrgID,
		EventType:  string(event.EventType),
		ActorID:    event.ActorID,
		ActorEmail: event.ActorEmail,
		TargetID:   event.TargetID,
		IPAddress:  event.IPAddress,
		UserAgent:  event.UserAgent,
		Details:    detailsJSON,
		Success:    event.Success,
		ErrorMsg:   event.ErrorMsg,
		CreatedAt:  event.Timestamp,
	}, nil
}
