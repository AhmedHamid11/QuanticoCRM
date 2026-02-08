package entity

import "time"

// Notification represents an in-app notification for a user
type Notification struct {
	ID          string     `json:"id" db:"id"`
	OrgID       string     `json:"orgId" db:"org_id"`
	UserID      string     `json:"userId" db:"user_id"`
	Type        string     `json:"type" db:"type"`
	Title       string     `json:"title" db:"title"`
	Message     string     `json:"message" db:"message"`
	LinkURL     *string    `json:"linkUrl,omitempty" db:"link_url"`
	IsRead      bool       `json:"isRead" db:"is_read"`
	IsDismissed bool       `json:"isDismissed" db:"is_dismissed"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" db:"expires_at"`
}

// Notification type constants
const (
	NotificationTypeScanComplete = "scan_complete"
	NotificationTypeScanFailed   = "scan_failed"
)
