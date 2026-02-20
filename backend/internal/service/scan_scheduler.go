package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/go-co-op/gocron/v2"
)

// ScanScheduler manages scheduled background scans using gocron v2
type ScanScheduler struct {
	scheduler   gocron.Scheduler
	scanService *ScanJobService
	scanJobRepo *repo.ScanJobRepo

	// Track gocron job UUIDs for removal when schedules change
	activeJobs map[string]gocron.Job // key: "{orgID}:{entityType}" -> gocron.Job
	mu         sync.Mutex

	// Org database resolver (need to get tenant DB for each org)
	// Stores orgID -> (dbURL, authToken) for scheduled execution
	orgDBInfo map[string]*orgDBCredentials
	orgDBMu   sync.RWMutex
}

type orgDBCredentials struct {
	dbURL      string
	authToken  string
	tenantDB   *sql.DB // Cached connection for scheduled scans
}

// NewScanScheduler creates a new scan scheduler
func NewScanScheduler(scanService *ScanJobService, scanJobRepo *repo.ScanJobRepo) (*ScanScheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create gocron scheduler: %w", err)
	}

	return &ScanScheduler{
		scheduler:   s,
		scanService: scanService,
		scanJobRepo: scanJobRepo,
		activeJobs:  make(map[string]gocron.Job),
		orgDBInfo:   make(map[string]*orgDBCredentials),
	}, nil
}

// RegisterOrgDB stores database credentials for an org (called by handler/service on schedule creation)
func (s *ScanScheduler) RegisterOrgDB(orgID string, dbURL, authToken string, tenantDB *sql.DB) {
	s.orgDBMu.Lock()
	defer s.orgDBMu.Unlock()
	s.orgDBInfo[orgID] = &orgDBCredentials{
		dbURL:     dbURL,
		authToken: authToken,
		tenantDB:  tenantDB,
	}
}

// Start loads all enabled schedules and starts the gocron scheduler
func (s *ScanScheduler) Start(ctx context.Context) error {
	// Load all enabled schedules from master DB
	schedules, err := s.scanJobRepo.ListAllEnabledSchedules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	// Register each schedule
	for _, schedule := range schedules {
		if err := s.registerSchedule(schedule); err != nil {
			log.Printf("Failed to register schedule %s for org %s entity %s: %v",
				schedule.ID, schedule.OrgID, schedule.EntityType, err)
			continue
		}
	}

	// Start the scheduler
	s.scheduler.Start()

	log.Printf("ScanScheduler started with %d active schedules", len(schedules))
	return nil
}

// Shutdown gracefully stops the scheduler and all scheduled jobs
func (s *ScanScheduler) Shutdown() error {
	log.Println("Shutting down ScanScheduler...")
	return s.scheduler.Shutdown()
}

// registerSchedule creates a gocron job for a schedule
func (s *ScanScheduler) registerSchedule(schedule entity.ScanSchedule) error {
	key := schedule.OrgID + ":" + schedule.EntityType

	// Build gocron job definition based on frequency
	var jobDef gocron.JobDefinition

	switch schedule.Frequency {
	case entity.ScanFrequencyDaily:
		jobDef = gocron.DailyJob(
			1, // every 1 day
			gocron.NewAtTimes(
				gocron.NewAtTime(uint(schedule.Hour), uint(schedule.Minute), 0),
			),
		)
	case entity.ScanFrequencyWeekly:
		if schedule.DayOfWeek == nil {
			return fmt.Errorf("day_of_week is required for weekly frequency")
		}
		weekday := time.Weekday(*schedule.DayOfWeek)
		jobDef = gocron.WeeklyJob(
			1, // every 1 week
			gocron.NewWeekdays(weekday),
			gocron.NewAtTimes(
				gocron.NewAtTime(uint(schedule.Hour), uint(schedule.Minute), 0),
			),
		)
	case entity.ScanFrequencyMonthly:
		if schedule.DayOfMonth == nil {
			return fmt.Errorf("day_of_month is required for monthly frequency")
		}
		jobDef = gocron.MonthlyJob(
			1, // every 1 month
			gocron.NewDaysOfTheMonth(*schedule.DayOfMonth),
			gocron.NewAtTimes(
				gocron.NewAtTime(uint(schedule.Hour), uint(schedule.Minute), 0),
			),
		)
	default:
		return fmt.Errorf("unknown frequency: %s", schedule.Frequency)
	}

	// Create task function
	taskFunc := func() {
		s.executeScheduledScan(schedule)
	}

	// Register job with gocron
	job, err := s.scheduler.NewJob(
		jobDef,
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule), // Skip if already running
		gocron.WithName(key), // Named for debugging
	)
	if err != nil {
		return fmt.Errorf("failed to register job: %w", err)
	}

	// Store job reference
	s.mu.Lock()
	s.activeJobs[key] = job
	s.mu.Unlock()

	log.Printf("Registered schedule %s: %s %s at %02d:%02d", key, schedule.Frequency, schedule.EntityType, schedule.Hour, schedule.Minute)

	return nil
}

// executeScheduledScan runs when a schedule triggers
func (s *ScanScheduler) executeScheduledScan(schedule entity.ScanSchedule) {
	ctx := context.Background()

	// Get tenant DB for this org
	s.orgDBMu.RLock()
	dbCreds, exists := s.orgDBInfo[schedule.OrgID]
	s.orgDBMu.RUnlock()

	if !exists || dbCreds.tenantDB == nil {
		log.Printf("No tenant DB registered for org %s, skipping scheduled scan", schedule.OrgID)
		return
	}

	// Check if already running (safety net, gocron singleton should prevent this)
	runningJob, err := s.scanJobRepo.WithDB(dbCreds.tenantDB).GetRunningJobForEntity(ctx, schedule.OrgID, schedule.EntityType)
	if err == nil && runningJob != nil {
		// Auto-fail zombie jobs older than 30 minutes
		isZombie := false
		if runningJob.StartedAt != nil {
			isZombie = time.Since(*runningJob.StartedAt) > 30*time.Minute
		} else {
			isZombie = time.Since(runningJob.CreatedAt) > 30*time.Minute
		}
		if isZombie {
			log.Printf("[SCAN] Auto-failing zombie job %s for scheduled scan %s/%s",
				runningJob.ID, schedule.OrgID, schedule.EntityType)
			_ = s.scanJobRepo.WithDB(dbCreds.tenantDB).UpdateJobCompletion(ctx, runningJob.ID, "failed",
				runningJob.TotalRecords, runningJob.ProcessedRecords, runningJob.DuplicatesFound)
			errMsg := "Job interrupted (server restarted or timed out)"
			_, _ = dbCreds.tenantDB.ExecContext(ctx,
				"UPDATE scan_jobs SET error_message = ? WHERE id = ?", errMsg, runningJob.ID)
		} else {
			log.Printf("Scan already running for org %s entity %s (job %s), skipping scheduled run", schedule.OrgID, schedule.EntityType, runningJob.ID)
			return
		}
	}

	// Execute scan
	scheduleID := schedule.ID
	jobID, err := s.scanService.ExecuteScan(ctx, dbCreds.tenantDB, schedule.OrgID, schedule.EntityType, entity.ScanTriggerScheduled, &scheduleID)
	if err != nil {
		log.Printf("Failed to execute scheduled scan for org %s entity %s: %v", schedule.OrgID, schedule.EntityType, err)
		return
	}

	log.Printf("Started scheduled scan job %s for org %s entity %s", jobID, schedule.OrgID, schedule.EntityType)

	// Update next_run_at in schedule
	nextRun := s.calculateNextRun(schedule)
	if err := s.scanJobRepo.UpdateNextRunAt(ctx, schedule.ID, nextRun); err != nil {
		log.Printf("Failed to update next_run_at for schedule %s: %v", schedule.ID, err)
	}
}

// UpdateSchedule hot-reloads schedule changes without restart
func (s *ScanScheduler) UpdateSchedule(ctx context.Context, schedule entity.ScanSchedule) error {
	key := schedule.OrgID + ":" + schedule.EntityType

	// Remove existing gocron job if present
	s.mu.Lock()
	if existingJob, ok := s.activeJobs[key]; ok {
		if err := s.scheduler.RemoveJob(existingJob.ID()); err != nil {
			log.Printf("Failed to remove existing job %s: %v", key, err)
		}
		delete(s.activeJobs, key)
	}
	s.mu.Unlock()

	// If enabled, register new job
	if schedule.IsEnabled {
		if err := s.registerSchedule(schedule); err != nil {
			return fmt.Errorf("failed to register updated schedule: %w", err)
		}
	}

	// Persist to DB
	if err := s.scanJobRepo.UpsertSchedule(ctx, &schedule); err != nil {
		return fmt.Errorf("failed to persist schedule: %w", err)
	}

	log.Printf("Updated schedule %s (enabled: %v)", key, schedule.IsEnabled)
	return nil
}

// RemoveSchedule deletes a schedule and removes its gocron job
func (s *ScanScheduler) RemoveSchedule(ctx context.Context, orgID, entityType string) error {
	key := orgID + ":" + entityType

	// Remove gocron job
	s.mu.Lock()
	if job, ok := s.activeJobs[key]; ok {
		if err := s.scheduler.RemoveJob(job.ID()); err != nil {
			log.Printf("Failed to remove job %s: %v", key, err)
		}
		delete(s.activeJobs, key)
	}
	s.mu.Unlock()

	// Delete from DB
	if err := s.scanJobRepo.DeleteSchedule(ctx, orgID, entityType); err != nil {
		return fmt.Errorf("failed to delete schedule from DB: %w", err)
	}

	log.Printf("Removed schedule %s", key)
	return nil
}

// TriggerManualScan triggers a scan immediately (Run Now button)
func (s *ScanScheduler) TriggerManualScan(ctx context.Context, tenantDB *sql.DB, orgID, entityType string) (string, error) {
	// Check if already running
	runningJob, err := s.scanJobRepo.WithDB(tenantDB).GetRunningJobForEntity(ctx, orgID, entityType)
	if err == nil && runningJob != nil {
		// Auto-fail zombie jobs: if a job has been "running" for >30 minutes,
		// it was likely interrupted by a server restart. Mark it as failed so
		// a new scan can start.
		isZombie := false
		if runningJob.StartedAt != nil {
			isZombie = time.Since(*runningJob.StartedAt) > 30*time.Minute
		} else {
			isZombie = time.Since(runningJob.CreatedAt) > 30*time.Minute
		}
		if isZombie {
			errMsg := "Job interrupted (server restarted or timed out)"
			log.Printf("[SCAN] Auto-failing zombie job %s for %s/%s (started %v)",
				runningJob.ID, orgID, entityType, runningJob.StartedAt)
			_ = s.scanJobRepo.WithDB(tenantDB).UpdateJobCompletion(ctx, runningJob.ID, "failed",
				runningJob.TotalRecords, runningJob.ProcessedRecords, runningJob.DuplicatesFound)
			// Also set error message
			_, _ = tenantDB.ExecContext(ctx,
				"UPDATE scan_jobs SET error_message = ? WHERE id = ?", errMsg, runningJob.ID)
		} else {
			return "", fmt.Errorf("scan already running for this entity (job %s)", runningJob.ID)
		}
	}

	// Execute scan
	jobID, err := s.scanService.ExecuteScan(ctx, tenantDB, orgID, entityType, entity.ScanTriggerManual, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start manual scan: %w", err)
	}

	log.Printf("Started manual scan job %s for org %s entity %s", jobID, orgID, entityType)
	return jobID, nil
}

// GetActiveSchedules returns list of active schedule keys for debugging
func (s *ScanScheduler) GetActiveSchedules() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.activeJobs))
	for key := range s.activeJobs {
		keys = append(keys, key)
	}
	return keys
}

// calculateNextRun computes the next run time based on frequency
func (s *ScanScheduler) calculateNextRun(schedule entity.ScanSchedule) time.Time {
	now := time.Now()

	switch schedule.Frequency {
	case entity.ScanFrequencyDaily:
		// Next occurrence of hour:minute today or tomorrow
		next := time.Date(now.Year(), now.Month(), now.Day(), schedule.Hour, schedule.Minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 0, 1) // Tomorrow
		}
		return next

	case entity.ScanFrequencyWeekly:
		if schedule.DayOfWeek == nil {
			return now.AddDate(0, 0, 7) // Fallback: 7 days from now
		}
		targetWeekday := time.Weekday(*schedule.DayOfWeek)
		daysUntilTarget := (int(targetWeekday) - int(now.Weekday()) + 7) % 7
		if daysUntilTarget == 0 {
			// Same weekday - check if time has passed
			next := time.Date(now.Year(), now.Month(), now.Day(), schedule.Hour, schedule.Minute, 0, 0, now.Location())
			if next.Before(now) {
				daysUntilTarget = 7 // Next week
			}
		}
		next := now.AddDate(0, 0, daysUntilTarget)
		return time.Date(next.Year(), next.Month(), next.Day(), schedule.Hour, schedule.Minute, 0, 0, now.Location())

	case entity.ScanFrequencyMonthly:
		if schedule.DayOfMonth == nil {
			return now.AddDate(0, 1, 0) // Fallback: 1 month from now
		}
		targetDay := *schedule.DayOfMonth
		next := time.Date(now.Year(), now.Month(), targetDay, schedule.Hour, schedule.Minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0) // Next month
		}
		return next

	default:
		return now.AddDate(0, 0, 1) // Fallback: 1 day from now
	}
}
