package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Connect to the database
	db, err := sql.Open("sqlite3", "/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/fastcrm.db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Generate IDs
	orgID := sfid.NewOrg()
	userID := sfid.NewUser()
	membershipID := sfid.NewMembership()

	log.Printf("Creating Guardare Operations with org_id: %s", orgID)

	// 1. Create the Guardare Operations organization
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
		INSERT INTO organizations (id, name, slug, plan, is_active, settings, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, orgID, "Guardare Operations", "guardare-operations", "pro", 1, "{}", now, now)
	if err != nil {
		log.Fatal("Failed to create organization:", err)
	}
	log.Println("Created organization: Guardare Operations")

	// 2. Create an admin user for this org
	// Generate bcrypt hash for "guardare2024!"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("guardare2024!"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, is_platform_admin, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, "admin@guardare.com", string(passwordHash), "Admin", "User", 1, 0, now, now)
	if err != nil {
		log.Fatal("Failed to create user:", err)
	}
	log.Println("Created user: admin@guardare.com (password: guardare2024!)")

	// 3. Create membership linking user to org
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_org_memberships (id, user_id, org_id, role, is_default, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, membershipID, userID, orgID, "owner", 1, now)
	if err != nil {
		log.Fatal("Failed to create membership:", err)
	}
	log.Println("Created membership for user in Guardare Operations")

	// 4. Create navigation tab for Home
	createNavigationTab(ctx, db, orgID, "Home", "/", "", 0)
	log.Println("Created Home navigation tab")

	log.Println("Seed completed successfully!")
	log.Printf("\n=== Login Credentials ===")
	log.Printf("Email: admin@guardare.com")
	log.Printf("Password: guardare2024!")
	log.Printf("Org ID: %s", orgID)
	log.Printf("User ID: %s", userID)
	log.Printf("=========================\n")
	log.Println("\nNext steps:")
	log.Println("1. Login to the application")
	log.Println("2. Use the API to create the Case entity and fields")
}

func createNavigationTab(ctx context.Context, db *sql.DB, orgID, label, href, entityName string, sortOrder int) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := sfid.New("0Nt")

	_, err := db.ExecContext(ctx, `
		INSERT INTO navigation_tabs (id, org_id, label, href, icon, entity_name, sort_order, is_visible, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, orgID, label, href, "", entityName, sortOrder, 1, now, now)
	if err != nil {
		log.Printf("Warning: Failed to create navigation tab %s: %v", label, err)
	}
}
