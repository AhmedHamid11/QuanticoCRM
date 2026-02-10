package entity

import "time"

// ValidationOperator represents the type of comparison for a condition
type ValidationOperator string

const (
	// Comparison operators
	OpEquals       ValidationOperator = "EQUALS"
	OpNotEquals    ValidationOperator = "NOT_EQUALS"
	OpIn           ValidationOperator = "IN"
	OpNotIn        ValidationOperator = "NOT_IN"
	OpIsEmpty      ValidationOperator = "IS_EMPTY"
	OpIsNotEmpty   ValidationOperator = "IS_NOT_EMPTY"
	OpGreaterThan  ValidationOperator = "GREATER_THAN"
	OpLessThan     ValidationOperator = "LESS_THAN"
	OpGreaterEqual ValidationOperator = "GREATER_EQUAL"
	OpLessEqual    ValidationOperator = "LESS_EQUAL"

	// Change detection operators (for UPDATE operations)
	OpChanged     ValidationOperator = "CHANGED"
	OpChangedTo   ValidationOperator = "CHANGED_TO"
	OpChangedFrom ValidationOperator = "CHANGED_FROM"

	// Boolean operators
	OpIsTrue  ValidationOperator = "IS_TRUE"
	OpIsFalse ValidationOperator = "IS_FALSE"

	// String operators
	OpContains   ValidationOperator = "CONTAINS"
	OpStartsWith ValidationOperator = "STARTS_WITH"
	OpEndsWith   ValidationOperator = "ENDS_WITH"
)

// ValidationActionType represents the type of action to take when validation fails
type ValidationActionType string

const (
	// BLOCK_SAVE - Prevents the save operation entirely
	ActionBlockSave ValidationActionType = "BLOCK_SAVE"

	// LOCK_FIELDS - Prevents modification of specific fields
	ActionLockFields ValidationActionType = "LOCK_FIELDS"

	// REQUIRE_VALUE - Requires specific fields to have non-empty values
	ActionRequireValue ValidationActionType = "REQUIRE_VALUE"

	// ENFORCE_VALUE - Forces specific fields to have specific values
	ActionEnforceValue ValidationActionType = "ENFORCE_VALUE"

	// SET_VALUE - Automatically sets field values (future enhancement)
	ActionSetValue ValidationActionType = "SET_VALUE"
)

// ValidationCondition represents a single condition in a validation rule
type ValidationCondition struct {
	ID        string             `json:"id"`
	FieldName string             `json:"fieldName"`
	Operator  ValidationOperator `json:"operator"`
	Value     interface{}        `json:"value,omitempty"`  // For single value comparisons
	Values    []string           `json:"values,omitempty"` // For IN, NOT_IN operators
}

// ValidationAction represents an action to take when conditions are met
type ValidationAction struct {
	Type         ValidationActionType `json:"type"`
	Fields       []string             `json:"fields,omitempty"`       // For LOCK_FIELDS, REQUIRE_VALUE
	FieldName    string               `json:"fieldName,omitempty"`    // For ENFORCE_VALUE, SET_VALUE
	Value        interface{}          `json:"value,omitempty"`        // For ENFORCE_VALUE, SET_VALUE
	ErrorMessage string               `json:"errorMessage,omitempty"` // Custom error message (supports {{field}} placeholder)
}

// ValidationRule represents a validation rule configuration
type ValidationRule struct {
	ID              string                `json:"id" db:"id"`
	OrgID           string                `json:"orgId" db:"org_id"`
	Name            string                `json:"name" db:"name"`
	Description     *string               `json:"description,omitempty" db:"description"`
	EntityType      string                `json:"entityType" db:"entity_type"`
	Enabled         bool                  `json:"enabled" db:"enabled"`
	TriggerOnCreate bool                  `json:"triggerOnCreate" db:"trigger_on_create"`
	TriggerOnUpdate bool                  `json:"triggerOnUpdate" db:"trigger_on_update"`
	TriggerOnDelete bool                  `json:"triggerOnDelete" db:"trigger_on_delete"`
	ConditionLogic  string                `json:"conditionLogic" db:"condition_logic"` // "AND" or "OR"
	Conditions      []ValidationCondition `json:"conditions" db:"-"`
	ConditionsJSON  string                `json:"-" db:"conditions"`
	Actions         []ValidationAction    `json:"actions" db:"-"`
	ActionsJSON     string                `json:"-" db:"actions"`
	ErrorMessage    *string               `json:"errorMessage,omitempty" db:"error_message"` // Default error message
	Priority        int                   `json:"priority" db:"priority"`
	CreatedAt       time.Time             `json:"createdAt" db:"created_at"`
	ModifiedAt      time.Time             `json:"modifiedAt" db:"modified_at"`
	CreatedBy       *string               `json:"createdBy,omitempty" db:"created_by"`
	ModifiedBy      *string               `json:"modifiedBy,omitempty" db:"modified_by"`
}

// ValidationRuleCreateInput represents the input for creating a validation rule
type ValidationRuleCreateInput struct {
	Name            string                `json:"name" validate:"required"`
	Description     *string               `json:"description"`
	EntityType      string                `json:"entityType" validate:"required"`
	Enabled         *bool                 `json:"enabled"`
	TriggerOnCreate *bool                 `json:"triggerOnCreate"`
	TriggerOnUpdate *bool                 `json:"triggerOnUpdate"`
	TriggerOnDelete *bool                 `json:"triggerOnDelete"`
	ConditionLogic  string                `json:"conditionLogic"`
	Conditions      []ValidationCondition `json:"conditions" validate:"required"`
	Actions         []ValidationAction    `json:"actions" validate:"required"`
	ErrorMessage    *string               `json:"errorMessage"`
	Priority        *int                  `json:"priority"`
}

// ValidationRuleUpdateInput represents the input for updating a validation rule
type ValidationRuleUpdateInput struct {
	Name            *string               `json:"name"`
	Description     *string               `json:"description"`
	EntityType      *string               `json:"entityType"`
	Enabled         *bool                 `json:"enabled"`
	TriggerOnCreate *bool                 `json:"triggerOnCreate"`
	TriggerOnUpdate *bool                 `json:"triggerOnUpdate"`
	TriggerOnDelete *bool                 `json:"triggerOnDelete"`
	ConditionLogic  *string               `json:"conditionLogic"`
	Conditions      []ValidationCondition `json:"conditions"`
	Actions         []ValidationAction    `json:"actions"`
	ErrorMessage    *string               `json:"errorMessage"`
	Priority        *int                  `json:"priority"`
}

// ValidationRuleListParams represents query parameters for listing validation rules
type ValidationRuleListParams struct {
	Search     string `query:"search"`
	EntityType string `query:"entityType"`
	Enabled    *bool  `query:"enabled"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
}

// ValidationRuleListResponse represents the response for listing validation rules
type ValidationRuleListResponse struct {
	Data       []ValidationRule `json:"data"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

// FieldValidationError represents a validation error for a specific field
type FieldValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	RuleID  string `json:"ruleId"`
}

// ValidationResult represents the result of validating a record
type ValidationResult struct {
	Valid       bool                   `json:"valid"`
	Message     string                 `json:"message,omitempty"`
	FieldErrors []FieldValidationError `json:"fieldErrors,omitempty"`
}

// TestValidationInput represents the input for testing a validation rule
type TestValidationInput struct {
	Rule      ValidationRuleCreateInput `json:"rule"`
	Operation string                    `json:"operation"` // CREATE, UPDATE, DELETE
	OldRecord map[string]interface{}    `json:"oldRecord,omitempty"`
	NewRecord map[string]interface{}    `json:"newRecord,omitempty"`
}
