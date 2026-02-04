package handler

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

type AuditHandler struct {
	repo *repo.AuditRepo
}

func NewAuditHandler(repo *repo.AuditRepo) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// List returns paginated audit logs with optional filters
func (h *AuditHandler) List(c *fiber.Ctx) error {
	// Get org ID from context (set by auth middleware)
	orgID, ok := c.Locals("orgID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found",
		})
	}

	// Platform admins can view other orgs' logs via orgId query param
	if isPlatformAdmin, _ := c.Locals("isPlatformAdmin").(bool); isPlatformAdmin {
		if queryOrgID := c.Query("orgId"); queryOrgID != "" {
			orgID = queryOrgID
		}
	}

	// Parse filters from query params
	filters := &entity.AuditLogFilters{
		Page:     1,
		PageSize: 50,
	}

	// Parse page
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}

	// Parse pageSize
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			filters.PageSize = pageSize
		}
	}

	// Parse event types (comma-separated)
	if eventTypesStr := c.Query("eventTypes"); eventTypesStr != "" {
		filters.EventTypes = strings.Split(eventTypesStr, ",")
	}

	// Parse actor ID
	if actorID := c.Query("userId"); actorID != "" {
		filters.ActorID = actorID
	}

	// Parse date range
	if dateFromStr := c.Query("dateFrom"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filters.DateFrom = &dateFrom
		}
	}
	if dateToStr := c.Query("dateTo"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			// Set to end of day
			endOfDay := dateTo.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.DateTo = &endOfDay
		}
	}

	// Query audit logs
	response, err := h.repo.List(c.Context(), orgID, filters)
	if err != nil {
		return util.NewAPIError(c, fiber.StatusInternalServerError, fmt.Errorf("failed to list audit logs: %w", err), util.ErrCategoryDatabase)
	}

	return c.JSON(response)
}

// Export returns audit logs in CSV or JSON format
func (h *AuditHandler) Export(c *fiber.Ctx) error {
	// Get org ID from context
	orgID, ok := c.Locals("orgID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found",
		})
	}

	// Platform admins can view other orgs' logs via orgId query param
	if isPlatformAdmin, _ := c.Locals("isPlatformAdmin").(bool); isPlatformAdmin {
		if queryOrgID := c.Query("orgId"); queryOrgID != "" {
			orgID = queryOrgID
		}
	}

	// Parse format (csv or json, default json)
	format := strings.ToLower(c.Query("format", "json"))
	if format != "csv" && format != "json" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid format. Use 'csv' or 'json'",
		})
	}

	// Parse filters (same as List but with large pageSize)
	filters := &entity.AuditLogFilters{
		Page:     1,
		PageSize: 10000, // Large limit for export
	}

	// Parse event types (comma-separated)
	if eventTypesStr := c.Query("eventTypes"); eventTypesStr != "" {
		filters.EventTypes = strings.Split(eventTypesStr, ",")
	}

	// Parse actor ID
	if actorID := c.Query("userId"); actorID != "" {
		filters.ActorID = actorID
	}

	// Parse date range
	if dateFromStr := c.Query("dateFrom"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filters.DateFrom = &dateFrom
		}
	}
	if dateToStr := c.Query("dateTo"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			endOfDay := dateTo.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.DateTo = &endOfDay
		}
	}

	// Query audit logs
	response, err := h.repo.List(c.Context(), orgID, filters)
	if err != nil {
		return util.NewAPIError(c, fiber.StatusInternalServerError, fmt.Errorf("failed to list audit logs for export: %w", err), util.ErrCategoryDatabase)
	}

	// Export based on format
	if format == "csv" {
		return h.exportCSV(c, response.Data)
	}

	// JSON export
	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", "attachment; filename=audit-logs-export.json")
	return c.JSON(response.Data)
}

// exportCSV writes audit logs as CSV
func (h *AuditHandler) exportCSV(c *fiber.Ctx, entries []entity.AuditLogEntry) error {
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=audit-logs-export.csv")

	// Create CSV writer
	writer := csv.NewWriter(c)
	defer writer.Flush()

	// Write header
	header := []string{
		"ID",
		"Timestamp",
		"Event Type",
		"Actor ID",
		"Actor Email",
		"Target ID",
		"Target Type",
		"IP Address",
		"User Agent",
		"Success",
		"Error Message",
		"Details",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write rows
	for _, entry := range entries {
		successStr := "true"
		if !entry.Success {
			successStr = "false"
		}

		row := []string{
			entry.ID,
			entry.CreatedAt.Format(time.RFC3339),
			entry.EventType,
			entry.ActorID,
			entry.ActorEmail,
			entry.TargetID,
			entry.TargetType,
			entry.IPAddress,
			entry.UserAgent,
			successStr,
			entry.ErrorMsg,
			entry.Details,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// VerifyChain verifies the hash chain integrity for an org
func (h *AuditHandler) VerifyChain(c *fiber.Ctx) error {
	// Get org ID from context
	orgID, ok := c.Locals("orgID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found",
		})
	}

	// Platform admins can verify other orgs' chains via orgId query param
	if isPlatformAdmin, _ := c.Locals("isPlatformAdmin").(bool); isPlatformAdmin {
		if queryOrgID := c.Query("orgId"); queryOrgID != "" {
			orgID = queryOrgID
		}
	}

	// Verify chain integrity
	result, err := h.repo.VerifyChainIntegrity(c.Context(), orgID)
	if err != nil {
		return util.NewAPIError(c, fiber.StatusInternalServerError, fmt.Errorf("failed to verify chain integrity: %w", err), util.ErrCategoryDatabase)
	}

	return c.JSON(result)
}

// GetEventTypes returns list of all event types for filter dropdown
func (h *AuditHandler) GetEventTypes(c *fiber.Ctx) error {
	eventTypes := []map[string]string{
		{"value": string(entity.AuditEventLoginSuccess), "label": "Login Success"},
		{"value": string(entity.AuditEventLoginFailed), "label": "Login Failed"},
		{"value": string(entity.AuditEventLogout), "label": "Logout"},
		{"value": string(entity.AuditEventPasswordReset), "label": "Password Reset"},
		{"value": string(entity.AuditEventPasswordChange), "label": "Password Change"},
		{"value": string(entity.AuditEventUserCreate), "label": "User Created"},
		{"value": string(entity.AuditEventUserUpdate), "label": "User Updated"},
		{"value": string(entity.AuditEventUserDelete), "label": "User Deleted"},
		{"value": string(entity.AuditEventUserInvite), "label": "User Invited"},
		{"value": string(entity.AuditEventRoleChange), "label": "Role Change"},
		{"value": string(entity.AuditEventUserStatusChange), "label": "User Status Change"},
		{"value": string(entity.AuditEventImpersonationStart), "label": "Impersonation Started"},
		{"value": string(entity.AuditEventImpersonationStop), "label": "Impersonation Stopped"},
		{"value": string(entity.AuditEventAPITokenCreate), "label": "API Token Created"},
		{"value": string(entity.AuditEventAPITokenRevoke), "label": "API Token Revoked"},
		{"value": string(entity.AuditEventAuthorizationDenied), "label": "Authorization Denied"},
		{"value": string(entity.AuditEventOrgSettingsChange), "label": "Organization Settings Changed"},
	}

	return c.JSON(fiber.Map{
		"eventTypes": eventTypes,
	})
}
