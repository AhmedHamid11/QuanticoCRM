package entity

import "time"

// ---------------------------------------------------------------------------
// Sequence statuses
// ---------------------------------------------------------------------------

const (
	SequenceStatusDraft    = "draft"
	SequenceStatusActive   = "active"
	SequenceStatusPaused   = "paused"
	SequenceStatusArchived = "archived"
)

// ---------------------------------------------------------------------------
// Step types
// ---------------------------------------------------------------------------

const (
	StepTypeEmail    = "email"
	StepTypeCall     = "call"
	StepTypeSMS      = "sms"
	StepTypeLinkedIn = "linkedin"
	StepTypeCustom   = "custom"
)

// ---------------------------------------------------------------------------
// Enrollment statuses
// ---------------------------------------------------------------------------

const (
	EnrollmentStatusEnrolled = "enrolled"
	EnrollmentStatusActive   = "active"
	EnrollmentStatusFinished = "finished"
	EnrollmentStatusPaused   = "paused"
	EnrollmentStatusReplied  = "replied"
	EnrollmentStatusBounced  = "bounced"
	EnrollmentStatusOptedOut = "opted_out"
)

// ---------------------------------------------------------------------------
// Step execution statuses
// ---------------------------------------------------------------------------

const (
	ExecutionStatusPending   = "pending"
	ExecutionStatusScheduled = "scheduled"
	ExecutionStatusExecuting = "executing"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusSkipped   = "skipped"
)

// ---------------------------------------------------------------------------
// Tracking event types
// ---------------------------------------------------------------------------

const (
	TrackingEventOpen   = "open"
	TrackingEventClick  = "click"
	TrackingEventReply  = "reply"
	TrackingEventBounce = "bounce"
	TrackingEventOOO    = "ooo"
)

// ---------------------------------------------------------------------------
// Call dispositions
// ---------------------------------------------------------------------------

const (
	DispositionConnected     = "connected"
	DispositionVoicemail     = "voicemail"
	DispositionNoAnswer      = "no_answer"
	DispositionWrongNumber   = "wrong_number"
	DispositionNotInterested = "not_interested"
)

// ---------------------------------------------------------------------------
// SMS directions
// ---------------------------------------------------------------------------

const (
	SMSDirectionOutbound = "outbound"
	SMSDirectionInbound  = "inbound"
)

// ---------------------------------------------------------------------------
// Opt-out channels
// ---------------------------------------------------------------------------

const (
	OptOutChannelEmail = "email"
	OptOutChannelSMS   = "sms"
	OptOutChannelAll   = "all"
)

// ---------------------------------------------------------------------------
// Warmup statuses
// ---------------------------------------------------------------------------

const (
	WarmupStatusActive    = "active"
	WarmupStatusCompleted = "completed"
	WarmupStatusPaused    = "paused"
)

// ---------------------------------------------------------------------------
// Domain structs
// ---------------------------------------------------------------------------

// Sequence is an automated multi-step outreach workflow scoped to an org.
// Sequences are org-level resources — there is no OwnerID (by design).
type Sequence struct {
	ID                 string    `json:"id" db:"id"`
	OrgID              string    `json:"orgId" db:"org_id"`
	Name               string    `json:"name" db:"name"`
	Description        *string   `json:"description,omitempty" db:"description"`
	Status             string    `json:"status" db:"status"`
	Timezone           string    `json:"timezone" db:"timezone"`
	BusinessHoursStart *string   `json:"businessHoursStart,omitempty" db:"business_hours_start"`
	BusinessHoursEnd   *string   `json:"businessHoursEnd,omitempty" db:"business_hours_end"`
	CreatedBy          string    `json:"createdBy" db:"created_by"`
	CreatedAt          time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time `json:"updatedAt" db:"updated_at"`
}

// SequenceStep defines a single step within a sequence.
type SequenceStep struct {
	ID         string    `json:"id" db:"id"`
	SequenceID string    `json:"sequenceId" db:"sequence_id"`
	StepNumber int       `json:"stepNumber" db:"step_number"`
	StepType   string    `json:"stepType" db:"step_type"`
	DelayDays  int       `json:"delayDays" db:"delay_days"`
	DelayHours int       `json:"delayHours" db:"delay_hours"`
	TemplateID *string   `json:"templateId,omitempty" db:"template_id"`
	ConfigJSON *string   `json:"configJson,omitempty" db:"config_json"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}

// SequenceEnrollment tracks a contact's progress through a sequence.
// EnrolledBy is the PRA user ID who triggered the enrollment.
type SequenceEnrollment struct {
	ID           string     `json:"id" db:"id"`
	SequenceID   string     `json:"sequenceId" db:"sequence_id"`
	ContactID    string     `json:"contactId" db:"contact_id"`
	OrgID        string     `json:"orgId" db:"org_id"`
	EnrolledBy   string     `json:"enrolledBy" db:"enrolled_by"`
	Status       string     `json:"status" db:"status"`
	CurrentStep  int        `json:"currentStep" db:"current_step"`
	ABVariantID  *string    `json:"abVariantId,omitempty" db:"ab_variant_id"`
	EnrolledAt   time.Time  `json:"enrolledAt" db:"enrolled_at"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty" db:"finished_at"`
	PausedAt     *time.Time `json:"pausedAt,omitempty" db:"paused_at"`
	CreatedAt    time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time  `json:"updatedAt" db:"updated_at"`
}

// StepExecution records the outcome of executing a single step for an enrollment.
type StepExecution struct {
	ID           string     `json:"id" db:"id"`
	EnrollmentID string     `json:"enrollmentId" db:"enrollment_id"`
	StepID       string     `json:"stepId" db:"step_id"`
	OrgID        string     `json:"orgId" db:"org_id"`
	Status       string     `json:"status" db:"status"`
	ScheduledAt  *time.Time `json:"scheduledAt,omitempty" db:"scheduled_at"`
	ExecutedAt   *time.Time `json:"executedAt,omitempty" db:"executed_at"`
	ErrorMessage *string    `json:"errorMessage,omitempty" db:"error_message"`
	CreatedAt    time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time  `json:"updatedAt" db:"updated_at"`
}

// EmailTemplate holds the HTML and text versions of an outreach email.
// HasComplianceFooter is stored as int in SQLite (0/1) and mapped to bool.
type EmailTemplate struct {
	ID                  string    `json:"id" db:"id"`
	OrgID               string    `json:"orgId" db:"org_id"`
	Name                string    `json:"name" db:"name"`
	Subject             string    `json:"subject" db:"subject"`
	BodyHTML            string    `json:"bodyHtml" db:"body_html"`
	BodyText            string    `json:"bodyText" db:"body_text"`
	HasComplianceFooter int       `json:"hasComplianceFooter" db:"has_compliance_footer"`
	CreatedBy           string    `json:"createdBy" db:"created_by"`
	CreatedAt           time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt           time.Time `json:"updatedAt" db:"updated_at"`
}

// TrackingEvent is an immutable record of an email engagement event.
// High-write table — written via EventBuffer to prevent SQLite contention.
// No UpdatedAt by design (immutable).
type TrackingEvent struct {
	ID              string    `json:"id" db:"id"`
	OrgID           string    `json:"orgId" db:"org_id"`
	EnrollmentID    string    `json:"enrollmentId" db:"enrollment_id"`
	StepExecutionID string    `json:"stepExecutionId" db:"step_execution_id"`
	EventType       string    `json:"eventType" db:"event_type"`
	LinkURL         *string   `json:"linkUrl,omitempty" db:"link_url"`
	MetadataJSON    *string   `json:"metadataJson,omitempty" db:"metadata_json"`
	OccurredAt      time.Time `json:"occurredAt" db:"occurred_at"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
}

// CallDisposition records the outcome of a call step execution.
type CallDisposition struct {
	ID              string     `json:"id" db:"id"`
	OrgID           string     `json:"orgId" db:"org_id"`
	StepExecutionID string     `json:"stepExecutionId" db:"step_execution_id"`
	ContactID       string     `json:"contactId" db:"contact_id"`
	EnrolledBy      string     `json:"enrolledBy" db:"enrolled_by"`
	Disposition     string     `json:"disposition" db:"disposition"`
	Notes           *string    `json:"notes,omitempty" db:"notes"`
	DurationSeconds *int       `json:"durationSeconds,omitempty" db:"duration_seconds"`
	CalledAt        time.Time  `json:"calledAt" db:"called_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
}

// SMSMessage records an outbound or inbound SMS message for a step execution.
type SMSMessage struct {
	ID              string     `json:"id" db:"id"`
	OrgID           string     `json:"orgId" db:"org_id"`
	EnrollmentID    string     `json:"enrollmentId" db:"enrollment_id"`
	StepExecutionID string     `json:"stepExecutionId" db:"step_execution_id"`
	ContactID       string     `json:"contactId" db:"contact_id"`
	Direction       string     `json:"direction" db:"direction"`
	Body            string     `json:"body" db:"body"`
	Status          string     `json:"status" db:"status"`
	ExternalID      *string    `json:"externalId,omitempty" db:"external_id"`
	SentAt          *time.Time `json:"sentAt,omitempty" db:"sent_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
}

// OptOutEntry records a contact's opt-out from a specific channel.
type OptOutEntry struct {
	ID          string    `json:"id" db:"id"`
	OrgID       string    `json:"orgId" db:"org_id"`
	ContactID   string    `json:"contactId" db:"contact_id"`
	Channel     string    `json:"channel" db:"channel"`
	Reason      *string   `json:"reason,omitempty" db:"reason"`
	OptedOutAt  time.Time `json:"optedOutAt" db:"opted_out_at"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

// WarmupSession tracks Gmail warmup progress for a user's connected account.
// CurrentDailyCount resets each day via the warmup scheduler.
type WarmupSession struct {
	ID                string    `json:"id" db:"id"`
	OrgID             string    `json:"orgId" db:"org_id"`
	UserID            string    `json:"userId" db:"user_id"`
	GmailAccountEmail string    `json:"gmailAccountEmail" db:"gmail_account_email"`
	DailyLimit        int       `json:"dailyLimit" db:"daily_limit"`
	CurrentDailyCount int       `json:"currentDailyCount" db:"current_daily_count"`
	Status            string    `json:"status" db:"status"`
	StartedAt         time.Time `json:"startedAt" db:"started_at"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
}

// ABTestVariant defines a variant for A/B testing a sequence step.
// IsWinner is stored as int in SQLite (0/1).
type ABTestVariant struct {
	ID               string    `json:"id" db:"id"`
	StepID           string    `json:"stepId" db:"step_id"`
	VariantLabel     string    `json:"variantLabel" db:"variant_label"`
	SubjectOverride  *string   `json:"subjectOverride,omitempty" db:"subject_override"`
	BodyHTMLOverride *string   `json:"bodyHtmlOverride,omitempty" db:"body_html_override"`
	TrafficPct       int       `json:"trafficPct" db:"traffic_pct"`
	IsWinner         int       `json:"isWinner" db:"is_winner"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

// ABTrackingStats accumulates aggregate performance metrics per A/B variant.
// UpdatedAt only — no CreatedAt (upserted row).
type ABTrackingStats struct {
	ID        string    `json:"id" db:"id"`
	VariantID string    `json:"variantId" db:"variant_id"`
	OrgID     string    `json:"orgId" db:"org_id"`
	Sends     int       `json:"sends" db:"sends"`
	Opens     int       `json:"opens" db:"opens"`
	Clicks    int       `json:"clicks" db:"clicks"`
	Replies   int       `json:"replies" db:"replies"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// GmailOAuthToken stores per-user per-org Gmail OAuth credentials in the tenant DB.
// Encrypted token fields use json:"-" so they are never serialised to API responses.
// DNS validity flags are stored as int in SQLite (0/1).
type GmailOAuthToken struct {
	ID                    string     `json:"id" db:"id"`
	OrgID                 string     `json:"orgId" db:"org_id"`
	UserID                string     `json:"userId" db:"user_id"`
	AccessTokenEncrypted  []byte     `json:"-" db:"access_token_encrypted"`
	RefreshTokenEncrypted []byte     `json:"-" db:"refresh_token_encrypted"`
	TokenExpiry           *time.Time `json:"tokenExpiry,omitempty" db:"token_expiry"`
	GmailAddress          string     `json:"gmailAddress" db:"gmail_address"`
	DNSSPFValid           int        `json:"dnsspfValid" db:"dns_spf_valid"`
	DNSDKIMValid          int        `json:"dnsdkimValid" db:"dns_dkim_valid"`
	DNSDMARCValid         int        `json:"dnsdmarcValid" db:"dns_dmarc_valid"`
	ConnectedAt           *time.Time `json:"connectedAt,omitempty" db:"connected_at"`
	CreatedAt             time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt             time.Time  `json:"updatedAt" db:"updated_at"`
}

// SFDCCDCCursor is the per-org watermark for Salesforce Change Data Capture polling.
// Stored in the tenant DB so each org tracks its own replay position.
type SFDCCDCCursor struct {
	ID                string     `json:"id" db:"id"`
	OrgID             string     `json:"orgId" db:"org_id"`
	ObjectType        string     `json:"objectType" db:"object_type"`
	LastEventReplayID *string    `json:"lastEventReplayId,omitempty" db:"last_event_replay_id"`
	LastPolledAt      *time.Time `json:"lastPolledAt,omitempty" db:"last_polled_at"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
}
