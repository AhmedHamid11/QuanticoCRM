package handler

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// ImportProgressEvent represents a streaming progress event during import
type ImportProgressEvent struct {
	Type      string             `json:"type"`                // "progress" or "complete"
	Processed int                `json:"processed"`
	Total     int                `json:"total"`
	Created   int                `json:"created"`
	Updated   int                `json:"updated"`
	Deleted   int                `json:"deleted,omitempty"`
	Failed    int                `json:"failed"`
	Skipped   int                `json:"skipped"`
	Merged    int                `json:"merged,omitempty"`
	Phase     string             `json:"phase,omitempty"`     // "validating", "importing", "deleting"
	Response  *ImportCSVResponse `json:"response,omitempty"`  // Only for "complete" type
}

// ImportHandler handles CSV import operations
type ImportHandler struct {
	db                  *sql.DB
	metadataRepo        *repo.MetadataRepo
	csvParser           *service.CSVParser
	csvValidator        *service.CSVValidatorService
	tripwireService     TripwireServiceInterface
	validationService   ValidationServiceInterface
	duplicateService    *service.ImportDuplicateService
	importJobRepo       *repo.ImportJobRepo
	dbManager           DBManagerInterface
	importQuotaService  *service.ImportQuotaService
}

// DBManagerInterface is the subset of db.Manager methods needed by import handler
type DBManagerInterface interface {
	TouchConnection(orgID string)
}

// NewImportHandler creates a new ImportHandler
func NewImportHandler(
	db *sql.DB,
	metadataRepo *repo.MetadataRepo,
	tripwireService TripwireServiceInterface,
	validationService ValidationServiceInterface,
	duplicateService *service.ImportDuplicateService,
	importJobRepo *repo.ImportJobRepo,
	dbManager ...DBManagerInterface,
) *ImportHandler {
	h := &ImportHandler{
		db:                db,
		metadataRepo:      metadataRepo,
		csvParser:         service.NewCSVParser(),
		csvValidator:      service.NewCSVValidatorService(),
		tripwireService:   tripwireService,
		validationService: validationService,
		duplicateService:  duplicateService,
		importJobRepo:     importJobRepo,
	}
	if len(dbManager) > 0 {
		h.dbManager = dbManager[0]
	}
	return h
}

// SetImportQuotaService sets the Turso quota service for preflight checks
func (h *ImportHandler) SetImportQuotaService(svc *service.ImportQuotaService) {
	h.importQuotaService = svc
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *ImportHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getDB returns the tenant database from context, falling back to default db
func (h *ImportHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.db
}

// ImportMode defines the operation mode for CSV import
type ImportMode string

const (
	ImportModeCreate ImportMode = "create" // Insert new records (default)
	ImportModeUpdate ImportMode = "update" // Update existing records by ID
	ImportModeUpsert ImportMode = "upsert" // Create or update based on match field
	ImportModeDelete ImportMode = "delete" // Delete records by ID or match field
)

// importReadBudget tracks estimated Turso row reads during an import and aborts if exceeded.
// This is a defense-in-depth measure — even with batching + indexes, unexpected conditions
// (missing indexes, huge tables) could cause runaway reads.
type importReadBudget struct {
	max  int64 // maximum allowed reads
	used int64 // reads consumed so far
}

// newImportReadBudget creates a budget based on record count.
// Default: recordCount * 100 reads, minimum 10,000.
func newImportReadBudget(recordCount int) *importReadBudget {
	max := int64(recordCount) * 100
	if max < 10000 {
		max = 10000
	}
	return &importReadBudget{max: max}
}

// charge adds reads to the budget. Returns an error if budget is exceeded.
func (b *importReadBudget) charge(reads int64) error {
	if b == nil {
		return nil
	}
	b.used += reads
	if b.used > b.max {
		return fmt.Errorf("import read budget exceeded: used %d of %d allowed row reads — aborting to protect Turso quota", b.used, b.max)
	}
	return nil
}

// summary returns a string summarizing budget usage (for logging).
func (b *importReadBudget) summary() string {
	if b == nil {
		return "no budget"
	}
	pct := float64(b.used) / float64(b.max) * 100
	return fmt.Sprintf("%d/%d reads (%.0f%%)", b.used, b.max, pct)
}

// LookupResolution specifies how to resolve a lookup field value to an ID
type LookupResolution struct {
	MatchField       string                            `json:"matchField"`       // Field to match on in related entity (e.g., "name")
	CreateIfNotFound bool                              `json:"createIfNotFound"` // Create the related record if not found
	NewRecordData    map[string]map[string]interface{} `json:"newRecordData"`    // Data for new records: matchValue -> field values
}

// DedupDecisionInput represents a dedup decision from the frontend
type DedupDecisionInput struct {
	KeptExternalID      string `json:"keptExternalId"`
	DiscardedExternalID string `json:"discardedExternalId"`
	MatchField          string `json:"matchField"`
	MatchValue          string `json:"matchValue"`
	DecisionType        string `json:"decisionType"`        // "within_file" or "db_match"
	Action              string `json:"action"`               // "skip", "update", "import", "merge"
	MatchedRecordID     string `json:"matchedRecordId,omitempty"` // Quantico record ID that was matched
}

// ImportCSVRequest represents optional parameters for CSV import
type ImportCSVRequest struct {
	Mode                   ImportMode                         `json:"mode"`                   // create, update, upsert, delete (default: create)
	MatchField             string                             `json:"matchField"`             // Field to match on for upsert/update/delete (e.g., "email")
	ColumnMapping          map[string]string                  `json:"columnMapping"`          // CSV header -> field name
	LookupResolution       map[string]LookupResolution        `json:"lookupResolution"`       // fieldName -> resolution config
	DuplicateResolutions   map[int]entity.ImportResolution    `json:"duplicateResolutions"`   // rowIndex -> resolution decision
	WithinFileSkipIndices  []int                              `json:"withinFileSkipIndices"`  // Row indices to skip from within-file duplicates
	ExternalIdField        string                             `json:"externalIdField"`        // Which column has the SF external ID
	DedupDecisions         []DedupDecisionInput               `json:"dedupDecisions"`         // Pre-computed dedup decisions from frontend
	SkipErrors             bool                               `json:"skipErrors"`
	FireTripwires          bool                               `json:"fireTripwires"`
	ValidateOnly           bool                               `json:"validateOnly"`
}

// ImportCSVResponse represents the response from a CSV import
type ImportCSVResponse struct {
	Mode          ImportMode               `json:"mode,omitempty"`
	Created       int                      `json:"created,omitempty"`
	Updated       int                      `json:"updated,omitempty"`
	Deleted       int                      `json:"deleted,omitempty"`
	Skipped       int                      `json:"skipped,omitempty"` // Records skipped (not found for update/delete or by duplicate resolution)
	Merged        int                      `json:"merged,omitempty"`  // Records sent to merge (resolution action = merge)
	Failed        int                      `json:"failed"`
	Validated     int                      `json:"validated,omitempty"`
	TotalRows     int                      `json:"totalRows"`
	Headers       []string                 `json:"headers,omitempty"`
	MappedHeaders []string                 `json:"mappedHeaders,omitempty"`
	Errors        []BulkError              `json:"errors,omitempty"`
	IDs           []string                 `json:"ids,omitempty"`
	AuditReport   string                   `json:"auditReport,omitempty"` // Base64-encoded CSV audit report
	ImportID      string                   `json:"importId,omitempty"`    // ID of the persisted import job
	Warnings      []string                 `json:"warnings,omitempty"`    // Non-fatal warnings (e.g. failed to persist dedup decisions)
}

// PreviewCSVResponse represents a preview of CSV data before import
type PreviewCSVResponse struct {
	Headers         []string                   `json:"headers"`
	MappedHeaders   []string                   `json:"mappedHeaders"`
	SampleRows      []map[string]interface{}   `json:"sampleRows"`
	TotalRows       int                        `json:"totalRows"`
	UnmappedCols    []string                   `json:"unmappedColumns"`
	Fields          []FieldMapping             `json:"fields"`
	AvailableFields []AvailableField           `json:"availableFields"` // All entity fields for dropdown
}

// AvailableField represents an entity field available for mapping
type AvailableField struct {
	Name                string           `json:"name"`
	Label               string           `json:"label"`
	Type                string           `json:"type"`
	RelatedEntity       string           `json:"relatedEntity,omitempty"`       // For link fields, the related entity name
	RelatedEntityFields []AvailableField `json:"relatedEntityFields,omitempty"` // Fields of the related entity (for lookup matching)
}

// FieldMapping shows how a CSV column maps to an entity field
type FieldMapping struct {
	CSVHeader  string `json:"csvHeader"`
	FieldName  string `json:"fieldName"`
	FieldLabel string `json:"fieldLabel"`
	FieldType  string `json:"fieldType"`
	Mapped     bool   `json:"mapped"`
}

// MissingLookup represents a lookup value that doesn't exist and needs to be created
type MissingLookup struct {
	FieldName      string           `json:"fieldName"`      // The link field name (e.g., "account")
	RelatedEntity  string           `json:"relatedEntity"`  // The entity to create (e.g., "Account")
	MatchValue     string           `json:"matchValue"`     // The value that wasn't found (e.g., "Acme Corp")
	MatchField     string           `json:"matchField"`     // Field used for matching (e.g., "name")
	RequiredFields []AvailableField `json:"requiredFields"` // Required fields that need values
	RowIndices     []int            `json:"rowIndices"`     // CSV rows that reference this value
}

// AnalyzeLookupResponse contains info about missing lookups that need to be created
type AnalyzeLookupResponse struct {
	MissingLookups []MissingLookup `json:"missingLookups"` // Lookups that don't exist
}

// getTableName converts entity name to table name
func (h *ImportHandler) getTableName(entityName string) string {
	return util.GetTableName(entityName)
}

// ImportCSV handles POST /api/v1/entities/:entity/import/csv
func (h *ImportHandler) ImportCSV(c *fiber.Ctx) error {
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

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use 'file' field in multipart form."})
	}

	// Validate file size (max 50MB)
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File too large. Maximum size is 50MB."})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid file type. Only CSV files are accepted."})
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content into buffer (for potential re-reading)
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Parse options from form data
	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse CSV
	var parseResult *service.CSVParseResult
	if len(options.ColumnMapping) > 0 {
		parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
	} else {
		parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check for parse errors
	if len(parseResult.Errors) > 0 && !options.SkipErrors {
		var bulkErrors []BulkError
		for _, e := range parseResult.Errors {
			bulkErrors = append(bulkErrors, BulkError{
				Index: e.Row,
				Error: e.Message,
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(ImportCSVResponse{
			Failed:    len(parseResult.Errors),
			TotalRows: parseResult.RowCount + len(parseResult.Errors),
			Errors:    bulkErrors,
		})
	}

	if len(parseResult.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid records found in CSV"})
	}

	// Default mode is create
	mode := options.Mode
	if mode == "" {
		mode = ImportModeCreate
	}

	// Validate mode-specific requirements
	if (mode == ImportModeUpsert || mode == ImportModeUpdate || mode == ImportModeDelete) && options.MatchField == "" {
		// Check if 'id' column is present
		hasID := false
		for _, h := range parseResult.MappedHeaders {
			if h == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Mode '%s' requires either 'matchField' option or 'id' column in CSV", mode),
			})
		}
		options.MatchField = "id"
	}

	// Validate matchField exists
	if options.MatchField != "" && options.MatchField != "id" {
		found := false
		for _, f := range fields {
			if f.Name == options.MatchField {
				found = true
				break
			}
		}
		if !found {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Match field '%s' does not exist on entity", options.MatchField),
			})
		}
	}

	// Ensure table exists
	if err := h.ensureTableExists(c.Context(), h.getDB(c), orgID, entityName, fields); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tableName := h.getTableName(entityName)
	now := time.Now().UTC().Format(time.RFC3339)

	var response ImportCSVResponse
	response.Mode = mode
	response.Headers = parseResult.Headers
	response.MappedHeaders = parseResult.MappedHeaders
	response.TotalRows = parseResult.RowCount

	// Check for async progress support BEFORE lookups and validation.
	// For create mode: lookups and validation already ran during the analyze step.
	// For delete mode: no lookups or validation needed, jump straight to async delete.
	// Returns 202 immediately so large imports don't block the HTTP request.
	asyncProgress := c.Get("X-Stream-Progress") == "true"
	if asyncProgress && mode == ImportModeCreate {
		failedIndices := make(map[int]bool)
		return h.handleAsyncImport(c, orgID, userID, entityName, tableName, fields, parseResult, failedIndices, now, options, nil)
	}
	if asyncProgress && mode == ImportModeDelete {
		db := h.getDB(c)
		return h.handleAsyncDelete(c, orgID, entityName, tableName, fields, parseResult.Records, options, db)
	}

	// Create read budget to protect Turso quota (defense in depth)
	budget := newImportReadBudget(len(parseResult.Records))

	// Resolve lookup field values to IDs (e.g., company name -> account ID)
	if len(options.LookupResolution) > 0 {
		lookupErrors, err := h.resolveLookups(c.Context(), h.getDB(c), orgID, userID, parseResult.Records, fields, options.LookupResolution, budget)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if len(lookupErrors) > 0 && !options.SkipErrors {
			response.Failed = len(lookupErrors)
			response.Errors = lookupErrors
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}
	}

	// Determine validation operation based on mode
	validationOp := "CREATE"
	if mode == ImportModeUpdate || mode == ImportModeUpsert {
		validationOp = "UPDATE"
	}

	var errors []BulkError
	failedIndices := make(map[int]bool)

	// Validation pass (skip for delete mode)
	if h.validationService != nil && mode != ImportModeDelete {
		for i, record := range parseResult.Records {
			result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, "", validationOp, nil, record)
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

		if len(errors) > 0 && !options.SkipErrors {
			response.Failed = len(errors)
			response.Errors = errors
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}
	}

	// Validate only mode
	if options.ValidateOnly {
		response.Validated = len(parseResult.Records) - len(errors)
		response.Failed = len(errors)
		response.Errors = errors
		return c.JSON(response)
	}

	// Process based on mode
	switch mode {
	case ImportModeCreate:
		return h.processCreateMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors, nil)
	case ImportModeUpdate:
		return h.processUpdateMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors)
	case ImportModeUpsert:
		return h.processUpsertMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors)
	case ImportModeDelete:
		return h.processDeleteMode(c, orgID, entityName, tableName, fields, parseResult.Records, options, &response)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid mode: %s", mode)})
	}
}

// handleAsyncImport starts the import in a background goroutine and returns a job ID.
// The frontend polls GetImportProgress to track progress.
func (h *ImportHandler) handleAsyncImport(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	parseResult *service.CSVParseResult,
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	errors []BulkError,
) error {
	db := h.getDB(c)
	jobID := sfid.New("Imp")

	// Create progress tracking job (scoped to orgID for tenant isolation)
	importProgressStore.Create(jobID, orgID, len(parseResult.Records))

	// Start processing in background goroutine
	go func() {
		// Recover from panics so progress doesn't stay stuck at 0% forever
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Import crashed: %v", r)
				log.Printf("[IMPORT] PANIC in job %s: %v", jobID, r)
				importProgressStore.SetError(jobID, errMsg)
			}
		}()

		var response ImportCSVResponse
		response.Mode = options.Mode
		response.Headers = parseResult.Headers
		response.MappedHeaders = parseResult.MappedHeaders
		response.TotalRows = parseResult.RowCount

		// Create read budget for async import
		asyncBudget := newImportReadBudget(len(parseResult.Records))

		// Resolve lookup fields in background (e.g., company name -> account ID)
		if len(options.LookupResolution) > 0 {
			importProgressStore.Update(jobID, ImportProgressEvent{
				Type:  "progress",
				Phase: "resolving_lookups",
				Total: len(parseResult.Records),
			})
			lookupErrors, err := h.resolveLookups(context.Background(), db, orgID, userID, parseResult.Records, fields, options.LookupResolution, asyncBudget)
			if err != nil {
				importProgressStore.SetError(jobID, fmt.Sprintf("Lookup resolution failed: %v", err))
				return
			}
			if len(lookupErrors) > 0 {
				errors = append(errors, lookupErrors...)
			}
		}

		progressFn := func(event ImportProgressEvent) {
			importProgressStore.Update(jobID, event)
			if h.dbManager != nil {
				h.dbManager.TouchConnection(orgID)
			}
		}

		// Emit initial progress so frontend knows processing has started
		progressFn(ImportProgressEvent{
			Type:  "progress",
			Phase: "importing",
			Total: len(parseResult.Records),
		})

		h.processCreateModeInternal(
			context.Background(), db, orgID, userID, entityName, tableName,
			fields, parseResult.Records, failedIndices, now, options, &response, errors, progressFn,
		)

		importProgressStore.Complete(jobID, &response)
		log.Printf("[IMPORT] Job %s completed: %d created, %d failed, read budget: %s", jobID, response.Created, response.Failed, asyncBudget.summary())
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobID": jobID,
		"total": len(parseResult.Records),
	})
}

// handleAsyncDelete starts a delete operation in a background goroutine and returns a job ID.
// The frontend polls GetImportProgress to track progress. This fixes the timeout issue
// for large deletes (50k+ records) where c.Context() would be cancelled after the response.
func (h *ImportHandler) handleAsyncDelete(
	c *fiber.Ctx,
	orgID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	options ImportCSVRequest,
	db *sql.DB,
) error {
	jobID := sfid.New("Imp")

	// Create progress tracking job (scoped to orgID for tenant isolation)
	importProgressStore.Create(jobID, orgID, len(records))

	// Start processing in background goroutine
	go func() {
		// Recover from panics so progress doesn't stay stuck at 0% forever
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Delete crashed: %v", r)
				log.Printf("[IMPORT-DELETE] PANIC in job %s: %v", jobID, r)
				importProgressStore.SetError(jobID, errMsg)
			}
		}()

		var response ImportCSVResponse
		response.Mode = options.Mode

		progressFn := func(event ImportProgressEvent) {
			importProgressStore.Update(jobID, event)
			if h.dbManager != nil {
				h.dbManager.TouchConnection(orgID)
			}
		}

		// Emit initial progress so frontend knows processing has started
		progressFn(ImportProgressEvent{
			Type:  "progress",
			Phase: "deleting",
			Total: len(records),
		})

		// Use context.Background() — NOT c.Context() which gets cancelled after 202 response
		if options.MatchField == "id" {
			h.processDeleteByIDInternal(context.Background(), db, orgID, entityName, tableName, records, options, &response, progressFn)
		} else {
			h.processDeleteModeInternal(context.Background(), db, orgID, entityName, tableName, fields, records, options, &response, progressFn)
		}

		importProgressStore.Complete(jobID, &response)
		log.Printf("[IMPORT-DELETE] Job %s completed: %d deleted, %d failed", jobID, response.Deleted, response.Failed)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"jobID": jobID,
		"total": len(records),
	})
}

// processDeleteByIDInternal is the context-aware version of processDeleteByID.
// It accepts context.Context and *sql.DB directly (instead of *fiber.Ctx) so it
// works correctly from a background goroutine after the HTTP response has been sent.
func (h *ImportHandler) processDeleteByIDInternal(
	ctx context.Context,
	db *sql.DB,
	orgID, entityName, tableName string,
	records []map[string]interface{},
	options ImportCSVRequest,
	response *ImportCSVResponse,
	progressFn func(ImportProgressEvent),
) {
	const batchSize = 500

	// Collect all IDs from the CSV
	var allIDs []string
	for _, record := range records {
		id, ok := record["id"]
		if !ok || id == nil {
			continue
		}
		idStr := fmt.Sprintf("%v", id)
		if idStr != "" {
			allIDs = append(allIDs, idStr)
		}
	}

	log.Printf("[IMPORT-DELETE] Starting async batch delete: %d IDs from %d records, table=%s, org=%s",
		len(allIDs), len(records), tableName, orgID)

	if len(allIDs) > 0 {
		log.Printf("[IMPORT-DELETE] Sample IDs: [%s, %s, ...]", allIDs[0], allIDs[min(1, len(allIDs)-1)])
	}

	// Count records before delete for verification
	var countBefore int64
	countRow := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName), orgID)
	if err := countRow.Scan(&countBefore); err != nil {
		log.Printf("[IMPORT-DELETE] Pre-delete count failed: %v", err)
	} else {
		log.Printf("[IMPORT-DELETE] Records before delete: %d", countBefore)
	}

	var totalRowsAffected int64

	// Delete in batches
	for i := 0; i < len(allIDs); i += batchSize {
		end := i + batchSize
		if end > len(allIDs) {
			end = len(allIDs)
		}
		batch := allIDs[i:end]
		batchNum := i/batchSize + 1

		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)+1)
		args = append(args, orgID)
		for j, id := range batch {
			placeholders[j] = "?"
			args = append(args, id)
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE org_id = ? AND id IN (%s)",
			tableName, strings.Join(placeholders, ","))

		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			log.Printf("[IMPORT-DELETE] Batch %d failed at offset %d: %v", batchNum, i, err)
			response.Failed = len(allIDs) - int(totalRowsAffected)
			response.Errors = append(response.Errors, BulkError{Index: i, Error: err.Error()})
			break
		}

		rowsAffected, _ := result.RowsAffected()
		totalRowsAffected += rowsAffected
		log.Printf("[IMPORT-DELETE] Batch %d: %d IDs submitted, %d rows affected", batchNum, len(batch), rowsAffected)

		// Emit progress after each batch
		if progressFn != nil {
			progressFn(ImportProgressEvent{
				Type:      "progress",
				Phase:     "deleting",
				Processed: end,
				Total:     len(allIDs),
				Deleted:   int(totalRowsAffected),
				Skipped:   end - int(totalRowsAffected),
				Failed:    len(response.Errors),
			})
		}
	}

	// Count records after delete for verified results
	var countAfter int64
	countRow = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName), orgID)
	if err := countRow.Scan(&countAfter); err != nil {
		log.Printf("[IMPORT-DELETE] Post-delete count failed: %v", err)
		// Fall back to RowsAffected
		response.Deleted = int(totalRowsAffected)
		response.Skipped = len(allIDs) - int(totalRowsAffected)
	} else {
		verifiedDeleted := int(countBefore - countAfter)
		log.Printf("[IMPORT-DELETE] Records after delete: %d (verified deleted: %d, RowsAffected total: %d)",
			countAfter, verifiedDeleted, totalRowsAffected)
		// Use verified count - more reliable than RowsAffected with Turso
		response.Deleted = verifiedDeleted
		response.Skipped = len(allIDs) - verifiedDeleted
		if response.Skipped < 0 {
			response.Skipped = 0
		}
	}

	response.IDs = allIDs
}

// processDeleteModeInternal is the context-aware version of processDeleteMode (non-ID match field path).
// It accepts context.Context and *sql.DB directly so it works from a background goroutine.
func (h *ImportHandler) processDeleteModeInternal(
	ctx context.Context,
	db *sql.DB,
	orgID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	options ImportCSVRequest,
	response *ImportCSVResponse,
	progressFn func(ImportProgressEvent),
) {
	var deletedIDs []string
	var errors []BulkError
	var skipped int

	for i, record := range records {
		existingID, _, err := h.findExistingRecord(ctx, db, orgID, tableName, options.MatchField, record, fields)
		if err != nil {
			errors = append(errors, BulkError{Index: i, Error: err.Error()})
			if !options.SkipErrors {
				response.Failed = len(errors)
				response.Errors = errors
				return
			}
			continue
		}

		if existingID == "" {
			skipped++
		} else {
			query := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND org_id = ?", tableName)
			_, err = db.ExecContext(ctx, query, existingID, orgID)
			if err != nil {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				if !options.SkipErrors {
					response.Failed = len(errors)
					response.Errors = errors
					return
				}
				continue
			}

			deletedIDs = append(deletedIDs, existingID)

			if options.FireTripwires && h.tripwireService != nil {
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, existingID, "DELETE", record, nil)
			}
		}

		// Emit progress every 100 records or at the last record
		if progressFn != nil && ((i+1)%100 == 0 || i == len(records)-1) {
			progressFn(ImportProgressEvent{
				Type:      "progress",
				Phase:     "deleting",
				Processed: i + 1,
				Total:     len(records),
				Deleted:   len(deletedIDs),
				Skipped:   skipped,
				Failed:    len(errors),
			})
		}
	}

	response.Deleted = len(deletedIDs)
	response.Skipped = skipped
	response.Failed = len(errors)
	response.IDs = deletedIDs
	if len(errors) > 0 {
		response.Errors = errors
	}
}

// GetImportProgress returns the current progress of an async import job.
func (h *ImportHandler) GetImportProgress(c *fiber.Ctx) error {
	jobID := c.Params("jobID")
	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "jobID required"})
	}
	orgID := c.Locals("orgID").(string)
	job := importProgressStore.Get(jobID)
	if job == nil || job.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Job not found"})
	}
	return c.JSON(job)
}

// processCreateMode handles create (insert) operations (non-streaming path)
func (h *ImportHandler) processCreateMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
	progressFn func(ImportProgressEvent),
) error {
	ctx := c.Context()
	db := h.getDB(c)

	h.processCreateModeInternal(ctx, db, orgID, userID, entityName, tableName, fields, records, failedIndices, now, options, response, errors, progressFn)

	if progressFn != nil {
		return nil // Streaming handler sends the response
	}

	if response.Failed > 0 && !options.SkipErrors {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
	}
	return c.Status(fiber.StatusCreated).JSON(response)
}

// processCreateModeInternal is the core create logic shared by streaming and non-streaming paths.
// It populates the response pointer and optionally emits progress events via progressFn.
func (h *ImportHandler) processCreateModeInternal(
	ctx context.Context,
	db *sql.DB,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
	progressFn func(ImportProgressEvent),
) {
	var createdIDs []string
	var updatedIDs []string
	var auditEntries []service.AuditEntry

	totalRecords := len(records)

	// Build skip map from within-file indices
	withinFileSkipMap := make(map[int]bool)
	for _, idx := range options.WithinFileSkipIndices {
		withinFileSkipMap[idx] = true
	}

	// Categorize records based on resolutions
	recordsToCreate := make(map[int]map[string]interface{})
	recordsToUpdate := make(map[int]string) // rowIndex -> recordID

	for i := range records {
		if failedIndices[i] {
			continue
		}

		// Check within-file skip
		if withinFileSkipMap[i] {
			response.Skipped++
			auditEntries = append(auditEntries, service.AuditEntry{
				RowIndex:   i,
				Action:     "Skipped",
				MatchedID:  "",
				Reason:     "Within-file duplicate (not selected as keeper)",
				RecordData: records[i],
			})
			continue
		}

		// Check duplicate resolution
		if resolution, ok := options.DuplicateResolutions[i]; ok {
			switch resolution.Action {
			case "skip":
				response.Skipped++
				auditEntries = append(auditEntries, service.AuditEntry{
					RowIndex:   i,
					Action:     "Skipped",
					MatchedID:  resolution.SelectedMatchID,
					Reason:     "User chose to skip (duplicate of existing record)",
					RecordData: records[i],
				})
				continue
			case "update":
				recordsToUpdate[i] = resolution.SelectedMatchID
				auditEntries = append(auditEntries, service.AuditEntry{
					RowIndex:   i,
					Action:     "Updated",
					MatchedID:  resolution.SelectedMatchID,
					Reason:     "User chose to update existing record",
					RecordData: records[i],
				})
				continue
			case "merge":
				response.Merged++
				auditEntries = append(auditEntries, service.AuditEntry{
					RowIndex:   i,
					Action:     "Sent to Merge",
					MatchedID:  resolution.SelectedMatchID,
					Reason:     "User chose to merge via merge wizard",
					RecordData: records[i],
				})
				continue
			case "import":
				// Fall through to create normally
				auditEntries = append(auditEntries, service.AuditEntry{
					RowIndex:   i,
					Action:     "Imported",
					MatchedID:  "",
					Reason:     "User chose to import as new record",
					RecordData: records[i],
				})
			}
		}

		// Default: create normally
		recordsToCreate[i] = records[i]
	}

	// Track how many records were resolved before batch processing
	preProcessed := response.Skipped + response.Merged + len(recordsToUpdate) + len(failedIndices)

	// Process updates
	for rowIdx, recordID := range recordsToUpdate {
		err := h.updateExistingRecord(ctx, db, orgID, userID, entityName, recordID, records[rowIdx], now)
		if err != nil {
			errors = append(errors, BulkError{Index: rowIdx, Error: err.Error()})
			if !options.SkipErrors && progressFn == nil {
				// Non-streaming: set response and return early (caller sends HTTP error)
				response.Failed = len(errors)
				response.Errors = errors
				return
			}
			continue
		}
		updatedIDs = append(updatedIDs, recordID)

		// Fire tripwires for update
		if options.FireTripwires && h.tripwireService != nil {
			go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, recordID, "UPDATE", nil, records[rowIdx])
		}
	}

	// Process creates in batches
	if len(recordsToCreate) > 0 {
		// Build sequential list from map
		var createList []map[string]interface{}
		var createIndices []int
		for idx := 0; idx < len(records); idx++ {
			if rec, ok := recordsToCreate[idx]; ok {
				createList = append(createList, rec)
				createIndices = append(createIndices, idx)
			}
		}

		log.Printf("[IMPORT-CREATE] Starting bulk insert: %d records into %s (org=%s)", len(createList), tableName, orgID)
		createStart := time.Now()

		batchSize := DefaultBatchSize
		for batchStart := 0; batchStart < len(createList); batchStart += batchSize {
			batchEnd := batchStart + batchSize
			if batchEnd > len(createList) {
				batchEnd = len(createList)
			}

			batch := createList[batchStart:batchEnd]
			batchIDs, batchErrors := h.processBatch(ctx, db, orgID, userID, tableName, fields, batch, batchStart, failedIndices, now, options.SkipErrors)
			log.Printf("[IMPORT-CREATE] Batch %d-%d: %d created, %d errors (elapsed: %v)",
				batchStart, batchEnd, len(batchIDs), len(batchErrors), time.Since(createStart))

			createdIDs = append(createdIDs, batchIDs...)
			errors = append(errors, batchErrors...)

			// Add audit entries for created records
			for i, id := range batchIDs {
				recordIdx := createIndices[batchStart+i]
				auditEntries = append(auditEntries, service.AuditEntry{
					RowIndex:   recordIdx,
					Action:     "Imported",
					CreatedID:  id,
					Reason:     "Created new record",
					RecordData: records[recordIdx],
				})
			}

			// Fire tripwires
			if options.FireTripwires && h.tripwireService != nil {
				for i, id := range batchIDs {
					recordIdx := createIndices[batchStart+i]
					go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, records[recordIdx])
				}
			}

			// Emit streaming progress after each batch
			if progressFn != nil {
				progressFn(ImportProgressEvent{
					Type:      "progress",
					Phase:     "importing",
					Processed: preProcessed + batchEnd,
					Total:     totalRecords,
					Created:   len(createdIDs),
					Updated:   len(updatedIDs),
					Failed:    len(errors),
					Skipped:   response.Skipped,
					Merged:    response.Merged,
				})
			}

			// Keep connection alive during long imports
			if h.dbManager != nil {
				h.dbManager.TouchConnection(orgID)
			}
		}
	}

	response.Created = len(createdIDs)
	response.Updated = len(updatedIDs)
	response.Failed = len(errors)
	response.IDs = append(createdIDs, updatedIDs...)
	if len(errors) > 0 {
		response.Errors = errors
	}

	// Always generate audit report when there are audit entries (includes created records too)
	if len(auditEntries) > 0 && h.duplicateService != nil {
		reportCSV := h.duplicateService.GenerateAuditReport(auditEntries)
		response.AuditReport = base64.StdEncoding.EncodeToString(reportCSV)
	}

	// Persist import job and dedup decisions (best-effort, don't fail the import)
	if h.importJobRepo != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[IMPORT] Warning: panic persisting import job: %v\n", r)
					response.Warnings = append(response.Warnings, "Import succeeded but failed to save dedup tracking data")
				}
			}()

			job := &entity.ImportJob{
				ID:              sfid.NewImportJob(),
				OrgID:           orgID,
				EntityType:      entityName,
				ExternalIdField: options.ExternalIdField,
				TotalRows:       response.TotalRows,
				CreatedCount:    response.Created,
				UpdatedCount:    response.Updated,
				SkippedCount:    response.Skipped,
				MergedCount:     response.Merged,
				FailedCount:     response.Failed,
			}

			// Validate and map frontend dedup decisions to entities
			var decisions []entity.ImportDedupDecision
			if len(options.DedupDecisions) > 0 {
				validDecisionTypes := map[string]bool{"within_file": true, "db_match": true}
				validActions := map[string]bool{"skip": true, "update": true, "import": true, "merge": true}

				var skippedInvalid int
				for _, d := range options.DedupDecisions {
					normalizedType := strings.ToLower(strings.TrimSpace(d.DecisionType))
					normalizedAction := strings.ToLower(strings.TrimSpace(d.Action))
					if !validDecisionTypes[normalizedType] || !validActions[normalizedAction] {
						skippedInvalid++
						continue
					}
					decisions = append(decisions, entity.ImportDedupDecision{
						ID:                  sfid.NewDedupDecision(),
						OrgID:               orgID,
						ImportJobID:         job.ID,
						DecisionType:        normalizedType,
						Action:              normalizedAction,
						KeptExternalID:      d.KeptExternalID,
						DiscardedExternalID: d.DiscardedExternalID,
						MatchField:          d.MatchField,
						MatchValue:          d.MatchValue,
						MatchedRecordID:     d.MatchedRecordID,
					})
				}
				if skippedInvalid > 0 {
					fmt.Printf("[IMPORT] Warning: skipped %d dedup decisions with invalid decisionType or action\n", skippedInvalid)
					response.Warnings = append(response.Warnings, fmt.Sprintf("Skipped %d dedup decisions with invalid type or action", skippedInvalid))
				}
			}

			// Atomically persist job + decisions in a single transaction (no orphaned rows)
			if err := h.importJobRepo.SaveJobWithDecisions(ctx, db, job, decisions); err != nil {
				fmt.Printf("[IMPORT] Warning: failed to persist import job and decisions: %v\n", err)
				response.Warnings = append(response.Warnings, "Import succeeded but failed to save import tracking data")
			} else {
				response.ImportID = job.ID
			}
		}()
	}
}

// updateExistingRecord updates an existing record with import row values
func (h *ImportHandler) updateExistingRecord(
	ctx context.Context,
	dbConn *sql.DB,
	orgID, userID, entityName, recordID string,
	importRow map[string]interface{},
	now string,
) error {
	tableName := util.GetTableName(entityName)

	// Get field definitions to properly handle the update
	fields, err := h.metadataRepo.ListFields(ctx, orgID, entityName)
	if err != nil {
		return fmt.Errorf("failed to get fields: %w", err)
	}

	// Build SET clause from import row
	var setClauses []string
	var args []interface{}

	for _, field := range fields {
		if field.Name == "id" || field.Name == "createdAt" || field.Name == "createdById" {
			continue // Don't overwrite system fields
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			baseName := strings.TrimSuffix(field.Name, "Id")
			snakeName := util.CamelToSnake(baseName)
			idKey := baseName + "Id"
			nameKey := baseName + "Name"

			if idVal, ok := importRow[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_id")))
				args = append(args, idVal)
			}
			if nameVal, ok := importRow[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_name")))
				args = append(args, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := importRow[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_ids")))
				args = append(args, idsVal)
			}
			if namesVal, ok := importRow[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_names")))
				args = append(args, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := importRow[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(util.CamelToSnake(field.Name))))
			args = append(args, val)
		}
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Add modified timestamp
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	args = append(args, now, userID)

	// Add WHERE clause values
	args = append(args, recordID, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName, strings.Join(setClauses, ", "))

	_, err = dbConn.ExecContext(ctx, query, args...)
	return err
}

// processUpdateMode handles update operations
func (h *ImportHandler) processUpdateMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
) error {
	// Batch pre-fetch all existing records
	existingMap, err := h.batchFindExistingRecords(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, records, fields, failedIndices)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Batch lookup failed: %v", err)})
	}

	var updatedIDs []string
	var skipped int

	for i, record := range records {
		if failedIndices[i] {
			continue
		}

		matchValue, ok := record[options.MatchField]
		if !ok {
			skipped++
			continue
		}
		matchStr := fmt.Sprintf("%v", matchValue)

		info, found := existingMap[strings.ToLower(matchStr)]
		if !found {
			skipped++
			continue
		}

		if err := h.updateRecord(c.Context(), h.getDB(c), orgID, userID, tableName, info.id, fields, record, now); err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		updatedIDs = append(updatedIDs, info.id)

		if options.FireTripwires && h.tripwireService != nil {
			go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, info.id, "UPDATE", info.oldRecord, record)
		}
	}

	response.Updated = len(updatedIDs)
	response.Skipped = skipped
	response.Failed = len(errors)
	response.IDs = updatedIDs
	if len(errors) > 0 {
		response.Errors = errors
	}

	return c.JSON(response)
}

// processUpsertMode handles upsert (create or update) operations
func (h *ImportHandler) processUpsertMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
) error {
	// Batch pre-fetch all existing records
	existingMap, err := h.batchFindExistingRecords(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, records, fields, failedIndices)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Batch lookup failed: %v", err)})
	}

	var createdIDs []string
	var updatedIDs []string

	for i, record := range records {
		if failedIndices[i] {
			continue
		}

		matchValue, ok := record[options.MatchField]
		if !ok {
			// No match value — treat as new record
			matchValue = ""
		}
		matchStr := fmt.Sprintf("%v", matchValue)

		info, found := existingMap[strings.ToLower(matchStr)]

		if found {
			// Update existing record
			if err := h.updateRecord(c.Context(), h.getDB(c), orgID, userID, tableName, info.id, fields, record, now); err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}
			updatedIDs = append(updatedIDs, info.id)

			if options.FireTripwires && h.tripwireService != nil {
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, info.id, "UPDATE", info.oldRecord, record)
			}
		} else {
			// Create new record
			tx, err := h.getDB(c).BeginTx(c.Context(), nil)
			if err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			id, err := h.insertRecord(c.Context(), tx, orgID, userID, tableName, fields, record, now)
			if err != nil {
				tx.Rollback()
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			if err := tx.Commit(); err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			createdIDs = append(createdIDs, id)

			if options.FireTripwires && h.tripwireService != nil {
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, record)
			}
		}
	}

	response.Created = len(createdIDs)
	response.Updated = len(updatedIDs)
	response.Failed = len(errors)
	response.IDs = append(createdIDs, updatedIDs...)
	if len(errors) > 0 {
		response.Errors = errors
	}

	status := fiber.StatusOK
	if len(createdIDs) > 0 && len(updatedIDs) == 0 {
		status = fiber.StatusCreated
	}

	return c.Status(status).JSON(response)
}

// processDeleteMode handles delete operations.
// When matching by "id", uses batch DELETE with IN clauses for performance.
// For other match fields, falls back to per-row find + delete.
func (h *ImportHandler) processDeleteMode(
	c *fiber.Ctx,
	orgID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	options ImportCSVRequest,
	response *ImportCSVResponse,
) error {
	// Fast path: batch delete by ID (avoids N find + N delete queries)
	if options.MatchField == "id" {
		return h.processDeleteByID(c, orgID, entityName, tableName, records, options, response)
	}

	// Slow path: per-row find + delete for non-ID match fields
	var deletedIDs []string
	var errors []BulkError
	var skipped int

	for i, record := range records {
		existingID, _, err := h.findExistingRecord(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, record, fields)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		if existingID == "" {
			skipped++
			continue
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND org_id = ?", tableName)
		_, err = h.getDB(c).ExecContext(c.Context(), query, existingID, orgID)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		deletedIDs = append(deletedIDs, existingID)

		if options.FireTripwires && h.tripwireService != nil {
			go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, existingID, "DELETE", record, nil)
		}
	}

	response.Deleted = len(deletedIDs)
	response.Skipped = skipped
	response.Failed = len(errors)
	response.IDs = deletedIDs
	if len(errors) > 0 {
		response.Errors = errors
	}
	return c.JSON(response)
}

// processDeleteByID batch-deletes records by ID using IN clauses.
// Processes in batches of 500 to avoid query size limits.
func (h *ImportHandler) processDeleteByID(
	c *fiber.Ctx,
	orgID, entityName, tableName string,
	records []map[string]interface{},
	options ImportCSVRequest,
	response *ImportCSVResponse,
) error {
	const batchSize = 500
	db := h.getDB(c)

	// Collect all IDs from the CSV
	var allIDs []string
	for _, record := range records {
		id, ok := record["id"]
		if !ok || id == nil {
			continue
		}
		idStr := fmt.Sprintf("%v", id)
		if idStr != "" {
			allIDs = append(allIDs, idStr)
		}
	}

	log.Printf("[IMPORT-DELETE] Starting batch delete: %d IDs from %d records, table=%s, org=%s",
		len(allIDs), len(records), tableName, orgID)

	if len(allIDs) > 0 {
		log.Printf("[IMPORT-DELETE] Sample IDs: [%s, %s, ...]", allIDs[0], allIDs[min(1, len(allIDs)-1)])
	}

	// Count records before delete for verification
	var countBefore int64
	countRow := db.QueryRowContext(c.Context(),
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName), orgID)
	if err := countRow.Scan(&countBefore); err != nil {
		log.Printf("[IMPORT-DELETE] Pre-delete count failed: %v", err)
	} else {
		log.Printf("[IMPORT-DELETE] Records before delete: %d", countBefore)
	}

	var totalRowsAffected int64

	// Delete in batches
	for i := 0; i < len(allIDs); i += batchSize {
		end := i + batchSize
		if end > len(allIDs) {
			end = len(allIDs)
		}
		batch := allIDs[i:end]
		batchNum := i/batchSize + 1

		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)+1)
		args = append(args, orgID)
		for j, id := range batch {
			placeholders[j] = "?"
			args = append(args, id)
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE org_id = ? AND id IN (%s)",
			tableName, strings.Join(placeholders, ","))

		result, err := db.ExecContext(c.Context(), query, args...)
		if err != nil {
			log.Printf("[IMPORT-DELETE] Batch %d failed at offset %d: %v", batchNum, i, err)
			response.Failed = len(allIDs) - int(totalRowsAffected)
			response.Errors = append(response.Errors, BulkError{Index: i, Error: err.Error()})
			break
		}

		rowsAffected, _ := result.RowsAffected()
		totalRowsAffected += rowsAffected
		log.Printf("[IMPORT-DELETE] Batch %d: %d IDs submitted, %d rows affected", batchNum, len(batch), rowsAffected)
	}

	// Count records after delete for verified results
	var countAfter int64
	countRow = db.QueryRowContext(c.Context(),
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE org_id = ?", tableName), orgID)
	if err := countRow.Scan(&countAfter); err != nil {
		log.Printf("[IMPORT-DELETE] Post-delete count failed: %v", err)
		// Fall back to RowsAffected
		response.Deleted = int(totalRowsAffected)
		response.Skipped = len(allIDs) - int(totalRowsAffected)
	} else {
		verifiedDeleted := int(countBefore - countAfter)
		log.Printf("[IMPORT-DELETE] Records after delete: %d (verified deleted: %d, RowsAffected total: %d)",
			countAfter, verifiedDeleted, totalRowsAffected)
		// Use verified count - more reliable than RowsAffected with Turso
		response.Deleted = verifiedDeleted
		response.Skipped = len(allIDs) - verifiedDeleted
		if response.Skipped < 0 {
			response.Skipped = 0
		}
	}

	response.IDs = allIDs
	return c.JSON(response)
}

// findExistingRecord finds an existing record by match field
func (h *ImportHandler) findExistingRecord(
	ctx context.Context,
	db *sql.DB,
	orgID, tableName, matchField string,
	record map[string]interface{},
	fields []entity.FieldDef,
) (string, map[string]interface{}, error) {
	// Get the match value from the record
	matchValue, ok := record[matchField]
	if !ok {
		return "", nil, fmt.Errorf("match field '%s' not found in record", matchField)
	}

	// Build the column name
	colName := matchField
	if matchField != "id" {
		colName = util.CamelToSnake(matchField)
	}

	// Query for existing record
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ? AND org_id = ? LIMIT 1",
		tableName, quoteIdentifier(colName))

	rows, err := db.QueryContext(ctx, query, matchValue, orgID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return "", nil, nil // Not found
	}

	// Scan into map
	columns, err := rows.Columns()
	if err != nil {
		return "", nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return "", nil, err
	}

	oldRecord := make(map[string]interface{})
	var existingID string
	for i, col := range columns {
		if col == "id" {
			if v, ok := values[i].(string); ok {
				existingID = v
			}
		}
		oldRecord[col] = values[i]
	}

	return existingID, oldRecord, nil
}

// batchFindExistingRecords pre-fetches existing records in bulk for update/upsert modes.
// Instead of one SELECT per record, it collects unique match values and queries in batches of 500.
// Returns a map: lowered match value -> {id, oldRecord}.
func (h *ImportHandler) batchFindExistingRecords(
	ctx context.Context,
	db *sql.DB,
	orgID, tableName, matchField string,
	records []map[string]interface{},
	fields []entity.FieldDef,
	failedIndices map[int]bool,
	budget ...*importReadBudget,
) (map[string]existingRecordInfo, error) {
	colName := matchField
	if matchField != "id" {
		colName = util.CamelToSnake(matchField)
	}

	// Collect unique match values
	uniqueValues := make(map[string]string) // lowered -> original
	for i, record := range records {
		if failedIndices[i] {
			continue
		}
		matchValue, ok := record[matchField]
		if !ok {
			continue
		}
		valueStr := fmt.Sprintf("%v", matchValue)
		if valueStr == "" {
			continue
		}
		lowered := strings.ToLower(valueStr)
		if _, exists := uniqueValues[lowered]; !exists {
			uniqueValues[lowered] = valueStr
		}
	}

	result := make(map[string]existingRecordInfo, len(uniqueValues))
	if len(uniqueValues) == 0 {
		return result, nil
	}

	// Query in batches of 500
	allValues := make([]string, 0, len(uniqueValues))
	for _, orig := range uniqueValues {
		allValues = append(allValues, orig)
	}

	const batchSize = 500
	for i := 0; i < len(allValues); i += batchSize {
		end := i + batchSize
		if end > len(allValues) {
			end = len(allValues)
		}
		batch := allValues[i:end]

		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)+1)
		args = append(args, orgID)
		for j, v := range batch {
			placeholders[j] = "?"
			args = append(args, v)
		}

		var bgt *importReadBudget
		if len(budget) > 0 {
			bgt = budget[0]
		}

		query := fmt.Sprintf("SELECT * FROM %s WHERE org_id = ? AND %s COLLATE NOCASE IN (%s)",
			tableName, quoteIdentifier(colName), strings.Join(placeholders, ","))

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("batch find existing records failed: %w", err)
		}

		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}

		matchColIdx := -1
		for ci, col := range columns {
			if col == colName {
				matchColIdx = ci
				break
			}
		}

		var rowCount int64
		for rows.Next() {
			rowCount++
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for vi := range values {
				valuePtrs[vi] = &values[vi]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return nil, err
			}

			oldRecord := make(map[string]interface{})
			var existingID string
			var matchVal string
			for ci, col := range columns {
				if col == "id" {
					if v, ok := values[ci].(string); ok {
						existingID = v
					}
				}
				if ci == matchColIdx {
					matchVal = fmt.Sprintf("%v", values[ci])
				}
				oldRecord[col] = values[ci]
			}

			if matchVal != "" {
				result[strings.ToLower(matchVal)] = existingRecordInfo{
					id:        existingID,
					oldRecord: oldRecord,
				}
			}
		}
		rows.Close()

		// Charge budget for actual rows read (not estimated batch size)
		if err := bgt.charge(rowCount); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// existingRecordInfo holds pre-fetched record data for batch lookups
type existingRecordInfo struct {
	id        string
	oldRecord map[string]interface{}
}

// updateRecord updates an existing record
func (h *ImportHandler) updateRecord(
	ctx context.Context,
	db *sql.DB,
	orgID, userID, tableName, id string,
	fields []entity.FieldDef,
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

	// Add modified timestamps
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	values = append(values, now, userID)

	// Add WHERE clause values
	values = append(values, id, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName, strings.Join(setClauses, ", "))

	_, err := db.ExecContext(ctx, query, values...)
	return err
}

// AnalyzeCSV handles POST /api/v1/entities/:entity/import/csv/analyze
func (h *ImportHandler) AnalyzeCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use 'file' field in multipart form."})
	}

	// Validate file size (max 50MB)
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File too large. Maximum size is 50MB."})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid file type. Only CSV files are accepted."})
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content into buffer
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Parse options from form data (for column mapping)
	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse CSV
	var parseResult *service.CSVParseResult
	if len(options.ColumnMapping) > 0 {
		parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
	} else {
		parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check for parse errors
	if len(parseResult.Errors) > 0 {
		var bulkErrors []BulkError
		for _, e := range parseResult.Errors {
			bulkErrors = append(bulkErrors, BulkError{
				Index: e.Row,
				Error: e.Message,
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "CSV parsing errors",
			"errors": bulkErrors,
		})
	}

	if len(parseResult.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid records found in CSV"})
	}

	// Skip validation for delete mode — only the match field is needed, not all required fields
	if options.Mode == ImportModeDelete {
		return c.JSON(service.AnalyzeResult{
			Valid:           true,
			TotalRows:       len(parseResult.Records),
			ValidRows:       len(parseResult.Records),
			InvalidRows:     0,
			Issues:          []service.ValidationIssue{},
			MissingRequired: []string{},
		})
	}

	// Validate records
	analyzeResult := h.csvValidator.Validate(parseResult.Records, fields)

	return c.JSON(analyzeResult)
}

// AnalyzeLookups handles POST /api/v1/entities/:entity/import/csv/analyze-lookups
// Returns information about lookup values that don't exist and would need to be created
func (h *ImportHandler) AnalyzeLookups(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Parse options from form data
	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Get field definitions for the main entity
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse CSV with mapping
	var parseResult *service.CSVParseResult
	if len(options.ColumnMapping) > 0 {
		parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
	} else {
		parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Build map of link fields
	linkFields := make(map[string]*entity.FieldDef)
	for i := range fields {
		if fields[i].Type == entity.FieldTypeLink {
			linkFields[fields[i].Name] = &fields[i]
		}
	}

	// Collect unique lookup values for each field with createIfNotFound
	// fieldName -> matchValue -> row indices
	lookupValues := make(map[string]map[string][]int)

	for rowIdx, record := range parseResult.Records {
		for fieldName, resolution := range options.LookupResolution {
			if !resolution.CreateIfNotFound {
				continue
			}

			field, ok := linkFields[fieldName]
			if !ok {
				continue
			}

			// Get the value from record
			// fieldName is already the full field name (e.g., "accountId") from LookupResolution
			lookupValue, ok := record[fieldName]
			if !ok {
				continue
			}

			valueStr, ok := lookupValue.(string)
			if !ok || valueStr == "" {
				continue
			}

			// Skip if it looks like an ID already
			if strings.HasPrefix(valueStr, "Rec") {
				continue
			}

			// Track this value
			if lookupValues[fieldName] == nil {
				lookupValues[fieldName] = make(map[string][]int)
			}
			lookupValues[fieldName][valueStr] = append(lookupValues[fieldName][valueStr], rowIdx)

			_ = field // used for validation
		}
	}

	// Check which values don't exist and build response
	var missingLookups []MissingLookup

	for fieldName, values := range lookupValues {
		field := linkFields[fieldName]
		resolution := options.LookupResolution[fieldName]

		relatedEntity := ""
		if field.LinkEntity != nil {
			relatedEntity = *field.LinkEntity
		}
		if relatedEntity == "" {
			continue
		}

		// Get required fields for the related entity
		relatedFields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, relatedEntity)
		if err != nil {
			continue
		}

		var requiredFields []AvailableField
		for _, rf := range relatedFields {
			if rf.IsRequired && rf.Name != "id" && rf.Name != "name" {
				requiredFields = append(requiredFields, AvailableField{
					Name:  rf.Name,
					Label: rf.Label,
					Type:  string(rf.Type),
				})
			}
		}

		// Batch-query to find which values already exist (avoids per-value round-trips)
		relatedTable := util.GetTableName(relatedEntity)
		matchColumn := util.CamelToSnake(resolution.MatchField)

		// Collect all unique values into a slice for batching
		allValues := make([]string, 0, len(values))
		for v := range values {
			allValues = append(allValues, v)
		}

		// Query in batches of 500 (SQLite parameter limit is 999)
		existingValues := make(map[string]bool)
		const batchSize = 500
		for i := 0; i < len(allValues); i += batchSize {
			end := i + batchSize
			if end > len(allValues) {
				end = len(allValues)
			}
			batch := allValues[i:end]

			placeholders := make([]string, len(batch))
			args := make([]interface{}, 0, len(batch)+1)
			args = append(args, orgID)
			for j, v := range batch {
				placeholders[j] = "?"
				args = append(args, v)
			}

			query := fmt.Sprintf(
				"SELECT %s FROM %s WHERE org_id = ? AND %s COLLATE NOCASE IN (%s)",
				quoteIdentifier(matchColumn),
				relatedTable,
				quoteIdentifier(matchColumn),
				strings.Join(placeholders, ","),
			)

			rows, err := h.getDB(c).QueryContext(c.Context(), query, args...)
			if err != nil {
				continue
			}
			for rows.Next() {
				var val string
				if err := rows.Scan(&val); err == nil {
					existingValues[strings.ToLower(val)] = true
				}
			}
			rows.Close()
		}

		// Any value not found is missing
		for matchValue, rowIndices := range values {
			if !existingValues[strings.ToLower(matchValue)] {
				missingLookups = append(missingLookups, MissingLookup{
					FieldName:      fieldName,
					RelatedEntity:  relatedEntity,
					MatchValue:     matchValue,
					MatchField:     resolution.MatchField,
					RequiredFields: requiredFields,
					RowIndices:     rowIndices,
				})
			}
		}
	}

	return c.JSON(AnalyzeLookupResponse{
		MissingLookups: missingLookups,
	})
}

// PreviewCSV handles POST /api/v1/entities/:entity/import/csv/preview
func (h *ImportHandler) PreviewCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Get sample rows for preview
	headers, sampleRows, err := h.csvParser.GetSampleRows(bytes.NewReader(fileContent), 5)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse full file to get total count and mapping
	parseResult, err := h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Build field mapping info
	var fieldMappings []FieldMapping
	var unmappedCols []string

	for i, header := range headers {
		var mappedField string
		if i < len(parseResult.MappedHeaders) {
			mappedField = parseResult.MappedHeaders[i]
		}

		mapping := FieldMapping{
			CSVHeader: header,
			FieldName: mappedField,
			Mapped:    mappedField != "",
		}

		// Find field details
		if mappedField != "" {
			for _, field := range fields {
				if field.Name == mappedField ||
				   field.Name+"Id" == mappedField ||
				   field.Name+"Name" == mappedField ||
				   field.Name+"Ids" == mappedField ||
				   field.Name+"Names" == mappedField {
					mapping.FieldLabel = field.Label
					mapping.FieldType = string(field.Type)
					break
				}
			}
		} else {
			unmappedCols = append(unmappedCols, header)
		}

		fieldMappings = append(fieldMappings, mapping)
	}

	// Convert sample rows to maps
	var sampleMaps []map[string]interface{}
	for _, row := range sampleRows {
		record := make(map[string]interface{})
		for i, val := range row {
			if i < len(headers) {
				record[headers[i]] = val
			}
		}
		sampleMaps = append(sampleMaps, record)
	}

	// Build list of all available fields for dropdown
	var availableFields []AvailableField
	// Include "ID" for update/upsert/delete modes where matching by ID is needed
	availableFields = append(availableFields, AvailableField{Name: "id", Label: "ID", Type: "varchar"})
	for _, field := range fields {
		// Skip system fields that shouldn't be imported (except id, added above)
		if field.Name == "id" || field.Name == "createdAt" || field.Name == "modifiedAt" ||
			field.Name == "createdById" || field.Name == "modifiedById" {
			continue
		}
		af := AvailableField{
			Name:  field.Name,
			Label: field.Label,
			Type:  string(field.Type),
		}
		// For link fields, include the related entity name and its fields
		if field.Type == entity.FieldTypeLink && field.LinkEntity != nil {
			af.RelatedEntity = *field.LinkEntity
			// Fetch fields from related entity for lookup matching dropdown
			relatedFields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, *field.LinkEntity)
			if err == nil {
				for _, rf := range relatedFields {
					// Include text-like fields that can be used for matching
					if rf.Type == entity.FieldTypeVarchar || rf.Type == entity.FieldTypeText ||
						rf.Type == entity.FieldTypeEmail || rf.Type == entity.FieldTypeURL ||
						rf.Type == entity.FieldTypePhone || rf.Name == "name" {
						af.RelatedEntityFields = append(af.RelatedEntityFields, AvailableField{
							Name:  rf.Name,
							Label: rf.Label,
							Type:  string(rf.Type),
						})
					}
				}
			}
		}
		availableFields = append(availableFields, af)
	}

	return c.JSON(PreviewCSVResponse{
		Headers:         headers,
		MappedHeaders:   parseResult.MappedHeaders,
		SampleRows:      sampleMaps,
		TotalRows:       parseResult.RowCount,
		UnmappedCols:    unmappedCols,
		Fields:          fieldMappings,
		AvailableFields: availableFields,
	})
}

// resolveLookups resolves lookup field values to actual record IDs using batch queries.
// Instead of one SELECT per record, it collects unique values across all records,
// resolves them in batch (500 at a time), then applies cached results.
// This reduces 48K individual queries to ~96 batch queries for a 48K-record import.
func (h *ImportHandler) resolveLookups(
	ctx context.Context,
	db *sql.DB,
	orgID, userID string,
	records []map[string]interface{},
	fields []entity.FieldDef,
	lookupResolution map[string]LookupResolution,
	budget ...*importReadBudget,
) ([]BulkError, error) {
	if len(lookupResolution) == 0 {
		return nil, nil
	}

	// Build a map of link fields for quick lookup
	linkFields := make(map[string]*entity.FieldDef)
	for i := range fields {
		if fields[i].Type == entity.FieldTypeLink {
			linkFields[fields[i].Name] = &fields[i]
			linkFields[fields[i].Name+"Id"] = &fields[i]
		}
	}

	// --- Phase A: Collect unique lookup values per (relatedEntity, matchField) ---
	type lookupKey struct {
		relatedEntity string
		matchField    string
		matchColumn   string
		relatedTable  string
	}
	// fieldName -> lookupKey (derived from resolution + field def)
	fieldKeys := make(map[string]*lookupKey)
	// lookupKey string -> set of unique values (lowered -> original)
	uniqueValues := make(map[string]map[string]string)

	for fieldName, resolution := range lookupResolution {
		field, ok := linkFields[fieldName]
		if !ok {
			continue
		}
		relatedEntity := ""
		if field.LinkEntity != nil {
			relatedEntity = *field.LinkEntity
		}
		if relatedEntity == "" {
			continue
		}

		lk := &lookupKey{
			relatedEntity: relatedEntity,
			matchField:    resolution.MatchField,
			matchColumn:   util.CamelToSnake(resolution.MatchField),
			relatedTable:  util.GetTableName(relatedEntity),
		}
		fieldKeys[fieldName] = lk
		cacheKey := relatedEntity + ":" + resolution.MatchField

		if uniqueValues[cacheKey] == nil {
			uniqueValues[cacheKey] = make(map[string]string)
		}

		for _, record := range records {
			lookupValue, ok := record[fieldName]
			if !ok {
				continue
			}
			valueStr, ok := lookupValue.(string)
			if !ok || valueStr == "" || strings.HasPrefix(valueStr, "Rec") {
				continue
			}
			lowered := strings.ToLower(valueStr)
			if _, exists := uniqueValues[cacheKey][lowered]; !exists {
				uniqueValues[cacheKey][lowered] = valueStr // keep first original casing
			}
		}
	}

	// --- Phase B: Batch-resolve all unique values ---
	// Cache: relatedEntity:matchField -> lowered value -> {id, displayName}
	type resolvedLookup struct {
		id          string
		displayName string
	}
	lookupCache := make(map[string]map[string]resolvedLookup)

	const batchSize = 500
	for cacheKey, valuesMap := range uniqueValues {
		if len(valuesMap) == 0 {
			continue
		}

		lookupCache[cacheKey] = make(map[string]resolvedLookup)

		// Find the lookupKey info for this cacheKey
		var lk *lookupKey
		for _, fk := range fieldKeys {
			if fk.relatedEntity+":"+fk.matchField == cacheKey {
				lk = fk
				break
			}
		}
		if lk == nil {
			continue
		}

		// Collect all lowered values into a slice
		allLowered := make([]string, 0, len(valuesMap))
		for lowered := range valuesMap {
			allLowered = append(allLowered, lowered)
		}

		// Query in batches of 500
		for i := 0; i < len(allLowered); i += batchSize {
			end := i + batchSize
			if end > len(allLowered) {
				end = len(allLowered)
			}
			batch := allLowered[i:end]

			placeholders := make([]string, len(batch))
			args := make([]interface{}, 0, len(batch)+1)
			args = append(args, orgID)
			for j, v := range batch {
				placeholders[j] = "?"
				args = append(args, valuesMap[v]) // use original casing for param
			}

			query := fmt.Sprintf(
				"SELECT id, COALESCE(%s, '') FROM %s WHERE org_id = ? AND %s COLLATE NOCASE IN (%s)",
				quoteIdentifier(lk.matchColumn),
				lk.relatedTable,
				quoteIdentifier(lk.matchColumn),
				strings.Join(placeholders, ","),
			)

			var bgt *importReadBudget
			if len(budget) > 0 {
				bgt = budget[0]
			}

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return nil, fmt.Errorf("batch lookup query failed for %s: %w", lk.relatedEntity, err)
			}
			var rowCount int64
			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err != nil {
					rows.Close()
					return nil, fmt.Errorf("batch lookup scan failed: %w", err)
				}
				lookupCache[cacheKey][strings.ToLower(name)] = resolvedLookup{id: id, displayName: name}
				rowCount++
			}
			rows.Close()

			// Charge budget for actual rows read (not estimated batch size)
			if err := bgt.charge(rowCount); err != nil {
				return nil, err
			}
		}
	}

	// --- Phase C: Apply results to records, create missing if needed ---
	var errors []BulkError

	for rowIdx, record := range records {
		for fieldName, resolution := range lookupResolution {
			field, ok := linkFields[fieldName]
			if !ok {
				continue
			}

			lookupValue, ok := record[fieldName]
			if !ok {
				continue
			}
			valueStr, ok := lookupValue.(string)
			if !ok || valueStr == "" {
				continue
			}
			if strings.HasPrefix(valueStr, "Rec") {
				continue
			}

			lk := fieldKeys[fieldName]
			if lk == nil {
				errors = append(errors, BulkError{
					Index: rowIdx,
					Error: fmt.Sprintf("Field '%s' has no related entity configured", fieldName),
				})
				continue
			}

			cacheKey := lk.relatedEntity + ":" + lk.matchField
			baseName := strings.TrimSuffix(field.Name, "Id")
			idKey := baseName + "Id"
			nameKey := baseName + "Name"

			// Check batch cache
			if resolved, ok := lookupCache[cacheKey][strings.ToLower(valueStr)]; ok {
				record[idKey] = resolved.id
				record[nameKey] = valueStr
				if fieldName != idKey {
					delete(record, fieldName)
				}
				continue
			}

			// Not found — create if allowed
			if resolution.CreateIfNotFound {
				// Check if we already created this value (from a previous row in this loop)
				if resolved, ok := lookupCache[cacheKey][strings.ToLower(valueStr)]; ok {
					record[idKey] = resolved.id
					record[nameKey] = valueStr
					if fieldName != idKey {
						delete(record, fieldName)
					}
					continue
				}

				newData := make(map[string]interface{})
				if resolution.NewRecordData != nil {
					if data, ok := resolution.NewRecordData[valueStr]; ok {
						newData = data
					}
				}
				newData[resolution.MatchField] = valueStr

				newID, createErr := h.createRelatedRecord(ctx, db, orgID, userID, lk.relatedEntity, newData)
				if createErr != nil {
					errors = append(errors, BulkError{
						Index: rowIdx,
						Error: fmt.Sprintf("Failed to create %s '%s': %v", lk.relatedEntity, valueStr, createErr),
					})
					delete(record, fieldName)
					continue
				}

				// Cache the newly created record so subsequent rows reuse it
				lookupCache[cacheKey][strings.ToLower(valueStr)] = resolvedLookup{id: newID, displayName: valueStr}
				record[idKey] = newID
				record[nameKey] = valueStr
				if fieldName != idKey {
					delete(record, fieldName)
				}
				continue
			}

			errors = append(errors, BulkError{
				Index: rowIdx,
				Error: fmt.Sprintf("No %s found with %s='%s'", lk.relatedEntity, resolution.MatchField, valueStr),
			})
			delete(record, fieldName)
		}
	}

	return errors, nil
}

// createRelatedRecord creates a new record in a related entity table during import
func (h *ImportHandler) createRelatedRecord(
	ctx context.Context,
	db *sql.DB,
	orgID, userID string,
	entityName string,
	data map[string]interface{},
) (string, error) {
	// Get field definitions for the related entity using the provided db
	metaRepo := h.metadataRepo.WithRawDB(db)
	fields, err := metaRepo.ListFields(ctx, orgID, entityName)
	if err != nil {
		return "", fmt.Errorf("failed to get fields for %s: %w", entityName, err)
	}

	tableName := util.GetTableName(entityName)
	now := time.Now().UTC().Format(time.RFC3339)

	// Build insert query
	var columns []string
	var placeholders []string
	var values []interface{}

	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		if val, ok := data[field.Name]; ok {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, val)
		}
	}

	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, now, now, userID, userID)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err = db.ExecContext(ctx, query, values...)
	if err != nil {
		return "", err
	}

	return id, nil
}

// processBatch processes a batch of records (reused from bulk handler pattern)
func (h *ImportHandler) processBatch(
	ctx context.Context,
	db *sql.DB,
	orgID, userID, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	batchOffset int,
	failedIndices map[int]bool,
	now string,
	skipErrors bool,
) ([]string, []BulkError) {
	var createdIDs []string
	var errors []BulkError

	// Filter out failed records
	type validRecord struct {
		index  int
		record map[string]interface{}
	}
	var validRecords []validRecord
	for i, record := range records {
		globalIndex := batchOffset + i
		if failedIndices[globalIndex] {
			continue
		}
		validRecords = append(validRecords, validRecord{index: globalIndex, record: record})
	}

	if len(validRecords) == 0 {
		return nil, nil
	}

	// Build deterministic column list from field definitions (same for all rows)
	columns := []string{"id", "org_id"}
	for _, field := range fields {
		if field.Name == "id" || field.Name == "createdAt" || field.Name == "modifiedAt" ||
			field.Name == "createdById" || field.Name == "modifiedById" {
			continue
		}
		if field.Type == entity.FieldTypeLink {
			baseName := strings.TrimSuffix(field.Name, "Id")
			snakeName := util.CamelToSnake(baseName)
			columns = append(columns, quoteIdentifier(snakeName+"_id"))
			columns = append(columns, quoteIdentifier(snakeName+"_name"))
			continue
		}
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			columns = append(columns, quoteIdentifier(snakeName+"_ids"))
			columns = append(columns, quoteIdentifier(snakeName+"_names"))
			continue
		}
		columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
	}
	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")

	numCols := len(columns)

	// Build single-row placeholder: (?,?,?...)
	phParts := make([]string, numCols)
	for i := range phParts {
		phParts[i] = "?"
	}
	rowPlaceholder := "(" + strings.Join(phParts, ",") + ")"

	// Calculate max rows per multi-row INSERT to stay within parameter limits
	// Turso/libsql supports up to 32766 params; use conservative limit
	maxRowsPerInsert := 500 / numCols
	if maxRowsPerInsert < 1 {
		maxRowsPerInsert = 1
	}
	if maxRowsPerInsert > 100 {
		maxRowsPerInsert = 100
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		for _, vr := range validRecords {
			errors = append(errors, BulkError{Index: vr.index, Error: "Failed to start transaction: " + err.Error()})
		}
		return nil, errors
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Process in sub-batches using multi-row INSERT
	for subStart := 0; subStart < len(validRecords); subStart += maxRowsPerInsert {
		subEnd := subStart + maxRowsPerInsert
		if subEnd > len(validRecords) {
			subEnd = len(validRecords)
		}
		subBatch := validRecords[subStart:subEnd]

		var rowPlaceholders []string
		var allValues []interface{}
		var batchIDs []string

		for _, vr := range subBatch {
			record := vr.record
			id := sfid.New("Rec")
			batchIDs = append(batchIDs, id)

			allValues = append(allValues, id, orgID)

			for _, field := range fields {
				if field.Name == "id" || field.Name == "createdAt" || field.Name == "modifiedAt" ||
					field.Name == "createdById" || field.Name == "modifiedById" {
					continue
				}
				if field.Type == entity.FieldTypeLink {
					baseName := strings.TrimSuffix(field.Name, "Id")
					idKey := baseName + "Id"
					nameKey := baseName + "Name"
					allValues = append(allValues, record[idKey])
					allValues = append(allValues, record[nameKey])
					continue
				}
				if field.Type == entity.FieldTypeLinkMultiple {
					idsKey := field.Name + "Ids"
					namesKey := field.Name + "Names"
					allValues = append(allValues, record[idsKey])
					allValues = append(allValues, record[namesKey])
					continue
				}
				// Regular field
				val := record[field.Name]
				if val == nil && field.DefaultValue != nil && *field.DefaultValue != "" {
					val = *field.DefaultValue
				}
				allValues = append(allValues, val)
			}

			allValues = append(allValues, now, now, userID, userID)
			rowPlaceholders = append(rowPlaceholders, rowPlaceholder)
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
			tableName, strings.Join(columns, ", "), strings.Join(rowPlaceholders, ", "))

		_, err := tx.ExecContext(ctx, query, allValues...)
		if err != nil {
			if skipErrors {
				// Fall back to individual inserts for this sub-batch
				for _, vr := range subBatch {
					id, insertErr := h.insertRecord(ctx, tx, orgID, userID, tableName, fields, vr.record, now)
					if insertErr != nil {
						errors = append(errors, BulkError{Index: vr.index, Error: insertErr.Error()})
					} else {
						createdIDs = append(createdIDs, id)
					}
				}
			} else {
				tx.Rollback()
				return nil, []BulkError{{Index: subBatch[0].index, Error: err.Error()}}
			}
			continue
		}

		createdIDs = append(createdIDs, batchIDs...)
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
func (h *ImportHandler) insertRecord(
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

	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			// For link fields, field.Name may already have "Id" suffix (e.g., "accountId")
			// We need the base name (e.g., "account") to build proper keys
			baseName := strings.TrimSuffix(field.Name, "Id")
			snakeName := util.CamelToSnake(baseName)
			idKey := baseName + "Id"
			nameKey := baseName + "Name"

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

// ensureTableExists creates the table if it doesn't exist
func (h *ImportHandler) ensureTableExists(ctx context.Context, db *sql.DB, orgID, entityName string, fields []entity.FieldDef) error {
	tableName := h.getTableName(entityName)

	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	var columns []string
	columns = append(columns, "id TEXT PRIMARY KEY")
	columns = append(columns, "org_id TEXT NOT NULL")

	for _, field := range fields {
		if field.Name == "id" || field.Name == "created_at" || field.Name == "modified_at" {
			continue
		}

		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			columns = append(columns, fmt.Sprintf("%s TEXT", quoteIdentifier(snakeName+"_id")))
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT ''", quoteIdentifier(snakeName+"_name")))
			continue
		}

		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT '[]'", quoteIdentifier(snakeName+"_ids")))
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT '[]'", quoteIdentifier(snakeName+"_names")))
			continue
		}

		colType := "TEXT"
		switch field.Type {
		case "int":
			colType = "INTEGER"
		case "float", "currency":
			colType = "REAL"
		case "bool":
			colType = "INTEGER"
		}

		colDef := fmt.Sprintf("%s %s", quoteIdentifier(util.CamelToSnake(field.Name)), colType)
		if field.IsRequired {
			colDef += " NOT NULL"
		}
		columns = append(columns, colDef)
	}

	columns = append(columns, "created_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "modified_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "created_by_id TEXT")
	columns = append(columns, "modified_by_id TEXT")

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ", "))
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	indexSQL := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_org_id ON %s(org_id)", tableName, tableName)
	_, err = db.ExecContext(ctx, indexSQL)
	return err
}

// CheckDuplicates handles POST /api/v1/entities/:entity/import/csv/check-duplicates
// Streams progress as NDJSON lines, with the final line containing the full result.
//
// The stream starts immediately to prevent Railway/proxy timeouts on large files.
// All heavy work (CSV parsing, dedup) happens inside the stream writer.
func (h *ImportHandler) CheckDuplicates(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Quick validation that can fail with normal HTTP status codes
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use 'file' field in multipart form."})
	}
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File too large. Maximum size is 50MB."})
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid file type. Only CSV files are accepted."})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Capture tenant DB and metadata repo before entering the stream writer
	tenantDB := middleware.GetTenantDB(c)
	if tenantDB == nil {
		tenantDB = h.db
	}
	metaRepo := h.getMetadataRepo(c)

	// Start NDJSON stream immediately to keep the connection alive
	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	ctx := c.Context()
	ctx.SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Helper to write an NDJSON line and flush
		writeLine := func(v interface{}) {
			line, _ := json.Marshal(v)
			w.Write(line)
			w.Write([]byte("\n"))
			w.Flush()
		}
		writeError := func(msg string) {
			writeLine(map[string]interface{}{"type": "error", "error": msg})
		}

		// Send initial progress to confirm the stream is open
		writeLine(map[string]interface{}{
			"type": "progress", "phase": "parsing", "processedRows": 0, "totalRows": 0, "duplicatesFound": 0,
		})

		// Heavy work: parse CSV inside the stream so timeouts don't kill us
		fields, err := metaRepo.ListFields(context.Background(), orgID, entityName)
		if err != nil {
			writeError(fmt.Sprintf("Failed to load fields: %v", err))
			return
		}

		var parseResult *service.CSVParseResult
		if len(options.ColumnMapping) > 0 {
			parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
		} else {
			parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
		}
		// Free raw file bytes now that parsing is done
		fileContent = nil

		if err != nil {
			writeError(fmt.Sprintf("CSV parse error: %v", err))
			return
		}
		if len(parseResult.Records) == 0 {
			writeError("No valid records found in CSV")
			return
		}

		records := parseResult.Records

		// Send parsing-complete progress
		writeLine(map[string]interface{}{
			"type": "progress", "phase": "preparing", "processedRows": 0, "totalRows": len(records), "duplicatesFound": 0,
		})

		// Run duplicate detection with streaming progress
		onProgress := func(p service.DuplicateCheckProgress) {
			writeLine(map[string]interface{}{
				"type":            "progress",
				"phase":           p.Phase,
				"processedRows":   p.ProcessedRows,
				"totalRows":       p.TotalRows,
				"duplicatesFound": p.DuplicatesFound,
			})
		}

		result, err := h.duplicateService.CheckDuplicatesWithProgress(
			context.Background(), tenantDB, orgID, entityName, records, onProgress,
		)
		if err != nil {
			writeError(fmt.Sprintf("Duplicate detection failed: %v. You can skip duplicate checking and proceed with import.", err))
			return
		}

		writeLine(map[string]interface{}{"type": "result", "result": result})
	}))

	return nil
}

// PreflightCheck handles GET /api/v1/entities/:entity/import/preflight-check
// Returns estimated Turso row reads and current usage, warning if near quota.
func (h *ImportHandler) PreflightCheck(c *fiber.Ctx) error {
	recordCount := c.QueryInt("recordCount", 0)
	if recordCount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "recordCount query param required (positive integer)"})
	}

	lookupFieldCount := c.QueryInt("lookupFieldCount", 0)

	if h.importQuotaService == nil {
		// No quota service configured — return estimate only
		estimated := service.EstimateImportCost(recordCount, lookupFieldCount, true)
		return c.JSON(service.PreflightResult{
			EstimatedReads:   estimated,
			MonthlyLimit:     service.TursoFreeRowReads,
			RemainingBudget:  service.TursoFreeRowReads,
			QuotaUnavailable: true,
			Warning:          "Turso quota check not configured",
		})
	}

	result, err := h.importQuotaService.CheckQuota(c.Context(), recordCount, lookupFieldCount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// RegisterRoutes registers import routes
func (h *ImportHandler) RegisterRoutes(app fiber.Router) {
	app.Post("/entities/:entity/import/csv", h.ImportCSV)
	app.Post("/entities/:entity/import/csv/preview", h.PreviewCSV)
	app.Post("/entities/:entity/import/csv/analyze", h.AnalyzeCSV)
	app.Post("/entities/:entity/import/csv/analyze-lookups", h.AnalyzeLookups)
	app.Post("/entities/:entity/import/csv/check-duplicates", h.CheckDuplicates)
	app.Get("/entities/:entity/import/preflight-check", h.PreflightCheck)
	app.Get("/import/progress/:jobID", h.GetImportProgress)
}
