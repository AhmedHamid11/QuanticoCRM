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
	"strconv"
	"strings"
	"sync"
	"time"

	backofflib "github.com/cenkalti/backoff/v4"
	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
)

// SFDeliveryService manages asynchronous delivery of merge instruction batches to Salesforce
type SFDeliveryService struct {
	oauthService     *SalesforceOAuthService
	payloadBuilder   *MergeInstructionBuilder
	batchAssembler   *BatchAssembler
	repo             *repo.SalesforceRepo
	dbManager        *db.Manager
	authRepo         *repo.AuthRepo
	rateLimitService *RateLimitService
	auditLogger      *AuditLogger

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
	rateLimitService *RateLimitService,
	auditLogger *AuditLogger,
) *SFDeliveryService {
	return &SFDeliveryService{
		oauthService:     oauthService,
		payloadBuilder:   payloadBuilder,
		batchAssembler:   batchAssembler,
		repo:             repo,
		dbManager:        dbManager,
		authRepo:         authRepo,
		rateLimitService: rateLimitService,
		auditLogger:      auditLogger,
		runningJobs:      make(map[string]bool),
	}
}

// DeliveryOptions controls delivery behavior
type DeliveryOptions struct {
	Force       bool   // Bypass rate limiting checks
	TriggerType string // manual, scheduled, realtime
}

// QueueMergeInstructionsWithOptions queues merge instructions with delivery options
func (s *SFDeliveryService) QueueMergeInstructionsWithOptions(
	ctx context.Context,
	orgID string,
	inputs []MergeInstructionInput,
	opts DeliveryOptions,
) (string, error) {
	// Pre-delivery quota check (unless force=true)
	if !opts.Force {
		canProceed, err := s.rateLimitService.CanMakeAPICalls(ctx, orgID, len(inputs))
		if err != nil {
			// Non-critical - log warning and proceed (graceful degradation)
			log.Printf("Warning: Failed to check API quota for org %s: %v", orgID, err)
		} else if !canProceed {
			// Quota exceeded - return error
			quota, _ := s.rateLimitService.GetQuotaStatus(ctx, orgID)
			return "", &entity.QuotaExceededError{
				OrgID:     orgID,
				Usage:     quota.Usage,
				Threshold: quota.Threshold,
				Message:   fmt.Sprintf("API quota threshold exceeded: %d/%d calls used (threshold: %d)", quota.Usage, quota.Limit, quota.Threshold),
			}
		}
	}

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

	// Set trigger type from options
	triggerType := opts.TriggerType
	if triggerType == "" {
		triggerType = entity.SyncTriggerManual
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
			TriggerType:       triggerType,
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

// QueueMergeInstructions queues merge instructions for async delivery to Salesforce
func (s *SFDeliveryService) QueueMergeInstructions(
	ctx context.Context,
	orgID string,
	inputs []MergeInstructionInput,
) (string, error) {
	return s.QueueMergeInstructionsWithOptions(ctx, orgID, inputs, DeliveryOptions{
		Force:       false,
		TriggerType: entity.SyncTriggerManual,
	})
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

	// Cleanup old API usage records (>25 hours)
	if err := s.rateLimitService.CleanupOldUsage(ctx, orgID); err != nil {
		log.Printf("Warning: Failed to cleanup old API usage for org %s: %v", orgID, err)
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
			// Record API usage even on failure (calls were made)
			if recErr := s.rateLimitService.RecordAPIUsage(ctx, orgID, job.ID, 1); recErr != nil {
				log.Printf("Warning: Failed to record API usage for failed job %s: %v", job.ID, recErr)
			}

			errMsg := err.Error()
			log.Printf("Job %s failed: %s", jobID, errMsg)

			// Audit log the delivery error
			s.auditLogger.LogSalesforceMergeDelivery(ctx, orgID, job.BatchID, "", "", "", "error", 0, "", job.RetryCount, errMsg)

			_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusFailed, 0, job.TotalInstructions, &errMsg)
			continue
		}

		// Record API usage for quota tracking
		if err := s.rateLimitService.RecordAPIUsage(ctx, orgID, job.ID, deliveredCount); err != nil {
			log.Printf("Warning: Failed to record API usage for job %s: %v", job.ID, err)
			// Non-critical - don't fail the job
		}

		// Mark completed
		log.Printf("Job %s completed: %d/%d instructions delivered", jobID, deliveredCount, job.TotalInstructions)

		// Audit log the successful delivery
		s.auditLogger.LogSalesforceMergeDelivery(ctx, orgID, job.BatchID, "", "", "", "success", 200, "", 0, "")

		_ = s.repo.WithDB(tenantDB).UpdateSyncJobCompletion(ctx, jobID, entity.SyncStatusCompleted, deliveredCount, 0, nil)
	}
}

// deliverBatch POSTs a batch to Salesforce REST API with exponential backoff retry
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

	var deliveredCount int

	// Configure exponential backoff: 5s, 10s, 20s, 40s with 50% jitter
	b := backofflib.NewExponentialBackOff()
	b.InitialInterval = 5 * time.Second
	b.Multiplier = 2.0
	b.MaxInterval = 40 * time.Second
	b.MaxElapsedTime = 0 // Use max retries instead
	b.RandomizationFactor = 0.5

	retryBackoff := backofflib.WithMaxRetries(b, 5) // Max 5 retries

	operation := func() error {
		// Create fresh request each attempt (body reader needs reset)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
		if err != nil {
			return backofflib.Permanent(fmt.Errorf("failed to create request: %w", err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", job.IdempotencyKey)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("HTTP request failed: %w", err) // Network error, retry
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		switch resp.StatusCode {
		case 200, 201:
			// Success
			var results []map[string]interface{}
			if err := json.Unmarshal(body, &results); err != nil {
				deliveredCount = job.TotalInstructions
				return nil
			}
			deliveredCount = len(results)
			return nil

		case 429:
			// Rate limit - check Retry-After header
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					log.Printf("Rate limited for job %s, Salesforce says retry after %ds", job.ID, seconds)
					time.Sleep(time.Duration(seconds) * time.Second)
				}
			}
			return fmt.Errorf("rate limit exceeded (429)")

		case 400, 403, 404:
			// Permanent errors - do NOT retry
			errMsg := s.parseSalesforceError(body)
			return backofflib.Permanent(fmt.Errorf("permanent error (%d): %s", resp.StatusCode, errMsg))

		case 401:
			// Unauthorized - retry once (oauth2 client may auto-refresh)
			errMsg := s.parseSalesforceError(body)
			return fmt.Errorf("unauthorized (401): %s", errMsg)

		case 500, 502, 503, 504:
			// Server errors - retry with backoff
			errMsg := s.parseSalesforceError(body)
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, errMsg)

		default:
			errMsg := s.parseSalesforceError(body)
			return backofflib.Permanent(fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errMsg))
		}
	}

	if err := backofflib.Retry(operation, retryBackoff); err != nil {
		return 0, err
	}

	return deliveredCount, nil
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
