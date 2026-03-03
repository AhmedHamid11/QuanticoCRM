package handler

import (
	"context"
	"database/sql"
	"log"

	"github.com/fastcrm/backend/internal/cache"
	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// AccountHandler handles HTTP requests for accounts
type AccountHandler struct {
	repo                *repo.AccountRepo
	taskRepo            *repo.TaskRepo
	db                  *sql.DB
	metadataRepo        *repo.MetadataRepo
	metadataCache       *cache.MetadataCache // optional cache for field lookups
	authRepo            *repo.AuthRepo
	tripwireService     TripwireServiceInterface
	validationService   ValidationServiceInterface
	notificationService NotificationServiceInterface
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(repo *repo.AccountRepo, taskRepo *repo.TaskRepo, db *sql.DB, metadataRepo *repo.MetadataRepo, authRepo *repo.AuthRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface, notificationService NotificationServiceInterface) *AccountHandler {
	return &AccountHandler{repo: repo, taskRepo: taskRepo, db: db, metadataRepo: metadataRepo, authRepo: authRepo, tripwireService: tripwireService, validationService: validationService, notificationService: notificationService}
}

// SetMetadataCache injects the shared metadata cache for field lookups.
func (h *AccountHandler) SetMetadataCache(mc *cache.MetadataCache) {
	h.metadataCache = mc
}

// getRepo returns the Account repo using the tenant database from context
func (h *AccountHandler) getRepo(c *fiber.Ctx) *repo.AccountRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// getTaskRepo returns the Task repo using the tenant database from context
func (h *AccountHandler) getTaskRepo(c *fiber.Ctx) *repo.TaskRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.taskRepo.WithDB(tenantDB)
	}
	return h.taskRepo
}

// getMetadataRepo returns the Metadata repo using the tenant database from context
func (h *AccountHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getMetadataCache returns the MetadataCache scoped to the tenant DB for this request.
func (h *AccountHandler) getMetadataCache(c *fiber.Ctx) *cache.MetadataCache {
	if h.metadataCache == nil {
		return cache.NewMetadataCache(h.getMetadataRepo(c), cache.DefaultMetadataTTL)
	}
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.metadataCache.WithDB(tenantDB)
	}
	return h.metadataCache
}

// getDBConn returns the tenant database connection as db.DBConn
func (h *AccountHandler) getDBConn(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.db
}

// getRawDB returns the raw database connection for the tenant
func (h *AccountHandler) getRawDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		if sqlDB, ok := tenantDB.(*sql.DB); ok {
			return sqlDB
		}
	}
	return h.db
}

// resolveUserNames adds createdByName and modifiedByName to records
func (h *AccountHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
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

// List returns all accounts for the current organization
func (h *AccountHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	owner := c.Query("owner")
	if owner == "me" {
		owner = c.Locals("userID").(string)
	}

	params := entity.AccountListParams{
		Search:     c.Query("search"),
		SortBy:     c.Query("sortBy"),
		SortDir:    c.Query("sortDir"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("pageSize", 20),
		Filter:     c.Query("filter"),
		KnownTotal: c.QueryInt("knownTotal", 0),
		Owner:      owner,
	}

	result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check for rollup fields
	fields, _ := h.getMetadataCache(c).ListFields(c.Context(), orgID, "Account")
	var rollupFields []*entity.FieldDef
	for i := range fields {
		if fields[i].Type == entity.FieldTypeRollup {
			rollupFields = append(rollupFields, &fields[i])
		}
	}

	// If no rollup fields or no data, still need to resolve user names
	if len(rollupFields) == 0 {
		if len(result.Data) == 0 {
			return c.JSON(result)
		}
		// Convert to maps for user name resolution
		records := make([]map[string]interface{}, len(result.Data))
		for i, account := range result.Data {
			records[i] = map[string]interface{}{
				"id":                        account.ID,
				"orgId":                     account.OrgID,
				"name":                      account.Name,
				"website":                   account.Website,
				"emailAddress":              account.EmailAddress,
				"phoneNumber":               account.PhoneNumber,
				"type":                      account.Type,
				"industry":                  account.Industry,
				"sicCode":                   account.SicCode,
				"billingAddressStreet":      account.BillingAddressStreet,
				"billingAddressCity":        account.BillingAddressCity,
				"billingAddressState":       account.BillingAddressState,
				"billingAddressCountry":     account.BillingAddressCountry,
				"billingAddressPostalCode":  account.BillingAddressPostalCode,
				"shippingAddressStreet":     account.ShippingAddressStreet,
				"shippingAddressCity":       account.ShippingAddressCity,
				"shippingAddressState":      account.ShippingAddressState,
				"shippingAddressCountry":    account.ShippingAddressCountry,
				"shippingAddressPostalCode": account.ShippingAddressPostalCode,
				"description":               account.Description,
				"stage":                     account.Stage,
				"assignedUserId":            account.AssignedUserID,
				"createdById":               account.CreatedByID,
				"modifiedById":              account.ModifiedByID,
				"createdAt":                 account.CreatedAt,
				"modifiedAt":                account.ModifiedAt,
				"deleted":                   account.Deleted,
				"customFields":              account.CustomFields,
			}
		}
		// Resolve user names
		h.resolveUserNames(c.Context(), records)
		return c.JSON(fiber.Map{
			"data":       records,
			"total":      result.Total,
			"page":       result.Page,
			"pageSize":   result.PageSize,
			"totalPages": result.TotalPages,
		})
	}

	if len(result.Data) == 0 {
		return c.JSON(result)
	}

	// Collect record IDs for batch execution
	recordIDs := make([]string, len(result.Data))
	for i, account := range result.Data {
		recordIDs[i] = account.ID
	}

	// Convert accounts to maps so we can add rollup values
	records := make([]map[string]interface{}, len(result.Data))
	for i, account := range result.Data {
		records[i] = map[string]interface{}{
			"id":                        account.ID,
			"orgId":                     account.OrgID,
			"name":                      account.Name,
			"website":                   account.Website,
			"emailAddress":              account.EmailAddress,
			"phoneNumber":               account.PhoneNumber,
			"type":                      account.Type,
			"industry":                  account.Industry,
			"sicCode":                   account.SicCode,
			"billingAddressStreet":      account.BillingAddressStreet,
			"billingAddressCity":        account.BillingAddressCity,
			"billingAddressState":       account.BillingAddressState,
			"billingAddressCountry":     account.BillingAddressCountry,
			"billingAddressPostalCode":  account.BillingAddressPostalCode,
			"shippingAddressStreet":     account.ShippingAddressStreet,
			"shippingAddressCity":       account.ShippingAddressCity,
			"shippingAddressState":      account.ShippingAddressState,
			"shippingAddressCountry":    account.ShippingAddressCountry,
			"shippingAddressPostalCode": account.ShippingAddressPostalCode,
			"description":               account.Description,
			"stage":                     account.Stage,
			"assignedUserId":            account.AssignedUserID,
			"createdById":               account.CreatedByID,
			"modifiedById":              account.ModifiedByID,
			"createdAt":                 account.CreatedAt,
			"modifiedAt":                account.ModifiedAt,
			"deleted":                   account.Deleted,
			"customFields":              account.CustomFields,
		}
	}

	// Build a map for quick lookup
	recordMap := make(map[string]map[string]interface{})
	for _, record := range records {
		recordMap[record["id"].(string)] = record
	}

	// Execute each rollup field as a batch query
	rollupSvc := service.NewRollupService(h.getRawDB(c))
	for _, fieldDef := range rollupFields {
		if fieldDef.RollupQuery == nil || fieldDef.RollupResultType == nil {
			continue
		}

		results, err := rollupSvc.ExecuteRollupBatch(c.Context(), *fieldDef.RollupQuery, recordIDs, orgID, *fieldDef.RollupResultType)
		if err != nil {
			// On batch error, set error for all records
			for _, record := range records {
				record[fieldDef.Name] = nil
				record[fieldDef.Name+"Error"] = err.Error()
			}
			continue
		}

		// Map results back to records
		for recordID, value := range results {
			if record, exists := recordMap[recordID]; exists {
				record[fieldDef.Name] = value
			}
		}
	}

	// Resolve user names for created_by and modified_by
	h.resolveUserNames(c.Context(), records)

	return c.JSON(fiber.Map{
		"data":       records,
		"total":      result.Total,
		"page":       result.Page,
		"pageSize":   result.PageSize,
		"totalPages": result.TotalPages,
	})
}

// Get returns a single account by ID
func (h *AccountHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	account, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if account == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Account not found",
		})
	}

	// Check for rollup fields and execute them
	fields, _ := h.getMetadataCache(c).ListFields(c.Context(), orgID, "Account")
	var hasRollupFields bool
	for _, f := range fields {
		if f.Type == entity.FieldTypeRollup {
			hasRollupFields = true
			break
		}
	}

	if hasRollupFields {
		// Convert account to a map so we can add rollup field values
		result := fiber.Map{
			"id":                        account.ID,
			"orgId":                     account.OrgID,
			"name":                      account.Name,
			"website":                   account.Website,
			"emailAddress":              account.EmailAddress,
			"phoneNumber":               account.PhoneNumber,
			"type":                      account.Type,
			"industry":                  account.Industry,
			"sicCode":                   account.SicCode,
			"billingAddressStreet":      account.BillingAddressStreet,
			"billingAddressCity":        account.BillingAddressCity,
			"billingAddressState":       account.BillingAddressState,
			"billingAddressCountry":     account.BillingAddressCountry,
			"billingAddressPostalCode":  account.BillingAddressPostalCode,
			"shippingAddressStreet":     account.ShippingAddressStreet,
			"shippingAddressCity":       account.ShippingAddressCity,
			"shippingAddressState":      account.ShippingAddressState,
			"shippingAddressCountry":    account.ShippingAddressCountry,
			"shippingAddressPostalCode": account.ShippingAddressPostalCode,
			"description":               account.Description,
			"stage":                     account.Stage,
			"assignedUserId":            account.AssignedUserID,
			"createdById":               account.CreatedByID,
			"modifiedById":              account.ModifiedByID,
			"createdAt":                 account.CreatedAt,
			"modifiedAt":                account.ModifiedAt,
			"deleted":                   account.Deleted,
		}

		// Add custom fields
		if account.CustomFields != nil {
			result["customFields"] = account.CustomFields
		}

		// Execute rollup fields
		rollupSvc := service.NewRollupService(h.getRawDB(c))
		for _, fieldDef := range fields {
			if fieldDef.Type == entity.FieldTypeRollup && fieldDef.RollupQuery != nil && fieldDef.RollupResultType != nil {
				rollupResult, rollupErr := rollupSvc.ExecuteRollup(c.Context(), *fieldDef.RollupQuery, id, orgID, *fieldDef.RollupResultType)
				if rollupErr != nil {
					result[fieldDef.Name] = nil
					result[fieldDef.Name+"Error"] = rollupErr.Error()
				} else {
					result[fieldDef.Name] = rollupResult
				}
			}
		}

		// Resolve user names
		h.resolveUserNames(c.Context(), []map[string]interface{}{result})

		return c.JSON(result)
	}

	// No rollup fields - still need to resolve user names
	result := map[string]interface{}{
		"id":                        account.ID,
		"orgId":                     account.OrgID,
		"name":                      account.Name,
		"website":                   account.Website,
		"emailAddress":              account.EmailAddress,
		"phoneNumber":               account.PhoneNumber,
		"type":                      account.Type,
		"industry":                  account.Industry,
		"sicCode":                   account.SicCode,
		"billingAddressStreet":      account.BillingAddressStreet,
		"billingAddressCity":        account.BillingAddressCity,
		"billingAddressState":       account.BillingAddressState,
		"billingAddressCountry":     account.BillingAddressCountry,
		"billingAddressPostalCode":  account.BillingAddressPostalCode,
		"shippingAddressStreet":     account.ShippingAddressStreet,
		"shippingAddressCity":       account.ShippingAddressCity,
		"shippingAddressState":      account.ShippingAddressState,
		"shippingAddressCountry":    account.ShippingAddressCountry,
		"shippingAddressPostalCode": account.ShippingAddressPostalCode,
		"description":               account.Description,
		"stage":                     account.Stage,
		"assignedUserId":            account.AssignedUserID,
		"createdById":               account.CreatedByID,
		"modifiedById":              account.ModifiedByID,
		"createdAt":                 account.CreatedAt,
		"modifiedAt":                account.ModifiedAt,
		"deleted":                   account.Deleted,
	}
	if account.CustomFields != nil {
		result["customFields"] = account.CustomFields
	}

	// Resolve user names
	h.resolveUserNames(c.Context(), []map[string]interface{}{result})

	return c.JSON(result)
}

// Create creates a new account
func (h *AccountHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.AccountCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Capture any unknown top-level JSON keys as custom fields.
	// This allows API consumers to send custom field values as top-level keys
	// without wrapping them in a "customFields" object.
	input.CustomFields = mergeUnknownFieldsIntoCustomFields(c.Body(), accountKnownFields, input.CustomFields)

	// Default assignedUserId to creating user if not set
	if input.AssignedUserID == nil || *input.AssignedUserID == "" {
		input.AssignedUserID = &userID
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Account", "", "CREATE", nil, newRecord)
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

	account, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Fire tripwires for CREATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Account", account.ID, "CREATE", nil, StructToMap(account))
	}

	// Notify assigned user (don't notify yourself)
	if h.notificationService != nil && account.AssignedUserID != nil && *account.AssignedUserID != "" && *account.AssignedUserID != userID {
		go h.notificationService.CreateAssignmentNotification(context.Background(), h.getDBConn(c), orgID, *account.AssignedUserID, "Account", account.ID, account.Name)
	}

	return c.Status(fiber.StatusCreated).JSON(account)
}

// Update updates an existing account
func (h *AccountHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.AccountUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Capture any unknown top-level JSON keys as custom fields.
	// Keys already in the explicit customFields object take priority.
	input.CustomFields = mergeUnknownFieldsIntoCustomFields(c.Body(), accountKnownFields, input.CustomFields)

	// Fetch old record for tripwire, validation, and notification evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil || h.notificationService != nil {
		oldAccount, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldAccount)
	}

	// Validate before save
	if h.validationService != nil {
		newRecord := StructToMap(input)
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Account", id, "UPDATE", oldRecord, newRecord)
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

	account, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if account == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Account not found",
		})
	}

	// Fire tripwires for UPDATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Account", id, "UPDATE", oldRecord, StructToMap(account))
	}

	// Notify if assignment changed
	if h.notificationService != nil && account.AssignedUserID != nil && *account.AssignedUserID != "" {
		oldAssigned, _ := oldRecord["assignedUserId"].(string)
		if *account.AssignedUserID != oldAssigned {
			go h.notificationService.CreateAssignmentNotification(context.Background(), h.getDBConn(c), orgID, *account.AssignedUserID, "Account", id, account.Name)
		}
	}

	return c.JSON(account)
}

// Delete soft-deletes an account
func (h *AccountHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Fetch old record for tripwire and validation evaluation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldAccount, _ := h.getRepo(c).GetByID(c.Context(), orgID, id)
		oldRecord = StructToMap(oldAccount)
	}

	// Validate before delete
	if h.validationService != nil && oldRecord != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, "Account", id, "DELETE", oldRecord, nil)
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
			"error": "Account not found",
		})
	}

	// Fire tripwires for DELETE event
	if h.tripwireService != nil && oldRecord != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, "Account", id, "DELETE", oldRecord, nil)
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ListTasks returns tasks linked to a specific account
func (h *AccountHandler) ListTasks(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	accountID := c.Params("id")

	// First verify the account exists
	account, err := h.getRepo(c).GetByID(c.Context(), orgID, accountID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if account == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Account not found",
		})
	}

	// Fetch tasks using limit/offset pagination (for Load More)
	params := entity.TaskListParams{
		ParentType: "Account",
		ParentID:   accountID,
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

	result, err := h.getTaskRepo(c).ListByOrg(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// RegisterRoutes registers account routes on the Fiber app
func (h *AccountHandler) RegisterRoutes(app fiber.Router) {
	accounts := app.Group("/accounts")
	accounts.Get("/", h.List)
	accounts.Get("/:id", h.Get)
	accounts.Get("/:id/tasks", h.ListTasks)
	accounts.Post("/", h.Create)
	accounts.Put("/:id", h.Update)
	accounts.Patch("/:id", h.Update)
	accounts.Delete("/:id", h.Delete)
}
