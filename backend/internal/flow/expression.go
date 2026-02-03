package flow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExpressionParser handles evaluation of flow expressions
type ExpressionParser struct {
	templateRegex *regexp.Regexp // Matches {{variable}}
	stringRegex   *regexp.Regexp // Matches 'string literals'
	numberRegex   *regexp.Regexp // Matches numeric values
	funcRegex     *regexp.Regexp // Matches FUNC(args)
}

// NewExpressionParser creates a new expression parser
func NewExpressionParser() *ExpressionParser {
	return &ExpressionParser{
		templateRegex: regexp.MustCompile(`\{\{(\$?\w+(?:\.\w+)*)\}\}`),
		stringRegex:   regexp.MustCompile(`^'([^']*)'$`),
		numberRegex:   regexp.MustCompile(`^-?\d+(?:\.\d+)?$`),
		funcRegex:     regexp.MustCompile(`^(\w+)\((.*)\)$`),
	}
}

// ExpressionContext provides values for expression evaluation
type ExpressionContext struct {
	Variables  map[string]interface{} // Flow variables
	ScreenData map[string]interface{} // Screen input data
	Record     map[string]interface{} // Source record ($record.field)
}

// NewContext creates an expression context from a flow execution
func NewContext(exec *FlowExecution) *ExpressionContext {
	return &ExpressionContext{
		Variables:  exec.Variables,
		ScreenData: exec.ScreenData,
		Record:     exec.Record,
	}
}

// =============================================================================
// Main Evaluation Methods
// =============================================================================

// Evaluate evaluates an expression and returns the result
func (p *ExpressionParser) Evaluate(expr string, ctx *ExpressionContext) (interface{}, error) {
	expr = strings.TrimSpace(expr)

	if expr == "" {
		return nil, nil
	}

	// Handle {{variable}} template syntax - unwrap and evaluate the inner expression
	if strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}") {
		inner := strings.TrimSpace(expr[2 : len(expr)-2])
		return p.Evaluate(inner, ctx)
	}

	// String literal: 'text'
	if matches := p.stringRegex.FindStringSubmatch(expr); len(matches) == 2 {
		return matches[1], nil
	}

	// String concatenation: 'text' + variable
	if strings.Contains(expr, "' + ") || strings.Contains(expr, "+ '") {
		return p.evaluateConcat(expr, ctx)
	}

	// Numeric literal
	if p.numberRegex.MatchString(expr) {
		return strconv.ParseFloat(expr, 64)
	}

	// Boolean literals
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}

	// NULL/nil
	if expr == "null" || expr == "nil" {
		return nil, nil
	}

	// Arithmetic expression: a + b, a - b, a * b, a / b
	// Must check BEFORE function calls because func regex is greedy
	if result, handled, err := p.tryArithmetic(expr, ctx); handled {
		return result, err
	}

	// Function call: FUNC(args)
	if matches := p.funcRegex.FindStringSubmatch(expr); len(matches) == 3 {
		return p.evaluateFunction(matches[1], matches[2], ctx)
	}

	// Variable reference
	return p.resolveVariable(expr, ctx)
}

// tryArithmetic attempts to evaluate arithmetic expressions
// Returns (result, wasHandled, error)
func (p *ExpressionParser) tryArithmetic(expr string, ctx *ExpressionContext) (interface{}, bool, error) {
	// Find the operator outside of parentheses
	// We need to handle operator precedence: + and - are lower than * and /
	// Process + and - first (they bind loosest)

	for _, op := range []string{" + ", " - "} {
		if idx := p.findOperatorOutsideParens(expr, op); idx >= 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])

			leftVal, err := p.Evaluate(left, ctx)
			if err != nil {
				return nil, true, err
			}
			rightVal, err := p.Evaluate(right, ctx)
			if err != nil {
				return nil, true, err
			}

			leftNum := p.toFloat(leftVal)
			rightNum := p.toFloat(rightVal)

			switch op {
			case " + ":
				return leftNum + rightNum, true, nil
			case " - ":
				return leftNum - rightNum, true, nil
			}
		}
	}

	// Then check * and /
	for _, op := range []string{" * ", " / "} {
		if idx := p.findOperatorOutsideParens(expr, op); idx >= 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])

			leftVal, err := p.Evaluate(left, ctx)
			if err != nil {
				return nil, true, err
			}
			rightVal, err := p.Evaluate(right, ctx)
			if err != nil {
				return nil, true, err
			}

			leftNum := p.toFloat(leftVal)
			rightNum := p.toFloat(rightVal)

			switch op {
			case " * ":
				return leftNum * rightNum, true, nil
			case " / ":
				if rightNum == 0 {
					return nil, true, fmt.Errorf("division by zero")
				}
				return leftNum / rightNum, true, nil
			}
		}
	}

	return nil, false, nil
}

// findOperatorOutsideParens finds an operator that's not inside parentheses
func (p *ExpressionParser) findOperatorOutsideParens(expr, op string) int {
	depth := 0
	inString := false

	for i := 0; i < len(expr)-len(op)+1; i++ {
		ch := expr[i]

		if ch == '\'' {
			inString = !inString
		} else if !inString {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			} else if depth == 0 && expr[i:i+len(op)] == op {
				return i
			}
		}
	}

	return -1
}

// EvaluateBool evaluates a boolean expression
func (p *ExpressionParser) EvaluateBool(condition string, ctx *ExpressionContext) (bool, error) {
	condition = strings.TrimSpace(condition)

	// Handle parenthesized expressions - strip outer parens and evaluate inner
	if strings.HasPrefix(condition, "(") && strings.HasSuffix(condition, ")") {
		// Check if these parens actually match (not separate groups)
		depth := 0
		matched := true
		for i, c := range condition {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 && i != len(condition)-1 {
					// Found closing paren before the end - parens don't match
					matched = false
					break
				}
			}
		}
		if matched && depth == 0 {
			return p.EvaluateBool(condition[1:len(condition)-1], ctx)
		}
	}

	// Handle logical OR first (lower precedence) - but only at top level, not inside parens
	if orIdx := p.findTopLevelOperator(condition, " OR "); orIdx >= 0 {
		left := strings.TrimSpace(condition[:orIdx])
		right := strings.TrimSpace(condition[orIdx+4:])
		leftResult, err := p.EvaluateBool(left, ctx)
		if err != nil {
			return false, err
		}
		if leftResult {
			return true, nil
		}
		return p.EvaluateBool(right, ctx)
	}

	// Handle logical AND (higher precedence than OR) - but only at top level
	if andIdx := p.findTopLevelOperator(condition, " AND "); andIdx >= 0 {
		left := strings.TrimSpace(condition[:andIdx])
		right := strings.TrimSpace(condition[andIdx+5:])
		leftResult, err := p.EvaluateBool(left, ctx)
		if err != nil {
			return false, err
		}
		if !leftResult {
			return false, nil
		}
		return p.EvaluateBool(right, ctx)
	}

	// Legacy: Handle logical AND/OR with simple split (for backwards compat with simple cases)
	if strings.Contains(condition, " AND ") && !strings.Contains(condition, "(") {
		parts := strings.Split(condition, " AND ")
		for _, part := range parts {
			result, err := p.EvaluateBool(strings.TrimSpace(part), ctx)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	if strings.Contains(condition, " OR ") && !strings.Contains(condition, "(") {
		parts := strings.Split(condition, " OR ")
		for _, part := range parts {
			result, err := p.EvaluateBool(strings.TrimSpace(part), ctx)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	// Handle NOT
	if strings.HasPrefix(condition, "NOT ") {
		result, err := p.EvaluateBool(strings.TrimPrefix(condition, "NOT "), ctx)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Handle comparison operators (order matters - check >= before >)
	operators := []string{">=", "<=", "!=", "==", ">", "<"}
	for _, op := range operators {
		if idx := strings.Index(condition, op); idx > 0 {
			left := strings.TrimSpace(condition[:idx])
			right := strings.TrimSpace(condition[idx+len(op):])
			return p.compare(left, right, op, ctx)
		}
	}

	// Direct boolean evaluation
	result, err := p.Evaluate(condition, ctx)
	if err != nil {
		return false, err
	}

	switch v := result.(type) {
	case bool:
		return v, nil
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case string:
		return v != "", nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("cannot convert %T to boolean", result)
	}
}

// InterpolateString replaces {{variable}} placeholders with values
func (p *ExpressionParser) InterpolateString(template string, ctx *ExpressionContext) string {
	return p.templateRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable path from {{path}}
		varPath := match[2 : len(match)-2]
		value, err := p.resolveVariable(varPath, ctx)
		if err != nil || value == nil {
			return match // Keep original if not found
		}
		return fmt.Sprintf("%v", value)
	})
}

// =============================================================================
// Variable Resolution
// =============================================================================

// resolveVariable resolves a variable reference like "name", "$record.field", or "obj.prop"
func (p *ExpressionParser) resolveVariable(expr string, ctx *ExpressionContext) (interface{}, error) {
	expr = strings.TrimSpace(expr)

	// Handle $record.field
	if strings.HasPrefix(expr, "$record.") {
		field := strings.TrimPrefix(expr, "$record.")
		if ctx.Record != nil {
			return p.getNestedValue(ctx.Record, field), nil
		}
		return nil, nil
	}

	// Handle $record (entire record)
	if expr == "$record" {
		return ctx.Record, nil
	}

	// Try variables first
	parts := strings.SplitN(expr, ".", 2)
	varName := parts[0]

	// Check variables
	if val, ok := ctx.Variables[varName]; ok {
		if len(parts) == 2 {
			return p.getNestedValue(val, parts[1]), nil
		}
		return val, nil
	}

	// Check screen data
	if val, ok := ctx.ScreenData[varName]; ok {
		if len(parts) == 2 {
			return p.getNestedValue(val, parts[1]), nil
		}
		return val, nil
	}

	// Not found
	return nil, fmt.Errorf("variable not found: %s", expr)
}

// getNestedValue retrieves a nested value from a map using dot notation
func (p *ExpressionParser) getNestedValue(obj interface{}, path string) interface{} {
	if obj == nil {
		return nil
	}

	parts := strings.Split(path, ".")
	current := obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// =============================================================================
// Function Evaluation
// =============================================================================

// evaluateFunction evaluates built-in functions
func (p *ExpressionParser) evaluateFunction(funcName, argsStr string, ctx *ExpressionContext) (interface{}, error) {
	funcName = strings.ToUpper(funcName)

	switch funcName {
	case "CASE":
		return p.evaluateCase(argsStr, ctx)
	case "IF":
		return p.evaluateIf(argsStr, ctx)
	case "NOW":
		return p.evaluateNow(argsStr)
	case "COALESCE":
		return p.evaluateCoalesce(argsStr, ctx)
	case "UPPER":
		return p.evaluateUpper(argsStr, ctx)
	case "LOWER":
		return p.evaluateLower(argsStr, ctx)
	case "CONCAT":
		return p.evaluateConcat(argsStr, ctx)
	case "LEN":
		return p.evaluateLen(argsStr, ctx)
	case "ROUND":
		return p.evaluateRound(argsStr, ctx)
	case "ABS":
		return p.evaluateAbs(argsStr, ctx)
	case "MIN":
		return p.evaluateMin(argsStr, ctx)
	case "MAX":
		return p.evaluateMax(argsStr, ctx)
	default:
		return nil, fmt.Errorf("unknown function: %s", funcName)
	}
}

// evaluateCase handles CASE(expr, match1, result1, match2, result2, ..., default)
func (p *ExpressionParser) evaluateCase(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	if len(args) < 3 {
		return nil, fmt.Errorf("CASE requires at least 3 arguments")
	}

	// Evaluate the expression to match
	exprValue, err := p.Evaluate(args[0], ctx)
	if err != nil {
		return nil, err
	}

	// Check each match/result pair
	for i := 1; i < len(args)-1; i += 2 {
		matchValue, err := p.Evaluate(args[i], ctx)
		if err != nil {
			return nil, err
		}

		if p.valuesEqual(exprValue, matchValue) {
			return p.Evaluate(args[i+1], ctx)
		}
	}

	// Return default (last argument)
	return p.Evaluate(args[len(args)-1], ctx)
}

// evaluateIf handles IF(condition, trueValue, falseValue)
func (p *ExpressionParser) evaluateIf(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	if len(args) != 3 {
		return nil, fmt.Errorf("IF requires exactly 3 arguments")
	}

	condition, err := p.EvaluateBool(args[0], ctx)
	if err != nil {
		return nil, err
	}

	if condition {
		return p.Evaluate(args[1], ctx)
	}
	return p.Evaluate(args[2], ctx)
}

// evaluateNow handles NOW and NOW + N DAYS/HOURS/MINUTES
func (p *ExpressionParser) evaluateNow(argsStr string) (interface{}, error) {
	now := time.Now()

	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return now.Format(time.RFC3339), nil
	}

	// Parse "N DAYS" or "N HOURS" etc.
	parts := strings.Fields(argsStr)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid NOW argument: %s", argsStr)
	}

	amount, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid number in NOW: %s", parts[0])
	}

	unit := strings.ToUpper(parts[1])
	switch unit {
	case "DAYS", "DAY":
		return now.AddDate(0, 0, amount).Format(time.RFC3339), nil
	case "HOURS", "HOUR":
		return now.Add(time.Duration(amount) * time.Hour).Format(time.RFC3339), nil
	case "MINUTES", "MINUTE":
		return now.Add(time.Duration(amount) * time.Minute).Format(time.RFC3339), nil
	case "MONTHS", "MONTH":
		return now.AddDate(0, amount, 0).Format(time.RFC3339), nil
	case "YEARS", "YEAR":
		return now.AddDate(amount, 0, 0).Format(time.RFC3339), nil
	default:
		return nil, fmt.Errorf("unknown time unit: %s", unit)
	}
}

// evaluateCoalesce returns the first non-null value
func (p *ExpressionParser) evaluateCoalesce(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	for _, arg := range args {
		val, err := p.Evaluate(arg, ctx)
		if err != nil {
			continue
		}
		if val != nil && val != "" {
			return val, nil
		}
	}
	return nil, nil
}

// evaluateUpper converts string to uppercase
func (p *ExpressionParser) evaluateUpper(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	val, err := p.Evaluate(argsStr, ctx)
	if err != nil {
		return nil, err
	}
	if str, ok := val.(string); ok {
		return strings.ToUpper(str), nil
	}
	return val, nil
}

// evaluateLower converts string to lowercase
func (p *ExpressionParser) evaluateLower(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	val, err := p.Evaluate(argsStr, ctx)
	if err != nil {
		return nil, err
	}
	if str, ok := val.(string); ok {
		return strings.ToLower(str), nil
	}
	return val, nil
}

// evaluateLen returns length of string or array
func (p *ExpressionParser) evaluateLen(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	val, err := p.Evaluate(argsStr, ctx)
	if err != nil {
		return nil, err
	}
	switch v := val.(type) {
	case string:
		return float64(len(v)), nil
	case []interface{}:
		return float64(len(v)), nil
	default:
		return 0, nil
	}
}

// evaluateRound rounds a number
func (p *ExpressionParser) evaluateRound(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	if len(args) < 1 {
		return nil, fmt.Errorf("ROUND requires at least 1 argument")
	}

	val, err := p.Evaluate(args[0], ctx)
	if err != nil {
		return nil, err
	}

	num := p.toFloat(val)
	decimals := 0

	if len(args) >= 2 {
		decVal, err := p.Evaluate(args[1], ctx)
		if err == nil {
			decimals = int(p.toFloat(decVal))
		}
	}

	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10
	}

	return float64(int(num*multiplier+0.5)) / multiplier, nil
}

// evaluateAbs returns absolute value
func (p *ExpressionParser) evaluateAbs(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	val, err := p.Evaluate(argsStr, ctx)
	if err != nil {
		return nil, err
	}
	num := p.toFloat(val)
	if num < 0 {
		return -num, nil
	}
	return num, nil
}

// evaluateMin returns minimum of values
func (p *ExpressionParser) evaluateMin(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	if len(args) == 0 {
		return nil, fmt.Errorf("MIN requires at least 1 argument")
	}

	var minVal *float64
	for _, arg := range args {
		val, err := p.Evaluate(arg, ctx)
		if err != nil {
			continue
		}
		num := p.toFloat(val)
		if minVal == nil || num < *minVal {
			minVal = &num
		}
	}
	if minVal == nil {
		return nil, nil
	}
	return *minVal, nil
}

// evaluateMax returns maximum of values
func (p *ExpressionParser) evaluateMax(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	args := p.parseArgs(argsStr)
	if len(args) == 0 {
		return nil, fmt.Errorf("MAX requires at least 1 argument")
	}

	var maxVal *float64
	for _, arg := range args {
		val, err := p.Evaluate(arg, ctx)
		if err != nil {
			continue
		}
		num := p.toFloat(val)
		if maxVal == nil || num > *maxVal {
			maxVal = &num
		}
	}
	if maxVal == nil {
		return nil, nil
	}
	return *maxVal, nil
}

// evaluateConcat concatenates strings
func (p *ExpressionParser) evaluateConcat(argsStr string, ctx *ExpressionContext) (interface{}, error) {
	// Handle + operator for string concatenation
	if strings.Contains(argsStr, " + ") {
		parts := strings.Split(argsStr, " + ")
		var result strings.Builder
		for _, part := range parts {
			val, err := p.Evaluate(strings.TrimSpace(part), ctx)
			if err != nil {
				continue
			}
			result.WriteString(fmt.Sprintf("%v", val))
		}
		return result.String(), nil
	}

	// Handle CONCAT(a, b, c) function
	args := p.parseArgs(argsStr)
	var result strings.Builder
	for _, arg := range args {
		val, err := p.Evaluate(arg, ctx)
		if err != nil {
			continue
		}
		result.WriteString(fmt.Sprintf("%v", val))
	}
	return result.String(), nil
}

// =============================================================================
// Comparison
// =============================================================================

// findTopLevelOperator finds an operator that's not inside parentheses
func (p *ExpressionParser) findTopLevelOperator(expr, op string) int {
	depth := 0
	for i := 0; i <= len(expr)-len(op); i++ {
		if expr[i] == '(' {
			depth++
		} else if expr[i] == ')' {
			depth--
		} else if depth == 0 && expr[i:i+len(op)] == op {
			return i
		}
	}
	return -1
}

// compare compares two values with the given operator
func (p *ExpressionParser) compare(leftExpr, rightExpr, op string, ctx *ExpressionContext) (bool, error) {
	left, err := p.Evaluate(leftExpr, ctx)
	if err != nil {
		return false, err
	}

	right, err := p.Evaluate(rightExpr, ctx)
	if err != nil {
		return false, err
	}

	// Handle nil comparisons
	if left == nil || right == nil {
		switch op {
		case "==":
			return left == nil && right == nil, nil
		case "!=":
			return !(left == nil && right == nil), nil
		default:
			return false, nil
		}
	}

	// Try numeric comparison first
	leftNum, leftIsNum := p.tryFloat(left)
	rightNum, rightIsNum := p.tryFloat(right)

	if leftIsNum && rightIsNum {
		switch op {
		case "==":
			return leftNum == rightNum, nil
		case "!=":
			return leftNum != rightNum, nil
		case ">":
			return leftNum > rightNum, nil
		case "<":
			return leftNum < rightNum, nil
		case ">=":
			return leftNum >= rightNum, nil
		case "<=":
			return leftNum <= rightNum, nil
		}
	}

	// String comparison
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)

	switch op {
	case "==":
		return leftStr == rightStr, nil
	case "!=":
		return leftStr != rightStr, nil
	case ">":
		return leftStr > rightStr, nil
	case "<":
		return leftStr < rightStr, nil
	case ">=":
		return leftStr >= rightStr, nil
	case "<=":
		return leftStr <= rightStr, nil
	}

	return false, fmt.Errorf("unsupported operator: %s", op)
}

// valuesEqual checks if two values are equal
func (p *ExpressionParser) valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try numeric comparison
	aNum, aIsNum := p.tryFloat(a)
	bNum, bIsNum := p.tryFloat(b)
	if aIsNum && bIsNum {
		return aNum == bNum
	}

	// String comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// =============================================================================
// Utility Methods
// =============================================================================

// parseArgs splits a comma-separated argument string, respecting nested parens and strings
func (p *ExpressionParser) parseArgs(argsStr string) []string {
	var args []string
	var current strings.Builder
	depth := 0
	inString := false

	for i := 0; i < len(argsStr); i++ {
		ch := argsStr[i]

		switch ch {
		case '\'':
			inString = !inString
			current.WriteByte(ch)
		case '(':
			if !inString {
				depth++
			}
			current.WriteByte(ch)
		case ')':
			if !inString {
				depth--
			}
			current.WriteByte(ch)
		case ',':
			if !inString && depth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

// tryFloat attempts to convert a value to float64
func (p *ExpressionParser) tryFloat(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// toFloat converts a value to float64, returning 0 if not possible
func (p *ExpressionParser) toFloat(val interface{}) float64 {
	f, _ := p.tryFloat(val)
	return f
}
