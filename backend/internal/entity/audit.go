package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// Authentication events
	AuditEventLoginSuccess  AuditEventType = "LOGIN_SUCCESS"
	AuditEventLoginFailed   AuditEventType = "LOGIN_FAILED"
	AuditEventLogout        AuditEventType = "LOGOUT"
	AuditEventPasswordReset AuditEventType = "PASSWORD_RESET"
	AuditEventPasswordChange AuditEventType = "PASSWORD_CHANGE"

	// User management events
	AuditEventUserCreate       AuditEventType = "USER_CREATE"
	AuditEventUserUpdate       AuditEventType = "USER_UPDATE"
	AuditEventUserDelete       AuditEventType = "USER_DELETE"
	AuditEventUserInvite       AuditEventType = "USER_INVITE"
	AuditEventRoleChange       AuditEventType = "ROLE_CHANGE"
	AuditEventUserStatusChange AuditEventType = "USER_STATUS_CHANGE"

	// Impersonation events
	AuditEventImpersonationStart AuditEventType = "IMPERSONATION_START"
	AuditEventImpersonationStop  AuditEventType = "IMPERSONATION_STOP"

	// API token events
	AuditEventAPITokenCreate AuditEventType = "API_TOKEN_CREATE"
	AuditEventAPITokenRevoke AuditEventType = "API_TOKEN_REVOKE"

	// Authorization events
	AuditEventAuthorizationDenied AuditEventType = "AUTHORIZATION_DENIED"

	// Organization settings events
	AuditEventOrgSettingsChange AuditEventType = "ORG_SETTINGS_CHANGE"

	// Merge events
	AuditEventRecordMerge AuditEventType = "RECORD_MERGE"
	AuditEventMergeUndo   AuditEventType = "MERGE_UNDO"
)

// AuditEvent represents a security audit event (input format)
type AuditEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"eventType"`
	ActorID     string                 `json:"actorId,omitempty"`
	ActorEmail  string                 `json:"actorEmail,omitempty"`
	TargetID    string                 `json:"targetId,omitempty"`
	TargetEmail string                 `json:"targetEmail,omitempty"`
	OrgID       string                 `json:"orgId,omitempty"`
	IPAddress   string                 `json:"ipAddress,omitempty"`
	UserAgent   string                 `json:"userAgent,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"errorMsg,omitempty"`
}

// AuditLogEntry represents a persisted audit log entry with hash chain
type AuditLogEntry struct {
	ID         string    `json:"id" db:"id"`
	OrgID      string    `json:"orgId" db:"org_id"`
	EventType  string    `json:"eventType" db:"event_type"`
	ActorID    string    `json:"actorId,omitempty" db:"actor_id"`
	ActorEmail string    `json:"actorEmail,omitempty" db:"actor_email"`
	TargetID   string    `json:"targetId,omitempty" db:"target_id"`
	TargetType string    `json:"targetType,omitempty" db:"target_type"`
	IPAddress  string    `json:"ipAddress,omitempty" db:"ip_address"`
	UserAgent  string    `json:"userAgent,omitempty" db:"user_agent"`
	Details    string    `json:"details,omitempty" db:"details"` // JSON string
	Success    bool      `json:"success" db:"success"`
	ErrorMsg   string    `json:"errorMsg,omitempty" db:"error_msg"`
	PrevHash   string    `json:"prevHash" db:"prev_hash"`
	EntryHash  string    `json:"entryHash" db:"entry_hash"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
}

// ComputeEntryHash generates a SHA-256 hash of the entry for tamper detection
// The hash includes all fields in a deterministic order to ensure consistency
func (e *AuditLogEntry) ComputeEntryHash() string {
	// Convert success bool to string for consistent hashing
	successStr := "0"
	if e.Success {
		successStr = "1"
	}

	// Build deterministic string from all fields
	// Format: ID|OrgID|EventType|ActorID|ActorEmail|TargetID|TargetType|IPAddress|Details|Success|ErrorMsg|PrevHash|CreatedAt
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		e.ID,
		e.OrgID,
		e.EventType,
		e.ActorID,
		e.ActorEmail,
		e.TargetID,
		e.TargetType,
		e.IPAddress,
		e.Details,
		successStr,
		e.ErrorMsg,
		e.PrevHash,
		e.CreatedAt.Format(time.RFC3339Nano),
	)

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// AuditLogFilters represents query filters for listing audit logs
type AuditLogFilters struct {
	EventTypes []string   `json:"eventTypes,omitempty"`
	ActorID    string     `json:"actorId,omitempty"`
	DateFrom   *time.Time `json:"dateFrom,omitempty"`
	DateTo     *time.Time `json:"dateTo,omitempty"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// AuditLogListResponse represents the response for listing audit logs
type AuditLogListResponse struct {
	Data     []AuditLogEntry `json:"data"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	HasMore  bool            `json:"hasMore"`
}

// ChainVerificationResult represents the result of verifying the hash chain
type ChainVerificationResult struct {
	Valid            bool     `json:"valid"`
	EntriesVerified  int      `json:"entriesVerified"`
	Errors           []string `json:"errors,omitempty"`
	FirstEntryID     string   `json:"firstEntryId,omitempty"`
	LastEntryID      string   `json:"lastEntryId,omitempty"`
	FirstBrokenEntry string   `json:"firstBrokenEntry,omitempty"`
}
