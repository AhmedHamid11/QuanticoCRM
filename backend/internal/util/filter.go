package util

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// FilterResult contains the parsed filter clause and its parameters
type FilterResult struct {
	WhereClause string
	Args        []interface{}
}

// ParseFilter parses a SQL-like filter string and returns a safe WHERE clause
// Validates field names against the provided field definitions to prevent SQL injection
func ParseFilter(filterStr string, fields []entity.FieldDef) (*FilterResult, error) {
	if strings.TrimSpace(filterStr) == "" {
		return &FilterResult{WhereClause: "", Args: []interface{}{}}, nil
	}

	// Build a set of valid field names (both camelCase and snake_case)
	validFields := make(map[string]bool)
	for _, f := range fields {
		validFields[f.Name] = true
		validFields[CamelToSnake(f.Name)] = true
	}
	// Always allow standard fields
	standardFields := []string{
		"id", "org_id", "created_at", "modified_at", "created_by_id", "modified_by_id",
		"createdAt", "modifiedAt", "createdById", "modifiedById",
		"name", "deleted",
	}
	for _, f := range standardFields {
		validFields[f] = true
		validFields[CamelToSnake(f)] = true
	}

	parser := &filterParser{
		input:       filterStr,
		validFields: validFields,
		args:        []interface{}{},
	}

	whereClause, err := parser.parse()
	if err != nil {
		return nil, err
	}

	return &FilterResult{
		WhereClause: whereClause,
		Args:        parser.args,
	}, nil
}

// ParseFilterWithColumns parses a filter string using a predefined column map
// This is for hardcoded entities where we don't have field metadata
func ParseFilterWithColumns(filterStr string, validColumns map[string]bool, tableAlias string) (*FilterResult, error) {
	if strings.TrimSpace(filterStr) == "" {
		return &FilterResult{WhereClause: "", Args: []interface{}{}}, nil
	}

	// Build valid fields map with both camelCase and snake_case
	validFields := make(map[string]bool)
	for col := range validColumns {
		validFields[col] = true
		validFields[SnakeToCamel(col)] = true
	}

	parser := &filterParser{
		input:       filterStr,
		validFields: validFields,
		args:        []interface{}{},
		tableAlias:  tableAlias,
	}

	whereClause, err := parser.parse()
	if err != nil {
		return nil, err
	}

	return &FilterResult{
		WhereClause: whereClause,
		Args:        parser.args,
	}, nil
}

type filterParser struct {
	input       string
	pos         int
	validFields map[string]bool
	args        []interface{}
	tableAlias  string
}

// getTableAlias returns the table alias for column prefixing, defaulting to "t" if not set
func (p *filterParser) getTableAlias() string {
	if p.tableAlias == "" {
		return "t"
	}
	return p.tableAlias
}

func (p *filterParser) parse() (string, error) {
	return p.parseExpression()
}

func (p *filterParser) parseExpression() (string, error) {
	left, err := p.parseTerm()
	if err != nil {
		return "", err
	}

	for {
		p.skipWhitespace()
		if p.matchKeyword("AND") {
			right, err := p.parseTerm()
			if err != nil {
				return "", err
			}
			left = fmt.Sprintf("(%s AND %s)", left, right)
		} else if p.matchKeyword("OR") {
			right, err := p.parseTerm()
			if err != nil {
				return "", err
			}
			left = fmt.Sprintf("(%s OR %s)", left, right)
		} else {
			break
		}
	}

	return left, nil
}

func (p *filterParser) parseTerm() (string, error) {
	p.skipWhitespace()

	// Handle parentheses
	if p.peek() == '(' {
		p.pos++
		expr, err := p.parseExpression()
		if err != nil {
			return "", err
		}
		p.skipWhitespace()
		if p.peek() != ')' {
			return "", fmt.Errorf("expected closing parenthesis")
		}
		p.pos++
		return fmt.Sprintf("(%s)", expr), nil
	}

	// Handle NOT
	if p.matchKeyword("NOT") {
		term, err := p.parseTerm()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("NOT %s", term), nil
	}

	return p.parseCondition()
}

func (p *filterParser) parseCondition() (string, error) {
	p.skipWhitespace()

	// Parse field name
	field := p.parseIdentifier()
	if field == "" {
		return "", fmt.Errorf("expected field name at position %d", p.pos)
	}

	// Convert to snake_case for database
	snakeField := CamelToSnake(field)

	// Validate field name
	if !p.validFields[field] && !p.validFields[snakeField] {
		return "", fmt.Errorf("invalid field name: %s", field)
	}

	p.skipWhitespace()

	// Check for IS NULL / IS NOT NULL
	if p.matchKeyword("IS") {
		p.skipWhitespace()
		if p.matchKeyword("NOT") {
			p.skipWhitespace()
			if !p.matchKeyword("NULL") {
				return "", fmt.Errorf("expected NULL after IS NOT")
			}
			return fmt.Sprintf("%s.%s IS NOT NULL", p.getTableAlias(), snakeField), nil
		}
		if !p.matchKeyword("NULL") {
			return "", fmt.Errorf("expected NULL after IS")
		}
		return fmt.Sprintf("%s.%s IS NULL", p.getTableAlias(), snakeField), nil
	}

	// Check for IN / NOT IN
	if p.matchKeyword("NOT") {
		p.skipWhitespace()
		if p.matchKeyword("IN") {
			return p.parseInClause(snakeField, true)
		}
		return "", fmt.Errorf("expected IN after NOT")
	}
	if p.matchKeyword("IN") {
		return p.parseInClause(snakeField, false)
	}

	// Parse operator
	op := p.parseOperator()
	if op == "" {
		return "", fmt.Errorf("expected operator at position %d", p.pos)
	}

	p.skipWhitespace()

	// Parse value
	value, err := p.parseValue()
	if err != nil {
		return "", err
	}

	// Handle LIKE operator
	if strings.ToUpper(op) == "LIKE" {
		p.args = append(p.args, value)
		return fmt.Sprintf("%s.%s LIKE ?", p.getTableAlias(), snakeField), nil
	}

	// Handle comparison operators
	p.args = append(p.args, value)
	return fmt.Sprintf("%s.%s %s ?", p.getTableAlias(), snakeField, op), nil
}

func (p *filterParser) parseInClause(field string, notIn bool) (string, error) {
	p.skipWhitespace()
	if p.peek() != '(' {
		return "", fmt.Errorf("expected ( after IN")
	}
	p.pos++

	var values []interface{}
	for {
		p.skipWhitespace()
		value, err := p.parseValue()
		if err != nil {
			return "", err
		}
		values = append(values, value)

		p.skipWhitespace()
		if p.peek() == ')' {
			p.pos++
			break
		}
		if p.peek() != ',' {
			return "", fmt.Errorf("expected , or ) in IN clause")
		}
		p.pos++
	}

	placeholders := make([]string, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		p.args = append(p.args, v)
	}

	op := "IN"
	if notIn {
		op = "NOT IN"
	}
	return fmt.Sprintf("%s.%s %s (%s)", p.getTableAlias(), field, op, strings.Join(placeholders, ", ")), nil
}

func (p *filterParser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos]
}

func (p *filterParser) parseOperator() string {
	p.skipWhitespace()

	// Check for two-character operators first
	if p.pos+1 < len(p.input) {
		twoChar := p.input[p.pos : p.pos+2]
		if twoChar == "<=" || twoChar == ">=" || twoChar == "!=" || twoChar == "<>" {
			p.pos += 2
			if twoChar == "<>" {
				return "!="
			}
			return twoChar
		}
	}

	// Single character operators
	if p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '=' || c == '<' || c == '>' {
			p.pos++
			return string(c)
		}
	}

	// LIKE operator
	if p.matchKeyword("LIKE") {
		return "LIKE"
	}

	return ""
}

func (p *filterParser) parseValue() (interface{}, error) {
	p.skipWhitespace()

	// String value
	if p.peek() == '\'' {
		return p.parseString()
	}

	// Check for TODAY keyword with optional arithmetic
	if p.matchKeyword("TODAY") {
		return p.parseTodayExpression()
	}

	// Number or boolean
	return p.parseNumberOrKeyword()
}

// parseTodayExpression parses TODAY with optional arithmetic: TODAY, TODAY + 7, TODAY - 2w, TODAY + 1m
func (p *filterParser) parseTodayExpression() (string, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	p.skipWhitespace()

	// Check for + or -
	if p.pos >= len(p.input) || (p.peek() != '+' && p.peek() != '-') {
		// Just TODAY, no arithmetic
		return today.Format("2006-01-02"), nil
	}

	// Parse operator
	op := p.peek()
	p.pos++
	p.skipWhitespace()

	// Parse number
	numStart := p.pos
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
	}
	if p.pos == numStart {
		return "", fmt.Errorf("expected number after TODAY %c", op)
	}

	numStr := p.input[numStart:p.pos]
	var num int
	for _, c := range numStr {
		num = num*10 + int(c-'0')
	}

	// Parse optional unit (d=days, w=weeks, m=months), default is days
	unit := byte('d')
	if p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == 'd' || c == 'D' || c == 'w' || c == 'W' || c == 'm' || c == 'M' {
			unit = c
			p.pos++
		}
	}

	// Calculate the date offset
	var result time.Time
	if op == '-' {
		num = -num
	}

	switch unit {
	case 'd', 'D':
		result = today.AddDate(0, 0, num)
	case 'w', 'W':
		result = today.AddDate(0, 0, num*7)
	case 'm', 'M':
		result = today.AddDate(0, num, 0)
	default:
		result = today.AddDate(0, 0, num)
	}

	return result.Format("2006-01-02"), nil
}

func (p *filterParser) parseString() (string, error) {
	if p.peek() != '\'' {
		return "", fmt.Errorf("expected string at position %d", p.pos)
	}
	p.pos++

	var result strings.Builder
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '\'' {
			// Check for escaped quote
			if p.pos+1 < len(p.input) && p.input[p.pos+1] == '\'' {
				result.WriteByte('\'')
				p.pos += 2
				continue
			}
			p.pos++
			return result.String(), nil
		}
		result.WriteByte(c)
		p.pos++
	}
	return "", fmt.Errorf("unterminated string")
}

func (p *filterParser) parseNumberOrKeyword() (interface{}, error) {
	start := p.pos

	// Check for NULL
	if p.matchKeyword("NULL") {
		return nil, nil
	}

	// Check for boolean
	if p.matchKeyword("TRUE") {
		return true, nil
	}
	if p.matchKeyword("FALSE") {
		return false, nil
	}

	// Parse number
	hasDecimal := false
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c >= '0' && c <= '9' {
			p.pos++
		} else if c == '.' && !hasDecimal {
			hasDecimal = true
			p.pos++
		} else if c == '-' && p.pos == start {
			p.pos++
		} else {
			break
		}
	}

	if p.pos == start {
		return nil, fmt.Errorf("expected value at position %d", p.pos)
	}

	return p.input[start:p.pos], nil
}

func (p *filterParser) skipWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *filterParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *filterParser) matchKeyword(keyword string) bool {
	p.skipWhitespace()
	if p.pos+len(keyword) > len(p.input) {
		return false
	}

	// Check if the keyword matches (case-insensitive)
	if strings.EqualFold(p.input[p.pos:p.pos+len(keyword)], keyword) {
		// Make sure it's not part of a larger identifier
		nextPos := p.pos + len(keyword)
		if nextPos < len(p.input) {
			c := p.input[nextPos]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				return false
			}
		}
		p.pos += len(keyword)
		return true
	}
	return false
}

// ValidateFilterSyntax does a quick syntax check without full parsing
func ValidateFilterSyntax(filterStr string) error {
	if strings.TrimSpace(filterStr) == "" {
		return nil
	}

	// Check for dangerous patterns (extra safety)
	dangerous := regexp.MustCompile(`(?i)(;|--|\bDROP\b|\bDELETE\b|\bUPDATE\b|\bINSERT\b|\bALTER\b|\bCREATE\b|\bTRUNCATE\b|\bEXEC\b|\bUNION\b)`)
	if dangerous.MatchString(filterStr) {
		return fmt.Errorf("invalid characters or keywords in filter")
	}

	return nil
}
