package entity

import "time"

// ConditionType represents the type of a tripwire condition
type ConditionType string

const (
	ConditionIsNew        ConditionType = "ISNEW"
	ConditionIsChanged    ConditionType = "ISCHANGED"
	ConditionIsDeleted    ConditionType = "ISDELETED"
	ConditionFieldEquals  ConditionType = "FIELD_EQUALS"
	ConditionFieldChanged ConditionType = "FIELD_CHANGED_TO"
)

// TripwireCondition represents a single condition in a tripwire
type TripwireCondition struct {
	ID        string        `json:"id"`
	Type      ConditionType `json:"type"`
	FieldName *string       `json:"fieldName,omitempty"`
	Value     *string       `json:"value,omitempty"`     // For FIELD_EQUALS
	FromValue *string       `json:"fromValue,omitempty"` // For FIELD_CHANGED_TO
	ToValue   *string       `json:"toValue,omitempty"`   // For FIELD_CHANGED_TO
}

// Tripwire represents a webhook trigger configuration
type Tripwire struct {
	ID             string              `json:"id" db:"id"`
	OrgID          string              `json:"orgId" db:"org_id"`
	Name           string              `json:"name" db:"name"`
	Description    *string             `json:"description,omitempty" db:"description"`
	EntityType     string              `json:"entityType" db:"entity_type"`
	EndpointURL    string              `json:"endpointUrl" db:"endpoint_url"`
	Enabled        bool                `json:"enabled" db:"enabled"`
	ConditionLogic string              `json:"conditionLogic" db:"condition_logic"`
	Conditions     []TripwireCondition `json:"conditions" db:"-"`
	ConditionsJSON string              `json:"-" db:"conditions"`
	CreatedAt      time.Time           `json:"createdAt" db:"created_at"`
	ModifiedAt     time.Time           `json:"modifiedAt" db:"modified_at"`
	CreatedBy      *string             `json:"createdBy,omitempty" db:"created_by"`
	ModifiedBy     *string             `json:"modifiedBy,omitempty" db:"modified_by"`
}

// TripwireCreateInput represents the input for creating a tripwire
type TripwireCreateInput struct {
	Name           string              `json:"name" validate:"required"`
	Description    *string             `json:"description"`
	EntityType     string              `json:"entityType" validate:"required"`
	EndpointURL    string              `json:"endpointUrl" validate:"required"`
	Enabled        *bool               `json:"enabled"`
	ConditionLogic string              `json:"conditionLogic"`
	Conditions     []TripwireCondition `json:"conditions" validate:"required"`
}

// TripwireUpdateInput represents the input for updating a tripwire
type TripwireUpdateInput struct {
	Name           *string             `json:"name"`
	Description    *string             `json:"description"`
	EntityType     *string             `json:"entityType"`
	EndpointURL    *string             `json:"endpointUrl"`
	Enabled        *bool               `json:"enabled"`
	ConditionLogic *string             `json:"conditionLogic"`
	Conditions     []TripwireCondition `json:"conditions"`
}

// TripwireListParams represents query parameters for listing tripwires
type TripwireListParams struct {
	Search     string `query:"search"`
	EntityType string `query:"entityType"`
	Enabled    *bool  `query:"enabled"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
}

// TripwireListResponse represents the response for listing tripwires
type TripwireListResponse struct {
	Data       []Tripwire `json:"data"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	TotalPages int        `json:"totalPages"`
}

// WebhookAuthType represents the type of webhook authentication
type WebhookAuthType string

const (
	WebhookAuthNone         WebhookAuthType = "none"
	WebhookAuthAPIKey       WebhookAuthType = "api_key"
	WebhookAuthBearer       WebhookAuthType = "bearer"
	WebhookAuthCustomHeader WebhookAuthType = "custom_header"
)

// OrgWebhookSettings represents org-level webhook authentication settings
type OrgWebhookSettings struct {
	ID                string          `json:"id" db:"id"`
	OrgID             string          `json:"orgId" db:"org_id"`
	AuthType          WebhookAuthType `json:"authType" db:"auth_type"`
	APIKey            *string         `json:"apiKey,omitempty" db:"api_key"`
	BearerToken       *string         `json:"bearerToken,omitempty" db:"bearer_token"`
	CustomHeaderName  *string         `json:"customHeaderName,omitempty" db:"custom_header_name"`
	CustomHeaderValue *string         `json:"customHeaderValue,omitempty" db:"custom_header_value"`
	TimeoutMs         int             `json:"timeoutMs" db:"timeout_ms"`
	CreatedAt         time.Time       `json:"createdAt" db:"created_at"`
	ModifiedAt        time.Time       `json:"modifiedAt" db:"modified_at"`
}

// OrgWebhookSettingsInput represents the input for saving webhook settings
type OrgWebhookSettingsInput struct {
	AuthType          WebhookAuthType `json:"authType"`
	APIKey            *string         `json:"apiKey,omitempty"`
	BearerToken       *string         `json:"bearerToken,omitempty"`
	CustomHeaderName  *string         `json:"customHeaderName,omitempty"`
	CustomHeaderValue *string         `json:"customHeaderValue,omitempty"`
	TimeoutMs         *int            `json:"timeoutMs,omitempty"`
}

// TripwireLog represents a log entry for tripwire execution
type TripwireLog struct {
	ID           string    `json:"id" db:"id"`
	TripwireID   string    `json:"tripwireId" db:"tripwire_id"`
	TripwireName *string   `json:"tripwireName,omitempty" db:"tripwire_name"`
	OrgID        string    `json:"orgId" db:"org_id"`
	RecordID     string    `json:"recordId" db:"record_id"`
	EntityType   string    `json:"entityType" db:"entity_type"`
	EventType    string    `json:"eventType" db:"event_type"`
	Status       string    `json:"status" db:"status"`
	ResponseCode *int      `json:"responseCode,omitempty" db:"response_code"`
	ErrorMessage *string   `json:"errorMessage,omitempty" db:"error_message"`
	DurationMs   *int      `json:"durationMs,omitempty" db:"duration_ms"`
	ExecutedAt   time.Time `json:"executedAt" db:"executed_at"`
}

// TripwireLogListParams represents query parameters for listing tripwire logs
type TripwireLogListParams struct {
	TripwireID string `query:"tripwireId"`
	Status     string `query:"status"`
	EventType  string `query:"eventType"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
}

// TripwireLogListResponse represents the response for listing tripwire logs
type TripwireLogListResponse struct {
	Data       []TripwireLog `json:"data"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}

// FieldChange represents a change to a single field
type FieldChange struct {
	OldValue interface{} `json:"oldValue"`
	NewValue interface{} `json:"newValue"`
}

// WebhookPayload represents the payload sent to the webhook endpoint
type WebhookPayload struct {
	TripwireID    string                  `json:"tripwireId"`
	TripwireName  string                  `json:"tripwireName"`
	Event         string                  `json:"event"`
	EntityType    string                  `json:"entityType"`
	RecordID      string                  `json:"recordId"`
	Timestamp     string                  `json:"timestamp"`
	ChangedFields []string                `json:"changedFields,omitempty"` // List of field names that changed
	Changes       map[string]*FieldChange `json:"changes,omitempty"`       // Detailed changes per field
	Record        map[string]interface{}  `json:"record,omitempty"`        // Current record data (new for CREATE/UPDATE, old for DELETE)
}
