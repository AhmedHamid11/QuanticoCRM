package middleware

import (
	"database/sql"
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// Context keys for database connections
const (
	// DBConnKey is for the retry-enabled DBConn interface (preferred)
	DBConnKey = "dbConn"
	// DBKey is for raw *sql.DB (backward compatibility)
	DBKey = "db"
)

// dbConnWrapper wraps a DBConn but does NOT implement io.Closer
// This prevents fasthttp from calling Close() on shared database connections
// when the request ends. The underlying connections are managed by the db.Manager.
type dbConnWrapper struct {
	conn db.DBConn
}

// sqlDBWrapper wraps a *sql.DB but does NOT implement io.Closer
// This prevents fasthttp from calling Close() on shared database connections
type sqlDBWrapper struct {
	db *sql.DB
}

// TenantMiddleware resolves tenant database connections
type TenantMiddleware struct {
	dbManager *db.Manager
	authRepo  *repo.AuthRepo
}

// NewTenantMiddleware creates a new tenant middleware
func NewTenantMiddleware(dbManager *db.Manager, authRepo *repo.AuthRepo) *TenantMiddleware {
	return &TenantMiddleware{
		dbManager: dbManager,
		authRepo:  authRepo,
	}
}

// ResolveTenant returns middleware that resolves the tenant database connection
// This should be used AFTER the auth middleware has set the orgID in context
func (m *TenantMiddleware) ResolveTenant() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get orgID from context (set by auth middleware)
		orgIDVal := c.Locals("orgID")
		if orgIDVal == nil {
			// No org context - might be a platform-level request
			// Set master DB for these cases
			masterDB := m.dbManager.GetMasterDB()
			if masterDB == nil {
				log.Printf("[TENANT-MW] CRITICAL: GetMasterDB() returned nil")
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"error": "Database service unavailable",
				})
			}
			// Use wrappers to prevent fasthttp from closing shared connections
			c.Locals(DBKey, &sqlDBWrapper{db: masterDB})
			c.Locals(DBConnKey, &dbConnWrapper{conn: masterDB})
			return c.Next()
		}

		orgID, ok := orgIDVal.(string)
		if !ok || orgID == "" {
			masterDB := m.dbManager.GetMasterDB()
			if masterDB == nil {
				log.Printf("[TENANT-MW] CRITICAL: GetMasterDB() returned nil (empty orgID path)")
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"error": "Database service unavailable",
				})
			}
			c.Locals(DBKey, &sqlDBWrapper{db: masterDB})
			c.Locals(DBConnKey, &dbConnWrapper{conn: masterDB})
			return c.Next()
		}

		// In local mode, use shared database (existing behavior)
		if m.dbManager.IsLocalMode() {
			tenantDB, err := m.dbManager.GetTenantDB(c.Context(), orgID, "", "")
			if err != nil {
				log.Printf("[TENANT-MW] Org=%s Error getting local tenant DB: %v", orgID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Database connection error",
				})
			}
			c.Locals(DBKey, &sqlDBWrapper{db: tenantDB})
			c.Locals(DBConnKey, &dbConnWrapper{conn: tenantDB})
			log.Printf("[TENANT-MW] Org=%s Local mode connection resolved", orgID)
			return c.Next()
		}

		// Production mode: Look up org's database URL from master DB
		org, err := m.authRepo.GetOrganizationByID(c.Context(), orgID)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Organization not found",
				})
			}
			log.Printf("[TENANT-MW] Org=%s Error fetching organization: %v", orgID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to resolve organization",
			})
		}

		if org == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Organization not found",
			})
		}

		if !org.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization is deactivated",
			})
		}

		// Get tenant database connection
		if org.DatabaseURL == "" {
			log.Printf("[TENANT-MW] Org=%s ERROR: No database URL configured, rejecting request", orgID)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Organization database not provisioned. Please contact support.",
			})
		}

		// Use GetTenantDBConn for retry-enabled connection (preferred)
		tenantDBConn, err := m.dbManager.GetTenantDBConn(c.Context(), orgID, org.DatabaseURL, org.DatabaseToken)
		if err != nil {
			log.Printf("[TENANT-MW] Org=%s Error connecting to tenant database: %v", orgID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database connection error",
			})
		}

		// Set both connection types in context for handlers
		// Use wrappers to prevent fasthttp from closing shared connections at request end
		// DBConnKey: retry-enabled interface (preferred for new code)
		c.Locals(DBConnKey, &dbConnWrapper{conn: tenantDBConn})

		// DBKey: raw *sql.DB for backward compatibility
		// Extract raw DB from TenantDB wrapper if available
		if tenantDB, ok := tenantDBConn.(*db.TenantDB); ok {
			rawDB := tenantDB.GetDB()
			if rawDB != nil {
				c.Locals(DBKey, &sqlDBWrapper{db: rawDB})
			}
		} else if rawDB, ok := tenantDBConn.(*sql.DB); ok {
			c.Locals(DBKey, &sqlDBWrapper{db: rawDB})
		}

		log.Printf("[TENANT-MW] Org=%s Tenant connection resolved with retry wrapper", orgID)
		return c.Next()
	}
}

// GetTenantDB is a helper to get the tenant DB from context (raw *sql.DB)
// DEPRECATED: Use GetTenantDBConn for retry-enabled connections
// Handlers should use this instead of a global db reference
func GetTenantDB(c *fiber.Ctx) *sql.DB {
	// Try wrapped value first (new approach - prevents fasthttp from closing)
	if wrapper, ok := c.Locals(DBKey).(*sqlDBWrapper); ok && wrapper != nil {
		return wrapper.db
	}
	// Try raw *sql.DB (legacy, kept for backward compatibility)
	if rawDB, ok := c.Locals(DBKey).(*sql.DB); ok {
		return rawDB
	}
	// Fall back to legacy key
	if rawDB, ok := c.Locals("db").(*sql.DB); ok {
		return rawDB
	}
	return nil
}

// GetTenantDBConn is a helper to get the retry-enabled tenant DB connection from context
// This is the preferred method for new code as it provides automatic retry on connection errors
func GetTenantDBConn(c *fiber.Ctx) db.DBConn {
	// Try wrapped value first (new approach - prevents fasthttp from closing)
	if wrapper, ok := c.Locals(DBConnKey).(*dbConnWrapper); ok && wrapper != nil {
		return wrapper.conn
	}
	// Try raw DBConn (legacy)
	if dbConn, ok := c.Locals(DBConnKey).(db.DBConn); ok {
		return dbConn
	}
	// Fall back to raw *sql.DB (which implements DBConn)
	if rawDB := GetTenantDB(c); rawDB != nil {
		return rawDB
	}
	return nil
}
