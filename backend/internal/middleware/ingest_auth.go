package middleware

import (
	"database/sql"
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// IngestAuthMiddleware validates X-API-Key headers for ingest endpoint
type IngestAuthMiddleware struct {
	apiKeyService *service.IngestAPIKeyService
	dbManager     *db.Manager
	authRepo      *repo.AuthRepo
}

// NewIngestAuthMiddleware creates a new IngestAuthMiddleware
func NewIngestAuthMiddleware(apiKeyService *service.IngestAPIKeyService, dbManager *db.Manager, authRepo *repo.AuthRepo) *IngestAuthMiddleware {
	return &IngestAuthMiddleware{
		apiKeyService: apiKeyService,
		dbManager:     dbManager,
		authRepo:      authRepo,
	}
}

// Authenticate returns middleware that authenticates via X-API-Key header
// This is a separate auth path from JWT auth - used exclusively for external ingest
func (m *IngestAuthMiddleware) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract API key from X-API-Key header (NOT Authorization header)
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "X-API-Key header required",
			})
		}

		// Validate the API key
		ingestKey, err := m.apiKeyService.ValidateKey(c.Context(), apiKey)
		if err != nil {
			log.Printf("[INGEST-AUTH] Invalid API key: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or inactive API key",
			})
		}

		// Look up the organization to verify it exists and is active
		org, err := m.authRepo.GetOrganizationByID(c.Context(), ingestKey.OrgID)
		if err != nil {
			log.Printf("[INGEST-AUTH] Error looking up org %s: %v", ingestKey.OrgID, err)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization not found or inactive",
			})
		}

		if !org.IsActive {
			log.Printf("[INGEST-AUTH] Inactive org %s attempted ingest", ingestKey.OrgID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization not found or inactive",
			})
		}

		// Resolve tenant database connection
		if m.dbManager.IsLocalMode() {
			// Local mode: use shared database
			tenantDB, err := m.dbManager.GetTenantDB(c.Context(), ingestKey.OrgID, "", "")
			if err != nil {
				log.Printf("[INGEST-AUTH] Error getting local tenant DB for org %s: %v", ingestKey.OrgID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Database connection error",
				})
			}
			// Set DB in context using wrappers
			c.Locals(DBKey, &sqlDBWrapper{db: tenantDB})
			c.Locals(DBConnKey, &dbConnWrapper{conn: tenantDB})
		} else {
			// Production mode: use org's dedicated database
			tenantDBConn, err := m.dbManager.GetTenantDBConn(c.Context(), ingestKey.OrgID, org.DatabaseURL, org.DatabaseToken)
			if err != nil {
				log.Printf("[INGEST-AUTH] Error getting tenant DB for org %s: %v", ingestKey.OrgID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Database connection error",
				})
			}
			// Set DBConn in context
			c.Locals(DBConnKey, &dbConnWrapper{conn: tenantDBConn})
			// Extract raw *sql.DB for backward compatibility
			if tenantDB, ok := tenantDBConn.(*db.TenantDB); ok {
				rawDB := tenantDB.GetDB()
				if rawDB != nil {
					c.Locals(DBKey, &sqlDBWrapper{db: rawDB})
				}
			} else if rawDB, ok := tenantDBConn.(*sql.DB); ok {
				c.Locals(DBKey, &sqlDBWrapper{db: rawDB})
			}
		}

		// Set context locals for the handler
		c.Locals("ingestOrgID", ingestKey.OrgID)
		c.Locals("ingestKeyID", ingestKey.ID)
		c.Locals("ingestRateLimit", ingestKey.RateLimit)

		log.Printf("[INGEST-AUTH] Authenticated org=%s key=%s", ingestKey.OrgID, ingestKey.ID)

		return c.Next()
	}
}
