// One-time migration script to update existing UUIDs to Salesforce-style IDs
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		// Default to project root (parent of backend directory)
		dbPath = "../fastcrm.db"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Migrate contacts
	if err := migrateContacts(db); err != nil {
		log.Fatalf("Failed to migrate contacts: %v", err)
	}

	// Migrate field_defs
	if err := migrateFieldDefs(db); err != nil {
		log.Fatalf("Failed to migrate field_defs: %v", err)
	}

	// Migrate layout_defs
	if err := migrateLayoutDefs(db); err != nil {
		log.Fatalf("Failed to migrate layout_defs: %v", err)
	}

	fmt.Println("Migration completed successfully!")
}

func migrateContacts(db *sql.DB) error {
	// Get all contacts with UUID-style IDs (contain hyphens)
	rows, err := db.Query("SELECT id FROM contacts WHERE id LIKE '%-%'")
	if err != nil {
		return fmt.Errorf("failed to query contacts: %w", err)
	}
	defer rows.Close()

	var oldIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan id: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}

	if len(oldIDs) == 0 {
		fmt.Println("No contacts to migrate")
		return nil
	}

	fmt.Printf("Migrating %d contacts...\n", len(oldIDs))

	for _, oldID := range oldIDs {
		newID := sfid.NewContact()
		_, err := db.Exec("UPDATE contacts SET id = ? WHERE id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("failed to update contact %s: %w", oldID, err)
		}
		fmt.Printf("  %s -> %s\n", oldID, newID)
	}

	fmt.Printf("Migrated %d contacts\n", len(oldIDs))
	return nil
}

func migrateFieldDefs(db *sql.DB) error {
	rows, err := db.Query("SELECT id FROM field_defs WHERE id LIKE '%-%'")
	if err != nil {
		return fmt.Errorf("failed to query field_defs: %w", err)
	}
	defer rows.Close()

	var oldIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan id: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}

	if len(oldIDs) == 0 {
		fmt.Println("No field_defs to migrate")
		return nil
	}

	fmt.Printf("Migrating %d field_defs...\n", len(oldIDs))

	for _, oldID := range oldIDs {
		newID := sfid.NewFieldDef()
		_, err := db.Exec("UPDATE field_defs SET id = ? WHERE id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("failed to update field_def %s: %w", oldID, err)
		}
		fmt.Printf("  %s -> %s\n", oldID, newID)
	}

	fmt.Printf("Migrated %d field_defs\n", len(oldIDs))
	return nil
}

func migrateLayoutDefs(db *sql.DB) error {
	rows, err := db.Query("SELECT id FROM layout_defs WHERE id LIKE '%-%'")
	if err != nil {
		return fmt.Errorf("failed to query layout_defs: %w", err)
	}
	defer rows.Close()

	var oldIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan id: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}

	if len(oldIDs) == 0 {
		fmt.Println("No layout_defs to migrate")
		return nil
	}

	fmt.Printf("Migrating %d layout_defs...\n", len(oldIDs))

	for _, oldID := range oldIDs {
		newID := sfid.NewLayout()
		_, err := db.Exec("UPDATE layout_defs SET id = ? WHERE id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("failed to update layout_def %s: %w", oldID, err)
		}
		fmt.Printf("  %s -> %s\n", oldID, newID)
	}

	fmt.Printf("Migrated %d layout_defs\n", len(oldIDs))
	return nil
}
