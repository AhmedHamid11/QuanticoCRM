package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// ProvisioningServiceInterface defines the methods needed for re-provisioning
type ProvisioningServiceInterface interface {
	ProvisionDefaultMetadata(ctx context.Context, orgID string) error
	SetDB(dbConn db.DBConn)
}

// AdminHandler handles HTTP requests for admin panel
type AdminHandler struct {
	db                  *sql.DB
	dbManager           *db.Manager
	metadataRepo        *repo.MetadataRepo
	navigationRepo      *repo.NavigationRepo
	layoutService       *service.LayoutService
	provisioningService ProvisioningServiceInterface
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(masterDB *sql.DB, metadataRepo *repo.MetadataRepo, navigationRepo *repo.NavigationRepo) *AdminHandler {
	return &AdminHandler{
		db:             masterDB,
		metadataRepo:   metadataRepo,
		navigationRepo: navigationRepo,
		layoutService:  service.NewLayoutService(),
	}
}

// NewAdminHandlerWithManager creates a new AdminHandler with database manager support
func NewAdminHandlerWithManager(masterDB *sql.DB, dbManager *db.Manager, metadataRepo *repo.MetadataRepo, navigationRepo *repo.NavigationRepo) *AdminHandler {
	return &AdminHandler{
		db:        masterDB,
		dbManager: dbManager,
		metadataRepo:   metadataRepo,
		navigationRepo: navigationRepo,
		layoutService:  service.NewLayoutService(),
	}
}

// SetProvisioningService sets the provisioning service for re-provisioning support
func (h *AdminHandler) SetProvisioningService(svc ProvisioningServiceInterface) {
	h.provisioningService = svc
}

// getMetadataRepo returns a metadata repo using the tenant database from context
// This ensures multi-tenant data isolation - each org's metadata is stored in their own database
func (h *AdminHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getNavigationRepo returns a navigation repo using the tenant database from context
func (h *AdminHandler) getNavigationRepo(c *fiber.Ctx) *repo.NavigationRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.navigationRepo.WithDB(tenantDB)
	}
	return h.navigationRepo
}

// getDB returns the tenant database from context, falling back to default db
func (h *AdminHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.db
}

// camelToSnakeAdmin converts a camelCase string to snake_case
func camelToSnakeAdmin(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32)) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// getTableNameAdmin converts entity name to table name (e.g., "Candidate" -> "candidates")
func getTableNameAdmin(entityName string) string {
	return strings.ToLower(entityName) + "s"
}

// quoteIdentifierAdmin quotes a SQL identifier (column/table name) for SQLite
func quoteIdentifierAdmin(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// --- Field Types ---

// ListFieldTypes returns all available field types
func (h *AdminHandler) ListFieldTypes(c *fiber.Ctx) error {
	return c.JSON(entity.GetFieldTypes())
}

// --- Entities ---

// ListEntities returns all entity definitions
func (h *AdminHandler) ListEntities(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entities, err := h.getMetadataRepo(c).ListEntities(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(entities)
}

// GetEntity returns a single entity definition
func (h *AdminHandler) GetEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	name := c.Params("name")

	entity, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Entity not found",
		})
	}

	return c.JSON(entity)
}

// CreateEntity creates a new entity definition
func (h *AdminHandler) CreateEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	var input entity.EntityDefCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Entity name is required",
		})
	}
	if input.Label == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Entity label is required",
		})
	}

	// Check if entity already exists
	existing, _ := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, input.Name)
	if existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Entity with this name already exists",
		})
	}

	ent, err := h.getMetadataRepo(c).CreateEntity(c.Context(), orgID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Create navigation tab for the new entity
	labelPlural := input.LabelPlural
	if labelPlural == "" {
		labelPlural = input.Label + "s"
	}
	// Use plural lowercase for href (e.g., "Candidate" -> "/candidates")
	href := "/" + strings.ToLower(labelPlural)
	icon := input.Icon
	if icon == "" {
		icon = "folder"
	}

	navRepo := h.getNavigationRepo(c)

	// Check if a navigation tab with this href already exists
	existingNav, _ := navRepo.GetByHref(c.Context(), orgID, href)
	if existingNav != nil {
		// Navigation tab with this href already exists
		if existingNav.EntityName == nil || *existingNav.EntityName == "" {
			// Existing tab has no entity linked - update it to link to this entity
			updateInput := entity.NavigationTabUpdateInput{
				EntityName: &input.Name,
			}
			_, updateErr := navRepo.Update(c.Context(), orgID, existingNav.ID, updateInput)
			if updateErr != nil {
				log.Printf("[WARN] Failed to update existing navigation tab %s to link entity %s (org: %s): %v", existingNav.ID, input.Name, orgID, updateErr)
			} else {
				log.Printf("[INFO] Updated existing navigation tab '%s' to link entity %s (org: %s)", existingNav.Label, input.Name, orgID)
			}
		} else {
			log.Printf("[INFO] Navigation tab for href '%s' already exists and is linked to entity '%s' (org: %s)", href, *existingNav.EntityName, orgID)
		}
	} else {
		// No existing tab - create a new one
		navInput := entity.NavigationTabCreateInput{
			Label:      labelPlural,
			Href:       href,
			Icon:       icon,
			EntityName: &input.Name,
			IsVisible:  true,
		}
		_, navErr := navRepo.Create(c.Context(), orgID, navInput)
		if navErr != nil {
			// Log the error but don't fail - entity was created successfully
			// Navigation can be added manually if needed
			log.Printf("[WARN] Failed to create navigation tab for entity %s (org: %s): %v", input.Name, orgID, navErr)
		} else {
			log.Printf("[INFO] Created navigation tab '%s' for entity %s (org: %s)", labelPlural, input.Name, orgID)
		}
	}

	return c.Status(fiber.StatusCreated).JSON(ent)
}

// UpdateEntity updates an entity definition
func (h *AdminHandler) UpdateEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	name := c.Params("name")

	var input entity.EntityDefUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ent, err := h.getMetadataRepo(c).UpdateEntity(c.Context(), orgID, name, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Entity not found",
		})
	}

	return c.JSON(ent)
}

// DeleteEntity soft-deletes an entity definition
func (h *AdminHandler) DeleteEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	name := c.Params("name")

	err := h.getMetadataRepo(c).SoftDeleteEntity(c.Context(), orgID, name)
	if err != nil {
		if err.Error() == "Cannot delete system entity" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Entity not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// --- Fields ---

// ListFields returns all field definitions for an entity
func (h *AdminHandler) ListFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// GetField returns a single field definition
func (h *AdminHandler) GetField(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	fieldName, _ := url.PathUnescape(c.Params("field"))

	field, err := h.getMetadataRepo(c).GetField(c.Context(), orgID, entityName, fieldName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if field == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.JSON(field)
}

// CreateField creates a new field definition
func (h *AdminHandler) CreateField(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	var input entity.FieldDefCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Field name is required",
		})
	}
	if input.Label == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Field label is required",
		})
	}
	if input.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Field type is required",
		})
	}

	// Check if field already exists
	existing, _ := h.getMetadataRepo(c).GetField(c.Context(), orgID, entityName, input.Name)
	if existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Field with this name already exists",
		})
	}

	// Validate rollup fields
	if input.Type == entity.FieldTypeRollup {
		if input.RollupQuery == nil || *input.RollupQuery == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "SQL query is required for rollup fields",
			})
		}
		if input.RollupResultType == nil || (*input.RollupResultType != "numeric" && *input.RollupResultType != "text") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Result type must be 'numeric' or 'text' for rollup fields",
			})
		}
		// Validate the SQL query
		rollupSvc := service.NewRollupService(h.db)
		if err := rollupSvc.ValidateRollupQuery(*input.RollupQuery); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	field, err := h.getMetadataRepo(c).CreateField(c.Context(), orgID, entityName, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Add columns to the entity table if it exists (for custom entities)
	// This ensures lookup fields work even on existing entities
	if err := h.addFieldColumnsToTable(c, entityName, input); err != nil {
		// Log the error - this is critical for debugging "no such column" issues
		log.Printf("WARNING: Failed to add column for field %s.%s: %v - column may need manual sync", entityName, input.Name, err)
	}

	return c.Status(fiber.StatusCreated).JSON(field)
}

// addFieldColumnsToTable adds the necessary columns to the entity table for a new field
func (h *AdminHandler) addFieldColumnsToTable(c *fiber.Ctx, entityName string, input entity.FieldDefCreateInput) error {
	tableName := getTableNameAdmin(entityName)
	db := h.getDB(c)

	// Check if table exists
	var count int
	err := db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if err != nil || count == 0 {
		return nil // Table doesn't exist yet, columns will be created when table is created
	}

	// For lookup fields, add both _id and _name columns
	if input.Type == entity.FieldTypeLink {
		snakeName := camelToSnakeAdmin(input.Name)

		// Add {field_name}_id column
		idColSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT",
			tableName, quoteIdentifierAdmin(snakeName+"_id"))
		if _, err := db.ExecContext(c.Context(), idColSQL); err != nil {
			// Column might already exist, ignore error
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}

		// Add {field_name}_name column
		nameColSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''",
			tableName, quoteIdentifierAdmin(snakeName+"_name"))
		if _, err := db.ExecContext(c.Context(), nameColSQL); err != nil {
			// Column might already exist, ignore error
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}

		return nil
	}

	// For stream fields, add both entry and _log columns
	if input.Type == entity.FieldTypeStream {
		snakeName := camelToSnakeAdmin(input.Name)

		// Add {field_name} column for entry (current input)
		entryColSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''",
			tableName, quoteIdentifierAdmin(snakeName))
		if _, err := db.ExecContext(c.Context(), entryColSQL); err != nil {
			// Column might already exist, ignore error
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}

		// Add {field_name}_log column for timestamped history
		logColSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''",
			tableName, quoteIdentifierAdmin(snakeName+"_log"))
		if _, err := db.ExecContext(c.Context(), logColSQL); err != nil {
			// Column might already exist, ignore error
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}

		return nil
	}

	// For regular fields, add single column
	snakeName := camelToSnakeAdmin(input.Name)
	colType := "TEXT"
	switch input.Type {
	case entity.FieldTypeInt:
		colType = "INTEGER"
	case entity.FieldTypeFloat, entity.FieldTypeCurrency:
		colType = "REAL"
	case entity.FieldTypeBool:
		colType = "INTEGER"
	}

	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		tableName, quoteIdentifierAdmin(snakeName), colType)
	if _, err := db.ExecContext(c.Context(), alterSQL); err != nil {
		// Column might already exist, ignore error
		if !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}

	return nil
}

// UpdateField updates a field definition
func (h *AdminHandler) UpdateField(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	fieldName, _ := url.PathUnescape(c.Params("field"))

	var input entity.FieldDefUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	field, err := h.getMetadataRepo(c).UpdateField(c.Context(), orgID, entityName, fieldName, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if field == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.JSON(field)
}

// DeleteField deletes a field definition
func (h *AdminHandler) DeleteField(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	fieldName, _ := url.PathUnescape(c.Params("field"))

	err := h.getMetadataRepo(c).DeleteField(c.Context(), orgID, entityName, fieldName)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ReorderFields updates the sort order of fields
func (h *AdminHandler) ReorderFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	var input struct {
		FieldOrder []string `json:"fieldOrder"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	err := h.getMetadataRepo(c).ReorderFields(c.Context(), orgID, entityName, input.FieldOrder)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// --- Layouts ---

// GetLayout returns a layout definition
func (h *AdminHandler) GetLayout(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	layout, err := h.getMetadataRepo(c).GetLayout(c.Context(), orgID, entityName, layoutType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if layout == nil {
		// Return empty layout with exists=false if not found
		return c.JSON(fiber.Map{
			"entityName": entityName,
			"layoutType": layoutType,
			"layoutData": "[]",
			"exists":     false,
		})
	}

	return c.JSON(fiber.Map{
		"id":         layout.ID,
		"entityName": layout.EntityName,
		"layoutType": layout.LayoutType,
		"layoutData": layout.LayoutData,
		"createdAt":  layout.CreatedAt,
		"modifiedAt": layout.ModifiedAt,
		"exists":     true,
	})
}

// SaveLayout creates or updates a layout definition
func (h *AdminHandler) SaveLayout(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	var input struct {
		LayoutData string `json:"layoutData"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	layout, err := h.getMetadataRepo(c).SaveLayout(c.Context(), orgID, entityName, layoutType, input.LayoutData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(layout)
}

// GetLayoutV2 returns a layout definition in v2 format (converting v1 if necessary)
func (h *AdminHandler) GetLayoutV2(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	layout, err := h.getMetadataRepo(c).GetLayout(c.Context(), orgID, entityName, layoutType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var layoutJSON string
	if layout == nil {
		layoutJSON = "[]"
	} else {
		layoutJSON = layout.LayoutData
	}

	// Parse and convert to v2 if needed
	layoutV2, err := h.layoutService.GetLayoutAsV2(layoutJSON)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"entityName": entityName,
		"layoutType": layoutType,
		"layout":     layoutV2,
		"exists":     layout != nil,
	})
}

// SaveLayoutV2 saves a layout in v2 format
func (h *AdminHandler) SaveLayoutV2(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	var input entity.LayoutDataV2
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Ensure version is set
	input.Version = entity.LayoutVersionV2

	// Serialize to JSON
	layoutJSON, err := json.Marshal(input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to serialize layout",
		})
	}

	layout, err := h.getMetadataRepo(c).SaveLayout(c.Context(), orgID, entityName, layoutType, string(layoutJSON))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"id":         layout.ID,
		"entityName": layout.EntityName,
		"layoutType": layout.LayoutType,
		"layout":     input,
		"createdAt":  layout.CreatedAt,
		"modifiedAt": layout.ModifiedAt,
	})
}

// ReprovisionMetadata re-runs the default metadata provisioning for the current org
// This is useful when an org was created but provisioning failed or was incomplete
// POST /admin/reprovision
func (h *AdminHandler) ReprovisionMetadata(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	if h.provisioningService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Provisioning service not configured",
		})
	}

	// Use tenant database for provisioning (multi-tenant support)
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		h.provisioningService.SetDB(tenantDB)
	}

	// Run provisioning (uses INSERT OR REPLACE so it's safe to re-run)
	if err := h.provisioningService.ProvisionDefaultMetadata(c.Context(), orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to provision metadata: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Metadata provisioned successfully. Default entities, fields, layouts, and navigation have been created.",
	})
}

// RepairAllOrgsMetadata repairs metadata tables for all organizations (admin-only bulk operation)
// This is used to fix corrupted metadata schemas across multiple organizations
// POST /admin/repair-all-orgs
func (h *AdminHandler) RepairAllOrgsMetadata(c *fiber.Ctx) error {
	if h.provisioningService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Provisioning service not configured",
		})
	}

	if h.dbManager == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Database manager not configured",
		})
	}

	ctx := c.Context()

	// Query all active organizations from master database
	query := `
		SELECT id, name, COALESCE(database_url, ''), COALESCE(database_token, '')
		FROM organizations
		WHERE is_active = 1
		ORDER BY created_at ASC
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to query organizations: %v", err),
		})
	}
	defer rows.Close()

	type orgInfo struct {
		ID           string
		Name         string
		DatabaseURL  string
		DatabaseToken string
	}

	var orgs []orgInfo
	for rows.Next() {
		var org orgInfo
		if err := rows.Scan(&org.ID, &org.Name, &org.DatabaseURL, &org.DatabaseToken); err != nil {
			log.Printf("Failed to scan org: %v", err)
			continue
		}
		orgs = append(orgs, org)
	}

	if err = rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error reading organizations: %v", err),
		})
	}

	// Repair each org
	type repairResult struct {
		OrgID   string `json:"orgId"`
		OrgName string `json:"orgName"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	var results []repairResult

	for _, org := range orgs {
		result := repairResult{
			OrgID:   org.ID,
			OrgName: org.Name,
		}

		// Skip orgs without database URL (shouldn't happen for active orgs, but safety check)
		if org.DatabaseURL == "" {
			result.Success = false
			result.Error = "No database URL configured"
			results = append(results, result)
			continue
		}

		// Get tenant database connection using manager
		tenantDB, err := h.dbManager.GetTenantDB(ctx, org.ID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to connect to database: %v", err)
			results = append(results, result)
			log.Printf("Failed to connect to org %s (%s) database: %v", org.ID, org.Name, err)
			continue
		}

		// Set provisioning service to use tenant database
		h.provisioningService.SetDB(tenantDB)

		// Run provisioning (will check and fix schema as needed)
		if err := h.provisioningService.ProvisionDefaultMetadata(ctx, org.ID); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Provisioning failed: %v", err)
			results = append(results, result)
			log.Printf("Failed to provision metadata for org %s (%s): %v", org.ID, org.Name, err)
			continue
		}

		result.Success = true
		results = append(results, result)
		log.Printf("Successfully repaired metadata for org %s (%s)", org.ID, org.Name)
	}

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	return c.JSON(fiber.Map{
		"success":      failureCount == 0,
		"total":        len(results),
		"repaired":     successCount,
		"failed":       failureCount,
		"results":      results,
		"message":      fmt.Sprintf("Repaired %d orgs, %d failed", successCount, failureCount),
	})
}

// RegisterRoutes registers admin routes on the Fiber app
func (h *AdminHandler) RegisterRoutes(app fiber.Router) {
	admin := app.Group("/admin")

	// Field types
	admin.Get("/field-types", h.ListFieldTypes)

	// Re-provisioning (for orgs that may have failed initial provisioning)
	admin.Post("/reprovision", h.ReprovisionMetadata)
	admin.Post("/repair-all-orgs", h.RepairAllOrgsMetadata)

	// Entities
	admin.Get("/entities", h.ListEntities)
	admin.Post("/entities", h.CreateEntity)
	admin.Get("/entities/:name", h.GetEntity)
	admin.Patch("/entities/:name", h.UpdateEntity)
	admin.Delete("/entities/:name", h.DeleteEntity)

	// Fields
	admin.Get("/entities/:entity/fields", h.ListFields)
	admin.Get("/entities/:entity/fields/:field", h.GetField)
	admin.Post("/entities/:entity/fields", h.CreateField)
	admin.Put("/entities/:entity/fields/:field", h.UpdateField)
	admin.Patch("/entities/:entity/fields/:field", h.UpdateField)
	admin.Delete("/entities/:entity/fields/:field", h.DeleteField)
	admin.Post("/entities/:entity/fields/reorder", h.ReorderFields)

	// Layouts (v1 - flat array format)
	admin.Get("/entities/:entity/layouts/:type", h.GetLayout)
	admin.Put("/entities/:entity/layouts/:type", h.SaveLayout)

	// Layouts (v2 - sections with visibility)
	admin.Get("/entities/:entity/layouts/:type/v2", h.GetLayoutV2)
	admin.Put("/entities/:entity/layouts/:type/v2", h.SaveLayoutV2)
}
