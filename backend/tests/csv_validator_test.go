package tests

import (
	"testing"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/service"
)

func TestCSVValidator_ValidEnum(t *testing.T) {
	validator := service.NewCSVValidatorService()

	options := `["Active","Inactive","Pending"]`
	fields := []entity.FieldDef{
		{
			Name:    "status",
			Label:   "Status",
			Type:    entity.FieldTypeEnum,
			Options: &options,
		},
	}

	records := []map[string]interface{}{
		{"status": "Active"},
		{"status": "Pending"},
	}

	result := validator.Validate(records, fields)

	if !result.Valid {
		t.Errorf("Expected valid=true, got false. Issues: %v", result.Issues)
	}
	if result.InvalidRows != 0 {
		t.Errorf("Expected 0 invalid rows, got %d", result.InvalidRows)
	}
}

func TestCSVValidator_InvalidEnum(t *testing.T) {
	validator := service.NewCSVValidatorService()

	options := `["Active","Inactive","Pending"]`
	fields := []entity.FieldDef{
		{
			Name:    "status",
			Label:   "Status",
			Type:    entity.FieldTypeEnum,
			Options: &options,
		},
	}

	records := []map[string]interface{}{
		{"status": "Invalid"},
	}

	result := validator.Validate(records, fields)

	if result.Valid {
		t.Error("Expected valid=false for invalid enum value")
	}
	if len(result.Issues) == 0 {
		t.Error("Expected validation issues, got none")
	}
	if result.Issues[0].IssueType != "invalid_enum" {
		t.Errorf("Expected issue type 'invalid_enum', got '%s'", result.Issues[0].IssueType)
	}
}

func TestCSVValidator_InvalidInt(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:  "age",
			Label: "Age",
			Type:  entity.FieldTypeInt,
		},
	}

	records := []map[string]interface{}{
		{"age": "abc"},
	}

	result := validator.Validate(records, fields)

	if result.Valid {
		t.Error("Expected valid=false for invalid int value")
	}
	if len(result.Issues) == 0 {
		t.Error("Expected validation issues, got none")
	}
	if result.Issues[0].IssueType != "invalid_type" {
		t.Errorf("Expected issue type 'invalid_type', got '%s'", result.Issues[0].IssueType)
	}
}

func TestCSVValidator_RequiredFieldMissing(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:       "firstName",
			Label:      "First Name",
			Type:       entity.FieldTypeVarchar,
			IsRequired: true,
		},
		{
			Name:  "lastName",
			Label: "Last Name",
			Type:  entity.FieldTypeVarchar,
		},
	}

	// Record missing required field
	records := []map[string]interface{}{
		{"lastName": "Doe"},
	}

	result := validator.Validate(records, fields)

	if result.Valid {
		t.Error("Expected valid=false for missing required field")
	}
	if len(result.Issues) == 0 {
		t.Error("Expected validation issues for missing required field")
	}

	foundRequiredIssue := false
	for _, issue := range result.Issues {
		if issue.IssueType == "required_missing" && issue.FieldName == "firstName" {
			foundRequiredIssue = true
			break
		}
	}
	if !foundRequiredIssue {
		t.Error("Expected required_missing issue for firstName field")
	}
}

func TestCSVValidator_EmailFormat(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:  "email",
			Label: "Email",
			Type:  entity.FieldTypeEmail,
		},
	}

	// Test invalid email
	records := []map[string]interface{}{
		{"email": "not-an-email"},
	}

	result := validator.Validate(records, fields)

	if result.Valid {
		t.Error("Expected valid=false for invalid email format")
	}
	if len(result.Issues) == 0 {
		t.Error("Expected validation issues for invalid email")
	}

	// Test valid email
	records2 := []map[string]interface{}{
		{"email": "user@example.com"},
	}

	result2 := validator.Validate(records2, fields)

	if !result2.Valid {
		t.Errorf("Expected valid=true for valid email, got issues: %v", result2.Issues)
	}
}

func TestCSVValidator_MultiEnum(t *testing.T) {
	validator := service.NewCSVValidatorService()

	options := `["Red","Blue","Green","Yellow"]`
	fields := []entity.FieldDef{
		{
			Name:    "colors",
			Label:   "Colors",
			Type:    entity.FieldTypeMultiEnum,
			Options: &options,
		},
	}

	// Test valid multi-select
	records := []map[string]interface{}{
		{"colors": "Red,Blue"},
	}

	result := validator.Validate(records, fields)

	if !result.Valid {
		t.Errorf("Expected valid=true for valid multi-enum, got issues: %v", result.Issues)
	}

	// Test invalid multi-select
	records2 := []map[string]interface{}{
		{"colors": "Red,Purple"},
	}

	result2 := validator.Validate(records2, fields)

	if result2.Valid {
		t.Error("Expected valid=false for invalid multi-enum value")
	}
}

func TestCSVValidator_DateFormat(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:  "birthDate",
			Label: "Birth Date",
			Type:  entity.FieldTypeDate,
		},
	}

	// Test valid date
	records := []map[string]interface{}{
		{"birthDate": "1990-05-15"},
	}

	result := validator.Validate(records, fields)

	if !result.Valid {
		t.Errorf("Expected valid=true for valid date, got issues: %v", result.Issues)
	}

	// Test invalid date
	records2 := []map[string]interface{}{
		{"birthDate": "05/15/1990"},
	}

	result2 := validator.Validate(records2, fields)

	if result2.Valid {
		t.Error("Expected valid=false for invalid date format")
	}
}

func TestCSVValidator_BoolFormat(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:  "active",
			Label: "Active",
			Type:  entity.FieldTypeBool,
		},
	}

	// Test valid bool values
	validBools := []string{"true", "false", "1", "0", "yes", "no", "True", "FALSE", "YES"}
	for _, val := range validBools {
		records := []map[string]interface{}{
			{"active": val},
		}

		result := validator.Validate(records, fields)

		if !result.Valid {
			t.Errorf("Expected valid=true for bool value '%s', got issues: %v", val, result.Issues)
		}
	}

	// Test invalid bool
	records2 := []map[string]interface{}{
		{"active": "maybe"},
	}

	result2 := validator.Validate(records2, fields)

	if result2.Valid {
		t.Error("Expected valid=false for invalid bool value")
	}
}

func TestCSVValidator_URLFormat(t *testing.T) {
	validator := service.NewCSVValidatorService()

	fields := []entity.FieldDef{
		{
			Name:  "website",
			Label: "Website",
			Type:  entity.FieldTypeURL,
		},
	}

	// Test valid URL
	records := []map[string]interface{}{
		{"website": "https://example.com"},
	}

	result := validator.Validate(records, fields)

	if !result.Valid {
		t.Errorf("Expected valid=true for valid URL, got issues: %v", result.Issues)
	}

	// Test invalid URL
	records2 := []map[string]interface{}{
		{"website": "not-a-url"},
	}

	result2 := validator.Validate(records2, fields)

	if result2.Valid {
		t.Error("Expected valid=false for invalid URL")
	}
}
