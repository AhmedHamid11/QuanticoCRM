package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// TripwireService handles tripwire evaluation and webhook firing
type TripwireService struct {
	db           *sql.DB
	tripwireRepo *repo.TripwireRepo
	httpClient   *http.Client
}

// NewTripwireService creates a new TripwireService
func NewTripwireService(db *sql.DB, tripwireRepo *repo.TripwireRepo) *TripwireService {
	return &TripwireService{
		db:           db,
		tripwireRepo: tripwireRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Default timeout, overridden per-org
		},
	}
}

// EvaluateAndFire evaluates all tripwires for an entity and fires webhooks if conditions match
// This is called from the generic entity handler after Create/Update/Delete operations
func (s *TripwireService) EvaluateAndFire(
	ctx context.Context,
	orgID, entityType, recordID string,
	eventType string, // "CREATE", "UPDATE", "DELETE"
	oldRecord map[string]interface{}, // nil for CREATE
	newRecord map[string]interface{}, // nil for DELETE
) {
	// Get all enabled tripwires for this entity type
	tripwires, err := s.tripwireRepo.ListByEntityType(ctx, orgID, entityType, true)
	if err != nil {
		log.Printf("Error fetching tripwires for %s: %v", entityType, err)
		return
	}

	if len(tripwires) == 0 {
		return
	}

	// Get webhook settings for this org
	settings, err := s.tripwireRepo.GetWebhookSettings(ctx, orgID)
	if err != nil {
		log.Printf("Error fetching webhook settings for org %s: %v", orgID, err)
		settings = &entity.OrgWebhookSettings{
			AuthType:  entity.WebhookAuthNone,
			TimeoutMs: 5000,
		}
	}

	// Evaluate each tripwire
	for _, tw := range tripwires {
		if s.evaluateConditions(&tw, eventType, oldRecord, newRecord) {
			// Fire webhook in background (fire-and-forget)
			go s.fireWebhook(context.Background(), &tw, recordID, eventType, settings, orgID, entityType, oldRecord, newRecord)
		}
	}
}

// evaluateConditions evaluates all conditions based on the logic (AND/OR)
func (s *TripwireService) evaluateConditions(
	tw *entity.Tripwire,
	eventType string,
	oldRecord map[string]interface{},
	newRecord map[string]interface{},
) bool {
	if len(tw.Conditions) == 0 {
		return false
	}

	logic := strings.ToUpper(tw.ConditionLogic)
	if logic != "AND" && logic != "OR" {
		logic = "AND"
	}

	for _, cond := range tw.Conditions {
		result := s.evaluateCondition(cond, eventType, oldRecord, newRecord)

		if logic == "OR" && result {
			return true // OR: one true is enough
		}
		if logic == "AND" && !result {
			return false // AND: one false fails all
		}
	}

	// For AND: all were true; For OR: none were true
	return logic == "AND"
}

// evaluateCondition evaluates a single condition
func (s *TripwireService) evaluateCondition(
	cond entity.TripwireCondition,
	eventType string,
	oldRecord map[string]interface{},
	newRecord map[string]interface{},
) bool {
	switch cond.Type {
	case entity.ConditionIsNew:
		return eventType == "CREATE"

	case entity.ConditionIsDeleted:
		return eventType == "DELETE"

	case entity.ConditionIsChanged:
		if eventType != "UPDATE" || cond.FieldName == nil {
			return false
		}
		fieldName := *cond.FieldName
		oldVal := s.getFieldValue(oldRecord, fieldName)
		newVal := s.getFieldValue(newRecord, fieldName)
		return !s.valuesEqual(oldVal, newVal)

	case entity.ConditionFieldEquals:
		if cond.FieldName == nil || cond.Value == nil {
			return false
		}
		fieldName := *cond.FieldName
		expectedValue := *cond.Value

		// For CREATE/UPDATE, check the new record
		var record map[string]interface{}
		if eventType == "DELETE" {
			record = oldRecord
		} else {
			record = newRecord
		}

		if record == nil {
			return false
		}

		actualValue := s.getFieldValue(record, fieldName)
		return s.stringEquals(actualValue, expectedValue)

	case entity.ConditionFieldChanged:
		if eventType != "UPDATE" || cond.FieldName == nil {
			return false
		}
		fieldName := *cond.FieldName
		oldVal := s.getFieldValue(oldRecord, fieldName)
		newVal := s.getFieldValue(newRecord, fieldName)

		// Check FROM value if specified
		if cond.FromValue != nil {
			if !s.stringEquals(oldVal, *cond.FromValue) {
				return false
			}
		}

		// Check TO value if specified
		if cond.ToValue != nil {
			if !s.stringEquals(newVal, *cond.ToValue) {
				return false
			}
		}

		// If neither FROM nor TO specified, just check if value changed
		if cond.FromValue == nil && cond.ToValue == nil {
			return !s.valuesEqual(oldVal, newVal)
		}

		return true

	default:
		return false
	}
}

// getFieldValue extracts a field value from a record, handling both snake_case and camelCase
func (s *TripwireService) getFieldValue(record map[string]interface{}, fieldName string) interface{} {
	if record == nil {
		return nil
	}

	// Try exact match first
	if val, ok := record[fieldName]; ok {
		return val
	}

	// Try camelCase version
	camelCase := s.snakeToCamel(fieldName)
	if val, ok := record[camelCase]; ok {
		return val
	}

	// Try snake_case version
	snakeCase := s.camelToSnake(fieldName)
	if val, ok := record[snakeCase]; ok {
		return val
	}

	return nil
}

// stringEquals compares a value to a string
func (s *TripwireService) stringEquals(val interface{}, expected string) bool {
	if val == nil {
		return expected == "" || expected == "null"
	}

	switch v := val.(type) {
	case string:
		return v == expected
	case float64:
		return fmt.Sprintf("%v", v) == expected
	case int:
		return fmt.Sprintf("%d", v) == expected
	case int64:
		return fmt.Sprintf("%d", v) == expected
	case bool:
		return fmt.Sprintf("%t", v) == expected
	default:
		return fmt.Sprintf("%v", v) == expected
	}
}

// valuesEqual compares two values for equality
func (s *TripwireService) valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert both to strings for comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// camelToSnake converts camelCase to snake_case
func (s *TripwireService) camelToSnake(str string) string {
	var result strings.Builder
	for i, r := range str {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// snakeToCamel converts snake_case to camelCase
func (s *TripwireService) snakeToCamel(str string) string {
	parts := strings.Split(str, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// computeChanges compares old and new records and returns changed fields
func (s *TripwireService) computeChanges(oldRecord, newRecord map[string]interface{}) ([]string, map[string]*entity.FieldChange) {
	changedFields := []string{}
	changes := make(map[string]*entity.FieldChange)

	if oldRecord == nil || newRecord == nil {
		return changedFields, changes
	}

	// Check all fields in both old and new records
	allFields := make(map[string]bool)
	for k := range oldRecord {
		allFields[k] = true
	}
	for k := range newRecord {
		allFields[k] = true
	}

	// Skip internal/metadata fields that always change
	skipFields := map[string]bool{
		"modifiedAt": true,
		"modifiedBy": true,
		"updatedAt":  true,
		"updatedBy":  true,
	}

	for field := range allFields {
		if skipFields[field] {
			continue
		}

		oldVal := oldRecord[field]
		newVal := newRecord[field]

		if !s.valuesEqual(oldVal, newVal) {
			changedFields = append(changedFields, field)
			changes[field] = &entity.FieldChange{
				OldValue: oldVal,
				NewValue: newVal,
			}
		}
	}

	return changedFields, changes
}

// fireWebhook sends the HTTP POST request to the webhook endpoint
func (s *TripwireService) fireWebhook(
	ctx context.Context,
	tw *entity.Tripwire,
	recordID string,
	eventType string,
	settings *entity.OrgWebhookSettings,
	orgID string,
	entityType string,
	oldRecord map[string]interface{},
	newRecord map[string]interface{},
) {
	startTime := time.Now()

	// SECURITY: Validate webhook URL to prevent SSRF attacks
	if err := util.IsAllowedWebhookURL(tw.EndpointURL); err != nil {
		log.Printf("SECURITY: Blocked webhook to unsafe URL for tripwire %s: %v", tw.ID, err)
		s.logExecution(ctx, tw, recordID, entityType, eventType, "blocked", nil, "SSRF protection: "+err.Error(), nil, orgID)
		return
	}

	// Build payload
	payload := entity.WebhookPayload{
		TripwireID:   tw.ID,
		TripwireName: tw.Name,
		Event:        eventType,
		EntityType:   tw.EntityType,
		RecordID:     recordID,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	// Set the record data based on event type
	switch eventType {
	case "CREATE":
		payload.Record = newRecord
	case "UPDATE":
		payload.Record = newRecord
		// Calculate changed fields
		payload.ChangedFields, payload.Changes = s.computeChanges(oldRecord, newRecord)
	case "DELETE":
		payload.Record = oldRecord
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling webhook payload: %v", err)
		s.logExecution(ctx, tw, recordID, entityType, eventType, "failed", nil, err.Error(), nil, orgID)
		return
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tw.EndpointURL, bytes.NewBuffer(payloadJSON))
	if err != nil {
		log.Printf("Error creating webhook request: %v", err)
		s.logExecution(ctx, tw, recordID, entityType, eventType, "failed", nil, err.Error(), nil, orgID)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tripwire-ID", tw.ID)
	req.Header.Set("X-Event-Type", eventType)

	// Add authentication headers
	s.addAuthHeaders(req, settings)

	// Create a client with the org-specific timeout
	timeout := time.Duration(settings.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Send request
	resp, err := client.Do(req)
	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	if err != nil {
		status := "failed"
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			status = "timeout"
		}
		log.Printf("Webhook request failed for tripwire %s: %v", tw.ID, err)
		s.logExecution(ctx, tw, recordID, entityType, eventType, status, nil, err.Error(), &durationMs, orgID)
		return
	}
	defer resp.Body.Close()

	// Log execution
	status := "success"
	var errMsg string
	if resp.StatusCode >= 400 {
		status = "failed"
		errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	s.logExecution(ctx, tw, recordID, entityType, eventType, status, &resp.StatusCode, errMsg, &durationMs, orgID)
}

// addAuthHeaders adds authentication headers based on org settings
func (s *TripwireService) addAuthHeaders(req *http.Request, settings *entity.OrgWebhookSettings) {
	switch settings.AuthType {
	case entity.WebhookAuthAPIKey:
		if settings.APIKey != nil {
			req.Header.Set("X-API-Key", *settings.APIKey)
		}
	case entity.WebhookAuthBearer:
		if settings.BearerToken != nil {
			req.Header.Set("Authorization", "Bearer "+*settings.BearerToken)
		}
	case entity.WebhookAuthCustomHeader:
		if settings.CustomHeaderName != nil && settings.CustomHeaderValue != nil {
			req.Header.Set(*settings.CustomHeaderName, *settings.CustomHeaderValue)
		}
	}
}

// logExecution logs the tripwire execution result
func (s *TripwireService) logExecution(
	ctx context.Context,
	tw *entity.Tripwire,
	recordID string,
	entityType string,
	eventType string,
	status string,
	responseCode *int,
	errorMessage string,
	durationMs *int,
	orgID string,
) {
	log := &entity.TripwireLog{
		ID:           sfid.NewTripwireLog(),
		TripwireID:   tw.ID,
		TripwireName: &tw.Name,
		OrgID:        orgID,
		RecordID:     recordID,
		EntityType:   entityType,
		EventType:    eventType,
		Status:       status,
		ResponseCode: responseCode,
		DurationMs:   durationMs,
		ExecutedAt:   time.Now().UTC(),
	}

	if errorMessage != "" {
		log.ErrorMessage = &errorMessage
	}

	if err := s.tripwireRepo.CreateLog(ctx, log); err != nil {
		// Just log the error, don't fail the main operation
		fmt.Printf("Failed to create tripwire log: %v\n", err)
	}
}
