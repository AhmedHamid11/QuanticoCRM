package entity

import "time"

// RelatedListConfig stores configuration for a related list on an entity's detail page
type RelatedListConfig struct {
	ID             string        `json:"id" db:"id"`
	OrgID          string        `json:"orgId" db:"org_id"`
	EntityType     string        `json:"entityType" db:"entity_type"`         // The parent entity (e.g., "Account")
	RelatedEntity  string        `json:"relatedEntity" db:"related_entity"`   // The child entity (e.g., "Contact")
	LookupField    string        `json:"lookupField" db:"lookup_field"`       // Field on child that points to parent (e.g., "accountId")
	Label          string        `json:"label" db:"label"`                    // Display name (e.g., "Contacts")
	Enabled        bool          `json:"enabled" db:"enabled"`                // Include/exclude toggle
	IsMultiLookup  bool          `json:"isMultiLookup" db:"is_multi_lookup"`  // True if lookup field is linkMultiple type
	EditInList     bool          `json:"editInList" db:"edit_in_list"`        // If true, clicking New shows inline editable row
	DisplayFields  []FieldConfig `json:"displayFields" db:"-"`                // Fields to show in columns
	DisplayFieldsRaw string      `json:"-" db:"display_fields"`               // JSON storage
	SortOrder      int           `json:"sortOrder" db:"sort_order"`           // Order among multiple related lists
	DefaultSort    string        `json:"defaultSort" db:"default_sort"`       // Default sort field
	DefaultSortDir string        `json:"defaultSortDir" db:"default_sort_dir"` // "asc" or "desc"
	PageSize       int           `json:"pageSize" db:"page_size"`             // Records per page (default 5)
	CreatedAt      time.Time     `json:"createdAt" db:"created_at"`
	ModifiedAt     time.Time     `json:"modifiedAt" db:"modified_at"`
}

// FieldConfig defines a column in a related list
type FieldConfig struct {
	Field    string `json:"field"`              // Field name
	Label    string `json:"label,omitempty"`    // Column header (optional override)
	Width    int    `json:"width,omitempty"`    // Column width percentage (optional)
	Position int    `json:"position"`           // Column order
}

// PossibleRelatedList represents a discovered related list option from metadata
type PossibleRelatedList struct {
	RelatedEntity  string `json:"relatedEntity"`  // The child entity (e.g., "Contact")
	LookupField    string `json:"lookupField"`    // Field on child that points to parent
	SuggestedLabel string `json:"suggestedLabel"` // Suggested display name
	FieldLabel     string `json:"fieldLabel"`     // The label of the lookup field
	IsMultiLookup  bool   `json:"isMultiLookup"`  // True if this is a linkMultiple field
}

// RelatedListConfigCreateInput for creating a new related list config
type RelatedListConfigCreateInput struct {
	RelatedEntity  string        `json:"relatedEntity" validate:"required"`
	LookupField    string        `json:"lookupField" validate:"required"`
	Label          string        `json:"label" validate:"required"`
	Enabled        bool          `json:"enabled"`
	IsMultiLookup  bool          `json:"isMultiLookup"`
	EditInList     bool          `json:"editInList"`
	DisplayFields  []FieldConfig `json:"displayFields"`
	SortOrder      int           `json:"sortOrder"`
	DefaultSort    string        `json:"defaultSort"`
	DefaultSortDir string        `json:"defaultSortDir"`
	PageSize       int           `json:"pageSize"`
}

// RelatedListConfigUpdateInput for updating a related list config
type RelatedListConfigUpdateInput struct {
	Label          *string       `json:"label,omitempty"`
	Enabled        *bool         `json:"enabled,omitempty"`
	EditInList     *bool         `json:"editInList,omitempty"`
	DisplayFields  []FieldConfig `json:"displayFields,omitempty"`
	SortOrder      *int          `json:"sortOrder,omitempty"`
	DefaultSort    *string       `json:"defaultSort,omitempty"`
	DefaultSortDir *string       `json:"defaultSortDir,omitempty"`
	PageSize       *int          `json:"pageSize,omitempty"`
}

// RelatedListConfigBulkInput for bulk saving related list configs
type RelatedListConfigBulkInput struct {
	Configs []RelatedListConfigCreateInput `json:"configs"`
}

// RelatedRecordsParams for querying related records
type RelatedRecordsParams struct {
	Page     int    `query:"page"`
	PageSize int    `query:"pageSize"`
	Sort     string `query:"sort"`
	Dir      string `query:"dir"`
}

// RelatedRecordsResponse for paginated related records
type RelatedRecordsResponse struct {
	Records    []map[string]interface{} `json:"records"`
	Total      int                      `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	TotalPages int                      `json:"totalPages"`
}
