package handler

import (
	"context"
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// ContactHandler handles HTTP requests for contacts
type ContactHandler struct {
	repo              *repo.ContactRepo
	taskRepo          *repo.TaskRepo
	authRepo          *repo.AuthRepo
	tripwireService   TripwireServiceInterface
	validationService ValidationServiceInterface
}

// NewContactHandler creates a new ContactHandler
func NewContactHandler(repo *repo.ContactRepo, taskRepo *repo.TaskRepo, authRepo *repo.AuthRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface) *ContactHandler {
	return &ContactHandler{repo: repo, taskRepo: taskRepo, authRepo: authRepo, tripwireService: tripwireService, validationService: validationService}
}

// getRepo returns the contact repo using the tenant database from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *ContactHandler) getRepo(c *fiber.Ctx) *repo.ContactRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// getTaskRepo returns the task repo using the tenant database from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *ContactHandler) getTaskRepo(c *fiber.Ctx) *repo.TaskRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.taskRepo.WithDB(tenantDB)
	}
	return h.taskRepo
}

// resolveUserNames adds createdByName and modifiedByName to records
func (h *ContactHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
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
		if id := extractUserID(record["createdById"]); id != "" {
			record["createdByName"] = userNames[id]
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			record["modifiedByName"] = userNames[id]
		}
	}
}

// List returns all contacts for the current organization
func (h *ContactHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	params := entity.ContactListParams{
		Search:     c.Query("search"),
		SortBy:     c.Query("sortBy"),
		SortDir:    c.Query("sortDir"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
		Filter:     c.Query("filter"),
		KnownTotal: c.QueryInt("knownTotal", 0),
	}

	result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Resolve user names for created_by and modified_by
	if h.authRepo != nil && len(result.Data) > 0 {
		userIDSet := make(map[string]bool)
		for _, contact := range result.Data {
			if contact.CreatedByID != nil && *contact.CreatedByID != "" {
				userIDSet[*contact.CreatedByID] = true
			}
			if contact.ModifiedByID != nil && *contact.ModifiedByID != "" {
				userIDSet[*contact.ModifiedByID] = true
			}
		}
		if len(userIDSet) > 0 {
			userIDs := make([]string, 0, len(userIDSet))
			for id := range userIDSet {
				userIDs = append(userIDs, id)
			}
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				for i := range result.Data {
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

// Get returns a single contact by ID
func (h *ContactHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	contact, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if contact == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Contact not found",
		})
	}

	// Resolve user names for created_by and modified_by
	if h.authRepo != nil {
		userIDs := make([]string, 0, 2)
		if contact.CreatedByID != nil && *contact.CreatedByID != "" {
			userIDs = append(userIDs, *contact.CreatedByID)
		}
		if contact.ModifiedByID != nil && *contact.ModifiedByID != "" {
			userIDs = append(userIDs, *contact.ModifiedByID)
		}
		if len(userIDs) > 0 {
			if userNames, err := h.authRepo.GetUserNamesByIDs(c.Context(), userIDs); err == nil {
				if contact.CreatedByID != nil {
					contact.CreatedByName = userNames[*contact.CreatedByID]
				}
				if contact.ModifiedByID != nil {
					contact.ModifiedByName = userNames[*contact.ModifiedByID]
				}
			}
		}
	}

	return c.JSON(contact)
}

// Create creates a new contact
func (h *ContactHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.ContactCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.LastName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "lastName is required",
		})
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Contact", "", "CREATE", nil, newRecord)
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

	contact, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Fire tripwires for CREATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Contact", contact.ID, "CREATE", nil, StructToMap(contact))
	}

	return c.Status(fiber.StatusCreated).JSON(contact)
}

// Update updates an existing contact
func (h *ContactHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.ContactUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldContact, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldContact)
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Contact", id, "UPDATE", oldRecord, newRecord)
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

	contact, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if contact == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Contact not found",
		})
	}

	// Fire tripwires for UPDATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Contact", id, "UPDATE", oldRecord, StructToMap(contact))
	}

	return c.JSON(contact)
}

// Delete soft-deletes a contact
func (h *ContactHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldContact, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldContact)
	}

	// Validate before delete
	if h.validationService != nil && oldRecord != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Contact", id, "DELETE", oldRecord, nil)
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
			"error": "Contact not found",
		})
	}

	// Fire tripwires for DELETE event
	if h.tripwireService != nil && oldRecord != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Contact", id, "DELETE", oldRecord, nil)
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ListTasks returns tasks linked to a specific contact
func (h *ContactHandler) ListTasks(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	contactID := c.Params("id")

	// First verify the contact exists
	contact, err := h.getRepo(c).GetByID(c.Context(), orgID, contactID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if contact == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Contact not found",
		})
	}

	// Fetch tasks using limit/offset pagination (for Load More)
	params := entity.TaskListParams{
		ParentType: "Contact",
		ParentID:   contactID,
		SortBy:     c.Query("sortBy", "created_at"),
		SortDir:    c.Query("sortDir", "desc"),
		Page:       1, // We'll use offset instead
		PageSize:   c.QueryInt("limit", 20),
	}

	// Convert offset to page for the repo
	offset := c.QueryInt("offset", 0)
	if offset > 0 && params.PageSize > 0 {
		params.Page = (offset / params.PageSize) + 1
	}

	result, err := h.taskRepo.ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// RegisterRoutes registers contact routes on the Fiber app
func (h *ContactHandler) RegisterRoutes(app fiber.Router) {
	contacts := app.Group("/contacts")
	contacts.Get("/", h.List)
	contacts.Get("/:id", h.Get)
	contacts.Get("/:id/tasks", h.ListTasks)
	contacts.Post("/", h.Create)
	contacts.Put("/:id", h.Update)
	contacts.Patch("/:id", h.Update)
	contacts.Delete("/:id", h.Delete)
}
