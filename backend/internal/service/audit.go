package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// Re-export types from entity package for backwards compatibility
type AuditEventType = entity.AuditEventType

// Re-export constants from entity package
const (
	AuditEventLoginSuccess        = entity.AuditEventLoginSuccess
	AuditEventLoginFailed         = entity.AuditEventLoginFailed
	AuditEventLogout              = entity.AuditEventLogout
	AuditEventPasswordReset       = entity.AuditEventPasswordReset
	AuditEventPasswordChange      = entity.AuditEventPasswordChange
	AuditEventUserCreate          = entity.AuditEventUserCreate
	AuditEventUserUpdate          = entity.AuditEventUserUpdate
	AuditEventUserDelete          = entity.AuditEventUserDelete
	AuditEventUserInvite          = entity.AuditEventUserInvite
	AuditEventRoleChange          = entity.AuditEventRoleChange
	AuditEventUserStatusChange    = entity.AuditEventUserStatusChange
	AuditEventImpersonationStart  = entity.AuditEventImpersonationStart
	AuditEventImpersonationStop   = entity.AuditEventImpersonationStop
	AuditEventAPITokenCreate      = entity.AuditEventAPITokenCreate
	AuditEventAPITokenRevoke      = entity.AuditEventAPITokenRevoke
	AuditEventAuthorizationDenied = entity.AuditEventAuthorizationDenied
	AuditEventOrgSettingsChange          = entity.AuditEventOrgSettingsChange
	AuditEventRecordMerge                = entity.AuditEventRecordMerge
	AuditEventMergeUndo                  = entity.AuditEventMergeUndo
	AuditEventSalesforceMergeDelivery    = entity.AuditEventSalesforceMergeDelivery
	AuditEventSalesforceMergeDeliveryError = entity.AuditEventSalesforceMergeDeliveryError
	AuditEventSalesforceMergeDeliveryRetry = entity.AuditEventSalesforceMergeDeliveryRetry
	AuditEventSalesforceConnectionChange = entity.AuditEventSalesforceConnectionChange
)

// Re-export AuditEvent from entity package
type AuditEvent = entity.AuditEvent

// AuditLogger handles security audit logging with database persistence
type AuditLogger struct {
	repo *repo.AuditRepo
}

// NewAuditLogger creates a new AuditLogger with database persistence
func NewAuditLogger(auditRepo *repo.AuditRepo) *AuditLogger {
	return &AuditLogger{
		repo: auditRepo,
	}
}

// WithDB returns a new AuditLogger with a different DB connection (for tenant switching)
func (a *AuditLogger) WithDB(auditRepo *repo.AuditRepo) *AuditLogger {
	return &AuditLogger{
		repo: auditRepo,
	}
}

// Log writes an audit event to stdout and persists to database
// SECURITY: All sensitive operations should be logged for forensic analysis
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	event.Timestamp = time.Now().UTC()

	// Serialize to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("[AUDIT ERROR] Failed to serialize audit event: %v", err)
		return
	}

	// Log to stdout in structured format (for backwards compatibility)
	log.Printf("[AUDIT] %s", string(eventJSON))

	// Persist to database if repo is available (fire-and-forget to avoid blocking)
	if a.repo != nil {
		go func() {
			ctx := context.Background()

			// Ensure table exists before persisting
			if err := a.repo.EnsureTableExists(ctx); err != nil {
				log.Printf("[AUDIT ERROR] Failed to ensure audit table: %v", err)
				return
			}

			// Convert AuditEvent to AuditLogEntry
			entry, err := repo.ConvertEventToEntry(event)
			if err != nil {
				log.Printf("[AUDIT ERROR] Failed to convert event: %v", err)
				return
			}

			// Persist to database
			if err := a.repo.Create(ctx, entry); err != nil {
				log.Printf("[AUDIT ERROR] Failed to persist audit log: %v", err)
			}
		}()
	}
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
	eventType := AuditEventLoginSuccess
	if !success {
		eventType = AuditEventLoginFailed
	}
	a.Log(ctx, AuditEvent{
		EventType:  eventType,
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

// LogLogout logs user logout
func (a *AuditLogger) LogLogout(ctx context.Context, userID, email, orgID, ipAddress, userAgent string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventLogout,
		ActorID:    userID,
		ActorEmail: email,
		OrgID:      orgID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    true,
	})
}

// LogUserCreate logs user creation
func (a *AuditLogger) LogUserCreate(ctx context.Context, actorID, actorEmail, targetID, targetEmail, orgID string, details map[string]interface{}) {
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventUserCreate,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		TargetID:    targetID,
		TargetEmail: targetEmail,
		OrgID:       orgID,
		Success:     true,
		Details:     details,
	})
}

// LogUserUpdate logs user updates
func (a *AuditLogger) LogUserUpdate(ctx context.Context, actorID, actorEmail, targetID, targetEmail, orgID string, details map[string]interface{}) {
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventUserUpdate,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		TargetID:    targetID,
		TargetEmail: targetEmail,
		OrgID:       orgID,
		Success:     true,
		Details:     details,
	})
}

// LogUserDelete logs user deletion
func (a *AuditLogger) LogUserDelete(ctx context.Context, actorID, actorEmail, targetID, targetEmail, orgID string, details map[string]interface{}) {
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventUserDelete,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		TargetID:    targetID,
		TargetEmail: targetEmail,
		OrgID:       orgID,
		Success:     true,
		Details:     details,
	})
}

// LogUserStatusChange logs user status changes (activate/deactivate)
func (a *AuditLogger) LogUserStatusChange(ctx context.Context, actorID, actorEmail, targetID, targetEmail, orgID string, isActive bool, ipAddress string) {
	status := "inactive"
	if isActive {
		status = "active"
	}
	a.Log(ctx, AuditEvent{
		EventType:   AuditEventUserStatusChange,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		TargetID:    targetID,
		TargetEmail: targetEmail,
		OrgID:       orgID,
		IPAddress:   ipAddress,
		Success:     true,
		Details: map[string]interface{}{
			"newStatus": status,
		},
	})
}

// LogPasswordChange logs password changes
func (a *AuditLogger) LogPasswordChange(ctx context.Context, userID, email, orgID, ipAddress string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventPasswordChange,
		ActorID:    userID,
		ActorEmail: email,
		OrgID:      orgID,
		IPAddress:  ipAddress,
		Success:    true,
	})
}

// LogOrgSettingsChange logs organization settings changes
func (a *AuditLogger) LogOrgSettingsChange(ctx context.Context, actorID, actorEmail, orgID string, changedFields []string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventOrgSettingsChange,
		ActorID:    actorID,
		ActorEmail: actorEmail,
		OrgID:      orgID,
		Success:    true,
		Details: map[string]interface{}{
			"changedFields": changedFields,
		},
	})
}

// LogAuthorizationDenied logs authorization failures
func (a *AuditLogger) LogAuthorizationDenied(ctx context.Context, actorID, actorEmail, orgID, path, method, ipAddress, userAgent string) {
	a.Log(ctx, AuditEvent{
		EventType:  AuditEventAuthorizationDenied,
		ActorID:    actorID,
		ActorEmail: actorEmail,
		OrgID:      orgID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    false,
		Details: map[string]interface{}{
			"path":   path,
			"method": method,
		},
	})
}

// LogRecordMerge logs a successful merge operation
func (a *AuditLogger) LogRecordMerge(ctx context.Context, actorID, orgID, entityType, survivorID, snapshotID string, duplicateIDs []string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventRecordMerge,
		ActorID:   actorID,
		TargetID:  survivorID,
		OrgID:     orgID,
		Success:   true,
		Details: map[string]interface{}{
			"entityType":   entityType,
			"survivorId":   survivorID,
			"duplicateIds": duplicateIDs,
			"snapshotId":   snapshotID,
		},
	})
}

// LogMergeUndo logs a merge undo operation
func (a *AuditLogger) LogMergeUndo(ctx context.Context, actorID, orgID, snapshotID, survivorID string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventMergeUndo,
		ActorID:   actorID,
		TargetID:  survivorID,
		OrgID:     orgID,
		Success:   true,
		Details: map[string]interface{}{
			"snapshotId": snapshotID,
			"survivorId": survivorID,
		},
	})
}

// LogSalesforceMergeDelivery logs a Salesforce merge delivery attempt with detailed metadata
func (a *AuditLogger) LogSalesforceMergeDelivery(ctx context.Context, orgID, batchID, instructionID, winnerID, loserID, deliveryStatus string, statusCode int, responseBody string, retryCount int, errorMsg string) {
	eventType := AuditEventSalesforceMergeDelivery
	success := true
	if deliveryStatus == "error" {
		eventType = AuditEventSalesforceMergeDeliveryError
		success = false
	} else if deliveryStatus == "retry" {
		eventType = AuditEventSalesforceMergeDeliveryRetry
		success = false
	}

	// Truncate responseBody to 1KB
	if len(responseBody) > 1024 {
		responseBody = responseBody[:1024] + "...[truncated]"
	}

	a.Log(ctx, AuditEvent{
		EventType: eventType,
		ActorID:   "system",
		OrgID:     orgID,
		Success:   success,
		ErrorMsg:  errorMsg,
		Details: map[string]interface{}{
			"batchId":        batchID,
			"instructionId":  instructionID,
			"winnerId":       winnerID,
			"loserId":        loserID,
			"deliveryStatus": deliveryStatus,
			"statusCode":     statusCode,
			"retryCount":     retryCount,
			"responseBody":   responseBody,
		},
	})
}
