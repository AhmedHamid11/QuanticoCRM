package entity

import "time"

// FieldType represents the available field types
type FieldType string

const (
	FieldTypeVarchar      FieldType = "varchar"
	FieldTypeText         FieldType = "text"
	FieldTypeInt          FieldType = "int"
	FieldTypeFloat        FieldType = "float"
	FieldTypeBool         FieldType = "bool"
	FieldTypeDate         FieldType = "date"
	FieldTypeDatetime     FieldType = "datetime"
	FieldTypeEmail        FieldType = "email"
	FieldTypePhone        FieldType = "phone"
	FieldTypeURL          FieldType = "url"
	FieldTypeEnum         FieldType = "enum"
	FieldTypeMultiEnum    FieldType = "multiEnum"
	FieldTypeLink         FieldType = "link"
	FieldTypeLinkMultiple FieldType = "linkMultiple"
	FieldTypeCurrency     FieldType = "currency"
	FieldTypeAddress      FieldType = "address"
	FieldTypeRollup       FieldType = "rollup"
	FieldTypeTextBlock    FieldType = "textBlock"
	FieldTypeStream       FieldType = "stream"
)

// FieldTypeInfo contains metadata about a field type
type FieldTypeInfo struct {
	Name        FieldType `json:"name"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Params      []FieldParam `json:"params"`
}

// FieldParam describes a configurable parameter for a field type
type FieldParam struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Label    string      `json:"label"`
	Default  interface{} `json:"default,omitempty"`
	Required bool        `json:"required,omitempty"`
	Min      *float64    `json:"min,omitempty"`
	Max      *float64    `json:"max,omitempty"`
	Options  []string    `json:"options,omitempty"`
}

// EntityDef represents an entity definition (scope)
type EntityDef struct {
	ID             string    `json:"id" db:"id"`
	OrgID          string    `json:"orgId" db:"org_id"`
	Name           string    `json:"name" db:"name"`
	Label          string    `json:"label" db:"label"`
	LabelPlural    string    `json:"labelPlural" db:"label_plural"`
	Icon           string    `json:"icon" db:"icon"`
	Color          string    `json:"color" db:"color"`
	IsCustom       bool      `json:"isCustom" db:"is_custom"`
	IsCustomizable bool      `json:"isCustomizable" db:"is_customizable"`
	HasStream      bool      `json:"hasStream" db:"has_stream"`
	HasActivities  bool      `json:"hasActivities" db:"has_activities"`
	DisplayField   string    `json:"displayField" db:"display_field"`   // SQL expression for lookup display name
	SearchFields   string    `json:"searchFields" db:"search_fields"`   // JSON array of searchable field names
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt     time.Time `json:"modifiedAt" db:"modified_at"`
}

// FieldDef represents a field definition for an entity
type FieldDef struct {
	ID               string    `json:"id" db:"id"`
	OrgID            string    `json:"orgId" db:"org_id"`
	EntityName       string    `json:"entityName" db:"entity_name"`
	Name             string    `json:"name" db:"name"`
	Label            string    `json:"label" db:"label"`
	Type             FieldType `json:"type" db:"type"`
	IsRequired       bool      `json:"isRequired" db:"is_required"`
	IsReadOnly       bool      `json:"isReadOnly" db:"is_read_only"`
	IsAudited        bool      `json:"isAudited" db:"is_audited"`
	IsCustom         bool      `json:"isCustom" db:"is_custom"`
	DefaultValue     *string   `json:"defaultValue,omitempty" db:"default_value"`
	Options          *string   `json:"options,omitempty" db:"options"` // JSON array
	MaxLength        *int      `json:"maxLength,omitempty" db:"max_length"`
	MinValue         *float64  `json:"minValue,omitempty" db:"min_value"`
	MaxValue         *float64  `json:"maxValue,omitempty" db:"max_value"`
	Pattern          *string   `json:"pattern,omitempty" db:"pattern"`
	Tooltip          *string   `json:"tooltip,omitempty" db:"tooltip"`
	LinkEntity       *string   `json:"linkEntity,omitempty" db:"link_entity"`
	LinkType         *string   `json:"linkType,omitempty" db:"link_type"`                  // belongsTo, hasMany, hasOne
	LinkForeignKey   *string   `json:"linkForeignKey,omitempty" db:"link_foreign_key"`     // FK column on related table
	LinkDisplayField    *string   `json:"linkDisplayField,omitempty" db:"link_display_field"` // Field to display from linked record
	RollupQuery         *string   `json:"rollupQuery,omitempty" db:"rollup_query"`
	RollupResultType    *string   `json:"rollupResultType,omitempty" db:"rollup_result_type"` // "numeric" or "text"
	RollupDecimalPlaces *int      `json:"rollupDecimalPlaces,omitempty" db:"rollup_decimal_places"`
	DefaultToToday      bool      `json:"defaultToToday" db:"default_to_today"` // For date/datetime fields: default to current date
	Variant             *string   `json:"variant,omitempty" db:"variant"`       // For textBlock: info, warning, error, success
	Content             *string   `json:"content,omitempty" db:"content"`       // For textBlock: message text with {{fieldName}} placeholders
	SortOrder           int       `json:"sortOrder" db:"sort_order"`
	CreatedAt           time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt          time.Time `json:"modifiedAt" db:"modified_at"`
}

// RelationshipDef represents a relationship between two entities
type RelationshipDef struct {
	ID               string    `json:"id" db:"id"`
	OrgID            string    `json:"orgId" db:"org_id"`
	Name             string    `json:"name" db:"name"`
	FromEntity       string    `json:"fromEntity" db:"from_entity"`
	ToEntity         string    `json:"toEntity" db:"to_entity"`
	FromField        string    `json:"fromField" db:"from_field"`
	ToField          *string   `json:"toField,omitempty" db:"to_field"`
	RelationshipType string    `json:"relationshipType" db:"relationship_type"` // belongsTo, hasMany, hasOne, manyToMany
	IsCustom         bool      `json:"isCustom" db:"is_custom"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt       time.Time `json:"modifiedAt" db:"modified_at"`
}

// LayoutDef represents a layout definition
type LayoutDef struct {
	ID         string    `json:"id" db:"id"`
	OrgID      string    `json:"orgId" db:"org_id"`
	EntityName string    `json:"entityName" db:"entity_name"`
	LayoutType string    `json:"layoutType" db:"layout_type"`
	LayoutData string    `json:"layoutData" db:"layout_data"` // JSON
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt time.Time `json:"modifiedAt" db:"modified_at"`
}

// FieldDefCreateInput for creating a new field
type FieldDefCreateInput struct {
	Name             string    `json:"name" validate:"required"`
	Label            string    `json:"label" validate:"required"`
	Type             FieldType `json:"type" validate:"required"`
	IsRequired       bool      `json:"isRequired"`
	IsReadOnly       bool      `json:"isReadOnly"`
	IsAudited        bool      `json:"isAudited"`
	DefaultValue     *string   `json:"defaultValue,omitempty"`
	Options          *string   `json:"options,omitempty"`
	MaxLength        *int      `json:"maxLength,omitempty"`
	MinValue         *float64  `json:"minValue,omitempty"`
	MaxValue         *float64  `json:"maxValue,omitempty"`
	Pattern          *string   `json:"pattern,omitempty"`
	Tooltip          *string   `json:"tooltip,omitempty"`
	LinkEntity          *string `json:"linkEntity,omitempty"`
	LinkType            *string `json:"linkType,omitempty"`            // belongsTo, hasMany, hasOne
	LinkDisplayField    *string `json:"linkDisplayField,omitempty"`    // Field to display from linked record (default: name)
	RollupQuery         *string `json:"rollupQuery,omitempty"`         // SQL query for rollup fields
	RollupResultType    *string `json:"rollupResultType,omitempty"`    // "numeric" or "text"
	RollupDecimalPlaces *int    `json:"rollupDecimalPlaces,omitempty"` // Decimal places for numeric rollups
	DefaultToToday      bool    `json:"defaultToToday"`                // For date/datetime fields: default to current date
	Variant             *string `json:"variant,omitempty"`             // For textBlock: info, warning, error, success
	Content             *string `json:"content,omitempty"`             // For textBlock: message text with {{fieldName}} placeholders
}

// FieldDefUpdateInput for updating a field
type FieldDefUpdateInput struct {
	Label               *string  `json:"label,omitempty"`
	IsRequired          *bool    `json:"isRequired,omitempty"`
	IsReadOnly          *bool    `json:"isReadOnly,omitempty"`
	IsAudited           *bool    `json:"isAudited,omitempty"`
	DefaultValue        *string  `json:"defaultValue,omitempty"`
	Options             *string  `json:"options,omitempty"`
	MaxLength           *int     `json:"maxLength,omitempty"`
	MinValue            *float64 `json:"minValue,omitempty"`
	MaxValue            *float64 `json:"maxValue,omitempty"`
	Pattern             *string  `json:"pattern,omitempty"`
	Tooltip             *string  `json:"tooltip,omitempty"`
	SortOrder           *int     `json:"sortOrder,omitempty"`
	RollupQuery         *string  `json:"rollupQuery,omitempty"`
	RollupResultType    *string  `json:"rollupResultType,omitempty"`
	RollupDecimalPlaces *int     `json:"rollupDecimalPlaces,omitempty"`
	DefaultToToday      *bool    `json:"defaultToToday,omitempty"` // For date/datetime fields
	Variant             *string  `json:"variant,omitempty"`        // For textBlock: info, warning, error, success
	Content             *string  `json:"content,omitempty"`        // For textBlock: message text
}

// GetFieldTypes returns all available field types with their metadata
func GetFieldTypes() []FieldTypeInfo {
	return []FieldTypeInfo{
		{
			Name:        FieldTypeVarchar,
			Label:       "Text",
			Description: "Single line text field",
			Params: []FieldParam{
				{Name: "maxLength", Type: "int", Label: "Max Length", Default: 255},
				{Name: "pattern", Type: "varchar", Label: "Pattern (Regex)"},
			},
		},
		{
			Name:        FieldTypeText,
			Label:       "Text Area",
			Description: "Multi-line text field",
			Params: []FieldParam{
				{Name: "maxLength", Type: "int", Label: "Max Length", Default: 65535},
				{Name: "rowsMin", Type: "int", Label: "Min Rows", Default: 3},
			},
		},
		{
			Name:        FieldTypeInt,
			Label:       "Integer",
			Description: "Whole number field",
			Params: []FieldParam{
				{Name: "min", Type: "int", Label: "Minimum Value"},
				{Name: "max", Type: "int", Label: "Maximum Value"},
			},
		},
		{
			Name:        FieldTypeFloat,
			Label:       "Decimal",
			Description: "Decimal number field",
			Params: []FieldParam{
				{Name: "min", Type: "float", Label: "Minimum Value"},
				{Name: "max", Type: "float", Label: "Maximum Value"},
				{Name: "decimalPlaces", Type: "int", Label: "Decimal Places", Default: 2},
			},
		},
		{
			Name:        FieldTypeBool,
			Label:       "Checkbox",
			Description: "Boolean true/false field",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypeDate,
			Label:       "Date",
			Description: "Date picker field",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypeDatetime,
			Label:       "Date & Time",
			Description: "Date and time picker field",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypeEmail,
			Label:       "Email",
			Description: "Email address field with validation",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypePhone,
			Label:       "Phone",
			Description: "Phone number field",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypeURL,
			Label:       "URL",
			Description: "Website URL field",
			Params:      []FieldParam{},
		},
		{
			Name:        FieldTypeEnum,
			Label:       "Dropdown",
			Description: "Single selection from options",
			Params: []FieldParam{
				{Name: "options", Type: "array", Label: "Options", Required: true},
			},
		},
		{
			Name:        FieldTypeMultiEnum,
			Label:       "Multi-Select",
			Description: "Multiple selection from options",
			Params: []FieldParam{
				{Name: "options", Type: "array", Label: "Options", Required: true},
			},
		},
		{
			Name:        FieldTypeCurrency,
			Label:       "Currency",
			Description: "Monetary value field",
			Params: []FieldParam{
				{Name: "currency", Type: "varchar", Label: "Currency", Default: "USD"},
			},
		},
		{
			Name:        FieldTypeLink,
			Label:       "Lookup",
			Description: "Reference to another record",
			Params: []FieldParam{
				{Name: "linkEntity", Type: "varchar", Label: "Related Entity", Required: true},
				{Name: "linkDisplayField", Type: "varchar", Label: "Display Field", Default: "name"},
			},
		},
		{
			Name:        FieldTypeLinkMultiple,
			Label:       "Multi-Lookup",
			Description: "Reference to multiple records",
			Params: []FieldParam{
				{Name: "linkEntity", Type: "varchar", Label: "Related Entity", Required: true},
				{Name: "linkDisplayField", Type: "varchar", Label: "Display Field", Default: "name"},
			},
		},
		{
			Name:        FieldTypeRollup,
			Label:       "Rollup",
			Description: "Calculated value from related records using SQL (admin only)",
			Params: []FieldParam{
				{Name: "rollupQuery", Type: "text", Label: "SQL Query", Required: true},
				{Name: "rollupResultType", Type: "enum", Label: "Result Type", Required: true, Options: []string{"numeric", "text"}},
				{Name: "rollupDecimalPlaces", Type: "int", Label: "Decimal Places", Default: 2},
			},
		},
		{
			Name:        FieldTypeTextBlock,
			Label:       "Text Block",
			Description: "Display styled messages on layouts (info, warning, error, success)",
			Params: []FieldParam{
				{Name: "variant", Type: "enum", Label: "Style", Required: true, Options: []string{"info", "warning", "error", "success"}, Default: "info"},
				{Name: "content", Type: "text", Label: "Message", Required: true},
			},
		},
		{
			Name:        FieldTypeStream,
			Label:       "Stream",
			Description: "Journal/Twitter-style field with timestamped entry log",
			Params:      []FieldParam{},
		},
	}
}

// EntityDefCreateInput for creating a new entity
type EntityDefCreateInput struct {
	Name          string `json:"name" validate:"required"`
	Label         string `json:"label" validate:"required"`
	LabelPlural   string `json:"labelPlural"`
	Icon          string `json:"icon"`
	Color         string `json:"color"`
	HasStream     bool   `json:"hasStream"`
	HasActivities bool   `json:"hasActivities"`
}

// EntityDefUpdateInput for updating an existing entity
type EntityDefUpdateInput struct {
	Label         *string `json:"label,omitempty"`
	LabelPlural   *string `json:"labelPlural,omitempty"`
	Icon          *string `json:"icon,omitempty"`
	Color         *string `json:"color,omitempty"`
	HasStream     *bool   `json:"hasStream,omitempty"`
	HasActivities *bool   `json:"hasActivities,omitempty"`
}

// LookupRecord represents a resolved lookup value with id and display name
type LookupRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LookupSearchResult represents search results for lookup autocomplete
type LookupSearchResult struct {
	Records []LookupRecord `json:"records"`
	Total   int            `json:"total"`
}
