package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
)

// NotificationService handles creation of in-app notifications
type NotificationService struct {
	notificationRepo *repo.NotificationRepo
	authRepo         *repo.AuthRepo
}

// NewNotificationService creates a new notification service
func NewNotificationService(notificationRepo *repo.NotificationRepo, authRepo *repo.AuthRepo) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		authRepo:         authRepo,
	}
}

// CreateScanCompleteNotification creates in-app notifications for scan completion
// Per CONTEXT.md locked decisions:
// - Message: "{EntityType} scan complete" (no duplicate count)
// - Link: /data-quality/scan-jobs/{jobID}
// - Notify all admin/owner users in org
// - Always notify, even when zero duplicates found
func (s *NotificationService) CreateScanCompleteNotification(ctx context.Context, conn db.DBConn, orgID, jobID, entityType string) error {
	// Get all admin users in the org
	adminUsers, err := s.getAdminUsers(ctx, conn, orgID)
	if err != nil {
		log.Printf("Warning: Failed to get admin users for org %s: %v", orgID, err)
		return err
	}

	if len(adminUsers) == 0 {
		log.Printf("Warning: No admin users found for org %s", orgID)
		return nil
	}

	// Create notification for each admin user
	title := fmt.Sprintf("%s scan complete", entityType)
	message := fmt.Sprintf("%s scan complete", entityType)
	linkURL := fmt.Sprintf("/data-quality/scan-jobs/%s", jobID)
	expiresAt := time.Now().Add(30 * 24 * time.Hour) // Auto-dismiss after 30 days

	for _, user := range adminUsers {
		notification := &entity.Notification{
			ID:          sfid.NewNotification(),
			OrgID:       orgID,
			UserID:      user.ID,
			Type:        entity.NotificationTypeScanComplete,
			Title:       title,
			Message:     message,
			LinkURL:     &linkURL,
			IsRead:      false,
			IsDismissed: false,
			CreatedAt:   time.Now().UTC(),
			ExpiresAt:   &expiresAt,
		}

		if err := s.notificationRepo.WithDB(conn).CreateNotification(ctx, notification); err != nil {
			log.Printf("Warning: Failed to create notification for user %s: %v", user.ID, err)
			// Continue creating notifications for other users (best effort)
		}
	}

	log.Printf("Created scan complete notifications for %d admin users in org %s", len(adminUsers), orgID)
	return nil
}

// CreateScanFailureNotification creates in-app notifications for scan failure
// Per CONTEXT.md: "{Entity} scan failed at X% -- click to retry"
func (s *NotificationService) CreateScanFailureNotification(ctx context.Context, conn db.DBConn, orgID, jobID, entityType string, progressPercent int) error {
	// Get all admin users in the org
	adminUsers, err := s.getAdminUsers(ctx, conn, orgID)
	if err != nil {
		log.Printf("Warning: Failed to get admin users for org %s: %v", orgID, err)
		return err
	}

	if len(adminUsers) == 0 {
		log.Printf("Warning: No admin users found for org %s", orgID)
		return nil
	}

	// Create notification for each admin user
	title := fmt.Sprintf("%s scan failed", entityType)
	message := fmt.Sprintf("%s scan failed at %d%% -- click to retry", entityType, progressPercent)
	linkURL := fmt.Sprintf("/data-quality/scan-jobs/%s", jobID)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	for _, user := range adminUsers {
		notification := &entity.Notification{
			ID:          sfid.NewNotification(),
			OrgID:       orgID,
			UserID:      user.ID,
			Type:        entity.NotificationTypeScanFailed,
			Title:       title,
			Message:     message,
			LinkURL:     &linkURL,
			IsRead:      false,
			IsDismissed: false,
			CreatedAt:   time.Now().UTC(),
			ExpiresAt:   &expiresAt,
		}

		if err := s.notificationRepo.WithDB(conn).CreateNotification(ctx, notification); err != nil {
			log.Printf("Warning: Failed to create notification for user %s: %v", user.ID, err)
			// Continue creating notifications for other users (best effort)
		}
	}

	log.Printf("Created scan failure notifications for %d admin users in org %s", len(adminUsers), orgID)
	return nil
}

// getAdminUsers retrieves all users with admin or owner role in an org
func (s *NotificationService) getAdminUsers(ctx context.Context, conn db.DBConn, orgID string) ([]entity.User, error) {
	// Use ListUsersByOrg to get all users with their roles
	// We'll fetch the first page with a large page size to get all users
	authRepoWithConn := s.authRepo.WithDB(conn)
	response, err := authRepoWithConn.ListUsersByOrg(ctx, orgID, 1, 1000)
	if err != nil {
		return nil, err
	}

	// Filter for admin and owner roles
	var adminUsers []entity.User
	for _, userWithMembership := range response.Data {
		if userWithMembership.Role == "admin" || userWithMembership.Role == "owner" {
			adminUsers = append(adminUsers, userWithMembership.User)
		}
	}

	return adminUsers, nil
}
