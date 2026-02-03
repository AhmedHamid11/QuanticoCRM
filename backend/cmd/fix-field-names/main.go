package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// This script fixes field name mismatches between field definitions and API JSON keys.
// The issue was: field defs used "phone"/"email" but API returns "phoneNumber"/"emailAddress"

func main() {
	// Load environment
	godotenv.Load()
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Connect to database
	var db *sql.DB
	var err error

	tursoURL := os.Getenv("TURSO_URL")
	tursoToken := os.Getenv("TURSO_AUTH_TOKEN")

	if tursoURL != "" && tursoToken != "" {
		connStr := tursoURL + "?authToken=" + tursoToken
		db, err = sql.Open("libsql", connStr)
		if err != nil {
			log.Fatalf("Failed to connect to Turso: %v", err)
		}
		log.Println("Connected to Turso master database")
	} else {
		dbPath := os.Getenv("DATABASE_PATH")
		if dbPath == "" {
			dbPath = "../fastcrm.db"
		}
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatalf("Failed to connect to local database: %v", err)
		}
		log.Printf("Connected to local database: %s", dbPath)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	// Fix field definitions
	log.Println("Fixing field definitions...")

	// Contact: email -> emailAddress
	updateFieldName(ctx, db, "Contact", "email", "emailAddress")
	// Contact: phone -> phoneNumber
	updateFieldName(ctx, db, "Contact", "phone", "phoneNumber")
	// Account: phone -> phoneNumber
	updateFieldName(ctx, db, "Account", "phone", "phoneNumber")

	// Fix layouts
	log.Println("Fixing layouts...")

	// Contact list layout
	updateLayoutField(ctx, db, "Contact", "list", `"email"`, `"emailAddress"`)
	updateLayoutField(ctx, db, "Contact", "list", `"phone"`, `"phoneNumber"`)
	// Contact detail layout
	updateLayoutField(ctx, db, "Contact", "detail", `"email"`, `"emailAddress"`)
	updateLayoutField(ctx, db, "Contact", "detail", `"phone"`, `"phoneNumber"`)
	// Account list layout
	updateLayoutField(ctx, db, "Account", "list", `"phone"`, `"phoneNumber"`)
	// Account detail layout
	updateLayoutField(ctx, db, "Account", "detail", `"phone"`, `"phoneNumber"`)

	log.Println("Done! Field names and layouts have been updated.")
}

func updateFieldName(ctx context.Context, db *sql.DB, entity, oldName, newName string) {
	result, err := db.ExecContext(ctx,
		`UPDATE field_defs SET name = ?, modified_at = ? WHERE entity_name = ? AND name = ?`,
		newName, time.Now().UTC().Format(time.RFC3339), entity, oldName)
	if err != nil {
		log.Printf("Error updating %s.%s: %v", entity, oldName, err)
		return
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("Updated %d field(s): %s.%s -> %s.%s", rows, entity, oldName, entity, newName)
	}
}

func updateLayoutField(ctx context.Context, db *sql.DB, entity, layoutType, oldField, newField string) {
	// Get current layout
	var layoutData string
	err := db.QueryRowContext(ctx,
		`SELECT layout_data FROM layout_defs WHERE entity_name = ? AND layout_type = ?`,
		entity, layoutType).Scan(&layoutData)
	if err != nil {
		if err == sql.ErrNoRows {
			return // No layout to update
		}
		log.Printf("Error reading %s %s layout: %v", entity, layoutType, err)
		return
	}

	// Replace old field name with new
	newLayoutData := replaceAll(layoutData, oldField, newField)
	if newLayoutData == layoutData {
		return // No changes needed
	}

	// Update layout
	_, err = db.ExecContext(ctx,
		`UPDATE layout_defs SET layout_data = ?, modified_at = ? WHERE entity_name = ? AND layout_type = ?`,
		newLayoutData, time.Now().UTC().Format(time.RFC3339), entity, layoutType)
	if err != nil {
		log.Printf("Error updating %s %s layout: %v", entity, layoutType, err)
		return
	}
	log.Printf("Updated %s %s layout: %s -> %s", entity, layoutType, oldField, newField)
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
