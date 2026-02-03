package repo

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

// ContextKey for storing DB in fiber context
const DBContextKey = "db"

// GetDBFromContext retrieves the tenant database from fiber context
// Falls back to the provided default DB if not found in context
func GetDBFromContext(c *fiber.Ctx, defaultDB *sql.DB) *sql.DB {
	if c == nil {
		return defaultDB
	}

	dbVal := c.Locals(DBContextKey)
	if dbVal == nil {
		return defaultDB
	}

	db, ok := dbVal.(*sql.DB)
	if !ok || db == nil {
		return defaultDB
	}

	return db
}

// DBProvider allows repos to get DB from context or use default
type DBProvider struct {
	defaultDB *sql.DB
}

// NewDBProvider creates a new DBProvider with a default database
func NewDBProvider(defaultDB *sql.DB) *DBProvider {
	return &DBProvider{defaultDB: defaultDB}
}

// GetDB returns the database from fiber context or the default
func (p *DBProvider) GetDB(c *fiber.Ctx) *sql.DB {
	return GetDBFromContext(c, p.defaultDB)
}

// GetDefault returns the default database
func (p *DBProvider) GetDefault() *sql.DB {
	return p.defaultDB
}
