package flow

import (
	"strings"
	"testing"
)

func TestExpressionParser_Evaluate(t *testing.T) {
	parser := NewExpressionParser()

	ctx := &ExpressionContext{
		Variables: map[string]interface{}{
			"name":      "John Doe",
			"age":       30,
			"score":     85.5,
			"isActive":  true,
			"company":   map[string]interface{}{"name": "Acme Corp", "size": 100},
			"emptyVar":  "",
			"nullVar":   nil,
			"leadScore": 75,
		},
		ScreenData: map[string]interface{}{
			"companySize": "51-200",
			"budget":      50000,
			"timeline":    "quarter",
		},
		Record: map[string]interface{}{
			"id":         "rec123",
			"name":       "Test Lead",
			"email":      "test@example.com",
			"account_id": "acc456",
		},
	}

	tests := []struct {
		name    string
		expr    string
		want    interface{}
		wantErr bool
	}{
		// String literals
		{"string literal", "'hello'", "hello", false},
		{"empty string", "''", "", false},
		{"string with spaces", "'hello world'", "hello world", false},

		// Number literals
		{"integer", "42", float64(42), false},
		{"float", "3.14", float64(3.14), false},
		{"negative number", "-10", float64(-10), false},

		// Boolean literals
		{"true", "true", true, false},
		{"false", "false", false, false},

		// Null
		{"null", "null", nil, false},

		// Variable access
		{"simple variable", "name", "John Doe", false},
		{"numeric variable", "age", 30, false},
		{"float variable", "score", 85.5, false},
		{"boolean variable", "isActive", true, false},
		{"nested variable", "company.name", "Acme Corp", false},
		{"screen data", "companySize", "51-200", false},

		// Record access
		{"record field", "$record.name", "Test Lead", false},
		{"record id", "$record.id", "rec123", false},

		// String concatenation
		{"concat with +", "'Hello ' + name", "Hello John Doe", false},
		{"concat record", "'ID: ' + $record.id", "ID: rec123", false},

		// CASE function
		{"case match first", "CASE(companySize, '1-10', 10, '11-50', 20, '51-200', 30, 5)", float64(30), false},
		{"case match second", "CASE(timeline, 'immediate', 40, 'quarter', 30, 'year', 20, 5)", float64(30), false},
		{"case default", "CASE(name, 'Alice', 1, 'Bob', 2, 0)", float64(0), false},

		// IF function
		{"if true", "IF(age > 25, 'adult', 'young')", "adult", false},
		{"if false", "IF(age < 25, 'young', 'adult')", "adult", false},
		{"if with variable", "IF(leadScore >= 70, 'hot', 'cold')", "hot", false},

		// COALESCE function
		{"coalesce first", "COALESCE(name, 'default')", "John Doe", false},
		{"coalesce skip null", "COALESCE(nullVar, name)", "John Doe", false},
		{"coalesce skip empty", "COALESCE(emptyVar, 'fallback')", "fallback", false},

		// String functions
		{"upper", "UPPER(name)", "JOHN DOE", false},
		{"lower", "LOWER(name)", "john doe", false},
		{"len string", "LEN(name)", float64(8), false},

		// Math functions
		{"round", "ROUND(score)", float64(86), false},
		{"round decimals", "ROUND(score, 1)", float64(85.5), false},
		{"abs positive", "ABS(score)", float64(85.5), false},
		{"min", "MIN(age, score, leadScore)", float64(30), false},
		{"max", "MAX(age, score, leadScore)", float64(85.5), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Evaluate(tt.expr, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !compareValues(got, tt.want) {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestExpressionParser_EvaluateBool(t *testing.T) {
	parser := NewExpressionParser()

	ctx := &ExpressionContext{
		Variables: map[string]interface{}{
			"score":    85,
			"name":     "John",
			"isActive": true,
			"count":    0,
		},
		ScreenData: map[string]interface{}{
			"budget": 50000,
		},
		Record: nil,
	}

	tests := []struct {
		name      string
		condition string
		want      bool
		wantErr   bool
	}{
		// Comparison operators
		{"greater than true", "score > 80", true, false},
		{"greater than false", "score > 90", false, false},
		{"less than true", "score < 90", true, false},
		{"less than false", "score < 80", false, false},
		{"greater equal true", "score >= 85", true, false},
		{"greater equal false", "score >= 86", false, false},
		{"less equal true", "score <= 85", true, false},
		{"less equal false", "score <= 84", false, false},
		{"equals true", "score == 85", true, false},
		{"equals false", "score == 80", false, false},
		{"not equals true", "score != 80", true, false},
		{"not equals false", "score != 85", false, false},

		// String comparison
		{"string equals", "name == 'John'", true, false},
		{"string not equals", "name != 'Jane'", true, false},

		// Boolean variable
		{"bool variable true", "isActive", true, false},

		// Logical operators
		{"AND true", "score > 80 AND name == 'John'", true, false},
		{"AND false", "score > 80 AND name == 'Jane'", false, false},
		{"OR true first", "score > 90 OR name == 'John'", true, false},
		{"OR true second", "score > 80 OR name == 'Jane'", true, false},
		{"OR false", "score > 90 OR name == 'Jane'", false, false},
		{"NOT true", "NOT count", true, false},
		{"NOT false", "NOT isActive", false, false},

		// Complex expressions
		{"complex", "score >= 70 AND budget > 40000", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.EvaluateBool(tt.condition, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpressionParser_InterpolateString(t *testing.T) {
	parser := NewExpressionParser()

	ctx := &ExpressionContext{
		Variables: map[string]interface{}{
			"name":  "John",
			"score": 85,
			"company": map[string]interface{}{
				"name": "Acme",
			},
		},
		ScreenData: map[string]interface{}{},
		Record: map[string]interface{}{
			"id": "rec123",
		},
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"simple variable", "Hello {{name}}!", "Hello John!"},
		{"multiple variables", "{{name}} scored {{score}}", "John scored 85"},
		{"nested variable", "Company: {{company.name}}", "Company: Acme"},
		{"no variables", "Plain text", "Plain text"},
		{"unknown variable", "Hello {{unknown}}!", "Hello {{unknown}}!"},
		{"mixed", "User {{name}} (ID: {{score}})", "User John (ID: 85)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.InterpolateString(tt.template, ctx)
			if got != tt.want {
				t.Errorf("InterpolateString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpressionParser_NOW(t *testing.T) {
	parser := NewExpressionParser()
	ctx := &ExpressionContext{
		Variables:  map[string]interface{}{},
		ScreenData: map[string]interface{}{},
		Record:     nil,
	}

	// Test NOW() returns a valid timestamp
	result, err := parser.Evaluate("NOW()", ctx)
	if err != nil {
		t.Errorf("NOW() returned error: %v", err)
	}
	if result == nil {
		t.Error("NOW() returned nil")
	}

	str, ok := result.(string)
	if !ok {
		t.Errorf("NOW() returned %T, expected string", result)
	}

	// Should contain date components
	if !strings.Contains(str, "T") {
		t.Errorf("NOW() result doesn't look like RFC3339: %s", str)
	}

	// Test NOW(90 DAYS)
	result2, err := parser.Evaluate("NOW(90 DAYS)", ctx)
	if err != nil {
		t.Errorf("NOW(90 DAYS) returned error: %v", err)
	}
	if result2 == nil {
		t.Error("NOW(90 DAYS) returned nil")
	}
}

func TestExpressionParser_ParseArgs(t *testing.T) {
	parser := NewExpressionParser()

	tests := []struct {
		name    string
		argsStr string
		want    []string
	}{
		{"simple", "a, b, c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"nested parens", "CASE(x, 1, 2), y", []string{"CASE(x, 1, 2)", "y"}},
		{"strings with commas", "'hello, world', b", []string{"'hello, world'", "b"}},
		{"complex", "CASE(a, '1', IF(b, 'x', 'y'), 0), z", []string{"CASE(a, '1', IF(b, 'x', 'y'), 0)", "z"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.parseArgs(tt.argsStr)
			if len(got) != len(tt.want) {
				t.Errorf("parseArgs() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLeadScoringExpression(t *testing.T) {
	// Test the actual lead scoring expression from our design
	parser := NewExpressionParser()

	ctx := &ExpressionContext{
		Variables:  map[string]interface{}{},
		ScreenData: map[string]interface{}{
			"companySize": "51-200",
			"budget":      float64(60000),
			"timeline":    "quarter",
		},
		Record: nil,
	}

	// Test each component of the scoring
	sizeScore, err := parser.Evaluate("CASE(companySize, '200+', 40, '51-200', 30, '11-50', 20, 10)", ctx)
	if err != nil {
		t.Errorf("Size score error: %v", err)
	}
	if sizeScore != float64(30) {
		t.Errorf("Size score = %v, want 30", sizeScore)
	}

	timeScore, err := parser.Evaluate("CASE(timeline, 'immediate', 40, 'quarter', 30, 'year', 20, 5)", ctx)
	if err != nil {
		t.Errorf("Timeline score error: %v", err)
	}
	if timeScore != float64(30) {
		t.Errorf("Timeline score = %v, want 30", timeScore)
	}

	budgetScore, err := parser.Evaluate("IF(budget > 50000, 20, IF(budget > 10000, 10, 0))", ctx)
	if err != nil {
		t.Errorf("Budget score error: %v", err)
	}
	if budgetScore != float64(20) {
		t.Errorf("Budget score = %v, want 20", budgetScore)
	}
}

func TestArithmeticExpressions(t *testing.T) {
	parser := NewExpressionParser()

	ctx := &ExpressionContext{
		Variables: map[string]interface{}{
			"a": float64(10),
			"b": float64(5),
			"companySize": "200+",
			"budget":      float64(75000),
		},
		ScreenData: map[string]interface{}{},
		Record:     nil,
	}

	tests := []struct {
		name    string
		expr    string
		want    float64
		wantErr bool
	}{
		{"simple add", "a + b", 15, false},
		{"simple subtract", "a - b", 5, false},
		{"simple multiply", "a * b", 50, false},
		{"simple divide", "a / b", 2, false},
		{"func + func", "CASE(companySize, '200+', 40, 10) + IF(budget > 50000, 20, 0)", 60, false},
		{"complex", "CASE(companySize, '200+', 40, '51-200', 30, '11-50', 20, 10) + IF(budget > 50000, 20, IF(budget > 10000, 10, 0))", 60, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Evaluate(tt.expr, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFloat, ok := got.(float64); ok {
				if gotFloat != tt.want {
					t.Errorf("Evaluate() = %v, want %v", gotFloat, tt.want)
				}
			} else {
				t.Errorf("Evaluate() returned non-float: %v (%T)", got, got)
			}
		})
	}
}

// Helper to compare values accounting for type differences
func compareValues(got, want interface{}) bool {
	if got == nil && want == nil {
		return true
	}
	if got == nil || want == nil {
		return false
	}

	// Handle numeric comparisons
	switch g := got.(type) {
	case float64:
		switch w := want.(type) {
		case float64:
			return g == w
		case int:
			return g == float64(w)
		}
	case int:
		switch w := want.(type) {
		case float64:
			return float64(g) == w
		case int:
			return g == w
		}
	}

	return got == want
}
