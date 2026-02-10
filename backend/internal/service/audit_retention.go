package service

import (
	"context"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/repo"
)

// RetentionService handles audit log retention and cleanup operations
type RetentionService struct {
	repo *repo.AuditRepo
}

// NewRetentionService creates a new RetentionService
func NewRetentionService(auditRepo *repo.AuditRepo) *RetentionService {
	return &RetentionService{repo: auditRepo}
}

// WithDB returns a new RetentionService using a different DB connection (for tenant switching)
func (s *RetentionService) WithDB(auditRepo *repo.AuditRepo) *RetentionService {
	return &RetentionService{repo: auditRepo}
}

// CleanupOldLogs deletes audit logs older than 7 years (SOX compliance boundary)
// Returns the number of deleted entries
func (s *RetentionService) CleanupOldLogs(ctx context.Context, orgID string) (int, error) {
	cutoffDate := time.Now().UTC().AddDate(-7, 0, 0)
	count, err := s.repo.DeleteOlderThan(ctx, orgID, cutoffDate)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		log.Printf("[AUDIT RETENTION] Deleted %d audit log entries older than %s for org %s", count, cutoffDate.Format("2006-01-02"), orgID)
	}
	return count, nil
}
