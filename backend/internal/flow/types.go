package flow

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Flow Definition Types
// =============================================================================

// FlowDefinition represents a complete screen flow configuration
type FlowDefinition struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description,omitempty"`
	Version           int                 `json:"version"`
	Trigger           FlowTrigger         `json:"trigger"`
	Variables         map[string]Variable `json:"variables,omitempty"`
	RefreshOnComplete bool                `json:"refreshOnComplete,omitempty"` // Refresh page when flow completes
	RawSteps          []json.RawMessage   `json:"steps"`                       // Raw JSON for each step
	parsedSteps       map[string]interface{}                                   // Cache of parsed steps
}

// FlowTrigger defines how a flow is initiated
type FlowTrigger struct {
	Type        TriggerType `json:"type"`
	EntityType  string      `json:"entityType,omitempty"`  // For entity-based triggers
	ButtonLabel string      `json:"buttonLabel,omitempty"` // For manual triggers
	ShowOn      []string    `json:"showOn,omitempty"`      // "detail", "list", "both"
}

// TriggerType defines how a flow can be started
type TriggerType string

const (
	TriggerManual       TriggerType = "manual"        // User clicks button
	TriggerRecordCreate TriggerType = "record_create" // Auto-trigger on record creation
	TriggerRecordUpdate TriggerType = "record_update" // Auto-trigger on record update
	TriggerScheduled    TriggerType = "scheduled"     // Scheduled execution
)

// Variable defines a flow variable with type and default value
type Variable struct {
	Type    VariableType `json:"type"`
	Default interface{}  `json:"default,omitempty"`
}

// VariableType defines the data type of a variable
type VariableType string

const (
	VarTypeString  VariableType = "string"
	VarTypeNumber  VariableType = "number"
	VarTypeBoolean VariableType = "boolean"
	VarTypeDate    VariableType = "date"
	VarTypeRecord  VariableType = "record" // Map of field values
	VarTypeList    VariableType = "list"   // Array of values
)

// =============================================================================
// Step Types
// =============================================================================

// Step represents a single step in the flow
type Step struct {
	ID   string   `json:"id"`
	Type StepType `json:"type"`
	Name string   `json:"name"`
	Next string   `json:"next,omitempty"` // Next step ID (for linear steps)
}

// StepType identifies the kind of step
type StepType string

const (
	StepTypeScreen       StepType = "screen"
	StepTypeDecision     StepType = "decision"
	StepTypeAssignment   StepType = "assignment"
	StepTypeRecordCreate StepType = "record_create"
	StepTypeRecordUpdate StepType = "record_update"
	StepTypeRecordGet    StepType = "record_get"
	StepTypeWebhook      StepType = "n8n_webhook"
	StepTypeEnd          StepType = "end"
)

// =============================================================================
// Screen Step
// =============================================================================

// ScreenStep displays a form to the user and collects input
type ScreenStep struct {
	Step
	Header *ScreenHeader `json:"header,omitempty"`
	Fields []ScreenField `json:"fields"`
}

// ScreenHeader displays contextual information at the top of a screen
type ScreenHeader struct {
	Type    string `json:"type"`    // "alert", "info"
	Variant string `json:"variant"` // "success", "warning", "error", "info"
	Message string `json:"message"` // Supports {{variable}} interpolation
}

// ScreenField defines a single input field on a screen
type ScreenField struct {
	Name         string        `json:"name"`
	Label        string        `json:"label"`
	Type         FieldType     `json:"type"`
	Required     bool          `json:"required,omitempty"`
	DefaultValue string        `json:"defaultValue,omitempty"` // Expression or literal
	Value        string        `json:"value,omitempty"`        // For display fields - template like {{variable}}
	Placeholder  string        `json:"placeholder,omitempty"`
	HelpText     string        `json:"helpText,omitempty"`
	Options      []FieldOption `json:"options,omitempty"`  // For select/radio fields
	Entity       string        `json:"entity,omitempty"`   // For lookup fields
	Filter       interface{}   `json:"filter,omitempty"`   // For lookup fields
	MinValue     *float64      `json:"minValue,omitempty"` // For number fields
	MaxValue     *float64      `json:"maxValue,omitempty"` // For number fields
	MinDate      string        `json:"minDate,omitempty"`  // For date fields (expression)
	MaxDate      string        `json:"maxDate,omitempty"`  // For date fields (expression)
	Rows         int           `json:"rows,omitempty"`     // For textarea
}

// FieldType defines the input type for a screen field
type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeTextarea FieldType = "textarea"
	FieldTypeNumber   FieldType = "number"
	FieldTypeCurrency FieldType = "currency"
	FieldTypePercent  FieldType = "percent"
	FieldTypeDate     FieldType = "date"
	FieldTypeDatetime FieldType = "datetime"
	FieldTypeSelect   FieldType = "select"
	FieldTypeRadio    FieldType = "radio"
	FieldTypeCheckbox FieldType = "checkbox"
	FieldTypeLookup   FieldType = "lookup"
	FieldTypeEmail    FieldType = "email"
	FieldTypePhone    FieldType = "phone"
	FieldTypeURL      FieldType = "url"
)

// FieldOption represents a choice in a select/radio field
type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// =============================================================================
// Decision Step
// =============================================================================

// DecisionStep evaluates conditions and branches the flow
type DecisionStep struct {
	Step
	Rules       []DecisionRule `json:"rules"`
	DefaultNext string         `json:"defaultNext"` // Fallback if no rule matches
}

// DecisionRule represents a single condition and its target
type DecisionRule struct {
	Condition string `json:"condition"` // Boolean expression
	Next      string `json:"next"`      // Step ID to go to if condition is true
}

// =============================================================================
// Assignment Step
// =============================================================================

// AssignmentStep sets variable values
type AssignmentStep struct {
	Step
	Assignments []Assignment `json:"assignments"`
}

// Assignment sets a variable to an expression result
type Assignment struct {
	Variable   string `json:"variable"`   // Variable name to set
	Expression string `json:"expression"` // Expression to evaluate
}

// =============================================================================
// Record Operation Steps
// =============================================================================

// RecordCreateStep creates a new record
type RecordCreateStep struct {
	Step
	Entity         string            `json:"entity"`                   // Entity type to create
	FieldMapping   map[string]string `json:"fieldMapping"`             // field -> expression
	OutputVariable string            `json:"outputVariable,omitempty"` // Variable to store created record
}

// RecordUpdateStep updates an existing record
type RecordUpdateStep struct {
	Step
	Entity       string            `json:"entity"`       // Entity type to update
	RecordID     string            `json:"recordId"`     // Expression for record ID
	FieldMapping map[string]string `json:"fieldMapping"` // field -> expression
}

// RecordGetStep retrieves a record
type RecordGetStep struct {
	Step
	Entity         string `json:"entity"`                   // Entity type to fetch
	RecordID       string `json:"recordId"`                 // Expression for record ID
	OutputVariable string `json:"outputVariable,omitempty"` // Variable to store fetched record
}

// =============================================================================
// Webhook Step
// =============================================================================

// WebhookStep calls an external webhook (n8n integration)
type WebhookStep struct {
	Step
	Webhook        string            `json:"webhook"`                  // Webhook path/name
	Payload        map[string]string `json:"payload"`                  // field -> expression
	Async          bool              `json:"async"`                    // Fire and forget
	OutputVariable string            `json:"outputVariable,omitempty"` // Store response (sync only)
}

// =============================================================================
// End Step
// =============================================================================

// EndStep terminates the flow
type EndStep struct {
	Step
	Message  string       `json:"message,omitempty"`  // Completion message (supports interpolation)
	Redirect *EndRedirect `json:"redirect,omitempty"` // Optional redirect after completion
}

// EndRedirect specifies where to navigate after flow completion
type EndRedirect struct {
	Entity   string `json:"entity"`   // Entity type
	RecordID string `json:"recordId"` // Expression for record ID
}

// =============================================================================
// Flow Execution Types
// =============================================================================

// FlowExecution represents the state of a running flow
type FlowExecution struct {
	ID          string                 `json:"id"`
	FlowID      string                 `json:"flowId"`
	FlowName    string                 `json:"flowName"`
	FlowVersion int                    `json:"flowVersion"`
	OrgID       string                 `json:"orgId"`
	UserID      string                 `json:"userId"`
	Status      ExecutionStatus        `json:"status"`
	CurrentStep string                 `json:"currentStep"`
	Variables   map[string]interface{} `json:"variables"`
	Record      map[string]interface{} `json:"record,omitempty"` // Source record (if entity-triggered)
	ScreenData  map[string]interface{} `json:"screenData"`       // Accumulated screen inputs
	ScreenDef   *ScreenStep            `json:"screenDef,omitempty"`
	EndMessage  string                 `json:"endMessage,omitempty"`
	Redirect    *EndRedirect           `json:"redirect,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
}

// ExecutionStatus represents the current state of a flow execution
type ExecutionStatus string

const (
	StatusRunning        ExecutionStatus = "running"
	StatusPausedAtScreen ExecutionStatus = "paused_at_screen"
	StatusCompleted      ExecutionStatus = "completed"
	StatusFailed         ExecutionStatus = "failed"
)

// =============================================================================
// Step Result Types
// =============================================================================

// StepAction indicates what the engine should do after executing a step
type StepAction string

const (
	ActionContinue       StepAction = "continue"
	ActionPauseForScreen StepAction = "pause_for_screen"
	ActionComplete       StepAction = "complete"
)

// StepResult is returned by step executors to control flow
type StepResult struct {
	Action     StepAction   `json:"action"`
	NextStep   string       `json:"nextStep,omitempty"`
	ScreenDef  *ScreenStep  `json:"screenDef,omitempty"`  // For screen steps
	EndMessage string       `json:"endMessage,omitempty"` // For end steps
	Redirect   *EndRedirect `json:"redirect,omitempty"`   // For end steps
}

// =============================================================================
// Database Models
// =============================================================================

// FlowDefinitionDB represents a flow definition as stored in the database
type FlowDefinitionDB struct {
	ID           string  `json:"id" db:"id"`
	OrgID        string  `json:"orgId" db:"org_id"`
	Name         string  `json:"name" db:"name"`
	Description  *string `json:"description,omitempty" db:"description"`
	Version      int     `json:"version" db:"version"`
	Definition   string  `json:"definition" db:"definition"` // JSON blob
	IsActive     bool    `json:"isActive" db:"is_active"`
	CreatedBy    string  `json:"createdBy" db:"created_by"`
	CreatedAt    string  `json:"createdAt" db:"created_at"`
	ModifiedAt   string  `json:"modifiedAt" db:"modified_at"`
	ModifiedBy   *string `json:"modifiedBy,omitempty" db:"modified_by"`
}

// FlowExecutionDB represents a flow execution as stored in the database
type FlowExecutionDB struct {
	ID             string  `json:"id" db:"id"`
	OrgID          string  `json:"orgId" db:"org_id"`
	FlowID         string  `json:"flowId" db:"flow_id"`
	UserID         string  `json:"userId" db:"user_id"`
	Status         string  `json:"status" db:"status"`
	CurrentStep    string  `json:"currentStep" db:"current_step"`
	Variables      string  `json:"-" db:"variables"`   // JSON
	ScreenData     string  `json:"-" db:"screen_data"` // JSON
	SourceEntity   *string `json:"sourceEntity,omitempty" db:"source_entity"`
	SourceRecordID *string `json:"sourceRecordId,omitempty" db:"source_record_id"`
	Error          *string `json:"error,omitempty" db:"error"`
	StartedAt      string  `json:"startedAt" db:"started_at"`
	CompletedAt    *string `json:"completedAt,omitempty" db:"completed_at"`
}

// =============================================================================
// API Request/Response Types
// =============================================================================

// StartFlowRequest is the request body for starting a flow
type StartFlowRequest struct {
	RecordID string `json:"recordId,omitempty"`
	Entity   string `json:"entity,omitempty"`
}

// SubmitScreenRequest is the request body for submitting screen data
type SubmitScreenRequest map[string]interface{}

// FlowListParams represents query parameters for listing flows
type FlowListParams struct {
	Search     string `query:"search"`
	EntityType string `query:"entityType"`
	IsActive   *bool  `query:"isActive"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
}

// FlowListResponse represents the response for listing flows
type FlowListResponse struct {
	Data       []FlowDefinitionDB `json:"data"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

// FlowCreateInput represents input for creating a flow
type FlowCreateInput struct {
	Name        string         `json:"name" validate:"required"`
	Description *string        `json:"description"`
	Definition  FlowDefinition `json:"definition" validate:"required"`
	IsActive    *bool          `json:"isActive"`
}

// FlowUpdateInput represents input for updating a flow
type FlowUpdateInput struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Definition  *FlowDefinition `json:"definition"`
	IsActive    *bool           `json:"isActive"`
}

// =============================================================================
// Helper Methods
// =============================================================================

// ParseDefinition parses the JSON definition into a FlowDefinition
func (f *FlowDefinitionDB) ParseDefinition() (*FlowDefinition, error) {
	var def FlowDefinition
	if err := json.Unmarshal([]byte(f.Definition), &def); err != nil {
		return nil, err
	}
	// Copy metadata from DB struct (not stored in JSON definition)
	def.ID = f.ID
	def.Name = f.Name
	def.Version = f.Version
	if f.Description != nil {
		def.Description = *f.Description
	}
	return &def, nil
}

// SetDefinition serializes a FlowDefinition to JSON
func (f *FlowDefinitionDB) SetDefinition(def *FlowDefinition) error {
	data, err := json.Marshal(def)
	if err != nil {
		return err
	}
	f.Definition = string(data)
	return nil
}

// ParseStep parses a generic step into its specific type
func ParseStep(stepData map[string]interface{}) (*Step, interface{}, error) {
	data, err := json.Marshal(stepData)
	if err != nil {
		return nil, nil, err
	}

	var base Step
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, nil, err
	}

	var specific interface{}
	switch base.Type {
	case StepTypeScreen:
		var s ScreenStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeDecision:
		var s DecisionStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeAssignment:
		var s AssignmentStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeRecordCreate:
		var s RecordCreateStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeRecordUpdate:
		var s RecordUpdateStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeRecordGet:
		var s RecordGetStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeWebhook:
		var s WebhookStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	case StepTypeEnd:
		var s EndStep
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, err
		}
		specific = &s
	default:
		specific = &base
	}

	return &base, specific, nil
}

// =============================================================================
// FlowDefinition Step Access Methods
// =============================================================================

// GetStep returns the parsed step by ID
func (f *FlowDefinition) GetStep(stepID string) interface{} {
	f.ensureStepsParsed()
	return f.parsedSteps[stepID]
}

// GetStepBase returns just the base step info by ID
func (f *FlowDefinition) GetStepBase(stepID string) *Step {
	for _, rawStep := range f.RawSteps {
		var base Step
		if json.Unmarshal(rawStep, &base) == nil && base.ID == stepID {
			return &base
		}
	}
	return nil
}

// GetFirstStepID returns the ID of the first step
func (f *FlowDefinition) GetFirstStepID() string {
	if len(f.RawSteps) == 0 {
		return ""
	}
	var base Step
	if json.Unmarshal(f.RawSteps[0], &base) == nil {
		return base.ID
	}
	return ""
}

// ensureStepsParsed lazily parses all steps
func (f *FlowDefinition) ensureStepsParsed() {
	if f.parsedSteps != nil {
		return
	}
	f.parsedSteps = make(map[string]interface{})
	for _, rawStep := range f.RawSteps {
		var base Step
		if err := json.Unmarshal(rawStep, &base); err != nil {
			continue
		}

		var parsed interface{}
		switch base.Type {
		case StepTypeScreen:
			var s ScreenStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeDecision:
			var s DecisionStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeAssignment:
			var s AssignmentStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeRecordCreate:
			var s RecordCreateStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeRecordUpdate:
			var s RecordUpdateStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeRecordGet:
			var s RecordGetStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeWebhook:
			var s WebhookStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		case StepTypeEnd:
			var s EndStep
			if json.Unmarshal(rawStep, &s) == nil {
				parsed = &s
			}
		default:
			parsed = &base
		}

		if parsed != nil {
			f.parsedSteps[base.ID] = parsed
		}
	}
}
