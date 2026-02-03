package entity

// LayoutVersion represents the layout data format version
type LayoutVersion int

const (
	LayoutVersionV1 LayoutVersion = 1 // Flat array of field names
	LayoutVersionV2 LayoutVersion = 2 // Sections with visibility rules
)

// VisibilityType represents how visibility is determined
type VisibilityType string

const (
	VisibilityAlways      VisibilityType = "always"
	VisibilityConditional VisibilityType = "conditional"
	VisibilityNever       VisibilityType = "never"
)

// VisibilityCondition represents a condition for showing/hiding an element
// Reuses operators from validation rules
type VisibilityCondition struct {
	ID       string             `json:"id"`
	Field    string             `json:"field"`
	Operator ValidationOperator `json:"operator"`
	Value    interface{}        `json:"value,omitempty"`
	Values   []string           `json:"values,omitempty"` // For IN, NOT_IN operators
}

// VisibilityRule defines when a section or field should be visible
type VisibilityRule struct {
	Type       VisibilityType        `json:"type"`
	Conditions []VisibilityCondition `json:"conditions,omitempty"`
	Logic      string                `json:"logic,omitempty"` // "AND" or "OR", defaults to "AND"
}

// LayoutFieldV2 represents a field in a section
type LayoutFieldV2 struct {
	Name       string         `json:"name"`
	Visibility VisibilityRule `json:"visibility"`
}

// LayoutSectionV2 represents a section in the layout
type LayoutSectionV2 struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Order       int             `json:"order"`
	Collapsible bool            `json:"collapsible"`
	Collapsed   bool            `json:"collapsed"` // Default collapsed state
	Columns     int             `json:"columns"`   // 1, 2, or 3
	Visibility  VisibilityRule  `json:"visibility"`
	Fields      []LayoutFieldV2 `json:"fields"`
}

// LayoutDataV2 represents the v2 layout data structure
type LayoutDataV2 struct {
	Version  LayoutVersion     `json:"version"`
	Sections []LayoutSectionV2 `json:"sections"`
}

// LayoutDataParsed is a union type for parsed layout data
// It can be either v1 (flat array) or v2 (sections)
type LayoutDataParsed struct {
	Version  LayoutVersion
	V1Fields []string      // For v1 layouts
	V2Data   *LayoutDataV2 // For v2 layouts
}

// NewDefaultVisibility creates an "always visible" visibility rule
func NewDefaultVisibility() VisibilityRule {
	return VisibilityRule{
		Type: VisibilityAlways,
	}
}

// NewDefaultSection creates a new section with default values
func NewDefaultSection(id, label string, order int) LayoutSectionV2 {
	return LayoutSectionV2{
		ID:          id,
		Label:       label,
		Order:       order,
		Collapsible: true,
		Collapsed:   false,
		Columns:     2,
		Visibility:  NewDefaultVisibility(),
		Fields:      []LayoutFieldV2{},
	}
}

// ConvertV1ToV2 converts a v1 layout (flat field array) to v2 format
func ConvertV1ToV2(fields []string) *LayoutDataV2 {
	// Create a single "General Information" section with all fields
	layoutFields := make([]LayoutFieldV2, len(fields))
	for i, fieldName := range fields {
		layoutFields[i] = LayoutFieldV2{
			Name:       fieldName,
			Visibility: NewDefaultVisibility(),
		}
	}

	return &LayoutDataV2{
		Version: LayoutVersionV2,
		Sections: []LayoutSectionV2{
			{
				ID:          "section_general",
				Label:       "General Information",
				Order:       1,
				Collapsible: false,
				Collapsed:   false,
				Columns:     2,
				Visibility:  NewDefaultVisibility(),
				Fields:      layoutFields,
			},
		},
	}
}

// GetAllFieldNames extracts all field names from a v2 layout
func (l *LayoutDataV2) GetAllFieldNames() []string {
	var fields []string
	for _, section := range l.Sections {
		for _, field := range section.Fields {
			fields = append(fields, field.Name)
		}
	}
	return fields
}
