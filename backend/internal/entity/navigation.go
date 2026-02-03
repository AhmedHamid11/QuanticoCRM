package entity

import "time"

// NavigationTab represents a navigation tab in the toolbar
type NavigationTab struct {
	ID         string    `json:"id" db:"id"`
	OrgID      string    `json:"orgId" db:"org_id"`
	Label      string    `json:"label" db:"label"`
	Href       string    `json:"href" db:"href"`
	Icon       string    `json:"icon" db:"icon"`
	EntityName *string   `json:"entityName,omitempty" db:"entity_name"`
	SortOrder  int       `json:"sortOrder" db:"sort_order"`
	IsVisible  bool      `json:"isVisible" db:"is_visible"`
	IsSystem   bool      `json:"isSystem" db:"is_system"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	ModifiedAt time.Time `json:"modifiedAt" db:"modified_at"`
}

// NavigationTabCreateInput for creating a new navigation tab
type NavigationTabCreateInput struct {
	Label      string  `json:"label" validate:"required"`
	Href       string  `json:"href" validate:"required"`
	Icon       string  `json:"icon"`
	EntityName *string `json:"entityName,omitempty"`
	SortOrder  int     `json:"sortOrder"`
	IsVisible  bool    `json:"isVisible"`
}

// NavigationTabUpdateInput for updating a navigation tab
type NavigationTabUpdateInput struct {
	Label      *string `json:"label,omitempty"`
	Href       *string `json:"href,omitempty"`
	Icon       *string `json:"icon,omitempty"`
	EntityName *string `json:"entityName,omitempty"`
	SortOrder  *int    `json:"sortOrder,omitempty"`
	IsVisible  *bool   `json:"isVisible,omitempty"`
}

// NavigationReorderInput for reordering tabs
type NavigationReorderInput struct {
	TabIDs []string `json:"tabIds" validate:"required"`
}
