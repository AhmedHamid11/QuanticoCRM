package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// SFDCActivityWritebackService batches completed sequence step executions into
// Salesforce Task records via Bulk API 2.0. It is designed as a single global
// service (NOT per-org) — RunHourlyBatch iterates all registered org DBs.
//
// Pattern: Same as SequenceScheduler's orgDBs map, but inverted: one scheduler
// iterates all orgs rather than one scheduler per org (per RESEARCH.md Pitfall 5).
type SFDCActivityWritebackService struct {
	sequenceRepo     *repo.SequenceRepo
	sfdcOAuthService *SalesforceOAuthService
	rateLimitService *RateLimitService

	orgDBs map[string]*sql.DB
	orgMu  sync.RWMutex
}

// NewSFDCActivityWritebackService creates a new SFDCActivityWritebackService.
func NewSFDCActivityWritebackService(
	seqRepo *repo.SequenceRepo,
	sfdcOAuth *SalesforceOAuthService,
	rateLimit *RateLimitService,
) *SFDCActivityWritebackService {
	return &SFDCActivityWritebackService{
		sequenceRepo:     seqRepo,
		sfdcOAuthService: sfdcOAuth,
		rateLimitService: rateLimit,
		orgDBs:           make(map[string]*sql.DB),
	}
}

// RegisterOrgDB registers a tenant DB for an org. Safe to call multiple times (idempotent).
func (s *SFDCActivityWritebackService) RegisterOrgDB(orgID string, tenantDB *sql.DB) {
	s.orgMu.Lock()
	s.orgDBs[orgID] = tenantDB
	s.orgMu.Unlock()
}

// QueueWriteback creates a pending writeback row for a completed step execution.
// It looks up the contact's SFDC ID from the field mappings in the tenant DB.
// If no SFDC ID is found, the writeback is inserted with status=failed.
// This call is non-blocking: it inserts synchronously but the actual SFDC API
// call happens in the next RunHourlyBatch run.
func (s *SFDCActivityWritebackService) QueueWriteback(
	ctx context.Context,
	tenantDB db.DBConn,
	orgID string,
	exec *entity.StepExecution,
	step *entity.SequenceStep,
	enrollment *entity.SequenceEnrollment,
) {
	tenantRepo := s.sequenceRepo.WithDB(tenantDB)

	// Look up the contact's SFDC contact ID from salesforce_field_mappings
	sfdcContactID := s.lookupSFDCContactID(ctx, tenantDB, orgID, enrollment.ContactID)

	wbID := uuid.New().String()
	wb := &entity.SFDCActivityWriteback{
		ID:              wbID,
		OrgID:           orgID,
		StepExecutionID: exec.ID,
		EnrollmentID:    enrollment.ID,
		ContactID:       enrollment.ContactID,
	}

	if sfdcContactID == "" {
		errMsg := "no sfdc_id mapped"
		wb.Status = entity.WritebackStatusFailed
		wb.ErrorMessage = &errMsg
	} else {
		wb.Status = entity.WritebackStatusPending
		wb.SFDCContactID = &sfdcContactID
	}

	if err := tenantRepo.CreateWriteback(ctx, wb); err != nil {
		log.Printf("[ActivityWriteback] CreateWriteback failed for exec %s: %v", exec.ID, err)
	}
}

// lookupSFDCContactID queries the contact record to find the field mapped to
// the Salesforce "Id" field for Contacts/Leads. Returns empty string if not found.
func (s *SFDCActivityWritebackService) lookupSFDCContactID(
	ctx context.Context,
	tenantDB db.DBConn,
	orgID, contactID string,
) string {
	// Step 1: Find which Quantico field maps to the SFDC "Id" for Contact entity
	// salesforce_field_mappings table: (org_id, sfdc_object, sfdc_field, quantico_field)
	var quanticoField string
	mappingQuery := `
		SELECT quantico_field
		FROM salesforce_field_mappings
		WHERE org_id = ? AND sfdc_object IN ('Contact', 'Lead') AND sfdc_field = 'Id'
		LIMIT 1
	`
	row := tenantDB.QueryRowContext(ctx, mappingQuery, orgID)
	if err := row.Scan(&quanticoField); err != nil {
		// No mapping found — SFDC not configured or no ID field mapped
		return ""
	}

	if quanticoField == "" {
		return ""
	}

	// Step 2: Read the mapped field value from the contact record
	// We use a dynamic column query — safe because quanticoField comes from our own DB
	// and cannot be user-supplied.
	contactQuery := fmt.Sprintf("SELECT %s FROM contacts WHERE id = ? AND org_id = ?", quanticoField)
	contactRow := tenantDB.QueryRowContext(ctx, contactQuery, contactID, orgID)

	var sfdcID sql.NullString
	if err := contactRow.Scan(&sfdcID); err != nil {
		return ""
	}
	if !sfdcID.Valid || sfdcID.String == "" {
		return ""
	}
	return sfdcID.String
}

// RunHourlyBatch processes pending writebacks for all registered orgs.
// It is designed to be called once per hour via gocron.
// For each org, it:
//  1. Lists pending writebacks (up to 200 per batch)
//  2. Checks the rate limit budget
//  3. Submits a Bulk API 2.0 insert job for Task records
//  4. Updates writeback statuses based on the job result
func (s *SFDCActivityWritebackService) RunHourlyBatch(ctx context.Context) {
	s.orgMu.RLock()
	orgIDs := make([]string, 0, len(s.orgDBs))
	for id := range s.orgDBs {
		orgIDs = append(orgIDs, id)
	}
	s.orgMu.RUnlock()

	for _, orgID := range orgIDs {
		s.processOrgBatch(ctx, orgID)
	}
}

// processOrgBatch handles the writeback batch for a single org.
func (s *SFDCActivityWritebackService) processOrgBatch(ctx context.Context, orgID string) {
	s.orgMu.RLock()
	tenantDB, ok := s.orgDBs[orgID]
	s.orgMu.RUnlock()
	if !ok || tenantDB == nil {
		return
	}

	tenantRepo := s.sequenceRepo.WithDB(tenantDB)

	// 1. List pending writebacks
	writebacks, err := tenantRepo.ListPendingWritebacks(ctx, orgID, 200)
	if err != nil {
		log.Printf("[ActivityWriteback] ListPendingWritebacks failed for org %s: %v", orgID, err)
		return
	}
	if len(writebacks) == 0 {
		return
	}

	// 2. Check rate limit (4 API calls per batch: create + upload + close + poll)
	canCall, err := s.rateLimitService.CanMakeAPICalls(ctx, orgID, 4)
	if err != nil {
		log.Printf("[ActivityWriteback] CanMakeAPICalls check failed for org %s: %v", orgID, err)
		return
	}
	if !canCall {
		log.Printf("[ActivityWriteback] Rate limit budget exhausted for org %s — deferring batch", orgID)
		return
	}

	// 3. Get SFDC HTTP client
	httpClient, err := s.sfdcOAuthService.GetHTTPClient(ctx, orgID)
	if err != nil {
		log.Printf("[ActivityWriteback] GetHTTPClient failed for org %s: %v — marking as failed", orgID, err)
		s.markAllFailed(ctx, tenantRepo, writebacks, "no sfdc connection")
		return
	}

	// 4. Get the Salesforce instance URL for this org
	instanceURL, err := s.getSFDCInstanceURL(ctx, orgID)
	if err != nil {
		log.Printf("[ActivityWriteback] getSFDCInstanceURL failed for org %s: %v", orgID, err)
		s.markAllFailed(ctx, tenantRepo, writebacks, "no sfdc connection")
		return
	}

	// We need step and sequence data to build the CSV Subject/Description fields.
	// Collect unique step IDs and sequence IDs from writebacks.
	stepMap, seqMap := s.buildStepAndSeqMaps(ctx, tenantRepo, orgID, writebacks)

	// 5. Build CSV payload
	csvData := buildActivityCSV(writebacks, stepMap, seqMap)

	// 6. Execute Bulk API 2.0 flow
	jobID, jobErr := s.submitBulkAPIJob(ctx, httpClient, instanceURL, csvData)
	if jobErr != nil {
		log.Printf("[ActivityWriteback] Bulk API job failed for org %s: %v", orgID, jobErr)
		s.markAllFailed(ctx, tenantRepo, writebacks, fmt.Sprintf("bulk api error: %v", jobErr))
		return
	}

	// 7. Update all writebacks to completed with the job ID
	jobIDPtr := &jobID
	for _, wb := range writebacks {
		if updateErr := tenantRepo.UpdateWritebackStatus(ctx, wb.ID, entity.WritebackStatusCompleted, jobIDPtr, nil, nil); updateErr != nil {
			log.Printf("[ActivityWriteback] UpdateWritebackStatus failed for wb %s: %v", wb.ID, updateErr)
		}
	}

	// 8. Record API usage
	if usageErr := s.rateLimitService.RecordAPIUsage(ctx, orgID, jobID, 4); usageErr != nil {
		log.Printf("[ActivityWriteback] RecordAPIUsage warning for org %s: %v", orgID, usageErr)
	}

	log.Printf("[ActivityWriteback] Batch complete for org %s: %d writebacks submitted (job=%s)", orgID, len(writebacks), jobID)
}

// submitBulkAPIJob orchestrates the Salesforce Bulk API 2.0 flow:
//   - POST to create the ingest job
//   - PUT to upload the CSV
//   - PATCH to mark upload complete
//   - GET to poll until job completes or fails
//
// Returns the job ID on success.
func (s *SFDCActivityWritebackService) submitBulkAPIJob(
	ctx context.Context,
	httpClient *http.Client,
	instanceURL string,
	csvData string,
) (string, error) {
	apiVersion := os.Getenv("SALESFORCE_API_VERSION")
	if apiVersion == "" {
		apiVersion = "v60.0"
	}
	baseURL := fmt.Sprintf("%s/services/data/%s/jobs/ingest", instanceURL, apiVersion)

	// Step 1: Create job
	jobID, err := s.createBulkJob(ctx, httpClient, baseURL)
	if err != nil {
		return "", fmt.Errorf("create job: %w", err)
	}

	// Step 2: Upload CSV with exponential backoff
	uploadURL := fmt.Sprintf("%s/%s/batches", baseURL, jobID)
	if err := backoffDo(ctx, func() error {
		return s.uploadCSV(ctx, httpClient, uploadURL, csvData)
	}); err != nil {
		return jobID, fmt.Errorf("upload CSV: %w", err)
	}

	// Step 3: Mark upload complete
	closeURL := fmt.Sprintf("%s/%s", baseURL, jobID)
	if err := backoffDo(ctx, func() error {
		return s.closeJob(ctx, httpClient, closeURL)
	}); err != nil {
		return jobID, fmt.Errorf("close job: %w", err)
	}

	// Step 4: Poll until complete
	if err := s.pollJobUntilDone(ctx, httpClient, closeURL); err != nil {
		return jobID, fmt.Errorf("poll job: %w", err)
	}

	return jobID, nil
}

// createBulkJob creates a Bulk API 2.0 ingest job for Task object (insert operation).
func (s *SFDCActivityWritebackService) createBulkJob(
	ctx context.Context,
	httpClient *http.Client,
	baseURL string,
) (string, error) {
	body := map[string]interface{}{
		"object":      "Task",
		"operation":   "insert",
		"contentType": "CSV",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("create job HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse create job response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("create job: empty job ID in response")
	}
	return result.ID, nil
}

// uploadCSV uploads the CSV data to the Bulk API 2.0 job batches endpoint.
func (s *SFDCActivityWritebackService) uploadCSV(
	ctx context.Context,
	httpClient *http.Client,
	uploadURL string,
	csvData string,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, strings.NewReader(csvData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/csv")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload CSV HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// closeJob marks the Bulk API 2.0 job as UploadComplete.
func (s *SFDCActivityWritebackService) closeJob(
	ctx context.Context,
	httpClient *http.Client,
	jobURL string,
) error {
	body := map[string]string{"state": "UploadComplete"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, jobURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("close job HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// pollJobUntilDone polls the job status endpoint until the job is done or fails.
// Max 10 attempts with 30s interval.
func (s *SFDCActivityWritebackService) pollJobUntilDone(
	ctx context.Context,
	httpClient *http.Client,
	jobURL string,
) error {
	maxAttempts := 10
	interval := 30 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("poll job HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			State        string `json:"state"`
			ErrorMessage string `json:"errorMessage"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("parse poll response: %w", err)
		}

		switch result.State {
		case "JobComplete":
			return nil
		case "Failed", "Aborted":
			return fmt.Errorf("job %s with error: %s", result.State, result.ErrorMessage)
		}

		// Still in progress — wait before next poll
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}
		}
	}
	return fmt.Errorf("job did not complete after %d poll attempts", maxAttempts)
}

// getSFDCInstanceURL retrieves the Salesforce instance URL for an org.
func (s *SFDCActivityWritebackService) getSFDCInstanceURL(ctx context.Context, orgID string) (string, error) {
	conn, err := s.sfdcOAuthService.GetConfig(ctx, orgID)
	if err != nil {
		return "", err
	}
	if conn == nil {
		return "", fmt.Errorf("no salesforce connection for org %s", orgID)
	}
	if conn.InstanceURL == "" {
		return "", fmt.Errorf("empty instance URL for org %s", orgID)
	}
	return conn.InstanceURL, nil
}

// buildStepAndSeqMaps loads step and sequence data needed for CSV Subject/Description.
func (s *SFDCActivityWritebackService) buildStepAndSeqMaps(
	ctx context.Context,
	tenantRepo *repo.SequenceRepo,
	orgID string,
	writebacks []entity.SFDCActivityWriteback,
) (stepMap map[string]*entity.SequenceStep, seqMap map[string]*entity.Sequence) {
	stepMap = make(map[string]*entity.SequenceStep)
	seqMap = make(map[string]*entity.Sequence)

	// Collect unique enrollment IDs to get sequence IDs
	enrollmentIDs := make(map[string]bool)
	for _, wb := range writebacks {
		enrollmentIDs[wb.EnrollmentID] = true
	}

	// Look up enrollments to get sequence IDs
	seqIDs := make(map[string]bool)
	for enrollmentID := range enrollmentIDs {
		enrollment, err := tenantRepo.GetEnrollment(ctx, enrollmentID)
		if err != nil || enrollment == nil {
			continue
		}
		seqIDs[enrollment.SequenceID] = true

		// Load steps for this sequence if not already loaded
		if _, loaded := stepMap[enrollment.SequenceID]; !loaded {
			steps, err := tenantRepo.ListStepsBySequence(ctx, enrollment.SequenceID)
			if err == nil {
				for _, st := range steps {
					stepMap[st.ID] = st
				}
			}
		}
	}

	// Load sequences
	for seqID := range seqIDs {
		seq, err := tenantRepo.GetSequence(ctx, orgID, seqID)
		if err == nil && seq != nil {
			seqMap[seqID] = seq
		}
	}

	return stepMap, seqMap
}

// markAllFailed marks all writebacks in the slice as failed with the given error message.
func (s *SFDCActivityWritebackService) markAllFailed(
	ctx context.Context,
	tenantRepo *repo.SequenceRepo,
	writebacks []entity.SFDCActivityWriteback,
	errMsg string,
) {
	for _, wb := range writebacks {
		if err := tenantRepo.UpdateWritebackStatus(ctx, wb.ID, entity.WritebackStatusFailed, nil, nil, &errMsg); err != nil {
			log.Printf("[ActivityWriteback] markAllFailed for wb %s: %v", wb.ID, err)
		}
	}
}

// buildActivityCSV generates a Salesforce Bulk API 2.0 CSV payload for Task records.
// Columns: Subject, Description, WhoId, ActivityDate, Status, Origin
//
// The stepMap and seqMap are keyed by step ID and sequence ID respectively.
// If a step or sequence is not found, the Subject falls back to generic values.
func buildActivityCSV(
	writebacks []entity.SFDCActivityWriteback,
	stepMap map[string]*entity.SequenceStep,
	seqMap map[string]*entity.Sequence,
) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header
	_ = w.Write([]string{"Subject", "Description", "WhoId", "ActivityDate", "Status", "Origin"})

	for _, wb := range writebacks {
		whoID := ""
		if wb.SFDCContactID != nil {
			whoID = *wb.SFDCContactID
		}

		// Build Subject from step type and sequence name
		subject := "Task: Outreach"
		var stepType, seqName string
		if step, ok := stepMap[wb.StepExecutionID]; ok {
			stepType = step.StepType
			if seq, seqOK := seqMap[step.SequenceID]; seqOK {
				seqName = seq.Name
			}
		}
		if stepType != "" && seqName != "" {
			subject = fmt.Sprintf("%s: %s", strings.Title(stepType), seqName)
		} else if stepType != "" {
			subject = fmt.Sprintf("%s: Outreach", strings.Title(stepType))
		}

		// Build Description
		executedAt := wb.CreatedAt.Format(time.RFC3339)
		description := fmt.Sprintf("Step executed at %s for contact %s", executedAt, wb.ContactID)

		// ActivityDate: date portion of execution
		activityDate := wb.CreatedAt.Format("2006-01-02")

		_ = w.Write([]string{
			subject,
			description,
			whoID,
			activityDate,
			"Completed",
			"Quantico",
		})
	}

	w.Flush()
	return buf.String()
}

// backoffDo executes fn with exponential backoff (4 attempts, 1s initial, 2x multiplier).
func backoffDo(ctx context.Context, fn func() error) error {
	bo := backoff.WithContext(
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
		ctx,
	)
	return backoff.Retry(fn, bo)
}
