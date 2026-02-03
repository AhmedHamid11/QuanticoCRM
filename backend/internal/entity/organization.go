package entity

import "time"

// Plan tiers
const (
	PlanFree = "free"
	PlanPro  = "pro"
)

// Tier limits
const (
	FreeTierMaxUsers = 20
	ProTierMaxUsers  = 0 // 0 = unlimited
)

// GetMaxUsers returns the maximum number of users allowed for a plan
func GetMaxUsers(plan string) int {
	switch plan {
	case PlanPro:
		return ProTierMaxUsers
	default:
		return FreeTierMaxUsers
	}
}

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Plan        string    `json:"plan" db:"plan"`
	IsActive    bool      `json:"isActive" db:"is_active"`
	Settings    string    `json:"settings" db:"settings"`
	// Multi-tenant database fields
	DatabaseURL   string `json:"-" db:"database_url"`             // Turso database URL (not exposed to API)
	DatabaseToken string `json:"-" db:"database_token"`           // Turso auth token (not exposed to API)
	DatabaseName  string `json:"databaseName,omitempty" db:"database_name"` // Turso database name for management
	// Platform version tracking
	CurrentVersion string `json:"currentVersion,omitempty" db:"current_version"` // Current platform version deployed to this org
	// Transient field for provisioning status (not stored in DB)
	ProvisioningError string `json:"provisioningError,omitempty" db:"-"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt    time.Time `json:"modifiedAt" db:"modified_at"`
}

// OrganizationCreateInput represents input for creating an organization
type OrganizationCreateInput struct {
	Name           string `json:"name" validate:"required"`
	Slug           string `json:"slug"`
	CurrentVersion string `json:"currentVersion"` // Platform version at creation time
}

// OrganizationUpdateInput represents input for updating an organization
type OrganizationUpdateInput struct {
	Name     *string `json:"name"`
	Plan     *string `json:"plan"`
	IsActive *bool   `json:"isActive"`
	Settings *string `json:"settings"`
}

// OrganizationListResponse represents the response for listing organizations
type OrganizationListResponse struct {
	Data       []Organization `json:"data"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}
