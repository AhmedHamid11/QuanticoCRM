package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// BulkCreateRequest represents the request body for bulk create operations
type BulkCreateRequest struct {
	Records []map[string]interface{} `json:"records"`
	Options BulkOptions              `json:"options"`
}

// BulkUpdateRequest represents the request body for bulk update operations
type BulkUpdateRequest struct {
	Records []map[string]interface{} `json:"records"` // Each record must have "id" field
	Options BulkOptions              `json:"options"`
}

// BulkOptions configures bulk operation behavior
type BulkOptions struct {
	SkipErrors     bool `json:"skipErrors"`     // Continue on validation errors
	ReturnErrors   bool `json:"returnErrors"`   // Return detailed error info
	FireTripwires  bool `json:"fireTripwires"`  // Fire tripwire webhooks (default: true if omitted)
	ValidateOnly   bool `json:"validateOnly"`   // Only validate, don't insert
	BatchSize      int  `json:"batchSize"`      // Records per transaction (default: 1000)
}

// BulkResponse represents the response for bulk operations
type BulkResponse struct {
	Created    int              `json:"created,omitempty"`
	Updated    int              `json:"updated,omitempty"`
	Failed     int              `json:"failed"`
	Errors     []BulkError      `json:"errors,omitempty"`
	IDs        []string         `json:"ids,omitempty"`        // IDs of created/updated records
	Validated  int              `json:"validated,omitempty"`  // For validateOnly mode
}

// BulkError represents an error for a specific record in a bulk operation
type BulkError struct {
	Index       int                            `json:"index"`
	Error       string                         `json:"error"`
	FieldErrors []entity.FieldValidationError  `json:"fieldErrors,omitempty"`
}

// BulkHandler handles bulk create/update operations
type BulkHandler struct {
	db                *sql.DB
	metadataRepo      *repo.MetadataRepo
	tripwireService   TripwireServiceInterface
	validationService ValidationServiceInterface
}

// NewBulkHandler creates a new BulkHandler
func NewBulkHandler(
	db *sql.DB,
	metadataRepo *repo.MetadataRepo,
	tripwireService TripwireServiceInterface,
	validationService ValidationServiceInterface,
) *BulkHandler {
	return &BulkHandler{
		db:                db,
		metadataRepo:      metadataRepo,
		tripwireService:   tripwireService,
		validationService: validationService,
	}
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *BulkHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getDB returns the tenant database from context, falling back to default db
func (h *BulkHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.db
}

// Maximum records per bulk request
const MaxBulkRecords = 10000
const DefaultBatchSize = 1000

// getTableName converts entity name to table name (e.g., "Contact" -> "contacts")
func (h *BulkHandler) getTableName(entityName string) string {
	return util.GetTableName(entityName)
}

// BulkCreate handles POST /api/v1/entities/:entity/bulk
func (h *BulkHandler) BulkCreate(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityName := c.Params("entity")

	// Verify entity exists for this org
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Parse request body
	var req BulkCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body: " + err.Error()})
	}

	// Validate record count
	if len(req.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No records provided"})
	}
	if len(req.Records) > MaxBulkRecords {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Too many records. Maximum is %d per request", MaxBulkRecords),
		})
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Ensure table exists
	if err := h.ensureTableExists(c.Context(), orgID, entityName, fields); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tableName := h.getTableName(entityName)
	now := time.Now().UTC().Format(time.RFC3339)

	// Set default options
	batchSize := req.Options.BatchSize
	if batchSize <= 0 || batchSize > DefaultBatchSize {
		batchSize = DefaultBatchSize
	}

	// Default fireTripwires to true if not explicitly set to false
	fireTripwires := true
	if !req.Options.FireTripwires && req.Options.ValidateOnly {
		fireTripwires = false
	}

	// Process records
	var response BulkResponse
	var errors []BulkError
	var createdIDs []string

	// First pass: validate all records if validation is enabled
	if h.validationService != nil {
		for i, record := range req.Records {
			result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, "", "CREATE", nil, record)
			if err != nil {
				errors = append(errors, BulkError{
					Index: i,
					Error: err.Error(),
				})
				continue
			}
			if !result.Valid {
				errors = append(errors, BulkError{
					Index:       i,
					Error:       result.Message,
					FieldErrors: result.FieldErrors,
				})
			}
		}

		// If not skipping errors and we have validation errors, return early
		if len(errors) > 0 && !req.Options.SkipErrors {
			response.Failed = len(errors)
			if req.Options.ReturnErrors {
				response.Errors = errors
			}
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}
	}

	// If validateOnly, return validation results
	if req.Options.ValidateOnly {
		response.Validated = len(req.Records) - len(errors)
		response.Failed = len(errors)
		if req.Options.ReturnErrors {
			response.Errors = errors
		}
		return c.JSON(response)
	}

	// Build a set of failed indices for quick lookup
	failedIndices := make(map[int]bool)
	for _, e := range errors {
		failedIndices[e.Index] = true
	}

	// Process in batches with transactions
	for batchStart := 0; batchStart < len(req.Records); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(req.Records) {
			batchEnd = len(req.Records)
		}

		batch := req.Records[batchStart:batchEnd]
		batchIDs, batchErrors := h.processBatch(c.Context(), orgID, userID, entityName, tableName, fields, batch, batchStart, failedIndices, now, req.Options.SkipErrors)

		createdIDs = append(createdIDs, batchIDs...)
		errors = append(errors, batchErrors...)

		// Fire tripwires for created records
		if fireTripwires && h.tripwireService != nil && len(batchIDs) > 0 {
			for i, id := range batchIDs {
				recordIdx := batchStart + i
				// Skip records that were skipped due to validation errors
				if failedIndices[recordIdx] {
					continue
				}
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, req.Records[recordIdx])
			}
		}
	}

	response.Created = len(createdIDs)
	response.Failed = len(errors)
	response.IDs = createdIDs
	if req.Options.ReturnErrors && len(errors) > 0 {
		response.Errors = errors
	}

	if response.Failed > 0 && !req.Options.SkipErrors {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// processBatch processes a batch of records in a single transaction
func (h *BulkHandler) processBatch(
	ctx context.Context,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	batchOffset int,
	failedIndices map[int]bool,
	now string,
	skipErrors bool,
) ([]string, []BulkError) {
	var createdIDs []string
	var errors []BulkError

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		for i := range records {
			errors = append(errors, BulkError{
				Index: batchOffset + i,
				Error: "Failed to start transaction: " + err.Error(),
			})
		}
		return nil, errors
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i, record := range records {
		globalIndex := batchOffset + i

		// Skip already-failed records
		if failedIndices[globalIndex] {
			continue
		}

		id, err := h.insertRecord(ctx, tx, orgID, userID, tableName, fields, record, now)
		if err != nil {
			if skipErrors {
				errors = append(errors, BulkError{
					Index: globalIndex,
					Error: err.Error(),
				})
				continue
			}
			// Rollback and return error
			tx.Rollback()
			return nil, []BulkError{{
				Index: globalIndex,
				Error: err.Error(),
			}}
		}
		createdIDs = append(createdIDs, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, []BulkError{{
			Index: batchOffset,
			Error: "Failed to commit transaction: " + err.Error(),
		}}
	}

	return createdIDs, errors
}

// insertRecord inserts a single record within a transaction
func (h *BulkHandler) insertRecord(
	ctx context.Context,
	tx *sql.Tx,
	orgID, userID, tableName string,
	fields []entity.FieldDef,
	record map[string]interface{},
	now string,
) (string, error) {
	var columns []string
	var placeholders []string
	var values []interface{}

	// Generate ID
	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	// Add org_id
	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	// Add fields from record
	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := record[idKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_id"))
				placeholders = append(placeholders, "?")
				values = append(values, idVal)
			}
			if nameVal, ok := record[nameKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_name"))
				placeholders = append(placeholders, "?")
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := record[idsKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_ids"))
				placeholders = append(placeholders, "?")
				values = append(values, idsVal)
			}
			if namesVal, ok := record[namesKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_names"))
				placeholders = append(placeholders, "?")
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := record[field.Name]; ok {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, val)
		} else if field.DefaultValue != nil && *field.DefaultValue != "" {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, *field.DefaultValue)
		}
	}

	// Add audit fields
	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, now, now, userID, userID)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := tx.ExecContext(ctx, query, values...)
	if err != nil {
		return "", err
	}

	return id, nil
}

// BulkUpdate handles PATCH /api/v1/entities/:entity/bulk
func (h *BulkHandler) BulkUpdate(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Parse request body
	var req BulkUpdateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body: " + err.Error()})
	}

	// Validate record count
	if len(req.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No records provided"})
	}
	if len(req.Records) > MaxBulkRecords {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Too many records. Maximum is %d per request", MaxBulkRecords),
		})
	}

	// Validate all records have IDs
	for i, record := range req.Records {
		if _, ok := record["id"]; !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Record at index %d is missing required 'id' field", i),
			})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tableName := h.getTableName(entityName)
	now := time.Now().UTC().Format(time.RFC3339)

	// Set default options
	batchSize := req.Options.BatchSize
	if batchSize <= 0 || batchSize > DefaultBatchSize {
		batchSize = DefaultBatchSize
	}

	fireTripwires := !req.Options.ValidateOnly

	// Process records
	var response BulkResponse
	var errors []BulkError
	var updatedIDs []string

	// Validation pass
	failedIndices := make(map[int]bool)
	if h.validationService != nil {
		for i, record := range req.Records {
			recordID := fmt.Sprintf("%v", record["id"])

			// Fetch old record for validation
			oldRecord, _ := h.fetchRecordAsMap(c.Context(), tableName, recordID, orgID)
			if oldRecord == nil {
				errors = append(errors, BulkError{
					Index: i,
					Error: "Record not found",
				})
				failedIndices[i] = true
				continue
			}

			result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, recordID, "UPDATE", oldRecord, record)
			if err != nil {
				errors = append(errors, BulkError{
					Index: i,
					Error: err.Error(),
				})
				failedIndices[i] = true
				continue
			}
			if !result.Valid {
				errors = append(errors, BulkError{
					Index:       i,
					Error:       result.Message,
					FieldErrors: result.FieldErrors,
				})
				failedIndices[i] = true
			}
		}

		if len(errors) > 0 && !req.Options.SkipErrors {
			response.Failed = len(errors)
			if req.Options.ReturnErrors {
				response.Errors = errors
			}
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}
	}

	if req.Options.ValidateOnly {
		response.Validated = len(req.Records) - len(errors)
		response.Failed = len(errors)
		if req.Options.ReturnErrors {
			response.Errors = errors
		}
		return c.JSON(response)
	}

	// Process updates in batches
	for batchStart := 0; batchStart < len(req.Records); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(req.Records) {
			batchEnd = len(req.Records)
		}

		batch := req.Records[batchStart:batchEnd]
		batchIDs, batchErrors := h.processUpdateBatch(c.Context(), orgID, userID, entityName, tableName, fields, batch, batchStart, failedIndices, now, req.Options.SkipErrors)

		updatedIDs = append(updatedIDs, batchIDs...)
		errors = append(errors, batchErrors...)

		// Fire tripwires
		if fireTripwires && h.tripwireService != nil {
			for i, id := range batchIDs {
				recordIdx := batchStart + i
				if failedIndices[recordIdx] {
					continue
				}
				oldRecord, _ := h.fetchRecordAsMap(c.Context(), tableName, id, orgID)
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "UPDATE", oldRecord, req.Records[recordIdx])
			}
		}
	}

	response.Updated = len(updatedIDs)
	response.Failed = len(errors)
	response.IDs = updatedIDs
	if req.Options.ReturnErrors && len(errors) > 0 {
		response.Errors = errors
	}

	if response.Failed > 0 && !req.Options.SkipErrors {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
	}

	return c.JSON(response)
}

// processUpdateBatch processes a batch of update records
func (h *BulkHandler) processUpdateBatch(
	ctx context.Context,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	batchOffset int,
	failedIndices map[int]bool,
	now string,
	skipErrors bool,
) ([]string, []BulkError) {
	var updatedIDs []string
	var errors []BulkError

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		for i := range records {
			errors = append(errors, BulkError{
				Index: batchOffset + i,
				Error: "Failed to start transaction: " + err.Error(),
			})
		}
		return nil, errors
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i, record := range records {
		globalIndex := batchOffset + i

		if failedIndices[globalIndex] {
			continue
		}

		recordID := fmt.Sprintf("%v", record["id"])
		err := h.updateRecord(ctx, tx, orgID, userID, tableName, fields, recordID, record, now)
		if err != nil {
			if skipErrors {
				errors = append(errors, BulkError{
					Index: globalIndex,
					Error: err.Error(),
				})
				continue
			}
			tx.Rollback()
			return nil, []BulkError{{
				Index: globalIndex,
				Error: err.Error(),
			}}
		}
		updatedIDs = append(updatedIDs, recordID)
	}

	if err := tx.Commit(); err != nil {
		return nil, []BulkError{{
			Index: batchOffset,
			Error: "Failed to commit transaction: " + err.Error(),
		}}
	}

	return updatedIDs, errors
}

// updateRecord updates a single record within a transaction
func (h *BulkHandler) updateRecord(
	ctx context.Context,
	tx *sql.Tx,
	orgID, userID, tableName string,
	fields []entity.FieldDef,
	recordID string,
	record map[string]interface{},
	now string,
) error {
	var setClauses []string
	var values []interface{}

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := record[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_id")))
				values = append(values, idVal)
			}
			if nameVal, ok := record[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_name")))
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := record[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_ids")))
				values = append(values, idsVal)
			}
			if namesVal, ok := record[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_names")))
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := record[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(util.CamelToSnake(field.Name))))
			values = append(values, val)
		}
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Add audit fields
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	values = append(values, now, userID)

	// Add WHERE clause values
	values = append(values, recordID, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName, strings.Join(setClauses, ", "))

	result, err := tx.ExecContext(ctx, query, values...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("record not found or not accessible")
	}

	return nil
}

// fetchRecordAsMap fetches a record by ID and returns it as a map
func (h *BulkHandler) fetchRecordAsMap(ctx context.Context, tableName, id, orgID string) (map[string]interface{}, error) {
	return util.FetchRecordAsMap(ctx, h.db, tableName, id, orgID)
}

// ensureTableExists creates the table if it doesn't exist
func (h *BulkHandler) ensureTableExists(ctx context.Context, orgID, entityName string, fields []entity.FieldDef) error {
	return util.EnsureTableExists(ctx, h.db, entityName, fields)
}

// RegisterRoutes registers bulk operation routes
func (h *BulkHandler) RegisterRoutes(app fiber.Router) {
	app.Post("/entities/:entity/bulk", h.BulkCreate)
	app.Patch("/entities/:entity/bulk", h.BulkUpdate)
}
