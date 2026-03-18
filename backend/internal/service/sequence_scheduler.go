package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// SequenceScheduler polls tenant databases every 60 seconds for due step executions,
// enforces business hours, and dispatches steps appropriately.
//
// Email steps are auto-dispatched via GmailProvider.
// Manual steps (call, linkedin, custom) remain with status=scheduled for the task queue.
type SequenceScheduler struct {
	scheduler      gocron.Scheduler
	repo           *repo.SequenceRepo
	svc            *SequenceService
	gmailProvider  *GmailProvider
	gmailOAuth     *GmailOAuthService
	templateEngine *TemplateEngine
	engagementRepo *repo.EngagementRepo
	contactRepo    *repo.ContactRepo

	// orgDBs maps orgID -> *sql.DB for tenant DBs registered at startup or via RegisterOrgDB.
	orgDBs map[string]*sql.DB
	orgMu  sync.RWMutex

	// activeJobs tracks the gocron job per org so we don't double-register.
	activeJobs map[string]gocron.Job
	jobMu      sync.Mutex
}

// NewSequenceScheduler creates a SequenceScheduler but does not start polling.
// Call Start to begin.
func NewSequenceScheduler(
	r *repo.SequenceRepo,
	svc *SequenceService,
	gmailProvider *GmailProvider,
	gmailOAuth *GmailOAuthService,
	templateEngine *TemplateEngine,
	engagementRepo *repo.EngagementRepo,
	contactRepo *repo.ContactRepo,
) (*SequenceScheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("sequence scheduler: failed to create gocron scheduler: %w", err)
	}
	return &SequenceScheduler{
		scheduler:      s,
		repo:           r,
		svc:            svc,
		gmailProvider:  gmailProvider,
		gmailOAuth:     gmailOAuth,
		templateEngine: templateEngine,
		engagementRepo: engagementRepo,
		contactRepo:    contactRepo,
		orgDBs:         make(map[string]*sql.DB),
		activeJobs:     make(map[string]gocron.Job),
	}, nil
}

// RegisterOrgDB registers a tenant DB for an org, and ensures a polling job exists.
// Safe to call multiple times for the same org (idempotent).
func (s *SequenceScheduler) RegisterOrgDB(orgID string, tenantDB *sql.DB) {
	s.orgMu.Lock()
	s.orgDBs[orgID] = tenantDB
	s.orgMu.Unlock()

	s.ensurePollingJob(orgID)
}

// Start registers a polling job for every org that currently has a tenant DB stored,
// then starts the gocron scheduler.
func (s *SequenceScheduler) Start(_ context.Context) error {
	s.orgMu.RLock()
	orgIDs := make([]string, 0, len(s.orgDBs))
	for id := range s.orgDBs {
		orgIDs = append(orgIDs, id)
	}
	s.orgMu.RUnlock()

	for _, orgID := range orgIDs {
		s.ensurePollingJob(orgID)
	}

	s.scheduler.Start()
	log.Printf("[SequenceScheduler] started with %d org(s) registered", len(orgIDs))
	return nil
}

// Shutdown stops the gocron scheduler and waits for running jobs to finish.
func (s *SequenceScheduler) Shutdown() error {
	log.Println("[SequenceScheduler] shutting down")
	return s.scheduler.Shutdown()
}

// ensurePollingJob creates a gocron job for orgID if one doesn't exist yet.
func (s *SequenceScheduler) ensurePollingJob(orgID string) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	if _, exists := s.activeJobs[orgID]; exists {
		return
	}

	taskFunc := func() {
		s.pollDueExecutions(orgID)
	}

	job, err := s.scheduler.NewJob(
		gocron.DurationJob(60*time.Second),
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName("seq-poll-"+orgID),
	)
	if err != nil {
		log.Printf("[SequenceScheduler] failed to register job for org %s: %v", orgID, err)
		return
	}

	s.activeJobs[orgID] = job
	log.Printf("[SequenceScheduler] registered polling job for org %s", orgID)
}

// pollDueExecutions queries for and processes all due step executions for a single org.
func (s *SequenceScheduler) pollDueExecutions(orgID string) {
	ctx := context.Background()

	s.orgMu.RLock()
	tenantDB, ok := s.orgDBs[orgID]
	s.orgMu.RUnlock()
	if !ok || tenantDB == nil {
		log.Printf("[SequenceScheduler] no tenant DB for org %s", orgID)
		return
	}

	tenantRepo := s.repo.WithDB(tenantDB)
	due, err := tenantRepo.GetDueExecutions(ctx, orgID, time.Now())
	if err != nil {
		log.Printf("[SequenceScheduler] GetDueExecutions failed for org %s: %v", orgID, err)
		return
	}

	for _, exec := range due {
		s.processExecution(ctx, orgID, exec, tenantDB, tenantRepo)
	}
}

// processExecution handles a single due step execution.
func (s *SequenceScheduler) processExecution(
	ctx context.Context,
	orgID string,
	exec *entity.StepExecution,
	tenantDB *sql.DB,
	tenantRepo *repo.SequenceRepo,
) {
	// Load enrollment
	enrollment, err := tenantRepo.GetEnrollment(ctx, exec.EnrollmentID)
	if err != nil || enrollment == nil {
		log.Printf("[SequenceScheduler] enrollment %s not found for exec %s: %v", exec.EnrollmentID, exec.ID, err)
		return
	}

	// Load sequence
	seq, err := tenantRepo.GetSequence(ctx, orgID, enrollment.SequenceID)
	if err != nil || seq == nil {
		log.Printf("[SequenceScheduler] sequence %s not found for exec %s: %v", enrollment.SequenceID, exec.ID, err)
		return
	}

	// Load step
	steps, err := tenantRepo.ListStepsBySequence(ctx, enrollment.SequenceID)
	if err != nil {
		log.Printf("[SequenceScheduler] ListStepsBySequence failed for seq %s: %v", enrollment.SequenceID, err)
		return
	}

	var currentStep *entity.SequenceStep
	for _, st := range steps {
		if st.ID == exec.StepID {
			currentStep = st
			break
		}
	}
	if currentStep == nil {
		log.Printf("[SequenceScheduler] step %s not found for exec %s", exec.StepID, exec.ID)
		return
	}

	// Business hours check
	if !isWithinBusinessHours(seq) {
		next := nextBusinessHoursStart(seq, time.Now())
		exec.ScheduledAt = &next
		exec.Status = entity.ExecutionStatusScheduled
		if err := tenantRepo.UpdateStepExecution(ctx, exec); err != nil {
			log.Printf("[SequenceScheduler] reschedule exec %s failed: %v", exec.ID, err)
		}
		return
	}

	// Suppression re-check at execution time
	var suppressionRules []SuppressionRule
	if currentStep.ConfigJSON != nil {
		suppressionRules = parseSuppressionRules(*currentStep.ConfigJSON)
	}

	suppResult, err := s.svc.CheckSuppression(ctx, orgID, enrollment.ContactID, suppressionRules)
	if err != nil {
		log.Printf("[SequenceScheduler] suppression check failed for exec %s: %v", exec.ID, err)
		return
	}
	if suppResult.Suppressed {
		// Opt out the enrollment
		if transErr := s.svc.TransitionEnrollment(enrollment, entity.EnrollmentStatusOptedOut); transErr == nil {
			_ = tenantRepo.UpdateEnrollmentStatus(ctx, enrollment.ID, entity.EnrollmentStatusOptedOut)
		}
		exec.Status = entity.ExecutionStatusSkipped
		_ = tenantRepo.UpdateStepExecution(ctx, exec)
		log.Printf("[SequenceScheduler] contact %s suppressed in exec %s: %s", enrollment.ContactID, exec.ID, suppResult.Reason)
		return
	}

	// Atomic claim
	claimed, err := tenantRepo.ClaimStepExecution(ctx, exec.ID)
	if err != nil {
		log.Printf("[SequenceScheduler] ClaimStepExecution failed for exec %s: %v", exec.ID, err)
		return
	}
	if !claimed {
		// Another scheduler cycle already claimed it
		return
	}
	// Update our local copy to reflect the claim
	exec.Status = entity.ExecutionStatusExecuting

	// Dispatch based on step type
	switch currentStep.StepType {
	case entity.StepTypeEmail:
		if err := s.dispatchEmailStep(ctx, orgID, exec, currentStep, enrollment, tenantDB, tenantRepo, steps); err != nil {
			log.Printf("[SequenceScheduler] dispatchEmailStep failed for exec %s: %v", exec.ID, err)
			exec.Status = entity.ExecutionStatusFailed
			errMsg := err.Error()
			exec.ErrorMessage = &errMsg
			_ = tenantRepo.UpdateStepExecution(ctx, exec)
		}

	case entity.StepTypeCall, entity.StepTypeLinkedIn, entity.StepTypeCustom, entity.StepTypeSMS:
		// Manual steps stay with status=scheduled so the task queue surfaces them.
		// Exception: if continue_without_completing is set and the step is stale > 24h,
		// auto-skip and schedule the next step.
		if shouldAutoSkipManualStep(currentStep, exec) {
			exec.Status = entity.ExecutionStatusSkipped
			now := time.Now()
			exec.ExecutedAt = &now
			_ = tenantRepo.UpdateStepExecution(ctx, exec)
			if err := s.scheduleNextStep(ctx, enrollment, currentStep, tenantRepo, steps, now); err != nil {
				log.Printf("[SequenceScheduler] scheduleNextStep after auto-skip failed for exec %s: %v", exec.ID, err)
			}
		} else {
			// Revert claim — leave as scheduled so task queue picks it up
			exec.Status = entity.ExecutionStatusScheduled
			_ = tenantRepo.UpdateStepExecution(ctx, exec)
		}

	default:
		log.Printf("[SequenceScheduler] unknown step type %q for exec %s", currentStep.StepType, exec.ID)
		exec.Status = entity.ExecutionStatusFailed
		errMsg := fmt.Sprintf("unknown step type: %s", currentStep.StepType)
		exec.ErrorMessage = &errMsg
		_ = tenantRepo.UpdateStepExecution(ctx, exec)
	}
}

// dispatchEmailStep fetches the template, renders it, sends via Gmail, marks completed,
// and schedules the next step.
func (s *SequenceScheduler) dispatchEmailStep(
	ctx context.Context,
	orgID string,
	exec *entity.StepExecution,
	step *entity.SequenceStep,
	enrollment *entity.SequenceEnrollment,
	tenantDB *sql.DB,
	tenantRepo *repo.SequenceRepo,
	allSteps []*entity.SequenceStep,
) error {
	// 1. Fetch template
	if step.TemplateID == nil {
		return fmt.Errorf("email step %s has no template_id", step.ID)
	}
	tmpl, err := s.engagementRepo.WithDB(tenantDB).GetEmailTemplate(ctx, orgID, *step.TemplateID)
	if err != nil {
		return fmt.Errorf("fetch template %s: %w", *step.TemplateID, err)
	}
	if tmpl == nil {
		return fmt.Errorf("template %s not found", *step.TemplateID)
	}

	// 2. Fetch contact
	contact, err := s.contactRepo.WithDB(tenantDB).GetByID(ctx, orgID, enrollment.ContactID)
	if err != nil || contact == nil {
		return fmt.Errorf("fetch contact %s: %w", enrollment.ContactID, err)
	}

	// 3. Build template vars
	contactMap := map[string]interface{}{
		"first_name":   contact.FirstName,
		"last_name":    contact.LastName,
		"email":        contact.EmailAddress,
		"phone":        contact.PhoneNumber,
		"account_name": contact.AccountName,
		"city":         contact.AddressCity,
		"state":        contact.AddressState,
		"country":      contact.AddressCountry,
	}
	vars := s.templateEngine.ContactToTemplateVars(contactMap)

	// 4. Render
	subject, bodyHTML := s.templateEngine.RenderTemplate(tmpl, vars)

	// 5. Get sender info (gmail address for the enrolledBy user)
	_, oauthToken, err := s.gmailOAuth.GetHTTPClient(ctx, orgID, enrollment.EnrolledBy)
	if err != nil {
		return fmt.Errorf("get gmail client for user %s: %w", enrollment.EnrolledBy, err)
	}
	fromEmail := oauthToken.GmailAddress
	toEmail := contact.EmailAddress

	// 6. Send
	if err := s.gmailProvider.Send(ctx, orgID, enrollment.EnrolledBy, fromEmail, toEmail, subject, bodyHTML); err != nil {
		return fmt.Errorf("gmail send: %w", err)
	}

	// 7. Mark execution completed
	now := time.Now()
	exec.Status = entity.ExecutionStatusCompleted
	exec.ExecutedAt = &now
	if err := tenantRepo.UpdateStepExecution(ctx, exec); err != nil {
		log.Printf("[SequenceScheduler] UpdateStepExecution completed failed for exec %s: %v", exec.ID, err)
	}

	// 8. Schedule next step (or finish enrollment if last step)
	if err := s.scheduleNextStep(ctx, enrollment, step, tenantRepo, allSteps, now); err != nil {
		log.Printf("[SequenceScheduler] scheduleNextStep failed for exec %s: %v", exec.ID, err)
	}

	return nil
}

// scheduleNextStep finds the next step after completedStep and inserts a new StepExecution,
// or transitions the enrollment to finished if there are no more steps.
func (s *SequenceScheduler) scheduleNextStep(
	ctx context.Context,
	enrollment *entity.SequenceEnrollment,
	completedStep *entity.SequenceStep,
	tenantRepo *repo.SequenceRepo,
	allSteps []*entity.SequenceStep,
	completedAt time.Time,
) error {
	// Find the next step by step_number
	var nextStep *entity.SequenceStep
	for _, st := range allSteps {
		if st.StepNumber > completedStep.StepNumber {
			if nextStep == nil || st.StepNumber < nextStep.StepNumber {
				nextStep = st
			}
		}
	}

	if nextStep == nil {
		// No more steps — finish the enrollment
		seq, err := tenantRepo.GetSequence(ctx, enrollment.OrgID, enrollment.SequenceID)
		if err != nil {
			return fmt.Errorf("GetSequence for finish: %w", err)
		}
		_ = seq // only needed for business hours if we were computing a future date

		if transErr := s.svc.TransitionEnrollment(enrollment, entity.EnrollmentStatusFinished); transErr == nil {
			_ = tenantRepo.UpdateEnrollmentStatus(ctx, enrollment.ID, entity.EnrollmentStatusFinished)
		}
		return nil
	}

	// Compute scheduled_at
	delay := time.Duration(nextStep.DelayDays)*24*time.Hour + time.Duration(nextStep.DelayHours)*time.Hour
	scheduledAt := completedAt.Add(delay)

	// If scheduled time is outside business hours, push to next window start.
	// We need the sequence for timezone info.
	seq, err := tenantRepo.GetSequence(ctx, enrollment.OrgID, enrollment.SequenceID)
	if err == nil && seq != nil {
		if !isWithinBusinessHoursAt(seq, scheduledAt) {
			scheduledAt = nextBusinessHoursStartAt(seq, scheduledAt)
		}
	}

	execID := uuid.New().String()
	nextExec := &entity.StepExecution{
		ID:           execID,
		EnrollmentID: enrollment.ID,
		StepID:       nextStep.ID,
		OrgID:        enrollment.OrgID,
		Status:       entity.ExecutionStatusScheduled,
		ScheduledAt:  &scheduledAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return tenantRepo.CreateStepExecution(ctx, nextExec)
}

// ========================================================================
// Business Hours Logic
// ========================================================================

// isWithinBusinessHours checks whether the current wall-clock time in the
// sequence's timezone falls within the configured business hours window
// (weekdays only — Saturday and Sunday always return false).
func isWithinBusinessHours(seq *entity.Sequence) bool {
	return isWithinBusinessHoursAt(seq, time.Now())
}

// isWithinBusinessHoursAt is the testable form: it accepts an explicit `now` instead of
// calling time.Now(), so tests can inject arbitrary times.
func isWithinBusinessHoursAt(seq *entity.Sequence, now time.Time) bool {
	loc, err := time.LoadLocation(seq.Timezone)
	if err != nil {
		log.Printf("[SequenceScheduler] invalid timezone %q for seq %s, falling back to UTC: %v", seq.Timezone, seq.ID, err)
		loc = time.UTC
	}

	local := now.In(loc)

	// Skip weekends
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return false
	}

	startMins := parseHHMM(seq.BusinessHoursStart, 9*60)  // default 09:00
	endMins := parseHHMM(seq.BusinessHoursEnd, 17*60)     // default 17:00
	nowMins := local.Hour()*60 + local.Minute()

	return nowMins >= startMins && nowMins < endMins
}

// nextBusinessHoursStart computes the next business hours window start after `from`,
// in the sequence's configured timezone.
func nextBusinessHoursStart(seq *entity.Sequence, from time.Time) time.Time {
	return nextBusinessHoursStartAt(seq, from)
}

// nextBusinessHoursStartAt is the testable form.
func nextBusinessHoursStartAt(seq *entity.Sequence, from time.Time) time.Time {
	loc, err := time.LoadLocation(seq.Timezone)
	if err != nil {
		loc = time.UTC
	}

	startMins := parseHHMM(seq.BusinessHoursStart, 9*60)
	endMins := parseHHMM(seq.BusinessHoursEnd, 17*60)

	startH := startMins / 60
	startM := startMins % 60

	local := from.In(loc)

	// If before business hours start today (weekday), return today's start
	nowMins := local.Hour()*60 + local.Minute()
	if local.Weekday() != time.Saturday && local.Weekday() != time.Sunday && nowMins < startMins {
		return time.Date(local.Year(), local.Month(), local.Day(), startH, startM, 0, 0, loc)
	}

	// Otherwise advance to next day (skip weekends)
	candidate := local
	// If currently within or past business hours end, or on weekend — advance day
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday || nowMins >= endMins {
		candidate = candidate.AddDate(0, 0, 1)
	}

	// Skip Saturday and Sunday
	for candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
		candidate = candidate.AddDate(0, 0, 1)
	}

	return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), startH, startM, 0, 0, loc)
}

// ========================================================================
// Helpers
// ========================================================================

// parseHHMM parses an "HH:MM" string into minutes-since-midnight.
// ptr may be nil; defaultMins is returned on nil or parse error.
func parseHHMM(ptr *string, defaultMins int) int {
	if ptr == nil {
		return defaultMins
	}
	parts := strings.SplitN(*ptr, ":", 2)
	if len(parts) != 2 {
		return defaultMins
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return defaultMins
	}
	return h*60 + m
}

// parseSuppressionRules attempts to decode suppression rules from a step or sequence config_json.
// Returns an empty slice on any error.
func parseSuppressionRules(configJSON string) []SuppressionRule {
	var cfg struct {
		SuppressionRules []SuppressionRule `json:"suppression_rules"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil
	}
	return cfg.SuppressionRules
}

// shouldAutoSkipManualStep returns true when the step has continue_without_completing=true
// in its config_json AND the step's scheduled_at is more than 24 hours in the past.
func shouldAutoSkipManualStep(step *entity.SequenceStep, exec *entity.StepExecution) bool {
	if step.ConfigJSON == nil {
		return false
	}
	var cfg struct {
		ContinueWithoutCompleting bool `json:"continue_without_completing"`
	}
	if err := json.Unmarshal([]byte(*step.ConfigJSON), &cfg); err != nil || !cfg.ContinueWithoutCompleting {
		return false
	}
	if exec.ScheduledAt == nil {
		return false
	}
	return time.Since(*exec.ScheduledAt) > 24*time.Hour
}
