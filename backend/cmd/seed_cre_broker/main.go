package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Command-line flags
	orgID := flag.String("org-id", "", "Organization ID to provision (optional — creates new org if not provided)")
	dbPath := flag.String("db", "", "Path to SQLite database (defaults to searching common paths)")
	createTables := flag.Bool("create-tables", true, "Create data tables for CRE entities (leads, properties, deals)")
	withSampleData := flag.Bool("sample-data", true, "Include sample CRE brokerage data")
	flag.Parse()

	// Determine database path
	dbFile := *dbPath
	if dbFile == "" {
		paths := []string{
			"fastcrm.db",
			"../fastcrm.db",
			"../../fastcrm.db",
			"/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/fastcrm.db",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				dbFile = p
				break
			}
		}
		if dbFile == "" {
			dbFile = "fastcrm.db"
		}
	}

	log.Printf("Using database: %s", dbFile)

	// Connect to database
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	// Determine org ID — create new org if not provided
	targetOrgID := *orgID
	if targetOrgID == "" {
		targetOrgID = sfid.NewOrg()
		log.Printf("Creating new org with ID: %s", targetOrgID)

		// Create the organization
		_, err = db.ExecContext(ctx, `
			INSERT INTO organizations (id, name, slug, plan, is_active, settings, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, targetOrgID, "Apex CRE Advisors", "apex-cre-advisors", "pro", 1, "{}", now, now)
		if err != nil {
			log.Fatalf("Failed to create organization: %v", err)
		}
		log.Println("Created organization: Apex CRE Advisors")

		// Create admin user
		passwordHash, err := bcrypt.GenerateFromPassword([]byte("apex2024!"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		userID := sfid.NewUser()
		_, err = db.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, is_platform_admin, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, "admin@apexcre.com", string(passwordHash), "Admin", "User", 1, 0, now, now)
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Created user: admin@apexcre.com")

		// Create membership linking user to org
		membershipID := sfid.NewMembership()
		_, err = db.ExecContext(ctx, `
			INSERT INTO user_org_memberships (id, user_id, org_id, role, is_default, joined_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, membershipID, userID, targetOrgID, "owner", 1, now)
		if err != nil {
			log.Fatalf("Failed to create membership: %v", err)
		}
		log.Printf("Created org membership for user")

		log.Printf("\n=== New Org Created ===")
		log.Printf("Org ID: %s", targetOrgID)
		log.Printf("User ID: %s", userID)
		log.Printf("======================\n")
	} else {
		log.Printf("Provisioning existing org: %s", targetOrgID)
	}

	// Create CRE data tables
	if *createTables {
		log.Println("Creating CRE data tables (leads, properties, deals)...")
		if err := createCRETables(ctx, db); err != nil {
			log.Fatalf("Failed to create CRE tables: %v", err)
		}
		log.Println("CRE data tables created successfully")
	}

	// Run default metadata provisioning (Account, Contact, Task, Quote entities)
	provSvc := service.NewProvisioningService(db)
	log.Println("Running default metadata provisioning...")
	if err := provSvc.ProvisionDefaultMetadata(ctx, targetOrgID); err != nil {
		log.Fatalf("Failed to provision default metadata: %v", err)
	}
	log.Println("Default metadata provisioned")

	// Run CRE broker provisioning (Lead, Property, Deal entities + CRE-specific fields/layouts/nav)
	log.Println("Running CRE Broker provisioning...")
	if *withSampleData {
		if err := provSvc.ProvisionCREBrokerComplete(ctx, targetOrgID); err != nil {
			log.Fatalf("Failed to provision CRE Broker: %v", err)
		}
	} else {
		if err := provSvc.ProvisionCREBroker(ctx, targetOrgID); err != nil {
			log.Fatalf("Failed to provision CRE Broker metadata: %v", err)
		}
	}

	log.Println("\n=== CRE Broker Provisioning Complete ===")
	log.Printf("Org ID: %s", targetOrgID)
	log.Println("")
	log.Println("Login credentials:")
	log.Println("  Email:    admin@apexcre.com")
	log.Println("  Password: apex2024!")
	log.Println("")
	log.Println("Entities created:")
	log.Println("  - Account  (Landlord, Tenant, Broker types)")
	log.Println("  - Contact  (linked to accounts)")
	log.Println("  - Property (commercial properties with landlord links)")
	log.Println("  - Lead     (space-seeking tenants in pipeline)")
	log.Println("  - Deal     (lease transactions with commission tracking)")
	log.Println("  - Task     (standard tasks)")
	log.Println("")
	if *withSampleData {
		log.Println("Sample data included:")
		log.Println("  - 10 accounts (3 Landlord, 5 Tenant, 2 Broker)")
		log.Println("  - 15 contacts (1-2 per account with CRE titles)")
		log.Println("  - 6 commercial properties (NYC/NJ/Chicago office + industrial)")
		log.Println("  - 8 leads (office, retail, industrial at various stages)")
		log.Println("  - 8 deals (Pipeline through Closed Won/Lost with commission data)")
		log.Println("")
	}
	log.Println("Next steps:")
	log.Println("  1. Start backend: cd backend && air")
	log.Println("  2. Start frontend: cd frontend && npm run dev")
	log.Println("  3. Login at http://localhost:5173 with admin@apexcre.com / apex2024!")
	log.Println("  4. Navigate: Accounts → Contacts → Properties → Leads → Deals")
}

// createCRETables creates the data tables for CRE entities
func createCRETables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		// Leads table
		`CREATE TABLE IF NOT EXISTS leads (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			company_name TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email_address TEXT,
			phone_number TEXT,
			status TEXT DEFAULT 'New',
			space_type_needed TEXT,
			estimated_sq_ft INTEGER,
			estimated_budget REAL,
			budget_per_sq_ft REAL,
			move_in_timeline TEXT,
			source TEXT,
			assigned_user_id TEXT,
			description TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,

		// Properties table
		`CREATE TABLE IF NOT EXISTS properties (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			address_street TEXT,
			address_city TEXT,
			address_state TEXT,
			address_postal_code TEXT,
			landlord_id TEXT,
			landlord_id_name TEXT,
			primary_contact_id TEXT,
			primary_contact_id_name TEXT,
			total_sq_ft INTEGER,
			available_sq_ft INTEGER,
			status TEXT DEFAULT 'Available',
			asking_price_per_sq_ft REAL,
			availability_date TEXT,
			property_type TEXT,
			description TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,

		// Deals table
		`CREATE TABLE IF NOT EXISTS deals (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			property_id TEXT,
			property_id_name TEXT,
			tenant_id TEXT,
			tenant_id_name TEXT,
			tenant_contact_id TEXT,
			tenant_contact_id_name TEXT,
			landlord_id TEXT,
			landlord_id_name TEXT,
			landlord_contact_id TEXT,
			landlord_contact_id_name TEXT,
			leasing_broker TEXT,
			represents TEXT,
			deal_value REAL,
			sq_footage_leased INTEGER,
			lease_term_length TEXT,
			base_rent REAL,
			commission_pct_landlord REAL,
			commission_amt_landlord REAL,
			commission_pct_tenant REAL,
			commission_amt_tenant REAL,
			total_commission REAL,
			status TEXT DEFAULT 'Pipeline',
			close_date TEXT,
			expected_close_date TEXT,
			deal_type TEXT,
			lease_start_date TEXT,
			lease_end_date TEXT,
			commission_status TEXT DEFAULT 'Not Yet Earned',
			commission_paid_date TEXT,
			notes TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,
	}

	indexes := []string{
		// Leads indexes
		"CREATE INDEX IF NOT EXISTS idx_leads_org ON leads(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_leads_company ON leads(org_id, company_name)",
		// Properties indexes
		"CREATE INDEX IF NOT EXISTS idx_properties_org ON properties(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_properties_status ON properties(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_properties_landlord ON properties(org_id, landlord_id)",
		"CREATE INDEX IF NOT EXISTS idx_properties_type ON properties(org_id, property_type)",
		// Deals indexes
		"CREATE INDEX IF NOT EXISTS idx_deals_org ON deals(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_deals_status ON deals(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_deals_property ON deals(property_id)",
		"CREATE INDEX IF NOT EXISTS idx_deals_tenant ON deals(tenant_id)",
		"CREATE INDEX IF NOT EXISTS idx_deals_landlord ON deals(landlord_id)",
	}

	for _, stmt := range tables {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	for _, stmt := range indexes {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			log.Printf("Warning: index creation failed (may already exist): %v", err)
		}
	}

	return nil
}
