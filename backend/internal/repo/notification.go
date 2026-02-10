package repo

import (
	"context"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

type NotificationRepo struct {
	db db.DBConn
}

func NewNotificationRepo(conn db.DBConn) *NotificationRepo {
	return &NotificationRepo{db: conn}
}

func (r *NotificationRepo) WithDB(conn db.DBConn) *NotificationRepo {
	return &NotificationRepo{db: conn}
}

// CreateNotification creates a new notification
func (r *NotificationRepo) CreateNotification(ctx context.Context, notification *entity.Notification) error {
	isReadInt := 0
	if notification.IsRead {
		isReadInt = 1
	}
	isDismissedInt := 0
	if notification.IsDismissed {
		isDismissedInt = 1
	}

	var expiresAt *string
	if notification.ExpiresAt != nil {
		t := notification.ExpiresAt.Format(time.RFC3339)
		expiresAt = &t
	}

	query := `
		INSERT INTO notifications
		(id, org_id, user_id, type, title, message, link_url, is_read, is_dismissed, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		notification.ID, notification.OrgID, notification.UserID, notification.Type,
		notification.Title, notification.Message, notification.LinkURL,
		isReadInt, isDismissedInt, now, expiresAt)

	return err
}

// ListForUser retrieves notifications for a user
func (r *NotificationRepo) ListForUser(ctx context.Context, orgID, userID string, includeRead bool, limit, offset int) ([]entity.Notification, int, error) {
	// Build query based on includeRead flag
	countQuery := `SELECT COUNT(*) FROM notifications WHERE org_id = ? AND user_id = ? AND is_dismissed = 0`
	query := `
		SELECT id, org_id, user_id, type, title, message, link_url, is_read, is_dismissed, created_at, expires_at
		FROM notifications
		WHERE org_id = ? AND user_id = ? AND is_dismissed = 0
	`

	if !includeRead {
		countQuery += ` AND is_read = 0`
		query += ` AND is_read = 0`
	}

	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, orgID, userID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated notifications
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, orgID, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []entity.Notification
	for rows.Next() {
		var notification entity.Notification
		var isReadInt, isDismissedInt int
		var createdAt *string
		var expiresAt *string

		err = rows.Scan(
			&notification.ID, &notification.OrgID, &notification.UserID, &notification.Type,
			&notification.Title, &notification.Message, &notification.LinkURL,
			&isReadInt, &isDismissedInt, &createdAt, &expiresAt,
		)
		if err != nil {
			continue
		}

		notification.IsRead = isReadInt == 1
		notification.IsDismissed = isDismissedInt == 1

		// Parse timestamps
		if createdAt != nil && *createdAt != "" {
			notification.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		if expiresAt != nil && *expiresAt != "" {
			t, _ := time.Parse(time.RFC3339, *expiresAt)
			notification.ExpiresAt = &t
		}

		notifications = append(notifications, notification)
	}

	return notifications, totalCount, nil
}

// CountUnread counts unread notifications for a user
func (r *NotificationRepo) CountUnread(ctx context.Context, orgID, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE org_id = ? AND user_id = ? AND is_read = 0 AND is_dismissed = 0`
	var count int
	err := r.db.QueryRowContext(ctx, query, orgID, userID).Scan(&count)
	return count, err
}

// MarkAsRead marks a notification as read
func (r *NotificationRepo) MarkAsRead(ctx context.Context, notificationID string) error {
	query := `UPDATE notifications SET is_read = 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, notificationID)
	return err
}

// MarkAllAsRead marks all notifications as read for a user
func (r *NotificationRepo) MarkAllAsRead(ctx context.Context, orgID, userID string) error {
	query := `UPDATE notifications SET is_read = 1 WHERE org_id = ? AND user_id = ? AND is_dismissed = 0`
	_, err := r.db.ExecContext(ctx, query, orgID, userID)
	return err
}

// Dismiss dismisses a notification
func (r *NotificationRepo) Dismiss(ctx context.Context, notificationID string) error {
	query := `UPDATE notifications SET is_dismissed = 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, notificationID)
	return err
}

// CleanupExpired deletes expired notifications
func (r *NotificationRepo) CleanupExpired(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `DELETE FROM notifications WHERE expires_at IS NOT NULL AND expires_at < ?`
	_, err := r.db.ExecContext(ctx, query, now)
	return err
}
