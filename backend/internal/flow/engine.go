package flow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine orchestrates flow execution
type Engine struct {
	repo           FlowRepository
	entityService  EntityService
	webhookService WebhookService
	exprParser     *ExpressionParser

	// In-memory execution cache for active flows
	// In production, you might use Redis for distributed state
	executions sync.Map
}

// NewEngine creates a new flow engine
func NewEngine(repo FlowRepository, entityService EntityService, webhookService WebhookService) *Engine {
	return &Engine{
		repo:           repo,
		entityService:  entityService,
		webhookService: webhookService,
		exprParser:     NewExpressionParser(),
	}
}

// NewEngineWithoutRepo creates an engine for testing without persistence
func NewEngineWithoutRepo(entityService EntityService, webhookService WebhookService) *Engine {
	return &Engine{
		entityService:  entityService,
		webhookService: webhookService,
		exprParser:     NewExpressionParser(),
	}
}

// =============================================================================
// Flow Lifecycle
// =============================================================================

// StartFlow begins execution of a flow
func (e *Engine) StartFlow(ctx context.Context, flow *FlowDefinition, orgID, userID string, record map[string]interface{}) (*FlowExecution, error) {
	firstStepID := flow.GetFirstStepID()
	if firstStepID == "" {
		return nil, fmt.Errorf("flow has no steps")
	}

	// Create execution state
	exec := &FlowExecution{
		ID:          uuid.NewString(),
		FlowID:      flow.ID,
		FlowName:    flow.Name,
		FlowVersion: flow.Version,
		OrgID:       orgID,
		UserID:      userID,
		Status:      StatusRunning,
		CurrentStep: firstStepID,
		Variables:   make(map[string]interface{}),
		Record:      record,
		ScreenData:  make(map[string]interface{}),
		StartedAt:   time.Now(),
	}

	// Initialize variables with defaults
	for name, v := range flow.Variables {
		exec.Variables[name] = v.Default
	}

	// Store in cache
	e.executions.Store(exec.ID, exec)

	// Run until we need to pause (screen) or complete
	return e.runFlow(ctx, exec, flow)
}

// ResumeFlow continues execution after user submits screen data
func (e *Engine) ResumeFlow(ctx context.Context, executionID string, screenData map[string]interface{}, flow *FlowDefinition) (*FlowExecution, error) {
	exec, err := e.GetExecution(executionID)
	if err != nil {
		return nil, err
	}

	if exec.Status != StatusPausedAtScreen {
		return nil, fmt.Errorf("flow is not paused at screen, status: %s", exec.Status)
	}

	// Merge screen data into variables and screenData
	for k, v := range screenData {
		exec.ScreenData[k] = v
		exec.Variables[k] = v
	}

	// Find current step and get next step ID
	currentStep := flow.GetStepBase(exec.CurrentStep)
	if currentStep == nil {
		return nil, fmt.Errorf("current step not found: %s", exec.CurrentStep)
	}

	// Move to next step
	exec.CurrentStep = currentStep.Next
	exec.Status = StatusRunning
	exec.ScreenDef = nil // Clear screen definition

	// Continue execution
	return e.runFlow(ctx, exec, flow)
}

// GetExecution retrieves an execution by ID
func (e *Engine) GetExecution(executionID string) (*FlowExecution, error) {
	execI, ok := e.executions.Load(executionID)
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}
	return execI.(*FlowExecution), nil
}

// =============================================================================
// Flow Execution Loop
// =============================================================================

// runFlow executes steps until pause or completion
func (e *Engine) runFlow(ctx context.Context, exec *FlowExecution, flow *FlowDefinition) (*FlowExecution, error) {
	maxIterations := 100 // Safety limit to prevent infinite loops

	for i := 0; i < maxIterations && exec.Status == StatusRunning; i++ {
		stepDef := flow.GetStep(exec.CurrentStep)
		if stepDef == nil {
			exec.Status = StatusFailed
			exec.Error = fmt.Sprintf("step not found: %s", exec.CurrentStep)
			break
		}

		result, err := e.executeStep(ctx, stepDef, exec)
		if err != nil {
			exec.Status = StatusFailed
			exec.Error = err.Error()
			break
		}

		switch result.Action {
		case ActionContinue:
			exec.CurrentStep = result.NextStep
		case ActionPauseForScreen:
			exec.Status = StatusPausedAtScreen
			exec.ScreenDef = result.ScreenDef
		case ActionComplete:
			exec.Status = StatusCompleted
			exec.EndMessage = result.EndMessage
			exec.Redirect = result.Redirect
			now := time.Now()
			exec.CompletedAt = &now
		}
	}

	// Update cache
	e.executions.Store(exec.ID, exec)

	return exec, nil
}

// =============================================================================
// Step Execution
// =============================================================================

// executeStep runs a single step and returns the result
func (e *Engine) executeStep(ctx context.Context, stepData interface{}, exec *FlowExecution) (*StepResult, error) {
	// Create expression context
	exprCtx := NewContext(exec)

	switch step := stepData.(type) {
	case *ScreenStep:
		return e.executeScreen(ctx, step, exec, exprCtx)
	case *DecisionStep:
		return e.executeDecision(ctx, step, exec, exprCtx)
	case *AssignmentStep:
		return e.executeAssignment(ctx, step, exec, exprCtx)
	case *RecordCreateStep:
		return e.executeRecordCreate(ctx, step, exec, exprCtx)
	case *RecordUpdateStep:
		return e.executeRecordUpdate(ctx, step, exec, exprCtx)
	case *RecordGetStep:
		return e.executeRecordGet(ctx, step, exec, exprCtx)
	case *WebhookStep:
		return e.executeWebhook(ctx, step, exec, exprCtx)
	case *EndStep:
		return e.executeEnd(ctx, step, exec, exprCtx)
	default:
		return nil, fmt.Errorf("unknown step type: %T", stepData)
	}
}

// executeScreen prepares a screen for display
func (e *Engine) executeScreen(ctx context.Context, step *ScreenStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	// Create a copy to avoid modifying the original
	screenCopy := *step

	// Interpolate header message if present
	if screenCopy.Header != nil {
		headerCopy := *screenCopy.Header
		headerCopy.Message = e.exprParser.InterpolateString(headerCopy.Message, exprCtx)
		screenCopy.Header = &headerCopy
	}

	// Resolve default values for fields
	fieldsCopy := make([]ScreenField, len(step.Fields))
	for i, field := range step.Fields {
		fieldsCopy[i] = field
		if field.DefaultValue != "" {
			val, err := e.exprParser.Evaluate(field.DefaultValue, exprCtx)
			if err == nil && val != nil {
				fieldsCopy[i].DefaultValue = fmt.Sprintf("%v", val)
			}
		}
	}
	screenCopy.Fields = fieldsCopy

	return &StepResult{
		Action:    ActionPauseForScreen,
		ScreenDef: &screenCopy,
	}, nil
}

// executeDecision evaluates conditions and determines next step
func (e *Engine) executeDecision(ctx context.Context, step *DecisionStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	for _, rule := range step.Rules {
		result, err := e.exprParser.EvaluateBool(rule.Condition, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate condition '%s': %w", rule.Condition, err)
		}
		if result {
			return &StepResult{
				Action:   ActionContinue,
				NextStep: rule.Next,
			}, nil
		}
	}

	// No rule matched, use default
	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.DefaultNext,
	}, nil
}

// executeAssignment sets variable values
func (e *Engine) executeAssignment(ctx context.Context, step *AssignmentStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	for _, assignment := range step.Assignments {
		value, err := e.exprParser.Evaluate(assignment.Expression, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression for %s: %w", assignment.Variable, err)
		}
		exec.Variables[assignment.Variable] = value
		// Update context for subsequent assignments in same step
		exprCtx.Variables[assignment.Variable] = value
	}

	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.Next,
	}, nil
}

// executeRecordCreate creates a new record
func (e *Engine) executeRecordCreate(ctx context.Context, step *RecordCreateStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	if e.entityService == nil {
		return nil, fmt.Errorf("entity service not configured")
	}

	// Resolve field mappings
	data := make(map[string]interface{})
	for field, expr := range step.FieldMapping {
		value, err := e.exprParser.Evaluate(expr, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate field %s: %w", field, err)
		}
		data[field] = value
	}

	// Create record
	record, err := e.entityService.Create(ctx, step.Entity, exec.OrgID, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", step.Entity, err)
	}

	// Store in output variable if specified
	if step.OutputVariable != "" {
		exec.Variables[step.OutputVariable] = record
		exprCtx.Variables[step.OutputVariable] = record
	}

	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.Next,
	}, nil
}

// executeRecordUpdate updates an existing record
func (e *Engine) executeRecordUpdate(ctx context.Context, step *RecordUpdateStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	if e.entityService == nil {
		return nil, fmt.Errorf("entity service not configured")
	}

	// Resolve record ID
	recordIDVal, err := e.exprParser.Evaluate(step.RecordID, exprCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve record ID: %w", err)
	}
	recordID := fmt.Sprintf("%v", recordIDVal)

	// Resolve field mappings
	data := make(map[string]interface{})
	for field, expr := range step.FieldMapping {
		value, err := e.exprParser.Evaluate(expr, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate field %s: %w", field, err)
		}
		data[field] = value
	}

	// Update record
	_, err = e.entityService.Update(ctx, step.Entity, exec.OrgID, recordID, data)
	if err != nil {
		return nil, fmt.Errorf("failed to update %s: %w", step.Entity, err)
	}

	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.Next,
	}, nil
}

// executeRecordGet fetches a record
func (e *Engine) executeRecordGet(ctx context.Context, step *RecordGetStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	if e.entityService == nil {
		return nil, fmt.Errorf("entity service not configured")
	}

	// Resolve record ID
	recordIDVal, err := e.exprParser.Evaluate(step.RecordID, exprCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve record ID: %w", err)
	}
	recordID := fmt.Sprintf("%v", recordIDVal)

	// Get record
	record, err := e.entityService.Get(ctx, step.Entity, exec.OrgID, recordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", step.Entity, err)
	}

	// Store in output variable if specified
	if step.OutputVariable != "" {
		exec.Variables[step.OutputVariable] = record
		exprCtx.Variables[step.OutputVariable] = record
	}

	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.Next,
	}, nil
}

// executeWebhook calls an external webhook
func (e *Engine) executeWebhook(ctx context.Context, step *WebhookStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	if e.webhookService == nil {
		// If no webhook service, just skip and continue
		return &StepResult{
			Action:   ActionContinue,
			NextStep: step.Next,
		}, nil
	}

	// Resolve payload
	payload := make(map[string]interface{})
	for key, expr := range step.Payload {
		value, err := e.exprParser.Evaluate(expr, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate payload %s: %w", key, err)
		}
		payload[key] = value
	}

	// Add execution context
	payload["_flowId"] = exec.FlowID
	payload["_executionId"] = exec.ID
	payload["_orgId"] = exec.OrgID

	// Fire webhook
	response, err := e.webhookService.Fire(ctx, step.Webhook, payload, step.Async)
	if err != nil && !step.Async {
		return nil, fmt.Errorf("webhook failed: %w", err)
	}

	// Store response if sync and output variable specified
	if !step.Async && step.OutputVariable != "" && response != nil {
		exec.Variables[step.OutputVariable] = response
		exprCtx.Variables[step.OutputVariable] = response
	}

	return &StepResult{
		Action:   ActionContinue,
		NextStep: step.Next,
	}, nil
}

// executeEnd completes the flow
func (e *Engine) executeEnd(ctx context.Context, step *EndStep, exec *FlowExecution, exprCtx *ExpressionContext) (*StepResult, error) {
	message := e.exprParser.InterpolateString(step.Message, exprCtx)

	var redirect *EndRedirect
	if step.Redirect != nil {
		recordIDVal, err := e.exprParser.Evaluate(step.Redirect.RecordID, exprCtx)
		if err == nil && recordIDVal != nil {
			redirect = &EndRedirect{
				Entity:   step.Redirect.Entity,
				RecordID: fmt.Sprintf("%v", recordIDVal),
			}
		}
	}

	return &StepResult{
		Action:     ActionComplete,
		EndMessage: message,
		Redirect:   redirect,
	}, nil
}

