package service

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventImpersonationStart AuditEventType = "IMPERSONATION_START"
	AuditEventImpersonationStop  AuditEventType = "IMPERSONATION_STOP"
	AuditEventLoginSuccess       AuditEventType = "LOGIN_SUCCESS"
	AuditEventLoginFailed        AuditEventType = "LOGIN_FAILED"
	AuditEventPasswordReset      AuditEventType = "PASSWORD_RESET"
	AuditEventPasswordChange     AuditEventType = "PASSWORD_CHANGE"
	AuditEventRoleChange         AuditEventType = "ROLE_CHANGE"
	AuditEventUserInvite         AuditEventType = "USER_INVITE"
	AuditEventAPITokenCreate     AuditEventType = "API_TOKEN_CREATE"
	AuditEventAPITokenRevoke     AuditEventType = "API_TOKEN_REVOKE"
)

// AuditEvent represents a security audit event
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

// AuditLogger handles security audit logging
type AuditLogger struct {
	// In production, this could write to a database, external service, or structured logging
}

// NewAuditLogger creates a new AuditLogger
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

// Log writes an audit event
// SECURITY: All sensitive operations should be logged for forensic analysis
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	event.Timestamp = time.Now().UTC()

	// Serialize to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("[AUDIT ERROR] Failed to serialize audit event: %v", err)
		return
	}

	// Log to stdout in structured format
	// In production, this should go to a dedicated audit log store
	log.Printf("[AUDIT] %s", string(eventJSON))
}

// LogImpersonationStart logs when a platform admin starts impersonating
func (a *AuditLogger) LogImpersonationStart(ctx context.Context, adminID, adminEmail, targetOrgID, targetUserID, ipAddress, userAgent string) {
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventImpersonationStart,
		ActorID:     adminID,
		ActorEmail:  adminEmail,
		TargetID:    targetUserID,
		OrgID:       targetOrgID,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Success:     true,
		Details: map[string]interface{}{
			"action": "Platform admin started impersonation session",
		},
	})
}

// LogImpersonationStop logs when impersonation ends
func (a *AuditLogger) LogImpersonationStop(ctx context.Context, adminID, adminEmail, ipAddress, userAgent string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventImpersonationStop,
		ActorID:    adminID,
		ActorEmail: adminEmail,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    true,
		Details: map[string]interface{}{
			"action": "Platform admin ended impersonation session",
		},
	})
}

// LogLoginAttempt logs login attempts (success or failure)
func (a *AuditLogger) LogLoginAttempt(ctx context.Context, email, ipAddress, userAgent string, success bool, errorMsg string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventLoginSuccess,
		ActorEmail: email,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    success,
		ErrorMsg:   errorMsg,
	})
}

// LogRoleChange logs when a user's role is changed
func (a *AuditLogger) LogRoleChange(ctx context.Context, actorID, actorEmail, targetID, targetEmail, orgID, oldRole, newRole, ipAddress string) {
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventRoleChange,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		TargetID:    targetID,
		TargetEmail: targetEmail,
		OrgID:       orgID,
		IPAddress:   ipAddress,
		Success:     true,
		Details: map[string]interface{}{
			"oldRole": oldRole,
			"newRole": newRole,
		},
	})
}

// LogAPITokenCreate logs API token creation
func (a *AuditLogger) LogAPITokenCreate(ctx context.Context, actorID, orgID, tokenName string, scopes []string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventAPITokenCreate,
		ActorID:   actorID,
		OrgID:     orgID,
		Success:   true,
		Details: map[string]interface{}{
			"tokenName": tokenName,
			"scopes":    scopes,
		},
	})
}

// LogAPITokenRevoke logs API token revocation
func (a *AuditLogger) LogAPITokenRevoke(ctx context.Context, actorID, orgID, tokenID string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventAPITokenRevoke,
		ActorID:   actorID,
		OrgID:     orgID,
		Success:   true,
		Details: map[string]interface{}{
			"tokenId": tokenID,
		},
	})
}
