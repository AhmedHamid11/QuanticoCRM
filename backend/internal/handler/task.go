package handler

import (
	"context"
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// TaskHandler handles HTTP requests for tasks
type TaskHandler struct {
	repo                *repo.TaskRepo
	authRepo            *repo.AuthRepo
	tripwireService     TripwireServiceInterface
	validationService   ValidationServiceInterface
	notificationService NotificationServiceInterface
	defaultDB           db.DBConn
}

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(repo *repo.TaskRepo, authRepo *repo.AuthRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface, notificationService NotificationServiceInterface, defaultDB db.DBConn) *TaskHandler {
	return &TaskHandler{repo: repo, authRepo: authRepo, tripwireService: tripwireService, validationService: validationService, notificationService: notificationService, defaultDB: defaultDB}
}

// getDB returns the tenant database from context, falling back to default db
func (h *TaskHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getRepo returns the Task repo using the tenant database from context
func (h *TaskHandler) getRepo(c *fiber.Ctx) *repo.TaskRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// resolveUserNames adds createdByName and modifiedByName to records
func (h *TaskHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
	if h.authRepo == nil || len(records) == 0 {
		return
	}

	// Helper to extract user ID from interface{} (handles both string and *string)
	extractUserID := func(val interface{}) string {
		if val == nil {
			return ""
		}
		if s, ok := val.(string); ok {
			return s
		}
		if sp, ok := val.(*string); ok && sp != nil {
			return *sp
		}
		return ""
	}

	// Collect unique user IDs
	userIDSet := make(map[string]bool)
	for _, record := range records {
		if id := extractUserID(record["assignedUserId"]); id != "" {
			userIDSet[id] = true
		}
		if id := extractUserID(record["createdById"]); id != "" {
			userIDSet[id] = true
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			userIDSet[id] = true
		}
	}

	if len(userIDSet) == 0 {
		return
	}

	// Convert to slice
	userIDs := make([]string, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	// Lookup names
	userNames, err := h.authRepo.GetUserNamesByIDs(ctx, userIDs)
	if err != nil {
		log.Printf("WARNING: Failed to lookup user names: %v", err)
		return
	}

	// Apply names to records
	for _, record := range records {
		if id := extractUserID(record["assignedUserId"]); id != "" {
			record["assignedUserName"] = userNames[id]
		}
		if id := extractUserID(record["createdById"]); id != "" {
			record["createdByName"] = userNames[id]
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			record["modifiedByName"] = userNames[id]
		}
	}
}

// List returns all tasks for the current organization
func (h *TaskHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	owner := c.Query("owner")
	if owner == "me" {
		owner = c.Locals("userID").(string)
	}

	params := entity.TaskListParams{
		Search:         c.Query("search"),
		SortBy:         c.Query("sortBy"),
		SortDir:        c.Query("sortDir"),
		Page:           c.QueryInt("page", 1),
		PageSize:       c.QueryInt("pageSize", 20),
		Status:         c.Query("status"),
		Type:           c.Query("type"),
		ParentType:     c.Query("parentType"),
		ParentID:       c.Query("parentId"),
		DueBefore:      c.Query("dueBefore"),
		DueAfter:       c.Query("dueAfter"),
		GmailMessageID: c.Query("gmailMessageId"),
		Filter:         c.Query("filter"),
		KnownTotal:     c.QueryInt("knownTotal", 0),
		Owner:          owner,
	}

	result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Resolve user names for assigned_user, created_by and modified_by
	if h.authRepo != nil && len(result.Data) > 0 {
		userIDSet := make(map[string]bool)
		for _, task := range result.Data {
			if task.AssignedUserID != nil && *task.AssignedUserID != "" {
				userIDSet[*task.AssignedUserID] = true
			}
			if task.CreatedByID != nil && *task.CreatedByID != "" {
				userIDSet[*task.CreatedByID] = true
			}
			if task.ModifiedByID != nil && *task.ModifiedByID != "" {
				userIDSet[*task.ModifiedByID] = true
			}
		}
		if len(userIDSet) > 0 {
			userIDs := make([]string, 0, len(userIDSet))
			for id := range userIDSet {
				userIDs = append(userIDs, id)
			}
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				for i := range result.Data {
					if result.Data[i].AssignedUserID != nil {
						result.Data[i].AssignedUserName = userNames[*result.Data[i].AssignedUserID]
					}
					if result.Data[i].CreatedByID != nil {
						result.Data[i].CreatedByName = userNames[*result.Data[i].CreatedByID]
					}
					if result.Data[i].ModifiedByID != nil {
						result.Data[i].ModifiedByName = userNames[*result.Data[i].ModifiedByID]
					}
				}
			}
		}
	}

	return c.JSON(result)
}

// Get returns a single task by ID
func (h *TaskHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	task, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if task == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Task not found",
		})
	}

	// Resolve user names for assigned_user, created_by and modified_by
	if h.authRepo != nil {
		userIDs := make([]string, 0, 3)
		if task.AssignedUserID != nil && *task.AssignedUserID != "" {
			userIDs = append(userIDs, *task.AssignedUserID)
		}
		if task.CreatedByID != nil && *task.CreatedByID != "" {
			userIDs = append(userIDs, *task.CreatedByID)
		}
		if task.ModifiedByID != nil && *task.ModifiedByID != "" {
			userIDs = append(userIDs, *task.ModifiedByID)
		}
		if len(userIDs) > 0 {
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				if task.AssignedUserID != nil {
					task.AssignedUserName = userNames[*task.AssignedUserID]
				}
				if task.CreatedByID != nil {
					task.CreatedByName = userNames[*task.CreatedByID]
				}
				if task.ModifiedByID != nil {
					task.ModifiedByName = userNames[*task.ModifiedByID]
				}
			}
		}
	}

	return c.JSON(task)
}

// Create creates a new task
func (h *TaskHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.TaskCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "subject is required",
		})
	}

	// Validate status if provided
	if input.Status != "" {
		validStatuses := map[entity.TaskStatus]bool{
			entity.TaskStatusOpen:       true,
			entity.TaskStatusInProgress: true,
			entity.TaskStatusCompleted:  true,
			entity.TaskStatusDeferred:   true,
			entity.TaskStatusCancelled:  true,
		}
		if !validStatuses[input.Status] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid status value",
			})
		}
	}

	// Validate priority if provided
	if input.Priority != "" {
		validPriorities := map[entity.TaskPriority]bool{
			entity.TaskPriorityLow:    true,
			entity.TaskPriorityNormal: true,
			entity.TaskPriorityHigh:   true,
			entity.TaskPriorityUrgent: true,
		}
		if !validPriorities[input.Priority] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid priority value",
			})
		}
	}

	// Validate type if provided
	if input.Type != "" {
		validTypes := map[entity.TaskType]bool{
			entity.TaskTypeCall:    true,
			entity.TaskTypeEmail:   true,
			entity.TaskTypeMeeting: true,
			entity.TaskTypeTodo:    true,
		}
		if !validTypes[input.Type] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid type value",
			})
		}
	}

	// Validate parent consistency - both must be provided together
	if (input.ParentID != nil && input.ParentType == nil) || (input.ParentID == nil && input.ParentType != nil) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Both parentId and parentType must be provided together",
		})
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Task", "", "CREATE", nil, newRecord)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	task, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Fire tripwires for CREATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Task", task.ID, "CREATE", nil, StructToMap(task))
	}

	// Notify assigned user (don't notify yourself)
	if h.notificationService != nil && task.AssignedUserID != nil && *task.AssignedUserID != "" && *task.AssignedUserID != userID {
		go h.notificationService.CreateAssignmentNotification(context.Background(), h.getDB(c), orgID, *task.AssignedUserID, "Task", task.ID, task.Subject)
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// Update updates an existing task
func (h *TaskHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.TaskUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Fetch old record for tripwire, validation, and notification evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil || h.notificationService != nil {
		oldTask, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldTask)
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Task", id, "UPDATE", oldRecord, newRecord)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	task, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if task == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Task not found",
		})
	}

	// Fire tripwires for UPDATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Task", id, "UPDATE", oldRecord, StructToMap(task))
	}

	// Notify if assignment changed
	if h.notificationService != nil && task.AssignedUserID != nil && *task.AssignedUserID != "" {
		oldAssigned, _ := oldRecord["assignedUserId"].(string)
		if *task.AssignedUserID != oldAssigned {
			go h.notificationService.CreateAssignmentNotification(context.Background(), h.getDB(c), orgID, *task.AssignedUserID, "Task", id, task.Subject)
		}
	}

	return c.JSON(task)
}

// Delete soft-deletes a task
func (h *TaskHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldTask, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldTask)
	}

	// Validate before delete
	if h.validationService != nil && oldRecord != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Task", id, "DELETE", oldRecord, nil)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	err := h.getRepo(c).Delete(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Task not found",
		})
	}

	// Fire tripwires for DELETE event
	if h.tripwireService != nil && oldRecord != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Task", id, "DELETE", oldRecord, nil)
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// RegisterRoutes registers task routes on the Fiber app
func (h *TaskHandler) RegisterRoutes(app fiber.Router) {
	tasks := app.Group("/tasks")
	tasks.Get("/", h.List)
	tasks.Get("/:id", h.Get)
	tasks.Post("/", h.Create)
	tasks.Put("/:id", h.Update)
	tasks.Patch("/:id", h.Update)
	tasks.Delete("/:id", h.Delete)
}
