package entity

import "time"

// IngestAPIKey represents an API key for external ingest authentication
type IngestAPIKey struct {
	ID         string    `json:"id" db:"id"`
	OrgID      string    `json:"orgId" db:"org_id"`
	Name       string    `json:"name" db:"name"`
	KeyHash    string    `json:"-" db:"key_hash"`         // Never exposed
	KeyPrefix  string    `json:"keyPrefix" db:"key_prefix"` // First 8 chars for identification
	IsActive   bool      `json:"isActive" db:"is_active"`
	RateLimit  int       `json:"rateLimit" db:"rate_limit"` // Requests per minute
	CreatedBy  string    `json:"createdBy" db:"created_by"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}

// IngestAPIKeyCreateInput represents input for creating a new ingest API key
type IngestAPIKeyCreateInput struct {
	Name      string `json:"name" validate:"required"`
	RateLimit *int   `json:"rateLimit"` // Optional, defaults to 500
}

// IngestAPIKeyCreateResponse includes the full key (only shown once at creation)
type IngestAPIKeyCreateResponse struct {
	Key       string    `json:"key"`       // Full key (only shown once!)
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OrgID     string    `json:"orgId"`
	RateLimit int       `json:"rateLimit"`
	CreatedAt time.Time `json:"createdAt"`
}

// IngestRequest represents an ingest request from external systems
type IngestRequest struct {
	OrgID    string                   `json:"org_id"`
	MirrorID string                   `json:"mirror_id"`
	Records  []map[string]interface{} `json:"records"`
}

// IngestResponse represents the response to an ingest request
type IngestResponse struct {
	JobID           string `json:"job_id"`
	Status          string `json:"status"` // Always "accepted" for Phase 20
	RecordsReceived int    `json:"records_received"`
	MirrorID        string `json:"mirror_id"`
	Message         string `json:"message"`
}

// IngestJob entity status constants
const (
	IngestJobStatusAccepted   = "accepted"
	IngestJobStatusProcessing = "processing"
	IngestJobStatusComplete   = "complete"
	IngestJobStatusPartial    = "partial" // Some records failed
	IngestJobStatusFailed     = "failed"
)

// IngestJob represents an async ingest job for tracking processing status
type IngestJob struct {
	ID               string        `json:"id" db:"id"`
	OrgID            string        `json:"orgId" db:"org_id"`
	MirrorID         string        `json:"mirrorId" db:"mirror_id"`
	KeyID            string        `json:"keyId" db:"key_id"` // Which API key created this job
	Status           string        `json:"status" db:"status"`
	RecordsReceived  int           `json:"recordsReceived" db:"records_received"`
	RecordsProcessed int           `json:"recordsProcessed" db:"records_processed"`
	RecordsPromoted  int           `json:"recordsPromoted" db:"records_promoted"`
	RecordsSkipped   int           `json:"recordsSkipped" db:"records_skipped"`
	RecordsFailed    int           `json:"recordsFailed" db:"records_failed"`
	Errors           []RecordError `json:"errors" db:"errors"` // Stored as JSON TEXT
	Warnings         []string      `json:"warnings" db:"warnings"` // Stored as JSON TEXT
	StartedAt        *time.Time    `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt      *time.Time    `json:"completedAt,omitempty" db:"completed_at"`
	CreatedAt        time.Time     `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time     `json:"updatedAt" db:"updated_at"`
}

// RecordError represents a per-record error during ingest processing
type RecordError struct {
	Index     int    `json:"index"`     // Position in batch (0-indexed)
	UniqueKey string `json:"uniqueKey"` // The unique key value that failed
	Field     string `json:"field"`     // Which field caused the error (if applicable)
	Message   string `json:"message"`   // Human-readable error message
	Code      string `json:"code"`      // Error code (e.g., "validation_failed", "duplicate_key", "type_mismatch")
}

// IngestJobResult is used to set the final result of a job in one call
type IngestJobResult struct {
	RecordsProcessed int           `json:"recordsProcessed"`
	RecordsPromoted  int           `json:"recordsPromoted"`
	RecordsSkipped   int           `json:"recordsSkipped"`
	RecordsFailed    int           `json:"recordsFailed"`
	Errors           []RecordError `json:"errors"`
	Warnings         []string      `json:"warnings"`
}
