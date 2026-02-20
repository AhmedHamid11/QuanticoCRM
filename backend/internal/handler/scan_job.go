package handler

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// ScanJobHandler handles scan job and notification API endpoints
type ScanJobHandler struct {
	defaultDB        *sql.DB
	scanJobRepo      *repo.ScanJobRepo
	notificationRepo *repo.NotificationRepo
	scheduler        *service.ScanScheduler
	scanJobService   *service.ScanJobService

	// SSE subscriber management
	subscribers map[string]map[string]chan service.ProgressEvent // orgID -> subscriberID -> channel
	subMu       sync.RWMutex
}

// NewScanJobHandler creates a new scan job handler
func NewScanJobHandler(
	defaultDB *sql.DB,
	scanJobRepo *repo.ScanJobRepo,
	notificationRepo *repo.NotificationRepo,
	scheduler *service.ScanScheduler,
	scanJobService *service.ScanJobService,
) *ScanJobHandler {
	h := &ScanJobHandler{
		defaultDB:        defaultDB,
		scanJobRepo:      scanJobRepo,
		notificationRepo: notificationRepo,
		scheduler:        scheduler,
		scanJobService:   scanJobService,
		subscribers:      make(map[string]map[string]chan service.ProgressEvent),
	}

	// Wire the SSE broadcast callback into scanJobService
	scanJobService.SetProgressCallback(func(event service.ProgressEvent) {
		h.broadcastProgress(event)
	})

	return h
}

// getDB returns tenant DB from context, falling back to default
func (h *ScanJobHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getDBConn returns tenant DBConn from context, falling back to default
func (h *ScanJobHandler) getDBConn(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getScanJobRepo returns scan job repo with tenant DB
func (h *ScanJobHandler) getScanJobRepo(c *fiber.Ctx) *repo.ScanJobRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.scanJobRepo.WithDB(tenantDB)
	}
	return h.scanJobRepo
}

// getNotificationRepo returns notification repo with tenant DB
func (h *ScanJobHandler) getNotificationRepo(c *fiber.Ctx) *repo.NotificationRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.notificationRepo.WithDB(tenantDB)
	}
	return h.notificationRepo
}

// ========== Schedule Management (Admin Only) ==========

// ListSchedules returns all scan schedules for the org
func (h *ScanJobHandler) ListSchedules(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	repo := h.getScanJobRepo(c)

	schedules, err := repo.ListSchedules(c.Context(), orgID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			if err2 := dedup.EnsureDedupSchema(c.Context(), h.getDBConn(c)); err2 == nil {
				schedules, err = repo.ListSchedules(c.Context(), orgID)
				if err == nil {
					return c.JSON(schedules)
				}
			}
			return c.JSON([]any{})
		}
		log.Printf("Error listing schedules for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list schedules",
		})
	}

	return c.JSON(schedules)
}

// GetSchedule returns a single schedule for an entity type
func (h *ScanJobHandler) GetSchedule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entityType")
	repo := h.getScanJobRepo(c)

	schedule, err := repo.GetSchedule(c.Context(), orgID, entityType)
	if err != nil {
		log.Printf("Error getting schedule for org %s entity %s: %v", orgID, entityType, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get schedule",
		})
	}

	if schedule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Schedule not found",
		})
	}

	return c.JSON(schedule)
}

// UpsertSchedule creates or updates a scan schedule
func (h *ScanJobHandler) UpsertSchedule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entityType")

	var input struct {
		Frequency  string `json:"frequency"`
		DayOfWeek  *int   `json:"dayOfWeek"`
		DayOfMonth *int   `json:"dayOfMonth"`
		Hour       int    `json:"hour"`
		Minute     int    `json:"minute"`
		IsEnabled  bool   `json:"isEnabled"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate frequency
	if input.Frequency != "daily" && input.Frequency != "weekly" && input.Frequency != "monthly" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Frequency must be daily, weekly, or monthly",
		})
	}

	// Validate hour and minute
	if input.Hour < 0 || input.Hour > 23 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Hour must be between 0 and 23",
		})
	}
	if input.Minute < 0 || input.Minute > 59 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Minute must be between 0 and 59",
		})
	}

	// Validate frequency-specific fields
	if input.Frequency == "weekly" && (input.DayOfWeek == nil || *input.DayOfWeek < 0 || *input.DayOfWeek > 6) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Weekly schedules require dayOfWeek (0-6)",
		})
	}
	if input.Frequency == "monthly" && (input.DayOfMonth == nil || *input.DayOfMonth < 1 || *input.DayOfMonth > 28) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Monthly schedules require dayOfMonth (1-28)",
		})
	}

	// Check if schedule exists
	repo := h.getScanJobRepo(c)
	existingSchedule, _ := repo.GetSchedule(c.Context(), orgID, entityType)

	scheduleID := sfid.NewScanSchedule()
	if existingSchedule != nil {
		scheduleID = existingSchedule.ID
	}

	schedule := &entity.ScanSchedule{
		ID:         scheduleID,
		OrgID:      orgID,
		EntityType: entityType,
		Frequency:  input.Frequency,
		DayOfWeek:  input.DayOfWeek,
		DayOfMonth: input.DayOfMonth,
		Hour:       input.Hour,
		Minute:     input.Minute,
		IsEnabled:  input.IsEnabled,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	// Update in scheduler (hot-reload gocron job)
	if h.scheduler != nil {
		if err := h.scheduler.UpdateSchedule(c.Context(), *schedule); err != nil {
			log.Printf("Error updating schedule in scheduler: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update schedule",
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(schedule)
}

// DeleteSchedule deletes a scan schedule
func (h *ScanJobHandler) DeleteSchedule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entityType")

	if h.scheduler == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Scheduler not available",
		})
	}

	if err := h.scheduler.RemoveSchedule(c.Context(), orgID, entityType); err != nil {
		log.Printf("Error deleting schedule: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete schedule",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ========== Job Management (Admin Only) ==========

// ListJobs returns all scan jobs with pagination and optional entity filter
func (h *ScanJobHandler) ListJobs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Query("entityType")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repo := h.getScanJobRepo(c)

	var jobs []entity.ScanJob
	var total int
	var err error

	if entityType != "" {
		jobs, total, err = repo.ListJobsByEntity(c.Context(), orgID, entityType, pageSize, offset)
	} else {
		jobs, total, err = repo.ListJobs(c.Context(), orgID, pageSize, offset)
	}

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			if err2 := dedup.EnsureDedupSchema(c.Context(), h.getDBConn(c)); err2 == nil {
				if entityType != "" {
					jobs, total, err = repo.ListJobsByEntity(c.Context(), orgID, entityType, pageSize, offset)
				} else {
					jobs, total, err = repo.ListJobs(c.Context(), orgID, pageSize, offset)
				}
				if err == nil {
					return c.JSON(fiber.Map{"data": jobs, "total": total, "page": page, "pageSize": pageSize})
				}
			}
			return c.JSON(fiber.Map{"data": []any{}, "total": 0, "page": page, "pageSize": pageSize})
		}
		log.Printf("Error listing jobs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list jobs",
		})
	}

	return c.JSON(fiber.Map{
		"data":     jobs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetJob returns a single job with details
func (h *ScanJobHandler) GetJob(c *fiber.Ctx) error {
	jobID := c.Params("id")
	repo := h.getScanJobRepo(c)

	job, err := repo.GetJob(c.Context(), jobID)
	if err != nil {
		log.Printf("Error getting job %s: %v", jobID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	// Include checkpoint info for failed jobs
	var checkpoint *entity.ScanCheckpoint
	if job.Status == entity.ScanStatusFailed {
		checkpoint, _ = repo.GetCheckpoint(c.Context(), jobID)
	}

	response := fiber.Map{
		"job": job,
	}
	if checkpoint != nil {
		response["checkpoint"] = checkpoint
	}

	return c.JSON(response)
}

// TriggerManualScan starts a manual "Run Now" scan
func (h *ScanJobHandler) TriggerManualScan(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tenantDB := h.getDB(c)

	var input struct {
		EntityType string `json:"entityType"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.EntityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "entityType is required",
		})
	}

	if h.scheduler == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Scheduler not available",
		})
	}

	// Trigger manual scan (async)
	jobID, err := h.scheduler.TriggerManualScan(c.Context(), tenantDB, orgID, input.EntityType)
	if err != nil {
		log.Printf("Error triggering manual scan: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobId":  jobID,
		"status": "running",
	})
}

// RetryJob retries a failed job from its last checkpoint
func (h *ScanJobHandler) RetryJob(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	jobID := c.Params("id")
	tenantDB := h.getDB(c)
	repo := h.getScanJobRepo(c)

	// Verify job exists and is failed
	job, err := repo.GetJob(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	if job.Status != entity.ScanStatusFailed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only failed jobs can be retried",
		})
	}

	// Retry the job
	newJobID, err := h.scanJobService.RetryJob(c.Context(), tenantDB, orgID, jobID)
	if err != nil {
		log.Printf("Error retrying job %s: %v", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retry job",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobId":  newJobID,
		"status": "running",
	})
}

// CancelJob cancels a currently running scan job
func (h *ScanJobHandler) CancelJob(c *fiber.Ctx) error {
	jobID := c.Params("id")
	tenantDB := h.getDB(c)
	repo := h.getScanJobRepo(c)

	// Verify job exists and is running
	job, err := repo.GetJob(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	if job.Status != entity.ScanStatusRunning {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only running jobs can be cancelled",
		})
	}

	if err := h.scanJobService.CancelJob(c.Context(), tenantDB, jobID); err != nil {
		log.Printf("Error cancelling job %s: %v", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to cancel job",
		})
	}

	return c.JSON(fiber.Map{
		"jobId":  jobID,
		"status": "cancelled",
	})
}

// ========== SSE Progress Stream ==========

// StreamProgress streams real-time progress events via SSE
func (h *ScanJobHandler) StreamProgress(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	subscriberID := sfid.New("sub")

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Status(fiber.StatusOK).Context().SetBodyStreamWriter(
		fasthttp.StreamWriter(func(w *bufio.Writer) {
			eventChan := h.subscribe(orgID, subscriberID)
			defer h.unsubscribe(orgID, subscriberID)

			// Send initial heartbeat
			fmt.Fprintf(w, ": heartbeat\n\n")
			w.Flush()

			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case event := <-eventChan:
					data, _ := json.Marshal(event)
					fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
					if err := w.Flush(); err != nil {
						return // Client disconnected
					}
				case <-ticker.C:
					// Keepalive ping every 30 seconds
					fmt.Fprintf(w, ": keepalive\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				case <-c.Context().Done():
					return
				}
			}
		}),
	)
	return nil
}

// subscribe adds a subscriber for progress events
func (h *ScanJobHandler) subscribe(orgID, subscriberID string) chan service.ProgressEvent {
	h.subMu.Lock()
	defer h.subMu.Unlock()

	if h.subscribers[orgID] == nil {
		h.subscribers[orgID] = make(map[string]chan service.ProgressEvent)
	}

	ch := make(chan service.ProgressEvent, 20)
	h.subscribers[orgID][subscriberID] = ch
	return ch
}

// unsubscribe removes a subscriber
func (h *ScanJobHandler) unsubscribe(orgID, subscriberID string) {
	h.subMu.Lock()
	defer h.subMu.Unlock()

	if orgSubs, ok := h.subscribers[orgID]; ok {
		if ch, exists := orgSubs[subscriberID]; exists {
			close(ch)
			delete(orgSubs, subscriberID)
		}
		if len(orgSubs) == 0 {
			delete(h.subscribers, orgID)
		}
	}
}

// broadcastProgress sends progress event to all subscribers for an org
func (h *ScanJobHandler) broadcastProgress(event service.ProgressEvent) {
	h.subMu.RLock()
	defer h.subMu.RUnlock()

	if orgSubs, ok := h.subscribers[event.OrgID]; ok {
		for _, ch := range orgSubs {
			select {
			case ch <- event:
			default:
				// Skip if channel is full (subscriber is slow)
			}
		}
	}
}

// ========== Notification Endpoints (All Authenticated Users) ==========

// ListNotifications returns user's notifications with pagination
func (h *ScanJobHandler) ListNotifications(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	includeRead := c.Query("includeRead") == "true"
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repo := h.getNotificationRepo(c)

	notifications, total, err := repo.ListForUser(c.Context(), orgID, userID, includeRead, pageSize, offset)
	if err != nil {
		log.Printf("Error listing notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list notifications",
		})
	}

	return c.JSON(fiber.Map{
		"data":     notifications,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// UnreadCount returns count of unread notifications for header badge
func (h *ScanJobHandler) UnreadCount(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	repo := h.getNotificationRepo(c)

	count, err := repo.CountUnread(c.Context(), orgID, userID)
	if err != nil {
		log.Printf("Error counting unread notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count unread notifications",
		})
	}

	return c.JSON(fiber.Map{
		"count": count,
	})
}

// MarkAsRead marks a single notification as read
func (h *ScanJobHandler) MarkAsRead(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	repo := h.getNotificationRepo(c)

	if err := repo.MarkAsRead(c.Context(), notificationID); err != nil {
		log.Printf("Error marking notification as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mark notification as read",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// MarkAllAsRead marks all user's notifications as read
func (h *ScanJobHandler) MarkAllAsRead(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	repo := h.getNotificationRepo(c)

	if err := repo.MarkAllAsRead(c.Context(), orgID, userID); err != nil {
		log.Printf("Error marking all notifications as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mark all notifications as read",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// DismissNotification dismisses a notification
func (h *ScanJobHandler) DismissNotification(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	repo := h.getNotificationRepo(c)

	if err := repo.Dismiss(c.Context(), notificationID); err != nil {
		log.Printf("Error dismissing notification: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to dismiss notification",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ========== Route Registration ==========

// RegisterAdminRoutes registers admin-only scan job routes
func (h *ScanJobHandler) RegisterAdminRoutes(app fiber.Router) {
	scanJobs := app.Group("/scan-jobs")

	// Schedule management
	scanJobs.Get("/schedules", h.ListSchedules)
	scanJobs.Get("/schedules/:entityType", h.GetSchedule)
	scanJobs.Put("/schedules/:entityType", h.UpsertSchedule)
	scanJobs.Delete("/schedules/:entityType", h.DeleteSchedule)

	// Job management
	scanJobs.Get("", h.ListJobs)
	scanJobs.Get("/:id", h.GetJob)
	scanJobs.Post("/run", h.TriggerManualScan)
	scanJobs.Post("/:id/retry", h.RetryJob)
	scanJobs.Post("/:id/cancel", h.CancelJob)

	// SSE progress stream
	scanJobs.Get("/progress/stream", h.StreamProgress)
}

// RegisterPublicRoutes registers notification routes for all authenticated users
func (h *ScanJobHandler) RegisterPublicRoutes(app fiber.Router) {
	notifications := app.Group("/notifications")
	notifications.Get("", h.ListNotifications)
	notifications.Get("/unread-count", h.UnreadCount)
	notifications.Post("/:id/read", h.MarkAsRead)
	notifications.Post("/read-all", h.MarkAllAsRead)
	notifications.Delete("/:id", h.DismissNotification)
}
