package entity

import "time"

// Mirror represents a schema contract for external data ingestion
type Mirror struct {
	ID                string              `json:"id"`
	OrgID             string              `json:"orgId"`
	Name              string              `json:"name"`                // Human-readable name like "Salesforce Contacts"
	TargetEntity      string              `json:"targetEntity"`        // Which Quantico entity to write to (e.g., "Contact", "Account")
	UniqueKeyField    string              `json:"uniqueKeyField"`      // Which incoming field is the unique key (e.g., "sf_id", "external_id")
	UnmappedFieldMode string              `json:"unmappedFieldMode"`   // "strict" (reject unknown fields) or "flexible" (accept with warning)
	RateLimit         int                 `json:"rateLimit"`           // Per-mirror rate limit, default 500/min
	IsActive          bool                `json:"isActive"`            // Whether this mirror is active
	SourceFields      []MirrorSourceField `json:"sourceFields"`        // Populated on read, stored in separate table
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}

// MirrorSourceField defines an expected field in incoming data
type MirrorSourceField struct {
	ID          string    `json:"id"`
	MirrorID    string    `json:"mirrorId"`
	FieldName   string    `json:"fieldName"`   // Expected field name in incoming data (e.g., "FirstName", "sf_account_id")
	FieldType   string    `json:"fieldType"`   // Expected type: "text", "number", "date", "boolean", "email", "phone"
	IsRequired  bool      `json:"isRequired"`  // Whether this field must be present in every record
	Description string    `json:"description"` // Optional human-readable description
	SortOrder   int       `json:"sortOrder"`   // Display order
	CreatedAt   time.Time `json:"createdAt"`
}

// MirrorCreateInput is the input for creating a new mirror
type MirrorCreateInput struct {
	Name              string                   `json:"name"`              // Required
	TargetEntity      string                   `json:"targetEntity"`      // Required
	UniqueKeyField    string                   `json:"uniqueKeyField"`    // Required
	UnmappedFieldMode string                   `json:"unmappedFieldMode"` // Defaults to "flexible" if not provided
	RateLimit         *int                     `json:"rateLimit"`         // Defaults to 500 if not provided
	SourceFields      []MirrorSourceFieldInput `json:"sourceFields"`      // Optional, can be added later
}

// MirrorSourceFieldInput is the input for creating/updating a source field
type MirrorSourceFieldInput struct {
	FieldName   string `json:"fieldName"`   // Required
	FieldType   string `json:"fieldType"`   // Defaults to "text"
	IsRequired  bool   `json:"isRequired"`  // Defaults to false
	Description string `json:"description"` // Optional
}

// MirrorUpdateInput is the input for updating a mirror
type MirrorUpdateInput struct {
	Name              *string                   `json:"name"`
	TargetEntity      *string                   `json:"targetEntity"`
	UniqueKeyField    *string                   `json:"uniqueKeyField"`
	UnmappedFieldMode *string                   `json:"unmappedFieldMode"`
	RateLimit         *int                      `json:"rateLimit"`
	IsActive          *bool                     `json:"isActive"`
	SourceFields      *[]MirrorSourceFieldInput `json:"sourceFields"` // If provided, replaces all source fields (delete + re-insert)
}

// Constants for unmapped field mode
const (
	UnmappedFieldModeStrict   = "strict"
	UnmappedFieldModeFlexible = "flexible"
)

// ValidFieldTypes defines the allowed field types
var ValidFieldTypes = []string{"text", "number", "date", "boolean", "email", "phone"}

// ValidateUnmappedFieldMode returns true if mode is valid
func ValidateUnmappedFieldMode(mode string) bool {
	return mode == UnmappedFieldModeStrict || mode == UnmappedFieldModeFlexible
}

// ValidateFieldType returns true if fieldType is valid
func ValidateFieldType(fieldType string) bool {
	for _, valid := range ValidFieldTypes {
		if fieldType == valid {
			return true
		}
	}
	return false
}
