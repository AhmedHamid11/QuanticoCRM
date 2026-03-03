package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// MergeService handles merge execution and undo operations
type MergeService struct {
	mergeRepo        *repo.MergeRepo
	metadataRepo     *repo.MetadataRepo
	discoveryService *MergeDiscoveryService
	auditLogger      *AuditLogger
	pendingAlertRepo *repo.PendingAlertRepo
}

// NewMergeService creates a new MergeService
func NewMergeService(
	mergeRepo *repo.MergeRepo,
	metadataRepo *repo.MetadataRepo,
	discoveryService *MergeDiscoveryService,
	auditLogger *AuditLogger,
	pendingAlertRepo *repo.PendingAlertRepo,
) *MergeService {
	return &MergeService{
		mergeRepo:        mergeRepo,
		metadataRepo:     metadataRepo,
		discoveryService: discoveryService,
		auditLogger:      auditLogger,
		pendingAlertRepo: pendingAlertRepo,
	}
}

// ExecuteMerge performs an atomic merge operation
// All steps happen within a single transaction: snapshot, transfer FKs, update survivor, archive duplicates
func (s *MergeService) ExecuteMerge(ctx context.Context, db *sql.DB, orgID, userID string, req entity.MergeRequest) (*entity.MergeResult, error) {
	// 1. Validate inputs
	if req.SurvivorID == "" {
		return nil, fmt.Errorf("survivorID is required")
	}
	if len(req.DuplicateIDs) == 0 {
		return nil, fmt.Errorf("at least one duplicate ID is required")
	}
	if req.EntityType == "" {
		return nil, fmt.Errorf("entityType is required")
	}

	// Verify entity exists
	ent, err := s.metadataRepo.GetEntity(ctx, orgID, req.EntityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}
	if ent == nil {
		return nil, fmt.Errorf("entity not found: %s", req.EntityType)
	}

	tableName := util.GetTableName(req.EntityType)
	now := time.Now().UTC().Format(time.RFC3339)

	// 2. Ensure archive columns exist on the table
	if err := s.mergeRepo.EnsureArchiveColumns(ctx, db, tableName); err != nil {
		return nil, fmt.Errorf("failed to ensure archive columns: %w", err)
	}

	// 3. Ensure merge_snapshots table exists
	if err := s.mergeRepo.EnsureTableExists(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure merge_snapshots table: %w", err)
	}

	// 4. Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Deferred rollback on panic (matching bulk.go pattern)
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-panic after rollback
		}
	}()

	// 5. Snapshot survivor pre-merge state
	survivorBefore, err := fetchRecordAsMapTx(ctx, tx, tableName, req.SurvivorID, orgID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to fetch survivor record: %w", err)
	}
	if survivorBefore == nil {
		tx.Rollback()
		return nil, fmt.Errorf("survivor record not found: %s", req.SurvivorID)
	}

	// 6. Snapshot each duplicate
	var duplicateSnapshots []map[string]interface{}
	for _, dupID := range req.DuplicateIDs {
		dupSnapshot, err := fetchRecordAsMapTx(ctx, tx, tableName, dupID, orgID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to fetch duplicate record %s: %w", dupID, err)
		}
		if dupSnapshot == nil {
			tx.Rollback()
			return nil, fmt.Errorf("duplicate record not found: %s", dupID)
		}
		duplicateSnapshots = append(duplicateSnapshots, dupSnapshot)
	}

	// 7. Discover related records for each duplicate (queries metadata, not in transaction)
	// Build a map of entity type -> FK field -> list of FK changes
	relatedRecordFKs := make(map[string][]entity.FKChange)

	for _, dupID := range req.DuplicateIDs {
		groups, err := s.discoveryService.DiscoverRelatedRecords(ctx, db, orgID, req.EntityType, dupID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to discover related records for %s: %w", dupID, err)
		}

		// 8. Transfer FK references (inside transaction)
		for _, group := range groups {
			groupTableName := util.GetTableName(group.EntityType)
			fkColumn := util.CamelToSnake(group.FKField) + "_id"

			for _, record := range group.Records {
				// Update the FK to point to survivor
				updateSQL := fmt.Sprintf(
					"UPDATE %s SET %s = ? WHERE id = ? AND org_id = ?",
					groupTableName,
					util.QuoteIdentifier(fkColumn),
				)
				_, err := tx.ExecContext(ctx, updateSQL, req.SurvivorID, record.ID, orgID)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to transfer FK for record %s: %w", record.ID, err)
				}

				// Track the change for undo
				groupKey := group.EntityType
				relatedRecordFKs[groupKey] = append(relatedRecordFKs[groupKey], entity.FKChange{
					RecordID: record.ID,
					FKField:  group.FKField,
					OldValue: record.FKValue, // original duplicate ID
				})
			}
		}
	}

	// 9. Update survivor fields (inside transaction)
	if len(req.MergedFields) > 0 {
		// Get field definitions for the entity type
		fields, err := s.metadataRepo.ListFields(ctx, orgID, req.EntityType)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to get field definitions: %w", err)
		}

		var setClauses []string
		var values []interface{}

		for _, field := range fields {
			if field.Name == "id" {
				continue
			}

			// Handle lookup fields (same pattern as bulk.go)
			if field.Type == entity.FieldTypeLink {
				snakeName := util.CamelToSnake(field.Name)
				idKey := field.Name + "Id"
				nameKey := field.Name + "Name"

				if idVal, ok := req.MergedFields[idKey]; ok {
					setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(snakeName+"_id")))
					values = append(values, idVal)
				}
				if nameVal, ok := req.MergedFields[nameKey]; ok {
					setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(snakeName+"_name")))
					values = append(values, nameVal)
				}
				continue
			}

			// Handle multi-lookup fields
			if field.Type == entity.FieldTypeLinkMultiple {
				snakeName := util.CamelToSnake(field.Name)
				idsKey := field.Name + "Ids"
				namesKey := field.Name + "Names"

				if idsVal, ok := req.MergedFields[idsKey]; ok {
					setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(snakeName+"_ids")))
					values = append(values, idsVal)
				}
				if namesVal, ok := req.MergedFields[namesKey]; ok {
					setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(snakeName+"_names")))
					values = append(values, namesVal)
				}
				continue
			}

			// Regular fields
			if val, ok := req.MergedFields[field.Name]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(util.CamelToSnake(field.Name))))
				values = append(values, val)
			}
		}

		if len(setClauses) > 0 {
			// Add audit fields
			setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
			values = append(values, now, userID)

			// Add WHERE clause values
			values = append(values, req.SurvivorID, orgID)

			updateSQL := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
				tableName, strings.Join(setClauses, ", "))

			result, err := tx.ExecContext(ctx, updateSQL, values...)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to update survivor: %w", err)
			}

			rows, _ := result.RowsAffected()
			if rows == 0 {
				tx.Rollback()
				return nil, fmt.Errorf("survivor record not found or not accessible")
			}
		}
	}

	// 10. Archive each duplicate (AFTER FK transfer per Pitfall #2)
	for _, dupID := range req.DuplicateIDs {
		archiveSQL := fmt.Sprintf(
			"UPDATE %s SET archived_at = ?, archived_reason = 'MERGED', survivor_id = ? WHERE id = ? AND org_id = ?",
			tableName,
		)
		result, err := tx.ExecContext(ctx, archiveSQL, now, req.SurvivorID, dupID, orgID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to archive duplicate %s: %w", dupID, err)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			tx.Rollback()
			return nil, fmt.Errorf("duplicate record %s not found or not accessible in org", dupID)
		}
	}

	// 11. Save snapshot (inside transaction using direct SQL)
	snapshotID := sfid.NewMergeSnapshot()
	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(30 * 24 * time.Hour) // 30 days

	_, err = tx.ExecContext(ctx,
		`INSERT INTO merge_snapshots (
			id, org_id, entity_type, survivor_id, survivor_before,
			duplicate_ids, duplicate_snapshots, related_record_fks,
			merged_by_id, consumed_at, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshotID,
		orgID,
		req.EntityType,
		req.SurvivorID,
		toJSON(survivorBefore),
		toJSON(req.DuplicateIDs),
		toJSON(duplicateSnapshots),
		toJSON(relatedRecordFKs),
		userID,
		nil, // consumed_at is NULL
		createdAt.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to save snapshot: %w", err)
	}

	// 12. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12.5 Resolve pending duplicate alerts (fire-and-forget, outside transaction)
	if s.pendingAlertRepo != nil {
		for _, dupID := range req.DuplicateIDs {
			s.pendingAlertRepo.Resolve(ctx, orgID, req.EntityType, dupID, "merged", userID, "")
		}
		s.pendingAlertRepo.Resolve(ctx, orgID, req.EntityType, req.SurvivorID, "merged", userID, "")

		// 12.6 Cascade: resolve alerts where matches_json references any merged duplicate
		// Best-effort — don't fail the merge if this errors
		allMergedIDs := append(req.DuplicateIDs, req.SurvivorID)
		for _, mergedID := range allMergedIDs {
			// Find pending alerts whose matches_json contains this record ID
			cascadeQuery := `
				SELECT record_id FROM pending_duplicate_alerts
				WHERE org_id = ? AND entity_type = ? AND status = 'pending'
				AND matches_json LIKE ?
			`
			likePattern := "%" + mergedID + "%"
			rows, err := db.QueryContext(ctx, cascadeQuery, orgID, req.EntityType, likePattern)
			if err != nil {
				log.Printf("WARNING: cascade alert cleanup query failed for %s: %v", mergedID, err)
				continue
			}
			var affectedRecordIDs []string
			for rows.Next() {
				var recordID string
				if err := rows.Scan(&recordID); err == nil {
					affectedRecordIDs = append(affectedRecordIDs, recordID)
				}
			}
			rows.Close()
			for _, recordID := range affectedRecordIDs {
				s.pendingAlertRepo.Resolve(ctx, orgID, req.EntityType, recordID, "merged", userID, "")
			}
		}
	}

	// 13. Create audit log (OUTSIDE transaction, fire-and-forget)
	if s.auditLogger != nil {
		go s.auditLogger.LogRecordMerge(ctx, userID, orgID, req.EntityType, req.SurvivorID, snapshotID, req.DuplicateIDs)
	}

	// 14. Cleanup expired snapshots (OUTSIDE transaction, opportunistic)
	go func() {
		ctx := context.Background()
		s.mergeRepo.CleanupExpired(ctx, orgID)
	}()

	// 15. Return result
	return &entity.MergeResult{
		SurvivorID: req.SurvivorID,
		SnapshotID: snapshotID,
		MergedAt:   now,
	}, nil
}

// UndoMerge reverses a merge operation atomically
// Restores survivor to pre-merge state, un-archives duplicates, reverts FK references
func (s *MergeService) UndoMerge(ctx context.Context, db *sql.DB, orgID, userID, snapshotID string) error {
	// 1. Fetch snapshot
	snapshot, err := s.mergeRepo.GetByID(ctx, orgID, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to fetch snapshot: %w", err)
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	// 2. Validate snapshot
	now := time.Now().UTC()
	if snapshot.ExpiresAt.Before(now) {
		return fmt.Errorf("snapshot has expired (undo window is 30 days)")
	}
	if snapshot.ConsumedAt != nil {
		return fmt.Errorf("snapshot has already been consumed (undo already performed)")
	}

	tableName := util.GetTableName(snapshot.EntityType)

	// 3. Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 4. Restore survivor to pre-merge state
	survivorBefore := fromJSON[map[string]interface{}](snapshot.SurvivorBefore)
	if err := restoreRecordFromSnapshot(ctx, tx, tableName, snapshot.SurvivorID, orgID, survivorBefore); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to restore survivor: %w", err)
	}

	// 5. Un-archive each duplicate
	duplicateSnapshots := fromJSON[[]map[string]interface{}](snapshot.DuplicateSnapshots)
	duplicateIDs := fromJSON[[]string](snapshot.DuplicateIDs)

	for i, dupID := range duplicateIDs {
		// Un-archive
		unarchiveSQL := fmt.Sprintf(
			"UPDATE %s SET archived_at = NULL, archived_reason = NULL, survivor_id = NULL WHERE id = ? AND org_id = ?",
			tableName,
		)
		_, err := tx.ExecContext(ctx, unarchiveSQL, dupID, orgID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to un-archive duplicate %s: %w", dupID, err)
		}

		// Restore duplicate fields from snapshot
		if i < len(duplicateSnapshots) {
			if err := restoreRecordFromSnapshot(ctx, tx, tableName, dupID, orgID, duplicateSnapshots[i]); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to restore duplicate %s: %w", dupID, err)
			}
		}
	}

	// 6. Revert FK references
	relatedRecordFKs := fromJSON[map[string][]entity.FKChange](snapshot.RelatedRecordFKs)

	for entityType, fkChanges := range relatedRecordFKs {
		groupTableName := util.GetTableName(entityType)

		for _, fkChange := range fkChanges {
			fkColumn := util.CamelToSnake(fkChange.FKField) + "_id"

			revertSQL := fmt.Sprintf(
				"UPDATE %s SET %s = ? WHERE id = ? AND org_id = ?",
				groupTableName,
				util.QuoteIdentifier(fkColumn),
			)
			_, err := tx.ExecContext(ctx, revertSQL, fkChange.OldValue, fkChange.RecordID, orgID)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to revert FK for record %s: %w", fkChange.RecordID, err)
			}
		}
	}

	// 7. Mark snapshot consumed
	consumedAt := time.Now().UTC().Format(time.RFC3339)
	markConsumedSQL := `UPDATE merge_snapshots SET consumed_at = ? WHERE id = ? AND org_id = ?`
	_, err = tx.ExecContext(ctx, markConsumedSQL, consumedAt, snapshotID, orgID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark snapshot as consumed: %w", err)
	}

	// 8. Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 9. Audit log (outside transaction)
	if s.auditLogger != nil {
		go s.auditLogger.LogMergeUndo(ctx, userID, orgID, snapshotID, snapshot.SurvivorID)
	}

	return nil
}

// GenerateMergeReport creates a CSV report of all merge operations.
// Each snapshot produces 1 survivor row + N duplicate rows. Field data comes from snapshot JSON.
func (s *MergeService) GenerateMergeReport(snapshots []entity.MergeSnapshot) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// First pass: collect all unique field keys across all snapshots
	type reportRow struct {
		RowNumber        int
		MergeDate        string
		MergeGroup       string
		Action           string
		RecordID         string
		SurvivorRecordID string
		MergedBy         string
		CanUndo          string
		EntityType       string
		Fields           map[string]interface{}
	}

	var rows []reportRow
	fieldSeen := map[string]bool{}
	rowNum := 0

	for _, snapshot := range snapshots {
		// Parse survivor_before JSON
		var survivorData map[string]interface{}
		if err := json.Unmarshal([]byte(snapshot.SurvivorBefore), &survivorData); err != nil {
			log.Printf("WARNING: malformed survivor_before JSON in snapshot %s: %v", snapshot.ID, err)
			continue
		}

		// Parse duplicate_snapshots JSON
		var dupSnapshots []map[string]interface{}
		if err := json.Unmarshal([]byte(snapshot.DuplicateSnapshots), &dupSnapshots); err != nil {
			log.Printf("WARNING: malformed duplicate_snapshots JSON in snapshot %s: %v", snapshot.ID, err)
			continue
		}

		// Parse duplicate_ids JSON
		var dupIDs []string
		if err := json.Unmarshal([]byte(snapshot.DuplicateIDs), &dupIDs); err != nil {
			log.Printf("WARNING: malformed duplicate_ids JSON in snapshot %s: %v", snapshot.ID, err)
			continue
		}

		canUndo := "Yes"
		if snapshot.ConsumedAt != nil {
			canUndo = "No"
		}

		mergeDate := snapshot.CreatedAt.Format("2006-01-02")

		// Collect field keys from survivor
		for k := range survivorData {
			if !isSystemField(k) {
				fieldSeen[k] = true
			}
		}

		// Survivor row
		rowNum++
		rows = append(rows, reportRow{
			RowNumber:        rowNum,
			MergeDate:        mergeDate,
			MergeGroup:       snapshot.ID,
			Action:           "Survivor",
			RecordID:         snapshot.SurvivorID,
			SurvivorRecordID: snapshot.SurvivorID,
			MergedBy:         snapshot.MergedByID,
			CanUndo:          canUndo,
			EntityType:       snapshot.EntityType,
			Fields:           survivorData,
		})

		// Duplicate rows
		for i, dupID := range dupIDs {
			var dupData map[string]interface{}
			if i < len(dupSnapshots) {
				dupData = dupSnapshots[i]
				for k := range dupData {
					if !isSystemField(k) {
						fieldSeen[k] = true
					}
				}
			}

			rowNum++
			rows = append(rows, reportRow{
				RowNumber:        rowNum,
				MergeDate:        mergeDate,
				MergeGroup:       snapshot.ID,
				Action:           "Merged (Deleted)",
				RecordID:         dupID,
				SurvivorRecordID: snapshot.SurvivorID,
				MergedBy:         snapshot.MergedByID,
				CanUndo:          canUndo,
				EntityType:       snapshot.EntityType,
				Fields:           dupData,
			})
		}
	}

	// Sort field keys alphabetically
	var fieldOrder []string
	for k := range fieldSeen {
		fieldOrder = append(fieldOrder, k)
	}
	sort.Strings(fieldOrder)

	// Write header
	header := []string{"Row Number", "Merge Date", "Merge Group", "Action", "Record ID", "Survivor Record ID", "Merged By", "Can Undo", "Entity Type"}
	header = append(header, fieldOrder...)
	writer.Write(header)

	// Write rows
	for _, r := range rows {
		row := []string{
			fmt.Sprintf("%d", r.RowNumber),
			r.MergeDate,
			r.MergeGroup,
			r.Action,
			r.RecordID,
			r.SurvivorRecordID,
			r.MergedBy,
			r.CanUndo,
			r.EntityType,
		}
		for _, key := range fieldOrder {
			val := ""
			if r.Fields != nil {
				if v, ok := r.Fields[key]; ok && v != nil {
					val = fmt.Sprintf("%v", v)
				}
			}
			row = append(row, val)
		}
		writer.Write(row)
	}

	writer.Flush()
	return buf.Bytes()
}

// isSystemField returns true for fields that should be excluded from the dynamic columns
func isSystemField(name string) bool {
	switch name {
	case "id", "orgId", "org_id", "createdAt", "created_at", "modifiedAt", "modified_at",
		"createdById", "created_by_id", "modifiedById", "modified_by_id",
		"archivedAt", "archived_at", "archivedReason", "archived_reason",
		"survivorId", "survivor_id":
		return true
	}
	return false
}

// Helper functions

// fetchRecordAsMapTx fetches a record within a transaction and returns it as a camelCase map
func fetchRecordAsMapTx(ctx context.Context, tx *sql.Tx, tableName, id, orgID string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND org_id = ?", tableName)
	rows, err := tx.QueryContext(ctx, query, id, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, nil // Record not found
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	record := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			record[col] = string(b)
		} else {
			record[col] = val
		}
	}

	// Convert snake_case to camelCase (matching util.FetchRecordAsMap pattern)
	camelRecord := make(map[string]interface{})
	for col, val := range record {
		camelCol := util.SnakeToCamel(col)
		camelRecord[camelCol] = val
	}

	return camelRecord, nil
}

// restoreRecordFromSnapshot restores a record to its snapshot state within a transaction
func restoreRecordFromSnapshot(ctx context.Context, tx *sql.Tx, tableName, id, orgID string, snapshot map[string]interface{}) error {
	if snapshot == nil || len(snapshot) == 0 {
		return nil // Nothing to restore
	}

	var setClauses []string
	var values []interface{}

	for key, val := range snapshot {
		// Skip system fields
		if key == "id" || key == "orgId" {
			continue
		}

		// Convert camelCase key to snake_case column name
		colName := util.CamelToSnake(key)
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(colName)))
		values = append(values, val)
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Add WHERE clause values
	values = append(values, id, orgID)

	updateSQL := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName, strings.Join(setClauses, ", "))

	result, err := tx.ExecContext(ctx, updateSQL, values...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("record not found or not accessible: %s", id)
	}

	return nil
}

// toJSON marshals a value to JSON string
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Return safe defaults on error
		switch v.(type) {
		case []interface{}, []string, []map[string]interface{}:
			return "[]"
		default:
			return "{}"
		}
	}
	return string(b)
}

// fromJSON unmarshals a JSON string to typed value
func fromJSON[T any](s string) T {
	var result T
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		// Return zero value on error
		return result
	}
	return result
}
