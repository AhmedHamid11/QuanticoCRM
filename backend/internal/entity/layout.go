package entity

import "encoding/json"

// LayoutVersion represents the layout data format version
type LayoutVersion int

const (
	LayoutVersionV1 LayoutVersion = 1 // Flat array of field names
	LayoutVersionV2 LayoutVersion = 2 // Sections with visibility rules
	LayoutVersionV3 LayoutVersion = 3 // Tabbed layout with sidebar and header
)

// Section card type constants
const (
	CardTypeField       = "field"
	CardTypeActivity    = "activity"
	CardTypeRelatedList = "relatedList"
	CardTypeCustomPage  = "customPage"
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

// SectionCardV3 represents an individual card within a section container
type SectionCardV3 struct {
	ID         string          `json:"id"`
	CardType   string          `json:"cardType"`
	Order      int             `json:"order"`
	Label      string          `json:"label,omitempty"`
	Fields     []LayoutFieldV2 `json:"fields,omitempty"`
	CardConfig json.RawMessage `json:"cardConfig,omitempty"`
	Columns    int             `json:"columns,omitempty"` // internal field grid columns for field cards
	Column     int             `json:"column,omitempty"`  // which section grid column this card sits in (1-indexed)
}

// LayoutSectionV2 represents a section in the layout
type LayoutSectionV2 struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Order       int             `json:"order"`
	Collapsible bool            `json:"collapsible"`
	Collapsed   bool            `json:"collapsed"`        // Default collapsed state
	Columns     int             `json:"columns"`          // 1, 2, or 3
	Visibility  VisibilityRule  `json:"visibility"`
	Fields      []LayoutFieldV2 `json:"fields"`
	CardType    string          `json:"cardType,omitempty"`   // DEPRECATED: use Cards[].CardType instead
	CardConfig  json.RawMessage `json:"cardConfig,omitempty"` // DEPRECATED: use Cards[].CardConfig instead
	Cards       []SectionCardV3 `json:"cards,omitempty"`      // Multi-card container: each card renders independently in the section grid
}

// LayoutDataV2 represents the v2 layout data structure
type LayoutDataV2 struct {
	Version  LayoutVersion     `json:"version"`
	Sections []LayoutSectionV2 `json:"sections"`
}

// LayoutTabV3 represents a named tab containing section IDs
type LayoutTabV3 struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Order      int      `json:"order"`
	SectionIDs []string `json:"sectionIds"`
}

// LayoutSidebarCardV3 represents a card in the right sidebar
type LayoutSidebarCardV3 struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Order  int      `json:"order"`
	Fields []string `json:"fields"`
}

// LayoutSidebarV3 holds the sidebar configuration
type LayoutSidebarV3 struct {
	Cards []LayoutSidebarCardV3 `json:"cards"`
}

// LayoutHeaderV3 holds the header strip configuration (4-6 key fields)
type LayoutHeaderV3 struct {
	Fields []string `json:"fields"`
}

// LayoutDataV3 is the V3 layout data structure with tabs, sidebar, and header.
// The Conditions field is reserved for future conditional visibility — always null.
type LayoutDataV3 struct {
	Version    LayoutVersion     `json:"version"`
	Sections   []LayoutSectionV2 `json:"sections"`
	Tabs       []LayoutTabV3     `json:"tabs"`
	Sidebar    LayoutSidebarV3   `json:"sidebar"`
	Header     LayoutHeaderV3    `json:"header"`
	Conditions interface{}       `json:"conditions"`
}

// LayoutDataParsed is a union type for parsed layout data
// It can be either v1 (flat array), v2 (sections), or v3 (tabbed with sidebar/header)
type LayoutDataParsed struct {
	Version  LayoutVersion
	V1Fields []string      // For v1 layouts
	V2Data   *LayoutDataV2 // For v2 layouts
	V3Data   *LayoutDataV3 // For v3 layouts
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

// ConvertV2ToV3 wraps a V2 layout in a V3 envelope.
// All existing sections are placed in a single default "Overview" tab.
// Sidebar and Header start empty (populated by admin via PUT later).
func ConvertV2ToV3(v2 *LayoutDataV2) *LayoutDataV3 {
	sectionIDs := make([]string, len(v2.Sections))
	for i, s := range v2.Sections {
		sectionIDs[i] = s.ID
	}

	return &LayoutDataV3{
		Version:  LayoutVersionV3,
		Sections: v2.Sections,
		Tabs: []LayoutTabV3{
			{
				ID:         "tab_overview",
				Label:      "Overview",
				Order:      1,
				SectionIDs: sectionIDs,
			},
		},
		Sidebar:    LayoutSidebarV3{Cards: []LayoutSidebarCardV3{}},
		Header:     LayoutHeaderV3{Fields: []string{}},
		Conditions: nil,
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
