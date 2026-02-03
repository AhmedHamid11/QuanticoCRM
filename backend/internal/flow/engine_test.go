package flow

import (
	"context"
	"encoding/json"
	"testing"
)

// parseFlowJSON is a helper that parses JSON into a FlowDefinition
func parseFlowJSON(jsonStr string) *FlowDefinition {
	var flow FlowDefinition
	json.Unmarshal([]byte(jsonStr), &flow)
	return &flow
}

// Full flow JSON for testing (simulates what would be stored in DB)
func createFullTestFlow() *FlowDefinition {
	flowJSON := `{
		"id": "lead-qual-test",
		"name": "Lead Qualification Test",
		"version": 1,
		"trigger": {"type": "manual", "entityType": "Lead"},
		"variables": {
			"leadScore": {"type": "number", "default": 0}
		},
		"steps": [
			{
				"id": "screen1",
				"type": "screen",
				"name": "Initial Assessment",
				"fields": [
					{"name": "companySize", "label": "Company Size", "type": "select", "required": true,
					 "options": [
						{"value": "1-10", "label": "1-10"},
						{"value": "11-50", "label": "11-50"},
						{"value": "51-200", "label": "51-200"},
						{"value": "200+", "label": "200+"}
					 ]},
					{"name": "budget", "label": "Budget", "type": "number", "required": true}
				],
				"next": "calculate"
			},
			{
				"id": "calculate",
				"type": "assignment",
				"name": "Calculate Score",
				"assignments": [
					{"variable": "leadScore", "expression": "CASE(companySize, '200+', 40, '51-200', 30, '11-50', 20, 10) + IF(budget > 50000, 20, IF(budget > 10000, 10, 0))"}
				],
				"next": "decision"
			},
			{
				"id": "decision",
				"type": "decision",
				"name": "Route by Score",
				"rules": [
					{"condition": "leadScore >= 50", "next": "hot_screen"}
				],
				"defaultNext": "cold_screen"
			},
			{
				"id": "hot_screen",
				"type": "screen",
				"name": "Hot Lead Actions",
				"header": {"type": "alert", "variant": "success", "message": "Hot lead! Score: {{leadScore}}"},
				"fields": [
					{"name": "assignedRep", "label": "Assign To", "type": "text", "required": true}
				],
				"next": "create_opp"
			},
			{
				"id": "cold_screen",
				"type": "screen",
				"name": "Cold Lead Actions",
				"header": {"type": "alert", "variant": "warning", "message": "Cold lead. Score: {{leadScore}}"},
				"fields": [
					{"name": "reason", "label": "Archive Reason", "type": "text", "required": true}
				],
				"next": "update_lead"
			},
			{
				"id": "create_opp",
				"type": "record_create",
				"name": "Create Opportunity",
				"entity": "Opportunity",
				"fieldMapping": {
					"name": "'Opportunity - ' + $record.name",
					"amount": "budget",
					"assigned_user_id": "assignedRep"
				},
				"outputVariable": "newOpp",
				"next": "notify"
			},
			{
				"id": "update_lead",
				"type": "record_update",
				"name": "Archive Lead",
				"entity": "Lead",
				"recordId": "$record.id",
				"fieldMapping": {
					"status": "'archived'",
					"archive_reason": "reason"
				},
				"next": "end_cold"
			},
			{
				"id": "notify",
				"type": "n8n_webhook",
				"name": "Notify Sales Rep",
				"webhook": "hot-lead-assigned",
				"payload": {
					"leadId": "$record.id",
					"oppId": "newOpp.id",
					"score": "leadScore"
				},
				"async": true,
				"next": "end_hot"
			},
			{
				"id": "end_hot",
				"type": "end",
				"name": "Completed (Hot)",
				"message": "Lead qualified! Opportunity created with score {{leadScore}}.",
				"redirect": {"entity": "Opportunity", "recordId": "newOpp.id"}
			},
			{
				"id": "end_cold",
				"type": "end",
				"name": "Completed (Cold)",
				"message": "Lead archived with reason: {{reason}}"
			}
		]
	}`

	var flow FlowDefinition
	json.Unmarshal([]byte(flowJSON), &flow)
	return &flow
}

func TestEngine_StartFlow(t *testing.T) {
	entityService := NewMockEntityService()
	webhookService := NewMockWebhookService()
	engine := NewEngineWithoutRepo(entityService, webhookService)

	flow := createFullTestFlow()
	record := map[string]interface{}{
		"id":    "lead-123",
		"name":  "Acme Corp",
		"email": "contact@acme.com",
	}

	exec, err := engine.StartFlow(context.Background(), flow, "org-1", "user-1", record)
	if err != nil {
		t.Fatalf("StartFlow error: %v", err)
	}

	// Should pause at first screen
	if exec.Status != StatusPausedAtScreen {
		t.Errorf("Expected status %s, got %s", StatusPausedAtScreen, exec.Status)
	}

	if exec.CurrentStep != "screen1" {
		t.Errorf("Expected current step screen1, got %s", exec.CurrentStep)
	}

	if exec.ScreenDef == nil {
		t.Error("Expected ScreenDef to be set")
	}

	if len(exec.ScreenDef.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(exec.ScreenDef.Fields))
	}
}

func TestEngine_HotLeadPath(t *testing.T) {
	entityService := NewMockEntityService()
	webhookService := NewMockWebhookService()
	engine := NewEngineWithoutRepo(entityService, webhookService)

	flow := createFullTestFlow()
	record := map[string]interface{}{
		"id":   "lead-123",
		"name": "Big Corp",
	}

	// Start flow
	exec, err := engine.StartFlow(context.Background(), flow, "org-1", "user-1", record)
	if err != nil {
		t.Fatalf("StartFlow error: %v", err)
	}

	// Submit first screen with high-value data (should score >= 50)
	// 200+ = 40 points, budget > 50000 = 20 points = 60 total
	exec, err = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
		"companySize": "200+",
		"budget":      float64(75000),
	}, flow)
	if err != nil {
		t.Fatalf("ResumeFlow error: %v", err)
	}

	// Should be at hot_screen now
	if exec.Status != StatusPausedAtScreen {
		t.Errorf("Expected status %s, got %s", StatusPausedAtScreen, exec.Status)
	}
	if exec.CurrentStep != "hot_screen" {
		t.Errorf("Expected hot_screen, got %s", exec.CurrentStep)
	}

	// Check lead score was calculated
	if exec.Variables["leadScore"] != float64(60) {
		t.Errorf("Expected leadScore 60, got %v", exec.Variables["leadScore"])
	}

	// Check header message was interpolated
	if exec.ScreenDef.Header == nil {
		t.Error("Expected header to be set")
	} else if exec.ScreenDef.Header.Message != "Hot lead! Score: 60" {
		t.Errorf("Expected interpolated message, got: %s", exec.ScreenDef.Header.Message)
	}

	// Submit hot lead screen
	exec, err = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
		"assignedRep": "rep-456",
	}, flow)
	if err != nil {
		t.Fatalf("ResumeFlow error: %v", err)
	}

	// Should be completed now
	if exec.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, exec.Status)
	}

	// Check opportunity was created
	if len(entityService.Records["Opportunity"]) != 1 {
		t.Errorf("Expected 1 Opportunity created, got %d", len(entityService.Records["Opportunity"]))
	}

	// Check webhook was fired
	if len(webhookService.Calls) != 1 {
		t.Errorf("Expected 1 webhook call, got %d", len(webhookService.Calls))
	}
	if webhookService.Calls[0].WebhookName != "hot-lead-assigned" {
		t.Errorf("Expected webhook 'hot-lead-assigned', got %s", webhookService.Calls[0].WebhookName)
	}

	// Check end message
	if exec.EndMessage != "Lead qualified! Opportunity created with score 60." {
		t.Errorf("Unexpected end message: %s", exec.EndMessage)
	}

	// Check redirect
	if exec.Redirect == nil {
		t.Error("Expected redirect to be set")
	} else if exec.Redirect.Entity != "Opportunity" {
		t.Errorf("Expected redirect to Opportunity, got %s", exec.Redirect.Entity)
	}
}

func TestEngine_ColdLeadPath(t *testing.T) {
	entityService := NewMockEntityService()
	webhookService := NewMockWebhookService()
	engine := NewEngineWithoutRepo(entityService, webhookService)

	flow := createFullTestFlow()
	record := map[string]interface{}{
		"id":   "lead-789",
		"name": "Small Shop",
	}

	// Pre-seed the lead in the mock entity service (so update_lead step can find it)
	entityService.Records["Lead"] = map[string]map[string]interface{}{
		"lead-789": {
			"id":     "lead-789",
			"name":   "Small Shop",
			"status": "new",
		},
	}

	// Start flow
	exec, err := engine.StartFlow(context.Background(), flow, "org-1", "user-1", record)
	if err != nil {
		t.Fatalf("StartFlow error: %v", err)
	}

	// Submit first screen with low-value data (should score < 50)
	// 1-10 = 10 points, budget < 10000 = 0 points = 10 total
	exec, err = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
		"companySize": "1-10",
		"budget":      float64(5000),
	}, flow)
	if err != nil {
		t.Fatalf("ResumeFlow error: %v", err)
	}

	// Should be at cold_screen now
	if exec.Status != StatusPausedAtScreen {
		t.Errorf("Expected status %s, got %s", StatusPausedAtScreen, exec.Status)
	}
	if exec.CurrentStep != "cold_screen" {
		t.Errorf("Expected cold_screen, got %s", exec.CurrentStep)
	}

	// Check lead score
	if exec.Variables["leadScore"] != float64(10) {
		t.Errorf("Expected leadScore 10, got %v", exec.Variables["leadScore"])
	}

	// Submit cold lead screen
	exec, err = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
		"reason": "No budget",
	}, flow)
	if err != nil {
		t.Fatalf("ResumeFlow error: %v", err)
	}

	// Should be completed
	if exec.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, exec.Status)
	}

	// No opportunity should be created
	if len(entityService.Records["Opportunity"]) != 0 {
		t.Errorf("Expected no Opportunity, got %d", len(entityService.Records["Opportunity"]))
	}

	// No webhook should be called
	if len(webhookService.Calls) != 0 {
		t.Errorf("Expected no webhook calls, got %d", len(webhookService.Calls))
	}

	// Check end message
	if exec.EndMessage != "Lead archived with reason: No budget" {
		t.Errorf("Unexpected end message: %s", exec.EndMessage)
	}

	// No redirect for cold path
	if exec.Redirect != nil {
		t.Error("Expected no redirect for cold path")
	}
}

func TestEngine_DecisionBranching(t *testing.T) {
	// Test a simple decision flow
	flowJSON := `{
		"id": "decision-test",
		"name": "Decision Test",
		"version": 1,
		"trigger": {"type": "manual"},
		"variables": {},
		"steps": [
			{
				"id": "screen1",
				"type": "screen",
				"name": "Input",
				"fields": [{"name": "value", "label": "Value", "type": "number"}],
				"next": "decide"
			},
			{
				"id": "decide",
				"type": "decision",
				"name": "Check Value",
				"rules": [
					{"condition": "value >= 100", "next": "high"},
					{"condition": "value >= 50", "next": "medium"}
				],
				"defaultNext": "low"
			},
			{"id": "high", "type": "end", "name": "High", "message": "High value"},
			{"id": "medium", "type": "end", "name": "Medium", "message": "Medium value"},
			{"id": "low", "type": "end", "name": "Low", "message": "Low value"}
		]
	}`

	var flow FlowDefinition
	json.Unmarshal([]byte(flowJSON), &flow)

	engine := NewEngineWithoutRepo(nil, nil)

	tests := []struct {
		name    string
		value   float64
		wantMsg string
	}{
		{"high value", 150, "High value"},
		{"medium value", 75, "Medium value"},
		{"low value", 25, "Low value"},
		{"boundary high", 100, "High value"},
		{"boundary medium", 50, "Medium value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, _ := engine.StartFlow(context.Background(), &flow, "org", "user", nil)
			exec, _ = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
				"value": tt.value,
			}, &flow)

			if exec.Status != StatusCompleted {
				t.Errorf("Expected completed, got %s", exec.Status)
			}
			if exec.EndMessage != tt.wantMsg {
				t.Errorf("Expected '%s', got '%s'", tt.wantMsg, exec.EndMessage)
			}
		})
	}
}

func TestEngine_RecordOperations(t *testing.T) {
	entityService := NewMockEntityService()
	engine := NewEngineWithoutRepo(entityService, nil)

	flowJSON := `{
		"id": "crud-test",
		"name": "CRUD Test",
		"version": 1,
		"trigger": {"type": "manual"},
		"variables": {},
		"steps": [
			{
				"id": "screen1",
				"type": "screen",
				"name": "Input",
				"fields": [{"name": "contactName", "label": "Name", "type": "text"}],
				"next": "create"
			},
			{
				"id": "create",
				"type": "record_create",
				"name": "Create Contact",
				"entity": "Contact",
				"fieldMapping": {
					"name": "contactName",
					"source": "'flow'"
				},
				"outputVariable": "newContact",
				"next": "update"
			},
			{
				"id": "update",
				"type": "record_update",
				"name": "Update Contact",
				"entity": "Contact",
				"recordId": "newContact.id",
				"fieldMapping": {
					"status": "'verified'"
				},
				"next": "end"
			},
			{
				"id": "end",
				"type": "end",
				"name": "Done",
				"message": "Created contact {{newContact.id}}"
			}
		]
	}`

	var flow FlowDefinition
	json.Unmarshal([]byte(flowJSON), &flow)

	exec, _ := engine.StartFlow(context.Background(), &flow, "org-1", "user-1", nil)
	exec, err := engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{
		"contactName": "Jane Doe",
	}, &flow)

	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	if exec.Status != StatusCompleted {
		t.Errorf("Expected completed, got %s (error: %s)", exec.Status, exec.Error)
	}

	// Check contact was created
	if len(entityService.Records["Contact"]) != 1 {
		t.Errorf("Expected 1 Contact, got %d", len(entityService.Records["Contact"]))
	}

	// Check contact was updated
	for _, contact := range entityService.Records["Contact"] {
		if contact["status"] != "verified" {
			t.Errorf("Expected status 'verified', got %v", contact["status"])
		}
		if contact["name"] != "Jane Doe" {
			t.Errorf("Expected name 'Jane Doe', got %v", contact["name"])
		}
		if contact["source"] != "flow" {
			t.Errorf("Expected source 'flow', got %v", contact["source"])
		}
	}
}

func TestEngine_GetExecution(t *testing.T) {
	engine := NewEngineWithoutRepo(nil, nil)

	flowJSON := `{
		"id": "simple",
		"name": "Simple",
		"version": 1,
		"trigger": {"type": "manual"},
		"variables": {},
		"steps": [
			{"id": "s1", "type": "screen", "name": "S1", "fields": [{"name": "x", "label": "X", "type": "text"}], "next": "end"},
			{"id": "end", "type": "end", "name": "End", "message": "Done"}
		]
	}`

	var flow FlowDefinition
	json.Unmarshal([]byte(flowJSON), &flow)

	exec, _ := engine.StartFlow(context.Background(), &flow, "org", "user", nil)

	// Should be able to retrieve it
	retrieved, err := engine.GetExecution(exec.ID)
	if err != nil {
		t.Fatalf("GetExecution error: %v", err)
	}

	if retrieved.ID != exec.ID {
		t.Errorf("Expected ID %s, got %s", exec.ID, retrieved.ID)
	}

	// Non-existent execution
	_, err = engine.GetExecution("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent execution")
	}
}

func TestEngine_InvalidResume(t *testing.T) {
	engine := NewEngineWithoutRepo(nil, nil)

	flowJSON := `{
		"id": "simple",
		"name": "Simple",
		"version": 1,
		"trigger": {"type": "manual"},
		"variables": {},
		"steps": [
			{"id": "s1", "type": "screen", "name": "S1", "fields": [{"name": "x", "label": "X", "type": "text"}], "next": "end"},
			{"id": "end", "type": "end", "name": "End", "message": "Done"}
		]
	}`

	var flow FlowDefinition
	json.Unmarshal([]byte(flowJSON), &flow)

	exec, _ := engine.StartFlow(context.Background(), &flow, "org", "user", nil)

	// Complete the flow
	exec, _ = engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{"x": "test"}, &flow)

	if exec.Status != StatusCompleted {
		t.Fatalf("Expected completed, got %s", exec.Status)
	}

	// Try to resume completed flow
	_, err := engine.ResumeFlow(context.Background(), exec.ID, map[string]interface{}{"x": "test2"}, &flow)
	if err == nil {
		t.Error("Expected error when resuming completed flow")
	}
}
