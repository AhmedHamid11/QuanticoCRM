package flow

import (
	"context"
	"fmt"
)

// =============================================================================
// External Service Interfaces
// =============================================================================
// These interfaces define what the flow engine needs from external services.
// This allows the flow module to remain decoupled and easily removable.

// EntityService provides CRUD operations on CRM entities
// Implement this interface to connect the flow engine to your entity layer
type EntityService interface {
	// Create creates a new record and returns it with ID populated
	Create(ctx context.Context, entityType, orgID string, data map[string]interface{}) (map[string]interface{}, error)

	// Update updates an existing record
	Update(ctx context.Context, entityType, orgID, recordID string, data map[string]interface{}) (map[string]interface{}, error)

	// Get retrieves a single record by ID
	Get(ctx context.Context, entityType, orgID, recordID string) (map[string]interface{}, error)

	// Delete soft-deletes a record
	Delete(ctx context.Context, entityType, orgID, recordID string) error
}

// WebhookService handles outbound webhook calls (e.g., to n8n)
type WebhookService interface {
	// Fire sends a webhook request
	// If async is true, fires in background and returns immediately
	Fire(ctx context.Context, webhookName string, payload map[string]interface{}, async bool) (map[string]interface{}, error)
}

// FlowRepository handles persistence of flow definitions and executions
type FlowRepository interface {
	// Flow definitions
	GetFlow(ctx context.Context, flowID, orgID string) (*FlowDefinitionDB, error)
	ListFlows(ctx context.Context, orgID string, params FlowListParams) (*FlowListResponse, error)
	CreateFlow(ctx context.Context, flow *FlowDefinitionDB) error
	UpdateFlow(ctx context.Context, flow *FlowDefinitionDB) error
	DeleteFlow(ctx context.Context, flowID, orgID string) error

	// Flow executions
	GetExecution(ctx context.Context, executionID string) (*FlowExecutionDB, error)
	SaveExecution(ctx context.Context, exec *FlowExecutionDB) error
	ListExecutions(ctx context.Context, orgID string, flowID *string, status *string, limit int) ([]FlowExecutionDB, error)
}

// =============================================================================
// Mock Implementations for Testing
// =============================================================================

// MockEntityService is a simple in-memory entity service for testing
type MockEntityService struct {
	Records map[string]map[string]map[string]interface{} // entityType -> recordID -> data
	Counter int
}

func NewMockEntityService() *MockEntityService {
	return &MockEntityService{
		Records: make(map[string]map[string]map[string]interface{}),
		Counter: 0,
	}
}

func (m *MockEntityService) Create(ctx context.Context, entityType, orgID string, data map[string]interface{}) (map[string]interface{}, error) {
	m.Counter++
	id := fmt.Sprintf("mock_%s_%d", entityType, m.Counter)

	if m.Records[entityType] == nil {
		m.Records[entityType] = make(map[string]map[string]interface{})
	}

	record := make(map[string]interface{})
	for k, v := range data {
		record[k] = v
	}
	record["id"] = id
	record["org_id"] = orgID

	m.Records[entityType][id] = record
	return record, nil
}

func (m *MockEntityService) Update(ctx context.Context, entityType, orgID, recordID string, data map[string]interface{}) (map[string]interface{}, error) {
	if m.Records[entityType] == nil {
		return nil, fmt.Errorf("entity type not found: %s", entityType)
	}

	record, ok := m.Records[entityType][recordID]
	if !ok {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	for k, v := range data {
		record[k] = v
	}

	return record, nil
}

func (m *MockEntityService) Get(ctx context.Context, entityType, orgID, recordID string) (map[string]interface{}, error) {
	if m.Records[entityType] == nil {
		return nil, fmt.Errorf("entity type not found: %s", entityType)
	}

	record, ok := m.Records[entityType][recordID]
	if !ok {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	return record, nil
}

func (m *MockEntityService) Delete(ctx context.Context, entityType, orgID, recordID string) error {
	if m.Records[entityType] == nil {
		return fmt.Errorf("entity type not found: %s", entityType)
	}

	delete(m.Records[entityType], recordID)
	return nil
}

// MockWebhookService captures webhook calls for testing
type MockWebhookService struct {
	Calls []WebhookCall
}

type WebhookCall struct {
	WebhookName string
	Payload     map[string]interface{}
	Async       bool
}

func NewMockWebhookService() *MockWebhookService {
	return &MockWebhookService{
		Calls: make([]WebhookCall, 0),
	}
}

func (m *MockWebhookService) Fire(ctx context.Context, webhookName string, payload map[string]interface{}, async bool) (map[string]interface{}, error) {
	m.Calls = append(m.Calls, WebhookCall{
		WebhookName: webhookName,
		Payload:     payload,
		Async:       async,
	})
	return map[string]interface{}{"status": "ok"}, nil
}

