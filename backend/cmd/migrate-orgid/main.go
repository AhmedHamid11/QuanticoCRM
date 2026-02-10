package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// This migration script adds org_id column to any custom entity tables
// that were created before the multi-tenant security fix.
//
// Custom entity tables are identified by:
// 1. Not being a known system table
// 2. Having an 'id' column but no 'org_id' column
//
// Usage: go run cmd/migrate-orgid/main.go

// Known system tables that should not be modified
var systemTables = map[string]bool{
	"_migrations":           true,
	"organizations":         true,
	"users":                 true,
	"user_org_memberships":  true,
	"sessions":              true,
	"org_invitations":       true,
	"sqlite_sequence":       true,
}

// Tables that already have proper org_id handling
var knownMultiTenantTables = map[string]bool{
	"contacts":             true,
	"accounts":             true,
	"tasks":                true,
	"entity_defs":          true,
	"field_defs":           true,
	"layout_configs":       true,
	"navigation_config":    true,
	"related_list_configs": true,
	"tripwire_rules":       true,
	"bearing_configs":      true,
	"validation_rules":     true,
}

func main() {
	// Get database path from environment or use default
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "../fastcrm.db"
	}

	log.Printf("=== Custom Entity org_id Migration ===")
	log.Printf("Database: %s", dbPath)

	// Connect to database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Find all tables
	tables, err := getAllTables(db)
	if err != nil {
		log.Fatalf("Failed to get tables: %v", err)
	}

	log.Printf("Found %d total tables", len(tables))

	// Find custom entity tables missing org_id
	tablesNeedingOrgID := []string{}
	for _, table := range tables {
		if systemTables[table] || knownMultiTenantTables[table] {
			continue
		}

		hasOrgID, err := tableHasColumn(db, table, "org_id")
		if err != nil {
			log.Printf("Warning: Could not check table %s: %v", table, err)
			continue
		}

		if !hasOrgID {
			// Check if it has an 'id' column (likely a custom entity)
			hasID, _ := tableHasColumn(db, table, "id")
			if hasID {
				tablesNeedingOrgID = append(tablesNeedingOrgID, table)
			}
		}
	}

	if len(tablesNeedingOrgID) == 0 {
		log.Println("No custom entity tables need org_id migration")
		return
	}

	log.Printf("Found %d custom entity tables needing org_id:", len(tablesNeedingOrgID))
	for _, table := range tablesNeedingOrgID {
		log.Printf("  - %s", table)
	}

	// Get the default org_id to use for existing records
	defaultOrgID, err := getDefaultOrgID(db)
	if err != nil {
		log.Fatalf("Failed to get default org_id: %v", err)
	}
	log.Printf("Using default org_id for existing records: %s", defaultOrgID)

	// Migrate each table
	for _, table := range tablesNeedingOrgID {
		if err := migrateTable(db, table, defaultOrgID); err != nil {
			log.Fatalf("Failed to migrate table %s: %v", table, err)
		}
		log.Printf("Migrated: %s", table)
	}

	log.Printf("=== Migration Complete ===")
	log.Printf("Migrated %d tables", len(tablesNeedingOrgID))
}

func getAllTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	var count int
	err := db.QueryRow(query, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func getDefaultOrgID(db *sql.DB) (string, error) {
	// Try to get the first org from organizations table
	var orgID string
	err := db.QueryRow("SELECT id FROM organizations ORDER BY created_at LIMIT 1").Scan(&orgID)
	if err == nil && orgID != "" {
		return orgID, nil
	}

	// Try to get from an existing multi-tenant table
	for table := range knownMultiTenantTables {
		var id string
		err := db.QueryRow(fmt.Sprintf("SELECT org_id FROM %s LIMIT 1", table)).Scan(&id)
		if err == nil && id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("no default org_id found - please create an organization first")
}

func migrateTable(db *sql.DB, table, defaultOrgID string) error {
	// SQLite doesn't support adding NOT NULL columns with data already present
	// So we need to:
	// 1. Add nullable org_id column
	// 2. Update all rows with default org_id
	// 3. We can't change to NOT NULL easily in SQLite, but the code will enforce it

	// Step 1: Add org_id column
	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN org_id TEXT", table)
	if _, err := db.Exec(alterSQL); err != nil {
		if strings.Contains(err.Error(), "duplicate column") {
			log.Printf("  Column org_id already exists in %s, skipping add", table)
		} else {
			return fmt.Errorf("failed to add org_id column: %w", err)
		}
	}

	// Step 2: Update existing rows with default org_id
	updateSQL := fmt.Sprintf("UPDATE %s SET org_id = ? WHERE org_id IS NULL", table)
	result, err := db.Exec(updateSQL, defaultOrgID)
	if err != nil {
		return fmt.Errorf("failed to update org_id values: %w", err)
	}
	rows, _ := result.RowsAffected()
	log.Printf("  Updated %d rows in %s with org_id", rows, table)

	// Step 3: Create index on org_id
	indexSQL := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_org_id ON %s(org_id)", table, table)
	if _, err := db.Exec(indexSQL); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}
