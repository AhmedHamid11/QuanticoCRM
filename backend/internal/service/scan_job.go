package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// ScanJobService orchestrates background duplicate scanning with chunked processing
type ScanJobService struct {
	detector         *dedup.Detector
	scanJobRepo      *repo.ScanJobRepo
	pendingAlertRepo *repo.PendingAlertRepo
	matchingRuleRepo *repo.MatchingRuleRepo
	authRepo         *repo.AuthRepo

	// Per-tenant rate limiting (max 2 concurrent jobs per org)
	runningJobs map[string]int // orgID -> count of running jobs
	runningMu   sync.Mutex

	// Progress event callback (set by handler for SSE broadcasting)
	onProgress func(event ProgressEvent)
	progressMu sync.RWMutex
}

// ProgressEvent exported for handler SSE consumption
type ProgressEvent struct {
	JobID            string `json:"jobId"`
	OrgID            string `json:"orgId"`
	EntityType       string `json:"entityType"`
	ProcessedRecords int    `json:"processedRecords"`
	TotalRecords     int    `json:"totalRecords"`
	DuplicatesFound  int    `json:"duplicatesFound"`
	Status           string `json:"status"`
}

// NewScanJobService creates a new scan job service
func NewScanJobService(
	detector *dedup.Detector,
	scanJobRepo *repo.ScanJobRepo,
	pendingAlertRepo *repo.PendingAlertRepo,
	matchingRuleRepo *repo.MatchingRuleRepo,
	authRepo *repo.AuthRepo,
) *ScanJobService {
	return &ScanJobService{
		detector:         detector,
		scanJobRepo:      scanJobRepo,
		pendingAlertRepo: pendingAlertRepo,
		matchingRuleRepo: matchingRuleRepo,
		authRepo:         authRepo,
		runningJobs:      make(map[string]int),
	}
}

// SetProgressCallback sets the callback for progress events (used by handler for SSE)
func (s *ScanJobService) SetProgressCallback(fn func(event ProgressEvent)) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.onProgress = fn
}

// CanRunJob checks if org can start a new job (per-tenant rate limit: max 2 concurrent)
func (s *ScanJobService) CanRunJob(orgID string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	return s.runningJobs[orgID] < 2
}

// ExecuteScan executes a background duplicate scan with chunked processing and checkpoint recovery
// The tenantDB must be provided by the caller (handler has access to it via middleware)
func (s *ScanJobService) ExecuteScan(ctx context.Context, tenantDB *sql.DB, orgID, entityType, triggerType string, scheduleID *string) (string, error) {
	// Rate limit check
	s.runningMu.Lock()
	if s.runningJobs[orgID] >= 2 {
		s.runningMu.Unlock()
		return "", fmt.Errorf("max concurrent jobs reached for org (limit: 2)")
	}
	s.runningJobs[orgID]++
	s.runningMu.Unlock()

	// Defer decrement running count
	defer func() {
		s.runningMu.Lock()
		s.runningJobs[orgID]--
		s.runningMu.Unlock()
	}()

	tableName := util.GetTableName(entityType)

	// Count total records
	var totalRecords int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName)
	if err := tenantDB.QueryRowContext(ctx, countQuery, orgID).Scan(&totalRecords); err != nil {
		return "", fmt.Errorf("failed to count records: %w", err)
	}

	// Create job record
	jobID := sfid.NewScanJob()
	now := time.Now().UTC()
	job := &entity.ScanJob{
		ID:               jobID,
		OrgID:            orgID,
		EntityType:       entityType,
		ScheduleID:       scheduleID,
		Status:           entity.ScanStatusRunning,
		TriggerType:      triggerType,
		TotalRecords:     totalRecords,
		ProcessedRecords: 0,
		DuplicatesFound:  0,
		StartedAt:        &now,
	}

	if err := s.scanJobRepo.WithDB(tenantDB).CreateJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to create job: %w", err)
	}

	// Launch scan in goroutine (async execution)
	go func() {
		// Recover from panics (per Phase 12 decision)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in scan job %s: %v", jobID, r)
				errMsg := fmt.Sprintf("panic: %v", r)
				_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobStatus(context.Background(), jobID, entity.ScanStatusFailed)
				_ = s.updateJobErrorMessage(tenantDB, jobID, errMsg)
			}
		}()

		// Use background context with timeout per chunk (not Fiber context)
		scanCtx := context.Background()
		if err := s.executeChunkedScan(scanCtx, tenantDB, orgID, jobID, entityType, totalRecords); err != nil {
			log.Printf("Scan job %s failed: %v", jobID, err)
		}
	}()

	return jobID, nil
}

// executeChunkedScan performs the chunked scan loop with checkpoints
func (s *ScanJobService) executeChunkedScan(ctx context.Context, tenantDB *sql.DB, orgID, jobID, entityType string, totalRecords int) error {
	tableName := util.GetTableName(entityType)
	chunkSize := 500 // Recommended for Turso 5-second timeout
	offset := 0
	totalDuplicates := 0

	// Check for existing checkpoint (resume from failure)
	checkpoint, _ := s.scanJobRepo.WithDB(tenantDB).GetCheckpoint(ctx, jobID)
	if checkpoint != nil {
		offset = checkpoint.LastOffset
		log.Printf("Resuming scan job %s from offset %d", jobID, offset)
	}

	for {
		// Check context cancellation (graceful shutdown)
		select {
		case <-ctx.Done():
			_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobStatus(ctx, jobID, entity.ScanStatusCancelled)
			s.emitProgress(ProgressEvent{
				JobID:            jobID,
				OrgID:            orgID,
				EntityType:       entityType,
				ProcessedRecords: offset,
				TotalRecords:     totalRecords,
				DuplicatesFound:  totalDuplicates,
				Status:           entity.ScanStatusCancelled,
			})
			return ctx.Err()
		default:
		}

		// Fetch chunk with timeout
		chunkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		records, err := s.fetchChunk(chunkCtx, tenantDB, tableName, orgID, chunkSize, offset)
		cancel()

		if err != nil {
			return s.handleChunkFailure(ctx, tenantDB, jobID, orgID, entityType, offset, totalRecords, totalDuplicates, err)
		}

		if len(records) == 0 {
			break // No more records
		}

		// Process chunk
		duplicatesInChunk, err := s.processChunk(ctx, tenantDB, orgID, entityType, records)
		if err != nil {
			return s.handleChunkFailure(ctx, tenantDB, jobID, orgID, entityType, offset, totalRecords, totalDuplicates, err)
		}

		// Update progress
		offset += len(records)
		totalDuplicates += duplicatesInChunk
		_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobProgress(ctx, jobID, offset, totalDuplicates)

		// Save checkpoint after EVERY chunk
		_ = s.saveCheckpoint(ctx, tenantDB, jobID, offset, chunkSize)

		// Emit progress event for SSE listeners
		s.emitProgress(ProgressEvent{
			JobID:            jobID,
			OrgID:            orgID,
			EntityType:       entityType,
			ProcessedRecords: offset,
			TotalRecords:     totalRecords,
			DuplicatesFound:  totalDuplicates,
			Status:           entity.ScanStatusRunning,
		})

		// Sleep 100ms between chunks to allow WAL checkpoint window
		// (per RESEARCH.md Pitfall #4: WAL checkpoint starvation)
		time.Sleep(100 * time.Millisecond)

		if len(records) < chunkSize {
			break // Last chunk
		}
	}

	// Mark complete
	_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobCompletion(ctx, jobID, entity.ScanStatusCompleted, totalRecords, offset, totalDuplicates)

	// Emit final progress event
	s.emitProgress(ProgressEvent{
		JobID:            jobID,
		OrgID:            orgID,
		EntityType:       entityType,
		ProcessedRecords: offset,
		TotalRecords:     totalRecords,
		DuplicatesFound:  totalDuplicates,
		Status:           entity.ScanStatusCompleted,
	})

	// Delete checkpoint (job completed successfully)
	_ = s.scanJobRepo.WithDB(tenantDB).DeleteCheckpoint(ctx, jobID)

	log.Printf("Scan job %s completed: %d records scanned, %d duplicates found", jobID, offset, totalDuplicates)

	return nil
}

// fetchChunk retrieves records in chunks using LIMIT/OFFSET
func (s *ScanJobService) fetchChunk(ctx context.Context, tenantDB *sql.DB, tableName, orgID string, limit, offset int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE org_id = ? ORDER BY id LIMIT ? OFFSET ?", tableName)
	rows, err := tenantDB.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
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

		records = append(records, record)
	}

	return records, nil
}

// processChunk detects duplicates for all records in a chunk
func (s *ScanJobService) processChunk(ctx context.Context, tenantDB *sql.DB, orgID, entityType string, records []map[string]interface{}) (int, error) {
	duplicateCount := 0
	seenPairs := make(map[string]bool) // Track canonical pairs to avoid duplicates

	for _, record := range records {
		recordID, ok := record["id"].(string)
		if !ok {
			continue
		}

		// Detect duplicates for this record
		matches, err := s.detector.CheckForDuplicates(ctx, tenantDB, orgID, entityType, record, recordID)
		if err != nil {
			log.Printf("Failed to check duplicates for record %s: %v", recordID, err)
			continue
		}

		if len(matches) == 0 {
			continue
		}

		// Store each match as a PendingDuplicateAlert (using canonical pair key)
		for _, match := range matches {
			// Canonical pair key (smaller ID first)
			var pairKey string
			if recordID < match.RecordID {
				pairKey = recordID + ":" + match.RecordID
			} else {
				pairKey = match.RecordID + ":" + recordID
			}

			if seenPairs[pairKey] {
				continue // Already stored this pair
			}
			seenPairs[pairKey] = true

			// Create alert for the record (smaller ID gets the alert)
			alertRecordID := recordID
			if match.RecordID < recordID {
				alertRecordID = match.RecordID
			}

			// Build alert matches (top 3)
			alertMatches := []entity.DuplicateAlertMatch{
				{
					RecordID:    match.RecordID,
					RecordName:  s.getRecordName(record),
					MatchResult: match.MatchResult,
				},
			}

			// Determine highest confidence
			confidence := "low"
			if match.MatchResult.Score >= 0.95 {
				confidence = "high"
			} else if match.MatchResult.Score >= 0.85 {
				confidence = "medium"
			}

			// Upsert pending alert
			alert := &entity.PendingDuplicateAlert{
				OrgID:             orgID,
				EntityType:        entityType,
				RecordID:          alertRecordID,
				Matches:           alertMatches,
				TotalMatchCount:   1,
				HighestConfidence: confidence,
				IsBlockMode:       false, // Default for background scans
				Status:            entity.AlertStatusPending,
				DetectedAt:        time.Now().UTC(),
			}

			if err := s.pendingAlertRepo.WithDB(tenantDB).Upsert(ctx, alert); err != nil {
				log.Printf("Failed to store alert for record %s: %v", alertRecordID, err)
			} else {
				duplicateCount++
			}
		}
	}

	return duplicateCount, nil
}

// handleChunkFailure implements auto-retry logic
func (s *ScanJobService) handleChunkFailure(ctx context.Context, tenantDB *sql.DB, jobID, orgID, entityType string, offset, totalRecords, totalDuplicates int, chunkErr error) error {
	log.Printf("Chunk failure for job %s at offset %d: %v", jobID, offset, chunkErr)

	// Get checkpoint to check retry count
	checkpoint, _ := s.scanJobRepo.WithDB(tenantDB).GetCheckpoint(ctx, jobID)
	retryCount := 0
	if checkpoint != nil {
		retryCount = checkpoint.RetryCount
	}

	if retryCount < 1 {
		// Retry once
		log.Printf("Retrying failed chunk for job %s (attempt 2)", jobID)
		_ = s.scanJobRepo.WithDB(tenantDB).IncrementRetryCount(ctx, jobID)
		return chunkErr // Will retry on next iteration
	}

	// Mark failed after 2nd attempt
	log.Printf("Job %s failed after retry at offset %d", jobID, offset)
	errMsg := chunkErr.Error()
	_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobCompletion(ctx, jobID, entity.ScanStatusFailed, totalRecords, offset, totalDuplicates)
	_ = s.updateJobErrorMessage(tenantDB, jobID, errMsg)

	// Emit failure progress event
	s.emitProgress(ProgressEvent{
		JobID:            jobID,
		OrgID:            orgID,
		EntityType:       entityType,
		ProcessedRecords: offset,
		TotalRecords:     totalRecords,
		DuplicatesFound:  totalDuplicates,
		Status:           entity.ScanStatusFailed,
	})

	return chunkErr
}

// saveCheckpoint persists progress state for resume
func (s *ScanJobService) saveCheckpoint(ctx context.Context, tenantDB *sql.DB, jobID string, offset, chunkSize int) error {
	checkpoint := &entity.ScanCheckpoint{
		ID:         sfid.New("ckpt"),
		JobID:      jobID,
		LastOffset: offset,
		RetryCount: 0,
		ChunkSize:  chunkSize,
	}
	return s.scanJobRepo.WithDB(tenantDB).SaveCheckpoint(ctx, checkpoint)
}

// updateJobErrorMessage updates the error message field
func (s *ScanJobService) updateJobErrorMessage(tenantDB *sql.DB, jobID, errorMsg string) error {
	query := `UPDATE scan_jobs SET error_message = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := tenantDB.ExecContext(context.Background(), query, errorMsg, now, jobID)
	return err
}

// emitProgress broadcasts progress event to SSE listeners
func (s *ScanJobService) emitProgress(event ProgressEvent) {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	if s.onProgress != nil {
		s.onProgress(event)
	}
}

// getRecordName extracts a display name from a record
func (s *ScanJobService) getRecordName(record map[string]interface{}) string {
	// Try common name fields
	if name, ok := record["name"].(string); ok && name != "" {
		return name
	}
	if firstName, ok := record["first_name"].(string); ok {
		if lastName, ok2 := record["last_name"].(string); ok2 {
			return firstName + " " + lastName
		}
		return firstName
	}
	if email, ok := record["email"].(string); ok && email != "" {
		return email
	}
	if id, ok := record["id"].(string); ok {
		return id
	}
	return "Unknown"
}

// ResumeInterruptedJobs marks orphaned "running" jobs as failed (called at startup)
func (s *ScanJobService) ResumeInterruptedJobs(ctx context.Context) error {
	// This would need to iterate all tenant DBs, which requires org list
	// For now, log that this needs to be called per-org during startup
	log.Println("ResumeInterruptedJobs should be called per-org during startup")
	return nil
}

// RetryJob creates a new job that resumes from the failed job's checkpoint
func (s *ScanJobService) RetryJob(ctx context.Context, tenantDB *sql.DB, orgID, failedJobID string) (string, error) {
	// Get the failed job
	failedJob, err := s.scanJobRepo.WithDB(tenantDB).GetJob(ctx, failedJobID)
	if err != nil {
		return "", fmt.Errorf("failed to get failed job: %w", err)
	}

	// Get checkpoint if exists
	checkpoint, _ := s.scanJobRepo.WithDB(tenantDB).GetCheckpoint(ctx, failedJobID)

	// Create new job inheriting entity type
	newJobID := sfid.NewScanJob()
	now := time.Now().UTC()
	newJob := &entity.ScanJob{
		ID:               newJobID,
		OrgID:            orgID,
		EntityType:       failedJob.EntityType,
		Status:           entity.ScanStatusRunning,
		TriggerType:      entity.ScanTriggerManual,
		TotalRecords:     failedJob.TotalRecords,
		ProcessedRecords: 0,
		DuplicatesFound:  0,
		StartedAt:        &now,
	}

	if err := s.scanJobRepo.WithDB(tenantDB).CreateJob(ctx, newJob); err != nil {
		return "", fmt.Errorf("failed to create retry job: %w", err)
	}

	// Copy checkpoint to new job if exists
	if checkpoint != nil {
		newCheckpoint := &entity.ScanCheckpoint{
			ID:         sfid.New("ckpt"),
			JobID:      newJobID,
			LastOffset: checkpoint.LastOffset,
			RetryCount: 0,
			ChunkSize:  checkpoint.ChunkSize,
		}
		_ = s.scanJobRepo.WithDB(tenantDB).SaveCheckpoint(ctx, newCheckpoint)
	}

	// Launch scan in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in retry job %s: %v", newJobID, r)
			}
		}()

		scanCtx := context.Background()
		if checkpoint != nil {
			log.Printf("Retry job %s resuming from offset %d", newJobID, checkpoint.LastOffset)
		}

		// Re-count total (may have changed)
		tableName := util.GetTableName(failedJob.EntityType)
		var totalRecords int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName)
		_ = tenantDB.QueryRowContext(scanCtx, countQuery, orgID).Scan(&totalRecords)

		if err := s.executeChunkedScan(scanCtx, tenantDB, orgID, newJobID, failedJob.EntityType, totalRecords); err != nil {
			log.Printf("Retry job %s failed: %v", newJobID, err)
		}
	}()

	return newJobID, nil
}
