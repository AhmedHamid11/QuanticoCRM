package main

import (
	"database/sql"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/fastcrm/backend/internal/migrations"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func main() {
	log.Println("[MIGRATE v2] Starting migration with idempotent ADD COLUMN support")
	var db *sql.DB
	var err error

	// Check for Turso connection (production)
	tursoURL := os.Getenv("TURSO_URL")
	tursoToken := os.Getenv("TURSO_AUTH_TOKEN")

	if tursoURL != "" && tursoToken != "" {
		// Production: Connect to Turso
		connStr := tursoURL + "?authToken=" + tursoToken
		db, err = sql.Open("libsql", connStr)
		if err != nil {
			log.Fatalf("Failed to connect to Turso: %v", err)
		}
		log.Printf("Connected to Turso: %s", tursoURL)
	} else {
		// Development: Use local SQLite
		dbPath := os.Getenv("DATABASE_PATH")
		if dbPath == "" {
			dbPath = "../fastcrm.db"
		}
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		log.Printf("Using local database: %s", dbPath)
	}
	defer db.Close()

	// Create migrations tracking table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		if isTursoPlanLimitError(err) {
			log.Printf("[MIGRATE] WARNING: Turso plan limit reached — operations are blocked. Skipping migrations. App will start without them.")
			return
		}
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	// Get list of applied migrations
	applied := make(map[string]bool)
	rows, err := db.Query("SELECT name FROM _migrations")
	if err != nil {
		if isTursoPlanLimitError(err) {
			log.Printf("[MIGRATE] WARNING: Turso plan limit reached — reads are blocked. Skipping migrations. App will start without them.")
			return
		}
		log.Fatalf("Failed to query migrations: %v", err)
	}
	for rows.Next() {
		var name string
		rows.Scan(&name)
		applied[name] = true
	}
	rows.Close()

	// Get list of embedded migration files
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		log.Fatalf("Failed to read embedded migrations: %v", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	log.Printf("Found %d embedded migrations", len(files))

	if len(files) == 0 {
		log.Println("No migration files found")
		return
	}

	// Run pending migrations
	pending := 0
	for _, name := range files {
		if applied[name] {
			continue
		}

		log.Printf("Applying migration: %s", name)

		// Read embedded migration file
		content, err := fs.ReadFile(migrations.Files, name)
		if err != nil {
			log.Fatalf("Failed to read migration %s: %v", name, err)
		}

		// Split by semicolons and execute each statement
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			// Strip comment lines from beginning of statement
			// (comments may precede actual SQL in the same statement block)
			stmt = stripLeadingComments(stmt)
			if stmt == "" {
				continue
			}
			_, err = db.Exec(stmt)
			if err != nil {
				// Handle idempotent ALTER TABLE ADD COLUMN - skip if column already exists
				if isAddColumnStatement(stmt) && isDuplicateColumnError(err) {
					log.Printf("[MIGRATE v2] Column already exists (safe to skip): %s", stmt)
					continue
				}
				// Handle statements on non-existent tables (table may be org-specific, created per-tenant)
				// This is safe because tenant-specific tables are created during provisioning
				if isTableNotExistsError(err) {
					log.Printf("[MIGRATE v2] Table does not exist in master DB (safe to skip - tenant-specific): %s", stmt)
					continue
				}
				log.Fatalf("Failed to execute migration %s: %v\nStatement: %s", name, err, stmt)
			}
		}

		// Record migration as applied
		_, err = db.Exec("INSERT INTO _migrations (name) VALUES (?)", name)
		if err != nil {
			log.Fatalf("Failed to record migration %s: %v", name, err)
		}

		log.Printf("Applied: %s", name)
		pending++
	}

	if pending == 0 {
		log.Println("No pending migrations")
	} else {
		log.Printf("Applied %d migration(s)", pending)
	}
}

// stripLeadingComments removes SQL comment lines from the beginning of a statement
func stripLeadingComments(stmt string) string {
	lines := strings.Split(stmt, "\n")
	var result []string
	foundSQL := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !foundSQL && strings.HasPrefix(trimmed, "--") {
			// Skip leading comment lines
			continue
		}
		foundSQL = true
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// isAddColumnStatement checks if a SQL statement is an ALTER TABLE ADD COLUMN
func isAddColumnStatement(stmt string) bool {
	upper := strings.ToUpper(stmt)
	return strings.Contains(upper, "ALTER TABLE") && strings.Contains(upper, "ADD COLUMN")
}

// isDuplicateColumnError checks if error is about a column already existing
func isDuplicateColumnError(err error) bool {
	errStr := strings.ToLower(err.Error())
	// Check various formats the error might appear in
	return strings.Contains(errStr, "duplicate column") ||
		strings.Contains(errStr, "column already exists") ||
		strings.Contains(errStr, "already has a column named")
}

// isTableNotExistsError checks if error is about a missing table
func isTableNotExistsError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "table does not exist") ||
		strings.Contains(errStr, "table not found")
}

// isTursoPlanLimitError detects when Turso blocks operations due to plan limits
func isTursoPlanLimitError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "operations are forbidden") ||
		strings.Contains(errStr, "reads are blocked") ||
		strings.Contains(errStr, "writes are blocked") ||
		strings.Contains(errStr, "need to upgrade your plan")
}
