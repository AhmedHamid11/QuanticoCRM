package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
)

// RateLimitService manages API usage tracking and quota enforcement for Salesforce integration
type RateLimitService struct {
	dbManager *db.Manager
	authRepo  *repo.AuthRepo
}

// NewRateLimitService creates a new RateLimitService
func NewRateLimitService(dbManager *db.Manager, authRepo *repo.AuthRepo) *RateLimitService {
	return &RateLimitService{
		dbManager: dbManager,
		authRepo:  authRepo,
	}
}

// GetAPIUsageLast24Hours returns the total API calls made by an org in the last 24 hours
func (s *RateLimitService) GetAPIUsageLast24Hours(ctx context.Context, orgID string) (int, error) {
	// Get org and tenant DB
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get organization: %w", err)
	}

	// Get tenant DB (fallback to master if no DatabaseURL)
	var tenantDB db.DBConn
	if org.DatabaseURL != "" {
		tenantDB, err = s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			return 0, fmt.Errorf("failed to get tenant database: %w", err)
		}
	} else {
		tenantDB = s.dbManager.GetMasterDB()
	}

	// Calculate 24-hour cutoff
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	// Query sum of api_calls in last 24 hours
	query := `SELECT COALESCE(SUM(api_calls), 0) FROM api_usage_log WHERE org_id = ? AND timestamp >= ?`
	var totalCalls int
	err = tenantDB.QueryRowContext(ctx, query, orgID, cutoff).Scan(&totalCalls)
	if err != nil {
		// Handle "no such table" gracefully (tenant DB may not have migration yet)
		if strings.Contains(err.Error(), "no such table") {
			log.Printf("[RATELIMIT] api_usage_log table not found for org %s, returning 0 usage", orgID)
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query API usage: %w", err)
	}

	return totalCalls, nil
}

// CanMakeAPICalls checks if an org can make additional API calls without exceeding the pause threshold
func (s *RateLimitService) CanMakeAPICalls(ctx context.Context, orgID string, callCount int) (bool, error) {
	currentUsage, err := s.GetAPIUsageLast24Hours(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Check if current usage + new calls would exceed the pause threshold (80% of max)
	projectedUsage := currentUsage + callCount
	return projectedUsage <= entity.SalesforcePauseThreshold, nil
}

// RecordAPIUsage records API usage for an org and updates the sync job's api_calls_made
func (s *RateLimitService) RecordAPIUsage(ctx context.Context, orgID, jobID string, callCount int) error {
	// Get org and tenant DB
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Get tenant DB (fallback to master if no DatabaseURL)
	var tenantDB db.DBConn
	if org.DatabaseURL != "" {
		tenantDB, err = s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			return fmt.Errorf("failed to get tenant database: %w", err)
		}
	} else {
		tenantDB = s.dbManager.GetMasterDB()
	}

	// Insert usage record
	usageID := sfid.New("apilog")
	timestamp := time.Now().UTC().Format(time.RFC3339)
	createdAt := time.Now().UTC().Format(time.RFC3339)

	insertQuery := `
		INSERT INTO api_usage_log (id, org_id, timestamp, api_calls, job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = tenantDB.ExecContext(ctx, insertQuery, usageID, orgID, timestamp, callCount, jobID, createdAt)
	if err != nil {
		// Handle "no such table" gracefully (non-critical operation)
		if strings.Contains(err.Error(), "no such table") {
			log.Printf("[RATELIMIT] api_usage_log table not found for org %s, skipping usage recording", orgID)
			return nil
		}
		return fmt.Errorf("failed to insert API usage record: %w", err)
	}

	// Update sync_jobs.api_calls_made
	updateQuery := `UPDATE sync_jobs SET api_calls_made = ? WHERE id = ?`
	_, err = tenantDB.ExecContext(ctx, updateQuery, callCount, jobID)
	if err != nil {
		// Log warning but don't fail (sync job update is secondary)
		log.Printf("[RATELIMIT] Warning: Failed to update api_calls_made for job %s: %v", jobID, err)
	}

	return nil
}

// GetQuotaStatus returns the current quota status for an org
func (s *RateLimitService) GetQuotaStatus(ctx context.Context, orgID string) (*entity.QuotaStatus, error) {
	usage, err := s.GetAPIUsageLast24Hours(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Calculate percentage (avoid division by zero)
	percentage := 0
	if entity.SalesforceMaxDailyAPICalls > 0 {
		percentage = (usage * 100) / entity.SalesforceMaxDailyAPICalls
	}

	// Determine if paused (at or above threshold)
	isPaused := usage >= entity.SalesforcePauseThreshold

	return &entity.QuotaStatus{
		Usage:      usage,
		Limit:      entity.SalesforceMaxDailyAPICalls,
		Percentage: percentage,
		Threshold:  entity.SalesforcePauseThreshold,
		IsPaused:   isPaused,
	}, nil
}

// CleanupOldUsage deletes API usage records older than 25 hours (provides buffer beyond 24-hour window)
func (s *RateLimitService) CleanupOldUsage(ctx context.Context, orgID string) error {
	// Get org and tenant DB
	org, err := s.authRepo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Get tenant DB (fallback to master if no DatabaseURL)
	var tenantDB db.DBConn
	if org.DatabaseURL != "" {
		tenantDB, err = s.dbManager.GetTenantDB(ctx, orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			return fmt.Errorf("failed to get tenant database: %w", err)
		}
	} else {
		tenantDB = s.dbManager.GetMasterDB()
	}

	// Calculate 25-hour cutoff (buffer to avoid deleting records right at boundary)
	cutoff := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339)

	// Delete old records
	deleteQuery := `DELETE FROM api_usage_log WHERE org_id = ? AND timestamp < ?`
	_, err = tenantDB.ExecContext(ctx, deleteQuery, orgID, cutoff)
	if err != nil {
		// Handle "no such table" gracefully
		if strings.Contains(err.Error(), "no such table") {
			log.Printf("[RATELIMIT] api_usage_log table not found for org %s, skipping cleanup", orgID)
			return nil
		}
		return fmt.Errorf("failed to cleanup old usage records: %w", err)
	}

	return nil
}
