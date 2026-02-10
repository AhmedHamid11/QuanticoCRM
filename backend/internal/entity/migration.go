package entity

import "time"

// MigrationRun represents a single org migration attempt
type MigrationRun struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"orgId"`
	OrgName      string     `json:"orgName"`
	FromVersion  string     `json:"fromVersion"`
	ToVersion    string     `json:"toVersion"`
	Status       string     `json:"status"` // "running", "success", "failed"
	ErrorMessage string     `json:"errorMessage,omitempty"`
	StartedAt    time.Time  `json:"startedAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// PropagationResult summarizes a propagation run across all orgs
type PropagationResult struct {
	StartedAt    time.Time      `json:"startedAt"`
	CompletedAt  time.Time      `json:"completedAt"`
	SuccessCount int            `json:"successCount"`
	FailedCount  int            `json:"failedCount"`
	SkippedCount int            `json:"skippedCount"` // Already up to date
	Runs         []MigrationRun `json:"runs"`
}

// MigrationStatusResponse is the API response for migration status
type MigrationStatusResponse struct {
	PlatformVersion string       `json:"platformVersion"`
	TotalOrgs       int          `json:"totalOrgs"`
	UpToDateCount   int          `json:"upToDateCount"`
	FailedCount     int          `json:"failedCount"`
	FailedOrgs      []FailedOrg  `json:"failedOrgs"`
	LastRunAt       *time.Time   `json:"lastRunAt,omitempty"`
}

// FailedOrg represents a failed migration for API display
type FailedOrg struct {
	OrgID            string    `json:"orgId"`
	OrgName          string    `json:"orgName"`
	ErrorMessage     string    `json:"errorMessage"`
	FailedAt         time.Time `json:"failedAt"`
	AttemptedVersion string    `json:"attemptedVersion"`
}
