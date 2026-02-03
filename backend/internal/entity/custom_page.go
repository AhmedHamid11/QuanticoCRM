package entity

import (
	"encoding/json"
	"time"
)

// ComponentType represents the type of a page component
type ComponentType string

const (
	ComponentTypeIframe     ComponentType = "iframe"
	ComponentTypeText       ComponentType = "text"
	ComponentTypeMarkdown   ComponentType = "markdown"
	ComponentTypeHTML       ComponentType = "html"
	ComponentTypeEntityList ComponentType = "entity_list"
	ComponentTypeLinkGroup  ComponentType = "link_group"
	ComponentTypeStats      ComponentType = "stats"
)

// ComponentWidth represents the width of a component in the layout
type ComponentWidth string

const (
	ComponentWidthFull     ComponentWidth = "full"
	ComponentWidthHalf     ComponentWidth = "1/2"
	ComponentWidthThird    ComponentWidth = "1/3"
	ComponentWidthTwoThird ComponentWidth = "2/3"
)

// PageComponent represents a single component on a custom page
type PageComponent struct {
	ID     string          `json:"id"`
	Type   ComponentType   `json:"type"`
	Title  string          `json:"title,omitempty"`
	Width  ComponentWidth  `json:"width"`
	Order  int             `json:"order"`
	Config json.RawMessage `json:"config"`
}

// IframeConfig holds configuration for an iframe component
type IframeConfig struct {
	URL     string `json:"url"`
	Height  int    `json:"height,omitempty"`
	Sandbox string `json:"sandbox,omitempty"`
}

// TextConfig holds configuration for a text/markdown component
type TextConfig struct {
	Content string `json:"content"`
}

// HTMLConfig holds configuration for a raw HTML component
type HTMLConfig struct {
	Content string `json:"content"`
}

// EntityListConfig holds configuration for an entity list component
type EntityListConfig struct {
	Entity   string                 `json:"entity"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	Columns  []string               `json:"columns,omitempty"`
	PageSize int                    `json:"pageSize,omitempty"`
	SortBy   string                 `json:"sortBy,omitempty"`
	SortDir  string                 `json:"sortDir,omitempty"`
}

// LinkItem represents a single link in a link group
type LinkItem struct {
	Label       string `json:"label"`
	Href        string `json:"href"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
	External    bool   `json:"external,omitempty"`
}

// LinkGroupConfig holds configuration for a link group component
type LinkGroupConfig struct {
	Links []LinkItem `json:"links"`
}

// StatItem represents a single stat in a stats component
type StatItem struct {
	Label string `json:"label"`
	Value string `json:"value"` // Can contain templates like {{count:contacts}}
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

// StatsConfig holds configuration for a stats component
type StatsConfig struct {
	Items []StatItem `json:"items"`
}

// CustomPage represents a custom page in the database
type CustomPage struct {
	ID          string          `json:"id" db:"id"`
	OrgID       string          `json:"orgId" db:"org_id"`
	Slug        string          `json:"slug" db:"slug"`
	Title       string          `json:"title" db:"title"`
	Description string          `json:"description,omitempty" db:"description"`
	Icon        string          `json:"icon" db:"icon"`
	IsEnabled   bool            `json:"isEnabled" db:"is_enabled"`
	IsPublic    bool            `json:"isPublic" db:"is_public"`
	Layout      string          `json:"layout" db:"layout"`
	Components  []PageComponent `json:"components" db:"-"`
	SortOrder   int             `json:"sortOrder" db:"sort_order"`
	CreatedAt   time.Time       `json:"createdAt" db:"created_at"`
	ModifiedAt  time.Time       `json:"modifiedAt" db:"modified_at"`
	CreatedBy   *string         `json:"createdBy,omitempty" db:"created_by"`
	ModifiedBy  *string         `json:"modifiedBy,omitempty" db:"modified_by"`
}

// CustomPageCreateInput for creating a new custom page
type CustomPageCreateInput struct {
	Slug        string          `json:"slug" validate:"required"`
	Title       string          `json:"title" validate:"required"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	IsEnabled   bool            `json:"isEnabled"`
	IsPublic    bool            `json:"isPublic"`
	Layout      string          `json:"layout,omitempty"`
	Components  []PageComponent `json:"components,omitempty"`
	SortOrder   int             `json:"sortOrder,omitempty"`
}

// CustomPageUpdateInput for updating an existing custom page
type CustomPageUpdateInput struct {
	Slug        *string          `json:"slug,omitempty"`
	Title       *string          `json:"title,omitempty"`
	Description *string          `json:"description,omitempty"`
	Icon        *string          `json:"icon,omitempty"`
	IsEnabled   *bool            `json:"isEnabled,omitempty"`
	IsPublic    *bool            `json:"isPublic,omitempty"`
	Layout      *string          `json:"layout,omitempty"`
	Components  *[]PageComponent `json:"components,omitempty"`
	SortOrder   *int             `json:"sortOrder,omitempty"`
}

// CustomPageListItem is a lightweight version for listing pages
type CustomPageListItem struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon"`
	IsEnabled   bool      `json:"isEnabled"`
	IsPublic    bool      `json:"isPublic"`
	SortOrder   int       `json:"sortOrder"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}
