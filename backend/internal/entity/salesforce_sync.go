package entity

import "time"

// SalesforceConnection represents OAuth credentials and config for Salesforce integration
type SalesforceConnection struct {
	ID                     string     `json:"id" db:"id"`
	OrgID                  string     `json:"orgId" db:"org_id"`
	ClientID               string     `json:"clientId" db:"client_id"`
	ClientSecretEncrypted  []byte     `json:"-" db:"client_secret_encrypted"`
	RedirectURL            string     `json:"redirectUrl" db:"redirect_url"`
	InstanceURL            string     `json:"instanceUrl" db:"instance_url"`
	AccessTokenEncrypted   []byte     `json:"-" db:"access_token_encrypted"`
	RefreshTokenEncrypted  []byte     `json:"-" db:"refresh_token_encrypted"`
	TokenType              string     `json:"tokenType" db:"token_type"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty" db:"expires_at"`
	IsEnabled              bool       `json:"isEnabled" db:"is_enabled"`
	ConnectedAt            *time.Time `json:"connectedAt,omitempty" db:"connected_at"`
	CreatedAt              time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt              time.Time  `json:"updatedAt" db:"updated_at"`
}

// SFSyncConfig is the input struct for admin configuration (unencrypted credentials)
type SFSyncConfig struct {
	ClientID     string `json:"clientId" validate:"required"`
	ClientSecret string `json:"clientSecret" validate:"required"`
	RedirectURL  string `json:"redirectUrl" validate:"required"`
}

// SyncJob represents an individual sync job execution
type SyncJob struct {
	ID                    string     `json:"id" db:"id"`
	OrgID                 string     `json:"orgId" db:"org_id"`
	BatchID               string     `json:"batchId" db:"batch_id"`
	EntityType            string     `json:"entityType" db:"entity_type"`
	Status                string     `json:"status" db:"status"`
	TotalInstructions     int        `json:"totalInstructions" db:"total_instructions"`
	DeliveredInstructions int        `json:"deliveredInstructions" db:"delivered_instructions"`
	FailedInstructions    int        `json:"failedInstructions" db:"failed_instructions"`
	BatchPayload          *string    `json:"batchPayload,omitempty" db:"batch_payload"`
	ErrorMessage          *string    `json:"errorMessage,omitempty" db:"error_message"`
	RetryCount            int        `json:"retryCount" db:"retry_count"`
	IdempotencyKey        string     `json:"idempotencyKey" db:"idempotency_key"`
	TriggerType           string     `json:"triggerType" db:"trigger_type"`
	StartedAt             *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt           *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	CreatedAt             time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt             time.Time  `json:"updatedAt" db:"updated_at"`
}

// SalesforceFieldMapping maps Quantico field names to Salesforce object/field names
type SalesforceFieldMapping struct {
	ID              string    `json:"id" db:"id"`
	OrgID           string    `json:"orgId" db:"org_id"`
	EntityType      string    `json:"entityType" db:"entity_type"`
	QuanticoField   string    `json:"quanticoField" db:"quantico_field"`
	SalesforceObject string   `json:"salesforceObject" db:"salesforce_object"`
	SalesforceField string    `json:"salesforceField" db:"salesforce_field"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

// MergeInstruction represents a single merge operation in a batch
type MergeInstruction struct {
	InstructionID  string                 `json:"instruction_id"`
	ObjectAPIName  string                 `json:"object_api_name"`
	WinnerID       string                 `json:"winner_id"`
	LoserID        string                 `json:"loser_id"`
	FieldValues    map[string]interface{} `json:"field_values"`
}

// MergeInstructionBatch represents a batch of merge instructions
type MergeInstructionBatch struct {
	BatchID           string             `json:"batch_id"`
	Timestamp         string             `json:"timestamp"`
	OrgID             string             `json:"org_id"`
	MergeInstructions []MergeInstruction `json:"merge_instructions"`
}

// Sync status constants
const (
	SyncStatusPending   = "pending"
	SyncStatusRunning   = "running"
	SyncStatusCompleted = "completed"
	SyncStatusFailed    = "failed"
	SyncStatusPaused    = "paused" // Paused due to API quota threshold
)

// Sync trigger type constants
const (
	SyncTriggerManual    = "manual"
	SyncTriggerScheduled = "scheduled"
	SyncTriggerRealtime  = "realtime"
)

// Salesforce API quota constants
const (
	SalesforceMaxDailyAPICalls = 100000 // Enterprise Edition limit
	SalesforcePauseThreshold   = 80000  // 80% of max - pause delivery
)

// QuotaExceededError indicates the org has reached the API usage threshold
type QuotaExceededError struct {
	OrgID     string
	Usage     int
	Threshold int
	Message   string
}

func (e *QuotaExceededError) Error() string {
	return e.Message
}

// QuotaStatus represents current API usage quota for an org
type QuotaStatus struct {
	Usage      int  `json:"usage"`
	Limit      int  `json:"limit"`
	Percentage int  `json:"percentage"`
	Threshold  int  `json:"threshold"`
	IsPaused   bool `json:"isPaused"`
}

// APIUsageLog represents a single API usage record
type APIUsageLog struct {
	ID        string    `json:"id" db:"id"`
	OrgID     string    `json:"orgId" db:"org_id"`
	Timestamp string    `json:"timestamp" db:"timestamp"`
	APICalls  int       `json:"apiCalls" db:"api_calls"`
	JobID     *string   `json:"jobId,omitempty" db:"job_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
