package entity

// OrgFeature represents a feature flag stored in a tenant database
type OrgFeature struct {
	FeatureKey string  `json:"featureKey" db:"feature_key"`
	Enabled    bool    `json:"enabled" db:"enabled"`
	EnabledAt  *string `json:"enabledAt,omitempty" db:"enabled_at"`
	EnabledBy  *string `json:"enabledBy,omitempty" db:"enabled_by"`
	CreatedAt  string  `json:"createdAt" db:"created_at"`
	ModifiedAt string  `json:"modifiedAt" db:"modified_at"`
}

// FeatureDefinition describes a known toggleable feature
type FeatureDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// FeatureStatus combines a feature definition with its enabled state
type FeatureStatus struct {
	FeatureDefinition
	Enabled   bool    `json:"enabled"`
	EnabledAt *string `json:"enabledAt,omitempty"`
	EnabledBy *string `json:"enabledBy,omitempty"`
}

// KnownFeatures is the registry of all toggleable features
var KnownFeatures = []FeatureDefinition{
	{Key: "cadences", Label: "Cadences", Description: "Multi-step outreach sequences, email templates, and task inbox"},
}
