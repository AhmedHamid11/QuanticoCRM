package service

import (
	"encoding/json"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
)

// LayoutService handles layout parsing and visibility evaluation
type LayoutService struct{}

// NewLayoutService creates a new LayoutService
func NewLayoutService() *LayoutService {
	return &LayoutService{}
}

// ParseLayoutData parses layout JSON and returns a structured representation
// Supports v1 (flat array), v1.5 (sections with rows), and v2 (sections with visibility) formats
func (s *LayoutService) ParseLayoutData(layoutJSON string) (*entity.LayoutDataParsed, error) {
	if layoutJSON == "" || layoutJSON == "[]" {
		return &entity.LayoutDataParsed{
			Version:  entity.LayoutVersionV1,
			V1Fields: []string{},
		}, nil
	}

	// First, try to parse as v2 (object with version field)
	var v2Data entity.LayoutDataV2
	if err := json.Unmarshal([]byte(layoutJSON), &v2Data); err == nil && v2Data.Version == entity.LayoutVersionV2 {
		return &entity.LayoutDataParsed{
			Version: entity.LayoutVersionV2,
			V2Data:  &v2Data,
		}, nil
	}

	// Try to parse as v1.5 (sections with rows format from provisioning)
	// Format: [{"label":"Overview","rows":[[{"field":"name"},...],...]},...]
	var sectionsWithRows []sectionWithRows
	if err := json.Unmarshal([]byte(layoutJSON), &sectionsWithRows); err == nil && len(sectionsWithRows) > 0 && sectionsWithRows[0].Label != "" {
		// Convert sections-with-rows to v2 format
		v2Layout := convertSectionsWithRowsToV2(sectionsWithRows)
		return &entity.LayoutDataParsed{
			Version: entity.LayoutVersionV2,
			V2Data:  v2Layout,
		}, nil
	}

	// Try to parse as v1 (flat array of strings)
	var v1Fields []string
	if err := json.Unmarshal([]byte(layoutJSON), &v1Fields); err == nil {
		return &entity.LayoutDataParsed{
			Version:  entity.LayoutVersionV1,
			V1Fields: v1Fields,
		}, nil
	}

	// Default to empty v1
	return &entity.LayoutDataParsed{
		Version:  entity.LayoutVersionV1,
		V1Fields: []string{},
	}, nil
}

// sectionWithRows represents the provisioned layout format with rows
type sectionWithRows struct {
	Label string          `json:"label"`
	Rows  [][]fieldInRow  `json:"rows"`
}

// fieldInRow represents a field in the rows format
type fieldInRow struct {
	Field string `json:"field"`
}

// convertSectionsWithRowsToV2 converts the provisioned sections-rows format to v2
func convertSectionsWithRowsToV2(sections []sectionWithRows) *entity.LayoutDataV2 {
	v2Sections := make([]entity.LayoutSectionV2, len(sections))

	for i, section := range sections {
		// Extract all fields from rows
		var fields []entity.LayoutFieldV2
		for _, row := range section.Rows {
			for _, fieldInRow := range row {
				if fieldInRow.Field != "" {
					fields = append(fields, entity.LayoutFieldV2{
						Name:       fieldInRow.Field,
						Visibility: entity.NewDefaultVisibility(),
					})
				}
			}
		}

		// Determine column count from first row with multiple fields
		columns := 2
		for _, row := range section.Rows {
			if len(row) > 0 {
				columns = len(row)
				if columns > 3 {
					columns = 3
				}
				break
			}
		}

		v2Sections[i] = entity.LayoutSectionV2{
			ID:          generateSectionID(section.Label),
			Label:       section.Label,
			Order:       i + 1,
			Collapsible: true,
			Collapsed:   false,
			Columns:     columns,
			Visibility:  entity.NewDefaultVisibility(),
			Fields:      fields,
		}
	}

	return &entity.LayoutDataV2{
		Version:  entity.LayoutVersionV2,
		Sections: v2Sections,
	}
}

// generateSectionID creates a URL-safe section ID from the label
func generateSectionID(label string) string {
	// Convert to lowercase and replace spaces with underscores
	id := strings.ToLower(label)
	id = strings.ReplaceAll(id, " ", "_")
	// Remove non-alphanumeric characters except underscores
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return "section_" + result.String()
}

// GetLayoutAsV2 returns layout data in v2 format, converting v1 if necessary
func (s *LayoutService) GetLayoutAsV2(layoutJSON string) (*entity.LayoutDataV2, error) {
	parsed, err := s.ParseLayoutData(layoutJSON)
	if err != nil {
		return nil, err
	}

	if parsed.Version == entity.LayoutVersionV2 {
		return parsed.V2Data, nil
	}

	// Convert v1 to v2
	return entity.ConvertV1ToV2(parsed.V1Fields), nil
}

// EvaluateVisibility checks if a visibility rule passes given the record data
func (s *LayoutService) EvaluateVisibility(rule entity.VisibilityRule, record map[string]interface{}) bool {
	switch rule.Type {
	case entity.VisibilityAlways:
		return true
	case entity.VisibilityNever:
		return false
	case entity.VisibilityConditional:
		return s.evaluateConditions(rule.Conditions, rule.Logic, record)
	default:
		return true // Default to visible
	}
}

// evaluateConditions evaluates a list of conditions with AND/OR logic
func (s *LayoutService) evaluateConditions(conditions []entity.VisibilityCondition, logic string, record map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true
	}

	isAnd := logic != "OR" // Default to AND

	for _, cond := range conditions {
		result := s.evaluateCondition(cond, record)
		if isAnd {
			if !result {
				return false // AND: any false makes all false
			}
		} else {
			if result {
				return true // OR: any true makes all true
			}
		}
	}

	// For AND: all were true; for OR: none were true
	return isAnd
}

// evaluateCondition evaluates a single visibility condition
func (s *LayoutService) evaluateCondition(cond entity.VisibilityCondition, record map[string]interface{}) bool {
	fieldValue, exists := record[cond.Field]

	switch cond.Operator {
	case entity.OpIsEmpty:
		return !exists || fieldValue == nil || fieldValue == ""
	case entity.OpIsNotEmpty:
		return exists && fieldValue != nil && fieldValue != ""
	case entity.OpIsTrue:
		return toBool(fieldValue) == true
	case entity.OpIsFalse:
		return toBool(fieldValue) == false
	case entity.OpEquals:
		return compareEqual(fieldValue, cond.Value)
	case entity.OpNotEquals:
		return !compareEqual(fieldValue, cond.Value)
	case entity.OpIn:
		return contains(cond.Values, toString(fieldValue))
	case entity.OpNotIn:
		return !contains(cond.Values, toString(fieldValue))
	case entity.OpContains:
		return strings.Contains(strings.ToLower(toString(fieldValue)), strings.ToLower(toString(cond.Value)))
	case entity.OpStartsWith:
		return strings.HasPrefix(strings.ToLower(toString(fieldValue)), strings.ToLower(toString(cond.Value)))
	case entity.OpEndsWith:
		return strings.HasSuffix(strings.ToLower(toString(fieldValue)), strings.ToLower(toString(cond.Value)))
	case entity.OpGreaterThan:
		return compareNumeric(fieldValue, cond.Value) > 0
	case entity.OpLessThan:
		return compareNumeric(fieldValue, cond.Value) < 0
	case entity.OpGreaterEqual:
		return compareNumeric(fieldValue, cond.Value) >= 0
	case entity.OpLessEqual:
		return compareNumeric(fieldValue, cond.Value) <= 0
	default:
		return true // Unknown operator, default to visible
	}
}

// Helper functions for comparison

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int, int64, float64:
		return val != 0
	case string:
		return val == "true" || val == "1" || val == "yes"
	default:
		return false
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		// Simple parse, ignore errors
		var f float64
		json.Unmarshal([]byte(val), &f)
		return f
	default:
		return 0
	}
}

func compareEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return toString(a) == toString(b)
}

func compareNumeric(a, b interface{}) int {
	aNum := toFloat(a)
	bNum := toFloat(b)
	if aNum < bNum {
		return -1
	} else if aNum > bNum {
		return 1
	}
	return 0
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// FilterVisibleSections returns only the sections that are visible based on record data
func (s *LayoutService) FilterVisibleSections(layout *entity.LayoutDataV2, record map[string]interface{}) []entity.LayoutSectionV2 {
	var visibleSections []entity.LayoutSectionV2

	for _, section := range layout.Sections {
		if s.EvaluateVisibility(section.Visibility, record) {
			// Filter visible fields within the section
			var visibleFields []entity.LayoutFieldV2
			for _, field := range section.Fields {
				if s.EvaluateVisibility(field.Visibility, record) {
					visibleFields = append(visibleFields, field)
				}
			}

			// Only include section if it has visible fields
			if len(visibleFields) > 0 {
				filteredSection := section
				filteredSection.Fields = visibleFields
				visibleSections = append(visibleSections, filteredSection)
			}
		}
	}

	return visibleSections
}
