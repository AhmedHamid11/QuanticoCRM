package entity

import "time"

// ListView represents a saved list view configuration
type ListView struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"orgId"`
	EntityName  string    `json:"entityName"`
	Name        string    `json:"name"`
	FilterQuery string    `json:"filterQuery"`
	Columns     string    `json:"columns"` // JSON array of column names
	SortBy      string    `json:"sortBy"`
	SortDir     string    `json:"sortDir"`
	IsDefault   bool      `json:"isDefault"`
	IsSystem    bool      `json:"isSystem"`
	CreatedByID string    `json:"createdById,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}

// ListViewInput represents input for creating/updating a list view
type ListViewInput struct {
	Name        string   `json:"name"`
	FilterQuery string   `json:"filterQuery"`
	Columns     []string `json:"columns"`
	SortBy      string   `json:"sortBy"`
	SortDir     string   `json:"sortDir"`
	IsDefault   bool     `json:"isDefault"`
}
