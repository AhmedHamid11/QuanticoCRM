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
