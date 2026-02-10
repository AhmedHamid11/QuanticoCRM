package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
)

// SFDeliveryService manages asynchronous delivery of merge instruction batches to Salesforce
type SFDeliveryService struct {
	oauthService   *SalesforceOAuthService
	payloadBuilder *MergeInstructionBuilder
	batchAssembler *BatchAssembler
	repo           *repo.SalesforceRepo
	dbManager      *db.Manager
	authRepo       *repo.AuthRepo

	// Per-org concurrency control (max 1 concurrent delivery per org)
	runningJobs map[string]bool
	runningMu   sync.Mutex
}

// NewSFDeliveryService creates a new SFDeliveryService
func NewSFDeliveryService(
	oauthService *SalesforceOAuthService,
	payloadBuilder *MergeInstructionBuilder,
	batchAssembler *BatchAssembler,
	repo *repo.SalesforceRepo,
	dbManager *db.Manager,
	authRepo *repo.AuthRepo,
) *SFDeliveryService {
	return &SFDeliveryService{
		oauthService:   oauthService,
		payloadBuilder: payloadBuilder,
		batchAssembler: batchAssembler,
		repo:           repo,
		dbManager:      dbManager,
		authRepo:       authRepo,
		runningJobs:    make(map[string]bool),
	}
}

// QueueMergeInstructions queues merge instructions for async delivery to Salesforce
func (s *SFDeliveryService) QueueMergeInstructions(
	ctx context.Context,
	orgID string,
	inputs []MergeInstructionInput,
) (string, error) {
	// 1. Check connection status (must be "connected")
	status, err := s.oauthService.GetConnectionStatus(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to check connection status: %w", err)
	}
	if status != "connected" {
		return "", fmt.Errorf("salesforce not connected (status: %s)", status)
	}

	// 2. Get connection for Salesforce org ID
	conn, err := s.oauthService.GetConfig(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get connection config: %w", err)
	}
	if conn == nil {
		return "", fmt.Errorf("salesforce connection not configured")
	}

	// Extract Salesforce org ID from instance_url (e.g., https://na1.salesforce.com -> na1)
	// For simplicity, use orgID as fallback if parsing fails
	sfOrgID := orgID
	if conn.InstanceURL != "" {
		// Instance URL format: https://[instance].salesforce.com or https://[custom].my.salesforce.com
		// For batch ID purposes, using Quantico orgID is sufficient
		sfOrgID = orgID
	}

	// 3. Build instructions via payloadBuilder
	instructions, err := s.payloadBuilder.BuildInstructions(ctx, orgID, inputs)
	if err != nil {
		return "", fmt.Errorf("failed to build merge instructions: %w", err)
	}

	if len(instructions) == 0 {
		return "", fmt.Errorf("no merge instructions to deliver")
	}

	// 4. Assemble batches via batchAssembler
	batches, err := s.batchAssembler.AssembleBatches(sfOrgID, instructions)
	if err != nil {
		return "", fmt.Errorf("failed to assemble batches: %w", err)
	}

	// 5. Get tenant DB for sync job creation
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	tenantDB, err := s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant database: %w", err)
	}

	// 6. Create sync job records for each batch
	jobIDs := make([]string, 0, len(batches))
	for i, batch := range batches {
		// Validate batch before creating job
		if err := s.batchAssembler.ValidateBatch(batch); err != nil {
			log.Printf("Warning: Batch %d failed validation: %v", i, err)
			continue
		}

		// Serialize batch payload for storage
		batchPayload, err := s.batchAssembler.SerializeBatch(batch)
		if err != nil {
			log.Printf("Warning: Failed to serialize batch %d: %v", i, err)
			continue
		}

		// Create sync job
		jobID := sfid.New("syncjob")
		idempotencyKey := fmt.Sprintf("%s-%s", orgID, batch.BatchID)
		batchPayloadStr := string(batchPayload)

		// Determine entity type from first instruction
		entityType := ""
		if len(batch.MergeInstructions) > 0 {
			entityType = batch.MergeInstructions[0].ObjectAPIName
		}

		job := &entity.SyncJob{
			ID:                jobID,
			OrgID:             orgID,
			BatchID:           batch.BatchID,
			EntityType:        entityType,
			Status:            entity.SyncStatusPending,
			TotalInstructions: len(batch.MergeInstructions),
			BatchPayload:      &batchPayloadStr,
			IdempotencyKey:    idempotencyKey,
			TriggerType:       entity.SyncTriggerManual, // Default to manual
		}

		if err := s.repo.WithDB(tenantDB).CreateSyncJob(ctx, job); err != nil {
			log.Printf("Warning: Failed to create sync job for batch %s: %v", batch.BatchID, err)
			continue
		}

		jobIDs = append(jobIDs, jobID)
	}

	if len(jobIDs) == 0 {
		return "", fmt.Errorf("failed to create any sync jobs")
	}

	// 7. Launch async goroutine for batch delivery
	go s.executeBatchDelivery(orgID, jobIDs)

	// Return first job ID (HTTP 202 pattern)
	return jobIDs[0], nil
}

// executeBatchDelivery executes batch delivery in the background (async goroutine)
func (s *SFDeliveryService) executeBatchDelivery(orgID string, jobIDs []string) {
	// Acquire per-org lock (only 1 concurrent delivery per org)
	s.runningMu.Lock()
	if s.runningJobs[orgID] {
		s.runningMu.Unlock()
		log.Printf("Delivery already running for org %s, skipping", orgID)
		return
	}
	s.runningJobs[orgID] = true
	s.runningMu.Unlock()

	// Release lock on completion
	defer func() {
		s.runningMu.Lock()
		delete(s.runningJobs, orgID)
		s.runningMu.Unlock()
	}()

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in batch delivery for org %s: %v", orgID, r)
		}
	}()

	// Use background context for async execution
	ctx := context.Background()

	// Get org details for tenant DB
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		log.Printf("Failed to get organization %s: %v", orgID, err)
		return
	}

	// Get tenant DB
	tenantDB, err := s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		log.Printf("Failed to get tenant DB for org %s: %v", orgID, err)
		return
	}

	// Process each job
	for _, jobID := range jobIDs {
		// Load job
		job, err := s.repo.WithDB(tenantDB).GetSyncJob(ctx, jobID)
		if err != nil {
			log.Printf("Failed to load sync job %s: %v", jobID, err)
			continue
		}

		// Update status to running
		now := time.Now().UTC()
		job.StartedAt = &now
		job.Status = entity.SyncStatusRunning
		if err := s.repo.WithDB(tenantDB).UpdateSyncJobStatus(ctx, jobID, entity.SyncStatusRunning); err != nil {
			log.Printf("Warning: Failed to update job status for %s: %v", jobID, err)
		}

		// Get authenticated HTTP client
		httpClient, err := s.oauthService.GetHTTPClient(ctx, orgID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get HTTP client: %v", err)
			log.Printf("Job %s failed: %s", jobID, errMsg)
			_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusFailed, 0, 0, &errMsg)
			continue
		}

		// Parse batch payload
		if job.BatchPayload == nil {
			errMsg := "batch payload is nil"
			log.Printf("Job %s failed: %s", jobID, errMsg)
			_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusFailed, 0, 0, &errMsg)
			continue
		}

		batchPayload := []byte(*job.BatchPayload)

		// Deliver batch
		deliveredCount, err := s.deliverBatch(ctx, orgID, httpClient, batchPayload, job)
		if err != nil {
			errMsg := err.Error()
			log.Printf("Job %s failed: %s", jobID, errMsg)
			_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusFailed, 0, job.TotalInstructions, &errMsg)
			continue
		}

		// Mark completed
		log.Printf("Job %s completed: %d/%d instructions delivered", jobID, deliveredCount, job.TotalInstructions)
		_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusCompleted, deliveredCount, 0, nil)
	}
}

// deliverBatch POSTs a batch to Salesforce REST API
func (s *SFDeliveryService) deliverBatch(
	ctx context.Context,
	orgID string,
	client *http.Client,
	payload []byte,
	job *entity.SyncJob,
) (int, error) {
	// Get connection for instance URL
	conn, err := s.oauthService.GetConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get connection: %w", err)
	}
	if conn == nil || conn.InstanceURL == "" {
		return 0, fmt.Errorf("instance URL not configured")
	}

	// Get Salesforce API version from env (default v60.0)
	apiVersion := os.Getenv("SALESFORCE_API_VERSION")
	if apiVersion == "" {
		apiVersion = "v60.0"
	}

	// Build Salesforce REST API URL
	url := fmt.Sprintf("%s/services/data/%s/composite/sobjects", conn.InstanceURL, apiVersion)

	// Create HTTP request with idempotency key
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", job.IdempotencyKey)

	// Execute request with basic retry for 5xx errors
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for job %s", attempt, job.ID)
			time.Sleep(2 * time.Second) // Fixed 2-second delay (Phase 18 adds exponential backoff)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			if isRetryableHTTPError(0) {
				continue // Network error, retry
			}
			return 0, lastErr
		}

		// Read response body
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Handle response status
		switch resp.StatusCode {
		case 200, 201:
			// Success - parse response for record count
			// Salesforce Composite API returns array of results
			var results []map[string]interface{}
			if err := json.Unmarshal(body, &results); err != nil {
				log.Printf("Warning: Failed to parse Salesforce response for job %s: %v", job.ID, err)
				return job.TotalInstructions, nil // Assume success
			}
			return len(results), nil

		case 400:
			// Bad Request - permanent error
			errMsg := s.parseSalesforceError(body)
			return 0, fmt.Errorf("bad request (400): %s", errMsg)

		case 401:
			// Unauthorized - token issue, attempt one refresh then retry
			if attempt == 0 {
				log.Printf("Token unauthorized for job %s, attempting refresh", job.ID)
				// GetHTTPClient already handles token refresh, so just retry
				continue
			}
			errMsg := s.parseSalesforceError(body)
			return 0, fmt.Errorf("unauthorized (401): %s", errMsg)

		case 429:
			// Rate limit - DO NOT retry here (Phase 18 handles advanced retry)
			errMsg := s.parseSalesforceError(body)
			return 0, fmt.Errorf("rate limit exceeded (429): %s", errMsg)

		case 500, 502, 503, 504:
			// Server error - retryable
			if attempt < maxRetries {
				lastErr = fmt.Errorf("server error (%d): %s", resp.StatusCode, s.parseSalesforceError(body))
				continue
			}
			errMsg := s.parseSalesforceError(body)
			return 0, fmt.Errorf("server error (%d) after %d retries: %s", resp.StatusCode, maxRetries, errMsg)

		default:
			// Unexpected status
			errMsg := s.parseSalesforceError(body)
			return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errMsg)
		}
	}

	return 0, lastErr
}

// parseSalesforceError extracts error message from Salesforce API error response
func (s *SFDeliveryService) parseSalesforceError(body []byte) string {
	// Salesforce error format: [{"message": "...", "errorCode": "...", "fields": [...]}]
	var errors []struct {
		Message   string   `json:"message"`
		ErrorCode string   `json:"errorCode"`
		Fields    []string `json:"fields"`
	}

	if err := json.Unmarshal(body, &errors); err != nil {
		// Failed to parse, return raw body (truncated)
		if len(body) > 500 {
			return string(body[:500]) + "..."
		}
		return string(body)
	}

	if len(errors) == 0 {
		return "unknown error"
	}

	// Return first error message
	firstErr := errors[0]
	if firstErr.ErrorCode != "" {
		return fmt.Sprintf("%s: %s", firstErr.ErrorCode, firstErr.Message)
	}
	return firstErr.Message
}

// isRetryableHTTPError checks if an HTTP status code should trigger retry
func isRetryableHTTPError(statusCode int) bool {
	switch statusCode {
	case 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// GetJobStatus retrieves a sync job by ID
func (s *SFDeliveryService) GetJobStatus(ctx context.Context, orgID, jobID string) (*entity.SyncJob, error) {
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	tenantDB, err := s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant database: %w", err)
	}

	job, err := s.repo.WithDB(tenantDB).GetSyncJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync job: %w", err)
	}

	return job, nil
}

// ListJobs retrieves sync jobs for an org with pagination
func (s *SFDeliveryService) ListJobs(ctx context.Context, orgID string, limit, offset int) ([]entity.SyncJob, int, error) {
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get organization: %w", err)
	}

	// Use master DB if no tenant DB configured
	var tenantDB db.DBConn
	if org.DatabaseURL != "" {
		var err error
		tenantDB, err = s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get tenant database: %w", err)
		}
	} else {
		tenantDB = s.dbManager.GetMasterDB()
	}

	jobs, total, err := s.repo.WithDB(tenantDB).ListSyncJobs(ctx, orgID, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			log.Printf("[SALESFORCE] sync_jobs table not found for org %s, returning empty list", orgID)
			return []entity.SyncJob{}, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to list sync jobs: %w", err)
	}

	return jobs, total, nil
}

// RetryJob retries a failed sync job
func (s *SFDeliveryService) RetryJob(ctx context.Context, orgID, jobID string) error {
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Use master DB if no tenant DB configured
	var tenantDB db.DBConn
	if org.DatabaseURL != "" {
		var err error
		tenantDB, err = s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			return fmt.Errorf("failed to get tenant database: %w", err)
		}
	} else {
		tenantDB = s.dbManager.GetMasterDB()
	}

	// Load job
	job, err := s.repo.WithDB(tenantDB).GetSyncJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get sync job: %w", err)
	}

	// Verify status is "failed"
	if job.Status != entity.SyncStatusFailed {
		return fmt.Errorf("job status is %s, can only retry failed jobs", job.Status)
	}

	// Reset status to pending
	if err := s.repo.WithDB(tenantDB).UpdateSyncJobStatus(ctx, jobID, entity.SyncStatusPending); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Increment retry count
	job.RetryCount++
	// Note: UpdateSyncJobStatus doesn't update retry_count, so we'll need to update it separately
	// For now, we'll launch the async delivery and let it proceed

	// Clear error message
	query := `UPDATE sync_jobs SET error_message = NULL, retry_count = retry_count + 1, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tenantDB.ExecContext(ctx, query, now, jobID)
	if err != nil {
		log.Printf("Warning: Failed to clear error message for job %s: %v", jobID, err)
	}

	// Launch async delivery
	go s.executeBatchDelivery(orgID, []string{jobID})

	return nil
}
