package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// ValidationIssue represents a validation error for a CSV record
type ValidationIssue struct {
	Row       int    `json:"row"`
	Column    string `json:"column"`
	FieldName string `json:"fieldName"`
	Value     string `json:"value"`
	IssueType string `json:"issueType"` // "invalid_enum", "invalid_type", "required_missing", "invalid_format"
	Message   string `json:"message"`
	Expected  string `json:"expected,omitempty"` // e.g., valid enum options
}

// AnalyzeResult contains validation results for CSV data
type AnalyzeResult struct {
	Valid           bool              `json:"valid"`
	TotalRows       int               `json:"totalRows"`
	ValidRows       int               `json:"validRows"`
	InvalidRows     int               `json:"invalidRows"`
	Issues          []ValidationIssue `json:"issues"`
	MappedFields    []string          `json:"mappedFields"`    // Fields that have data
	MissingRequired []string          `json:"missingRequired"` // Required fields not mapped
}

// CSVValidatorService validates CSV records against entity field definitions
type CSVValidatorService struct{}

// NewCSVValidatorService creates a new CSV validator
func NewCSVValidatorService() *CSVValidatorService {
	return &CSVValidatorService{}
}

// Validate validates all records against field definitions
func (v *CSVValidatorService) Validate(records []map[string]interface{}, fields []entity.FieldDef) *AnalyzeResult {
	result := &AnalyzeResult{
		TotalRows:       len(records),
		Issues:          []ValidationIssue{},
		MappedFields:    []string{},
		MissingRequired: []string{},
	}

	// Build field lookup map
	fieldMap := make(map[string]*entity.FieldDef)
	requiredFields := make(map[string]bool)
	for i := range fields {
		field := &fields[i]
		fieldMap[field.Name] = field
		if field.IsRequired {
			requiredFields[field.Name] = true
		}
	}

	// Track which fields are mapped
	mappedFields := make(map[string]bool)
	if len(records) > 0 {
		for key := range records[0] {
			mappedFields[key] = true
			result.MappedFields = append(result.MappedFields, key)
		}
	}

	// Check for missing required fields
	for fieldName := range requiredFields {
		if !mappedFields[fieldName] {
			result.MissingRequired = append(result.MissingRequired, fieldName)
		}
	}

	// Validate each record
	validRowCount := 0
	invalidRowsMap := make(map[int]bool)

	for rowIdx, record := range records {
		rowNumber := rowIdx + 1 // 1-indexed for user display
		rowValid := true

		// Sanitize all string values first
		for key, val := range record {
			if strVal, ok := val.(string); ok {
				record[key] = v.sanitizeString(strVal)
			}
		}

		// Validate each field in the record
		for fieldName, value := range record {
			field, exists := fieldMap[fieldName]
			if !exists {
				continue // Skip unmapped fields
			}

			// Convert value to string for validation
			strVal := fmt.Sprintf("%v", value)

			// Check required fields
			if field.IsRequired && (value == nil || strVal == "") {
				issue := ValidationIssue{
					Row:       rowNumber,
					Column:    field.Label,
					FieldName: fieldName,
					Value:     strVal,
					IssueType: "required_missing",
					Message:   fmt.Sprintf("Required field '%s' is empty", field.Label),
				}
				result.Issues = append(result.Issues, issue)
				rowValid = false
				continue
			}

			// Skip validation for empty optional fields
			if strVal == "" {
				continue
			}

			// Validate based on field type
			if issue := v.validateFieldValue(field, strVal, rowNumber); issue != nil {
				result.Issues = append(result.Issues, *issue)
				rowValid = false
			}
		}

		// Check for missing required fields in this row
		for fieldName := range requiredFields {
			if _, exists := record[fieldName]; !exists {
				field := fieldMap[fieldName]
				issue := ValidationIssue{
					Row:       rowNumber,
					Column:    field.Label,
					FieldName: fieldName,
					Value:     "",
					IssueType: "required_missing",
					Message:   fmt.Sprintf("Required field '%s' is missing", field.Label),
				}
				result.Issues = append(result.Issues, issue)
				rowValid = false
			}
		}

		if rowValid {
			validRowCount++
		} else {
			invalidRowsMap[rowNumber] = true
		}
	}

	result.ValidRows = validRowCount
	result.InvalidRows = len(invalidRowsMap)
	result.Valid = result.InvalidRows == 0

	return result
}

// validateFieldValue validates a single field value based on its type
func (v *CSVValidatorService) validateFieldValue(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	switch field.Type {
	case entity.FieldTypeEnum:
		return v.validateEnum(field, value, rowNumber)
	case entity.FieldTypeMultiEnum:
		return v.validateMultiEnum(field, value, rowNumber)
	case entity.FieldTypeInt:
		return v.validateInt(field, value, rowNumber)
	case entity.FieldTypeFloat, entity.FieldTypeCurrency:
		return v.validateFloat(field, value, rowNumber)
	case entity.FieldTypeBool:
		return v.validateBool(field, value, rowNumber)
	case entity.FieldTypeDate:
		return v.validateDate(field, value, rowNumber)
	case entity.FieldTypeDatetime:
		return v.validateDatetime(field, value, rowNumber)
	case entity.FieldTypeEmail:
		return v.validateEmail(field, value, rowNumber)
	case entity.FieldTypeURL:
		return v.validateURL(field, value, rowNumber)
	}

	return nil
}

// validateEnum validates enum field values
func (v *CSVValidatorService) validateEnum(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	if field.Options == nil {
		return nil
	}

	var options []string
	if err := json.Unmarshal([]byte(*field.Options), &options); err != nil {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_enum",
			Message:   "Failed to parse enum options",
		}
	}

	// Check if value matches any option (case-sensitive)
	for _, opt := range options {
		if value == opt {
			return nil
		}
	}

	return &ValidationIssue{
		Row:       rowNumber,
		Column:    field.Label,
		FieldName: field.Name,
		Value:     value,
		IssueType: "invalid_enum",
		Message:   fmt.Sprintf("'%s' is not a valid option", value),
		Expected:  fmt.Sprintf("Valid options: %s", strings.Join(options, ", ")),
	}
}

// validateMultiEnum validates multi-enum field values
func (v *CSVValidatorService) validateMultiEnum(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	if field.Options == nil {
		return nil
	}

	var options []string
	if err := json.Unmarshal([]byte(*field.Options), &options); err != nil {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_enum",
			Message:   "Failed to parse enum options",
		}
	}

	// Split on comma and validate each value
	values := strings.Split(value, ",")
	var invalidValues []string

	for _, val := range values {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		found := false
		for _, opt := range options {
			if val == opt {
				found = true
				break
			}
		}
		if !found {
			invalidValues = append(invalidValues, val)
		}
	}

	if len(invalidValues) > 0 {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_enum",
			Message:   fmt.Sprintf("Invalid values: %s", strings.Join(invalidValues, ", ")),
			Expected:  fmt.Sprintf("Valid options: %s", strings.Join(options, ", ")),
		}
	}

	return nil
}

// validateInt validates integer field values
func (v *CSVValidatorService) validateInt(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("'%s' is not a valid integer", value),
			Expected:  "Must be a whole number",
		}
	}

	// Check min/max constraints
	if field.MinValue != nil && float64(intVal) < *field.MinValue {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("Value %d is below minimum %v", intVal, *field.MinValue),
		}
	}

	if field.MaxValue != nil && float64(intVal) > *field.MaxValue {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("Value %d is above maximum %v", intVal, *field.MaxValue),
		}
	}

	return nil
}

// validateFloat validates float/currency field values
func (v *CSVValidatorService) validateFloat(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("'%s' is not a valid number", value),
			Expected:  "Must be a decimal number",
		}
	}

	// Check min/max constraints
	if field.MinValue != nil && floatVal < *field.MinValue {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("Value %f is below minimum %v", floatVal, *field.MinValue),
		}
	}

	if field.MaxValue != nil && floatVal > *field.MaxValue {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_type",
			Message:   fmt.Sprintf("Value %f is above maximum %v", floatVal, *field.MaxValue),
		}
	}

	return nil
}

// validateBool validates boolean field values
func (v *CSVValidatorService) validateBool(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	lowerVal := strings.ToLower(strings.TrimSpace(value))
	validValues := []string{"true", "false", "1", "0", "yes", "no"}

	for _, valid := range validValues {
		if lowerVal == valid {
			return nil
		}
	}

	return &ValidationIssue{
		Row:       rowNumber,
		Column:    field.Label,
		FieldName: field.Name,
		Value:     value,
		IssueType: "invalid_type",
		Message:   fmt.Sprintf("'%s' is not a valid boolean", value),
		Expected:  "Must be: true, false, 1, 0, yes, or no",
	}
}

// validateDate validates date field values
func (v *CSVValidatorService) validateDate(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	// Accept YYYY-MM-DD format
	_, err := time.Parse("2006-01-02", value)
	if err != nil {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_format",
			Message:   fmt.Sprintf("'%s' is not a valid date", value),
			Expected:  "Must be in YYYY-MM-DD format",
		}
	}

	return nil
}

// validateDatetime validates datetime field values
func (v *CSVValidatorService) validateDatetime(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	// Accept YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}

	for _, format := range formats {
		if _, err := time.Parse(format, value); err == nil {
			return nil
		}
	}

	return &ValidationIssue{
		Row:       rowNumber,
		Column:    field.Label,
		FieldName: field.Name,
		Value:     value,
		IssueType: "invalid_format",
		Message:   fmt.Sprintf("'%s' is not a valid datetime", value),
		Expected:  "Must be in YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS format",
	}
}

// validateEmail validates email field values
func (v *CSVValidatorService) validateEmail(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	// Basic email regex
	emailRegex := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	if !emailRegex.MatchString(value) {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_format",
			Message:   fmt.Sprintf("'%s' is not a valid email address", value),
			Expected:  "Must be in format: user@domain.com",
		}
	}

	return nil
}

// validateURL validates URL field values
func (v *CSVValidatorService) validateURL(field *entity.FieldDef, value string, rowNumber int) *ValidationIssue {
	parsedURL, err := url.Parse(value)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return &ValidationIssue{
			Row:       rowNumber,
			Column:    field.Label,
			FieldName: field.Name,
			Value:     value,
			IssueType: "invalid_format",
			Message:   fmt.Sprintf("'%s' is not a valid URL", value),
			Expected:  "Must be a valid HTTP or HTTPS URL",
		}
	}

	return nil
}

// sanitizeString removes HTML tags and escapes special characters
func (v *CSVValidatorService) sanitizeString(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)

	// Strip HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	s = htmlTagRegex.ReplaceAllString(s, "")

	// Escape special characters (& must be first to avoid double-escaping)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	return s
}
