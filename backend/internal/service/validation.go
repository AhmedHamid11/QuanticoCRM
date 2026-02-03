package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// ValidationService handles validation rule evaluation
type ValidationService struct {
	db             *sql.DB
	validationRepo *repo.ValidationRepo
	ruleCache      *sync.Map // map[string]cachedRules where key is "orgID:entityType"
	cacheTTL       time.Duration
}

// cachedRules holds cached validation rules with expiration
type cachedRules struct {
	rules     []entity.ValidationRule
	expiresAt time.Time
}

// NewValidationService creates a new ValidationService
func NewValidationService(db *sql.DB, validationRepo *repo.ValidationRepo) *ValidationService {
	return &ValidationService{
		db:             db,
		validationRepo: validationRepo,
		ruleCache:      &sync.Map{},
		cacheTTL:       5 * time.Minute,
	}
}

// ValidateOperation validates a record operation against all applicable rules
// Returns a ValidationResult indicating whether the operation is allowed
func (s *ValidationService) ValidateOperation(
	ctx context.Context,
	orgID, entityType, recordID string,
	operation string, // "CREATE", "UPDATE", "DELETE"
	oldRecord, newRecord map[string]interface{},
) (*entity.ValidationResult, error) {
	// Fetch rules from cache or DB
	rules, err := s.getRulesForEntity(ctx, orgID, entityType)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		return &entity.ValidationResult{Valid: true}, nil
	}

	// Collect all field errors
	var allFieldErrors []entity.FieldValidationError
	var generalMessage string

	// Evaluate each rule
	for _, rule := range rules {
		// Check if rule applies to this operation
		if !s.ruleAppliesToOperation(&rule, operation) {
			continue
		}

		// Evaluate conditions
		if s.evaluateConditions(&rule, operation, oldRecord, newRecord) {
			// Conditions matched - execute actions
			fieldErrors, err := s.executeActions(&rule, operation, oldRecord, newRecord)
			if err != nil {
				return nil, err
			}

			if len(fieldErrors) > 0 {
				allFieldErrors = append(allFieldErrors, fieldErrors...)
				if generalMessage == "" && rule.ErrorMessage != nil {
					generalMessage = *rule.ErrorMessage
				}
			}
		}
	}

	// Build result
	if len(allFieldErrors) > 0 {
		if generalMessage == "" {
			generalMessage = "Validation failed"
		}
		return &entity.ValidationResult{
			Valid:       false,
			Message:     generalMessage,
			FieldErrors: allFieldErrors,
		}, nil
	}

	return &entity.ValidationResult{Valid: true}, nil
}

// ruleAppliesToOperation checks if a rule should be evaluated for the given operation
func (s *ValidationService) ruleAppliesToOperation(rule *entity.ValidationRule, operation string) bool {
	switch operation {
	case "CREATE":
		return rule.TriggerOnCreate
	case "UPDATE":
		return rule.TriggerOnUpdate
	case "DELETE":
		return rule.TriggerOnDelete
	default:
		return false
	}
}

// getRulesForEntity fetches rules from cache or database
func (s *ValidationService) getRulesForEntity(ctx context.Context, orgID, entityType string) ([]entity.ValidationRule, error) {
	cacheKey := orgID + ":" + entityType

	// Check cache first
	if cached, ok := s.ruleCache.Load(cacheKey); ok {
		cr := cached.(*cachedRules)
		if time.Now().Before(cr.expiresAt) {
			return cr.rules, nil
		}
		// Cache expired, delete it
		s.ruleCache.Delete(cacheKey)
	}

	// Fetch from database
	rules, err := s.validationRepo.ListByEntityType(ctx, orgID, entityType, true)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.ruleCache.Store(cacheKey, &cachedRules{
		rules:     rules,
		expiresAt: time.Now().Add(s.cacheTTL),
	})

	return rules, nil
}

// InvalidateCache clears the rule cache for a specific entity type or all entities
func (s *ValidationService) InvalidateCache(orgID, entityType string) {
	if entityType == "" {
		// Clear all caches for this org
		s.ruleCache.Range(func(key, value interface{}) bool {
			if strings.HasPrefix(key.(string), orgID+":") {
				s.ruleCache.Delete(key)
			}
			return true
		})
	} else {
		// Clear specific cache
		s.ruleCache.Delete(orgID + ":" + entityType)
	}
}

// evaluateConditions evaluates all conditions based on the logic (AND/OR)
func (s *ValidationService) evaluateConditions(
	rule *entity.ValidationRule,
	operation string,
	oldRecord, newRecord map[string]interface{},
) bool {
	if len(rule.Conditions) == 0 {
		// No conditions = always matches (useful for rules that just check actions)
		return true
	}

	logic := strings.ToUpper(rule.ConditionLogic)
	if logic != "AND" && logic != "OR" {
		logic = "AND"
	}

	for _, cond := range rule.Conditions {
		result := s.evaluateCondition(cond, operation, oldRecord, newRecord)

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
func (s *ValidationService) evaluateCondition(
	cond entity.ValidationCondition,
	operation string,
	oldRecord, newRecord map[string]interface{},
) bool {
	// Get field values
	var currentValue interface{}
	var oldValue interface{}

	if newRecord != nil {
		currentValue = s.getFieldValue(newRecord, cond.FieldName)
	}
	if oldRecord != nil {
		oldValue = s.getFieldValue(oldRecord, cond.FieldName)
	}

	// For DELETE operations, use old record
	if operation == "DELETE" && newRecord == nil {
		currentValue = oldValue
	}

	switch cond.Operator {
	case entity.OpEquals:
		return s.valuesEqual(currentValue, cond.Value)

	case entity.OpNotEquals:
		return !s.valuesEqual(currentValue, cond.Value)

	case entity.OpIn:
		return s.valueInList(currentValue, cond.Values)

	case entity.OpNotIn:
		return !s.valueInList(currentValue, cond.Values)

	case entity.OpIsEmpty:
		return s.isEmpty(currentValue)

	case entity.OpIsNotEmpty:
		return !s.isEmpty(currentValue)

	case entity.OpGreaterThan:
		return s.compareNumbers(currentValue, cond.Value) > 0

	case entity.OpLessThan:
		return s.compareNumbers(currentValue, cond.Value) < 0

	case entity.OpGreaterEqual:
		return s.compareNumbers(currentValue, cond.Value) >= 0

	case entity.OpLessEqual:
		return s.compareNumbers(currentValue, cond.Value) <= 0

	case entity.OpChanged:
		if operation != "UPDATE" {
			return false
		}
		return !s.valuesEqual(oldValue, currentValue)

	case entity.OpChangedTo:
		if operation != "UPDATE" {
			return false
		}
		return !s.valuesEqual(oldValue, currentValue) && s.valuesEqual(currentValue, cond.Value)

	case entity.OpChangedFrom:
		if operation != "UPDATE" {
			return false
		}
		return !s.valuesEqual(oldValue, currentValue) && s.valuesEqual(oldValue, cond.Value)

	case entity.OpIsTrue:
		return s.isTruthy(currentValue)

	case entity.OpIsFalse:
		return !s.isTruthy(currentValue)

	case entity.OpContains:
		return s.stringContains(currentValue, cond.Value)

	case entity.OpStartsWith:
		return s.stringStartsWith(currentValue, cond.Value)

	case entity.OpEndsWith:
		return s.stringEndsWith(currentValue, cond.Value)

	default:
		return false
	}
}

// executeActions executes validation actions and returns any field errors
func (s *ValidationService) executeActions(
	rule *entity.ValidationRule,
	operation string,
	oldRecord, newRecord map[string]interface{},
) ([]entity.FieldValidationError, error) {
	var fieldErrors []entity.FieldValidationError

	for _, action := range rule.Actions {
		switch action.Type {
		case entity.ActionBlockSave:
			// Block the entire save operation
			errorMsg := action.ErrorMessage
			if errorMsg == "" {
				errorMsg = "This operation is not allowed"
				if rule.ErrorMessage != nil {
					errorMsg = *rule.ErrorMessage
				}
			}
			fieldErrors = append(fieldErrors, entity.FieldValidationError{
				Field:   "_form",
				Message: errorMsg,
				RuleID:  rule.ID,
			})

		case entity.ActionLockFields:
			// Check if any locked fields are being modified
			if operation == "UPDATE" && len(action.Fields) > 0 {
				for _, fieldName := range action.Fields {
					oldVal := s.getFieldValue(oldRecord, fieldName)
					newVal := s.getFieldValue(newRecord, fieldName)

					if !s.valuesEqual(oldVal, newVal) {
						errorMsg := action.ErrorMessage
						if errorMsg == "" {
							errorMsg = "This field cannot be modified"
						}
						// Replace {{field}} placeholder
						errorMsg = strings.ReplaceAll(errorMsg, "{{field}}", fieldName)

						fieldErrors = append(fieldErrors, entity.FieldValidationError{
							Field:   fieldName,
							Message: errorMsg,
							RuleID:  rule.ID,
						})
					}
				}
			}

		case entity.ActionRequireValue:
			// Ensure specified fields have values
			if len(action.Fields) > 0 {
				for _, fieldName := range action.Fields {
					val := s.getFieldValue(newRecord, fieldName)

					if s.isEmpty(val) {
						errorMsg := action.ErrorMessage
						if errorMsg == "" {
							errorMsg = "This field is required"
						}
						errorMsg = strings.ReplaceAll(errorMsg, "{{field}}", fieldName)

						fieldErrors = append(fieldErrors, entity.FieldValidationError{
							Field:   fieldName,
							Message: errorMsg,
							RuleID:  rule.ID,
						})
					}
				}
			}

		case entity.ActionEnforceValue:
			// Ensure a specific field has a specific value
			if action.FieldName != "" && action.Value != nil {
				val := s.getFieldValue(newRecord, action.FieldName)

				if !s.valuesEqual(val, action.Value) {
					errorMsg := action.ErrorMessage
					if errorMsg == "" {
						errorMsg = fmt.Sprintf("This field must be set to %v", action.Value)
					}
					errorMsg = strings.ReplaceAll(errorMsg, "{{field}}", action.FieldName)
					errorMsg = strings.ReplaceAll(errorMsg, "{{value}}", fmt.Sprintf("%v", action.Value))

					fieldErrors = append(fieldErrors, entity.FieldValidationError{
						Field:   action.FieldName,
						Message: errorMsg,
						RuleID:  rule.ID,
					})
				}
			}
		}
	}

	return fieldErrors, nil
}

// TestRule tests a validation rule against sample data
func (s *ValidationService) TestRule(
	rule *entity.ValidationRuleCreateInput,
	operation string,
	oldRecord, newRecord map[string]interface{},
) *entity.ValidationResult {
	// Convert input to full rule for testing
	testRule := &entity.ValidationRule{
		ID:              "test",
		Name:            rule.Name,
		EntityType:      rule.EntityType,
		Enabled:         true,
		TriggerOnCreate: rule.TriggerOnCreate != nil && *rule.TriggerOnCreate,
		TriggerOnUpdate: rule.TriggerOnUpdate == nil || *rule.TriggerOnUpdate,
		TriggerOnDelete: rule.TriggerOnDelete != nil && *rule.TriggerOnDelete,
		ConditionLogic:  rule.ConditionLogic,
		Conditions:      rule.Conditions,
		Actions:         rule.Actions,
		ErrorMessage:    rule.ErrorMessage,
	}

	if testRule.ConditionLogic == "" {
		testRule.ConditionLogic = "AND"
	}

	// Check if rule applies
	if !s.ruleAppliesToOperation(testRule, operation) {
		return &entity.ValidationResult{
			Valid:   true,
			Message: "Rule does not apply to this operation type",
		}
	}

	// Evaluate conditions
	if s.evaluateConditions(testRule, operation, oldRecord, newRecord) {
		// Execute actions
		fieldErrors, _ := s.executeActions(testRule, operation, oldRecord, newRecord)

		if len(fieldErrors) > 0 {
			message := "Validation failed"
			if testRule.ErrorMessage != nil {
				message = *testRule.ErrorMessage
			}
			return &entity.ValidationResult{
				Valid:       false,
				Message:     message,
				FieldErrors: fieldErrors,
			}
		}
	}

	return &entity.ValidationResult{
		Valid:   true,
		Message: "Validation passed",
	}
}

// Helper functions

// getFieldValue extracts a field value from a record, handling both snake_case and camelCase
func (s *ValidationService) getFieldValue(record map[string]interface{}, fieldName string) interface{} {
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

// valuesEqual compares two values for equality
func (s *ValidationService) valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert both to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr
}

// valueInList checks if a value is in a list of values
func (s *ValidationService) valueInList(val interface{}, list []string) bool {
	if val == nil {
		return false
	}

	valStr := fmt.Sprintf("%v", val)
	for _, item := range list {
		if valStr == item {
			return true
		}
	}
	return false
}

// isEmpty checks if a value is empty
func (s *ValidationService) isEmpty(val interface{}) bool {
	if val == nil {
		return true
	}

	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case int, int32, int64:
		return false // Numbers are never "empty" by this definition
	case float32, float64:
		return false
	case bool:
		return false
	default:
		return fmt.Sprintf("%v", val) == ""
	}
}

// isTruthy checks if a value is truthy
func (s *ValidationService) isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case int, int32, int64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	case string:
		lower := strings.ToLower(v)
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return false
	}
}

// compareNumbers compares two values as numbers
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func (s *ValidationService) compareNumbers(a, b interface{}) int {
	aFloat := s.toFloat(a)
	bFloat := s.toFloat(b)

	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}

// toFloat converts a value to float64
func (s *ValidationService) toFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// stringContains checks if a string contains a substring
func (s *ValidationService) stringContains(val, substr interface{}) bool {
	if val == nil || substr == nil {
		return false
	}
	return strings.Contains(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(fmt.Sprintf("%v", substr)))
}

// stringStartsWith checks if a string starts with a prefix
func (s *ValidationService) stringStartsWith(val, prefix interface{}) bool {
	if val == nil || prefix == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(fmt.Sprintf("%v", prefix)))
}

// stringEndsWith checks if a string ends with a suffix
func (s *ValidationService) stringEndsWith(val, suffix interface{}) bool {
	if val == nil || suffix == nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(fmt.Sprintf("%v", suffix)))
}

// camelToSnake converts camelCase to snake_case
func (s *ValidationService) camelToSnake(str string) string {
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
func (s *ValidationService) snakeToCamel(str string) string {
	parts := strings.Split(str, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
