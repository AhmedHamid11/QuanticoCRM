package entity

import "time"

// ScanSchedule represents an admin-configured background scan schedule
type ScanSchedule struct {
	ID          string     `json:"id" db:"id"`
	OrgID       string     `json:"orgId" db:"org_id"`
	EntityType  string     `json:"entityType" db:"entity_type"`
	Frequency   string     `json:"frequency" db:"frequency"` // "daily", "weekly", "monthly"
	DayOfWeek   *int       `json:"dayOfWeek,omitempty" db:"day_of_week"`
	DayOfMonth  *int       `json:"dayOfMonth,omitempty" db:"day_of_month"`
	Hour        int        `json:"hour" db:"hour"`
	Minute      int        `json:"minute" db:"minute"`
	IsEnabled   bool       `json:"isEnabled" db:"is_enabled"`
	LastRunAt   *time.Time `json:"lastRunAt,omitempty" db:"last_run_at"`
	NextRunAt   *time.Time `json:"nextRunAt,omitempty" db:"next_run_at"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`
}

// ScanJob represents an individual scan execution
type ScanJob struct {
	ID               string     `json:"id" db:"id"`
	OrgID            string     `json:"orgId" db:"org_id"`
	EntityType       string     `json:"entityType" db:"entity_type"`
	ScheduleID       *string    `json:"scheduleId,omitempty" db:"schedule_id"`
	Status           string     `json:"status" db:"status"`
	TriggerType      string     `json:"triggerType" db:"trigger_type"`
	TotalRecords     int        `json:"totalRecords" db:"total_records"`
	ProcessedRecords int        `json:"processedRecords" db:"processed_records"`
	DuplicatesFound  int        `json:"duplicatesFound" db:"duplicates_found"`
	ErrorMessage     *string    `json:"errorMessage,omitempty" db:"error_message"`
	StartedAt        *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt      *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	CreatedAt        time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time  `json:"updatedAt" db:"updated_at"`
}

// ScanCheckpoint represents progress state for resume-from-failure
type ScanCheckpoint struct {
	ID              string    `json:"id" db:"id"`
	JobID           string    `json:"jobId" db:"job_id"`
	LastOffset      int       `json:"lastOffset" db:"last_offset"`
	LastProcessedID *string   `json:"lastProcessedId,omitempty" db:"last_processed_id"`
	RetryCount      int       `json:"retryCount" db:"retry_count"`
	ChunkSize       int       `json:"chunkSize" db:"chunk_size"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

// ScanScheduleInput for creating/updating schedules
type ScanScheduleInput struct {
	EntityType string `json:"entityType" validate:"required"`
	Frequency  string `json:"frequency" validate:"required,oneof=daily weekly monthly"`
	DayOfWeek  *int   `json:"dayOfWeek,omitempty"`
	DayOfMonth *int   `json:"dayOfMonth,omitempty"`
	Hour       int    `json:"hour" validate:"min=0,max=23"`
	Minute     int    `json:"minute" validate:"min=0,max=59"`
	IsEnabled  bool   `json:"isEnabled"`
}

// Scan status constants
const (
	ScanStatusPending   = "pending"
	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
	ScanStatusCancelled = "cancelled"
)

// Scan trigger type constants
const (
	ScanTriggerScheduled = "scheduled"
	ScanTriggerManual    = "manual"
)

// Scan frequency constants
const (
	ScanFrequencyDaily   = "daily"
	ScanFrequencyWeekly  = "weekly"
	ScanFrequencyMonthly = "monthly"
)
