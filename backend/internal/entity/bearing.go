package entity

import "time"

// BearingConfig stores configuration for a stage progress indicator on an entity's detail page
// Bearings display the current position within a workflow using a visual stage stepper
type BearingConfig struct {
	ID              string    `json:"id" db:"id"`
	OrgID           string    `json:"orgId" db:"org_id"`
	EntityType      string    `json:"entityType" db:"entity_type"`         // The entity this bearing belongs to (e.g., "Account")
	Name            string    `json:"name" db:"name"`                      // Display name (e.g., "Sales Stage", "Onboarding Status")
	SourcePicklist  string    `json:"sourcePicklist" db:"source_picklist"` // The picklist field that drives the stages
	DisplayOrder    int       `json:"displayOrder" db:"display_order"`     // Sort position when multiple Bearings exist (1-12)
	Active          bool      `json:"active" db:"active"`                  // Toggle to show/hide the Bearing
	ConfirmBackward bool      `json:"confirmBackward" db:"confirm_backward"` // Confirm before allowing backward movement
	AllowUpdates    bool      `json:"allowUpdates" db:"allow_updates"`     // Whether clicking stages updates the field value
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt      time.Time `json:"modifiedAt" db:"modified_at"`
}

// BearingConfigCreateInput for creating a new bearing config
type BearingConfigCreateInput struct {
	Name            string `json:"name" validate:"required"`
	SourcePicklist  string `json:"sourcePicklist" validate:"required"`
	DisplayOrder    int    `json:"displayOrder"`
	Active          bool   `json:"active"`
	ConfirmBackward bool   `json:"confirmBackward"`
	AllowUpdates    bool   `json:"allowUpdates"`
}

// BearingConfigUpdateInput for updating a bearing config
type BearingConfigUpdateInput struct {
	Name            *string `json:"name,omitempty"`
	SourcePicklist  *string `json:"sourcePicklist,omitempty"`
	DisplayOrder    *int    `json:"displayOrder,omitempty"`
	Active          *bool   `json:"active,omitempty"`
	ConfirmBackward *bool   `json:"confirmBackward,omitempty"`
	AllowUpdates    *bool   `json:"allowUpdates,omitempty"`
}

// PicklistOption represents a single option in a picklist field
type PicklistOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Order int    `json:"order"`
}

// BearingWithStages combines bearing config with resolved picklist stages
type BearingWithStages struct {
	BearingConfig
	Stages []PicklistOption `json:"stages"`
}
