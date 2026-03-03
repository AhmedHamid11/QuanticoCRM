package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Command-line flags
	orgID := flag.String("org-id", "", "Organization ID to provision (optional — creates new org if not provided)")
	dbPath := flag.String("db", "", "Path to SQLite database (defaults to searching common paths)")
	createTables := flag.Bool("create-tables", true, "Create data tables for CRE entities (leads, properties, deals)")
	withSampleData := flag.Bool("sample-data", true, "Include sample CRE brokerage data")

	// Turso mode flags — when provided, uses Turso instead of local SQLite
	masterURL := flag.String("master-url", "", "Turso master DB URL (e.g., libsql://quantico-org.turso.io)")
	masterToken := flag.String("master-token", "", "Turso master DB auth token")
	tenantURL := flag.String("tenant-url", "", "Turso tenant DB URL (e.g., libsql://org-apex-cre-advisors-org.turso.io)")
	tenantToken := flag.String("tenant-token", "", "Turso tenant DB auth token")
	tenantDBName := flag.String("tenant-db-name", "", "Turso tenant database name (for storing in org record)")
	flag.Parse()

	tursoMode := *masterURL != "" && *masterToken != "" && *tenantURL != "" && *tenantToken != ""

	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	var masterDB, tenantDB *sql.DB
	var err error

	if tursoMode {
		log.Println("=== TURSO MODE ===")
		log.Printf("Master DB: %s", *masterURL)
		log.Printf("Tenant DB: %s", *tenantURL)

		// Connect to master DB (for org/user/membership records)
		masterConnStr := *masterURL + "?authToken=" + *masterToken
		masterDB, err = sql.Open("libsql", masterConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to master DB: %v", err)
		}
		defer masterDB.Close()
		if err := masterDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping master DB: %v", err)
		}
		log.Println("Connected to master DB")

		// Connect to tenant DB (for tables/metadata/data)
		tenantConnStr := *tenantURL + "?authToken=" + *tenantToken
		tenantDB, err = sql.Open("libsql", tenantConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to tenant DB: %v", err)
		}
		defer tenantDB.Close()
		if err := tenantDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping tenant DB: %v", err)
		}
		log.Println("Connected to tenant DB")

		// Run tenant migrations first (create base tables: contacts, accounts, entity_defs, etc.)
		log.Println("Running tenant database migrations...")
		if err := runTenantMigrations(ctx, tenantDB); err != nil {
			log.Fatalf("Failed to run tenant migrations: %v", err)
		}
		log.Println("Tenant migrations complete")
	} else {
		// Local SQLite mode (existing behavior — single DB for everything)
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

		log.Printf("Using local database: %s", dbFile)
		localDB, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer localDB.Close()

		// In local mode, master and tenant are the same DB
		masterDB = localDB
		tenantDB = localDB
	}

	// Determine org ID — create new org if not provided
	targetOrgID := *orgID
	if targetOrgID == "" {
		targetOrgID = sfid.NewOrg()
		log.Printf("Creating new org with ID: %s", targetOrgID)

		// Create the organization (in master DB)
		if tursoMode {
			// Include database_url and database_token for Turso mode
			_, err = masterDB.ExecContext(ctx, `
				INSERT INTO organizations (id, name, slug, plan, is_active, settings, database_url, database_token, database_name, created_at, modified_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, targetOrgID, "Apex CRE Advisors", "apex-cre-advisors", "pro", 1, "{}", *tenantURL, *tenantToken, *tenantDBName, now, now)
		} else {
			_, err = masterDB.ExecContext(ctx, `
				INSERT INTO organizations (id, name, slug, plan, is_active, settings, created_at, modified_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, targetOrgID, "Apex CRE Advisors", "apex-cre-advisors", "pro", 1, "{}", now, now)
		}
		if err != nil {
			log.Fatalf("Failed to create organization: %v", err)
		}
		log.Println("Created organization: Apex CRE Advisors")

		// Create admin user (in master DB)
		passwordHash, err := bcrypt.GenerateFromPassword([]byte("apex2024!"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		userID := sfid.NewUser()
		_, err = masterDB.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, is_platform_admin, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, "admin@apexcre.com", string(passwordHash), "Admin", "User", 1, 0, now, now)
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Created user: admin@apexcre.com")

		// Create membership linking user to org (in master DB)
		membershipID := sfid.NewMembership()
		_, err = masterDB.ExecContext(ctx, `
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

	// Create CRE data tables (in tenant DB)
	if *createTables {
		log.Println("Creating CRE data tables (leads, properties, deals)...")
		if err := createCRETables(ctx, tenantDB); err != nil {
			log.Fatalf("Failed to create CRE tables: %v", err)
		}
		log.Println("CRE data tables created successfully")
	}

	// Run default metadata provisioning (Account, Contact, Task, Quote entities) — on tenant DB
	provSvc := service.NewProvisioningService(tenantDB)
	log.Println("Running default metadata provisioning...")
	if err := provSvc.ProvisionDefaultMetadata(ctx, targetOrgID); err != nil {
		log.Fatalf("Failed to provision default metadata: %v", err)
	}
	log.Println("Default metadata provisioned")

	// Run CRE broker provisioning (Lead, Property, Deal entities + CRE-specific fields/layouts/nav) — on tenant DB
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
	if tursoMode {
		log.Printf("Master DB: %s", *masterURL)
		log.Printf("Tenant DB: %s", *tenantURL)
	}
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
	if tursoMode {
		log.Println("Next steps:")
		log.Println("  1. Visit the production app URL")
		log.Println("  2. Login with admin@apexcre.com / apex2024!")
		log.Println("  3. Navigate: Accounts → Contacts → Properties → Leads → Deals")
	} else {
		log.Println("Next steps:")
		log.Println("  1. Start backend: cd backend && air")
		log.Println("  2. Start frontend: cd frontend && npm run dev")
		log.Println("  3. Login at http://localhost:5173 with admin@apexcre.com / apex2024!")
		log.Println("  4. Navigate: Accounts → Contacts → Properties → Leads → Deals")
	}
}

// runTenantMigrations runs all tenant schema migrations on a new Turso database
// Mirrors the migrations from tenant_provisioning.go
func runTenantMigrations(ctx context.Context, db *sql.DB) error {
	migrations := []string{
		// Contacts table
		`CREATE TABLE IF NOT EXISTS contacts (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			salutation_name TEXT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email_address TEXT,
			phone_number TEXT,
			phone_number_type TEXT,
			do_not_call INTEGER DEFAULT 0,
			description TEXT,
			address_street TEXT,
			address_city TEXT,
			address_state TEXT,
			address_country TEXT,
			address_postal_code TEXT,
			account_id TEXT,
			account_name TEXT,
			assigned_user_id TEXT,
			created_by_id TEXT,
			modified_by_id TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_org ON contacts(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(email_address)`,

		// Accounts table
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			website TEXT,
			email_address TEXT,
			phone_number TEXT,
			type TEXT,
			industry TEXT,
			sic_code TEXT,
			billing_address_street TEXT,
			billing_address_city TEXT,
			billing_address_state TEXT,
			billing_address_country TEXT,
			billing_address_postal_code TEXT,
			shipping_address_street TEXT,
			shipping_address_city TEXT,
			shipping_address_state TEXT,
			shipping_address_country TEXT,
			shipping_address_postal_code TEXT,
			description TEXT,
			stage TEXT,
			assigned_user_id TEXT,
			created_by_id TEXT,
			modified_by_id TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_org ON accounts(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_name ON accounts(name)`,

		// Tasks table
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			description TEXT,
			status TEXT DEFAULT 'Open',
			priority TEXT DEFAULT 'Normal',
			type TEXT,
			due_date TEXT,
			parent_id TEXT,
			parent_type TEXT,
			parent_name TEXT,
			assigned_user_id TEXT,
			created_by_id TEXT,
			modified_by_id TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}',
			gmail_message_id TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_org ON tasks(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id, parent_type)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_date)`,

		// Quotes table
		`CREATE TABLE IF NOT EXISTS quotes (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			quote_number TEXT,
			status TEXT DEFAULT 'Draft',
			account_id TEXT,
			account_name TEXT,
			contact_id TEXT,
			contact_name TEXT,
			valid_until TEXT,
			subtotal REAL DEFAULT 0,
			discount_percent REAL DEFAULT 0,
			discount_amount REAL DEFAULT 0,
			tax_percent REAL DEFAULT 0,
			tax_amount REAL DEFAULT 0,
			shipping_amount REAL DEFAULT 0,
			grand_total REAL DEFAULT 0,
			currency TEXT DEFAULT 'USD',
			description TEXT,
			terms TEXT,
			notes TEXT,
			billing_address_street TEXT,
			billing_address_city TEXT,
			billing_address_state TEXT,
			billing_address_country TEXT,
			billing_address_postal_code TEXT,
			shipping_address_street TEXT,
			shipping_address_city TEXT,
			shipping_address_state TEXT,
			shipping_address_country TEXT,
			shipping_address_postal_code TEXT,
			assigned_user_id TEXT,
			created_by_id TEXT,
			modified_by_id TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_quotes_org ON quotes(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_quotes_account ON quotes(account_id)`,

		// Quote line items
		`CREATE TABLE IF NOT EXISTS quote_line_items (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			quote_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			sku TEXT,
			quantity REAL DEFAULT 1,
			unit_price REAL DEFAULT 0,
			discount_percent REAL DEFAULT 0,
			discount_amount REAL DEFAULT 0,
			tax_percent REAL DEFAULT 0,
			total REAL DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (quote_id) REFERENCES quotes(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_quote_line_items_quote ON quote_line_items(quote_id)`,

		// Entity definitions
		`CREATE TABLE IF NOT EXISTS entity_defs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			label TEXT NOT NULL,
			label_plural TEXT,
			icon TEXT DEFAULT '',
			color TEXT DEFAULT '',
			is_custom INTEGER DEFAULT 0,
			is_customizable INTEGER DEFAULT 1,
			has_stream INTEGER DEFAULT 0,
			has_activities INTEGER DEFAULT 0,
			display_field TEXT DEFAULT 'name',
			search_fields TEXT DEFAULT '["name"]',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, name)
		)`,

		// Field definitions
		`CREATE TABLE IF NOT EXISTS field_defs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_name TEXT NOT NULL,
			name TEXT NOT NULL,
			label TEXT NOT NULL,
			type TEXT NOT NULL,
			is_required INTEGER DEFAULT 0,
			is_read_only INTEGER DEFAULT 0,
			is_audited INTEGER DEFAULT 0,
			is_custom INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			options TEXT,
			default_value TEXT,
			max_length INTEGER,
			min_value REAL,
			max_value REAL,
			pattern TEXT,
			tooltip TEXT,
			link_entity TEXT,
			link_type TEXT,
			link_foreign_key TEXT,
			link_display_field TEXT,
			rollup_query TEXT,
			rollup_result_type TEXT,
			rollup_decimal_places INTEGER,
			default_to_today INTEGER DEFAULT 0,
			variant TEXT DEFAULT 'info',
			content TEXT DEFAULT '',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_name, name)
		)`,

		// Layout definitions
		`CREATE TABLE IF NOT EXISTS layout_defs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_name TEXT NOT NULL,
			layout_type TEXT NOT NULL,
			layout_data TEXT NOT NULL,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_name, layout_type)
		)`,

		// Navigation tabs
		`CREATE TABLE IF NOT EXISTS navigation_tabs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			label TEXT NOT NULL,
			href TEXT NOT NULL,
			icon TEXT DEFAULT '',
			entity_name TEXT,
			sort_order INTEGER DEFAULT 0,
			is_visible INTEGER DEFAULT 1,
			is_system INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, href)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_navigation_org ON navigation_tabs(org_id)`,

		// Related list configs
		`CREATE TABLE IF NOT EXISTS related_list_configs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			related_entity TEXT NOT NULL,
			lookup_field TEXT NOT NULL,
			label TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			is_multi_lookup INTEGER DEFAULT 0,
			display_fields TEXT NOT NULL,
			sort_order INTEGER DEFAULT 0,
			default_sort TEXT,
			default_sort_dir TEXT DEFAULT 'desc',
			page_size INTEGER DEFAULT 5,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_type, related_entity, lookup_field)
		)`,

		// Bearing configs
		`CREATE TABLE IF NOT EXISTS bearing_configs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			name TEXT NOT NULL,
			source_picklist TEXT NOT NULL,
			display_order INTEGER DEFAULT 0,
			active INTEGER DEFAULT 1,
			confirm_backward INTEGER DEFAULT 0,
			allow_updates INTEGER DEFAULT 1,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_type, source_picklist)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bearing_org ON bearing_configs(org_id, entity_type)`,

		// Org settings
		`CREATE TABLE IF NOT EXISTS org_settings (
			org_id TEXT PRIMARY KEY,
			home_page TEXT DEFAULT '/',
			idle_timeout_minutes INTEGER NOT NULL DEFAULT 30,
			absolute_timeout_minutes INTEGER NOT NULL DEFAULT 1440,
			settings_json TEXT DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// List views
		`CREATE TABLE IF NOT EXISTS list_views (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_name TEXT NOT NULL,
			name TEXT NOT NULL,
			filter_query TEXT DEFAULT '',
			columns TEXT DEFAULT '[]',
			sort_by TEXT DEFAULT '',
			sort_dir TEXT DEFAULT 'desc',
			is_default INTEGER DEFAULT 0,
			is_system INTEGER DEFAULT 0,
			created_by_id TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_list_views_org ON list_views(org_id, entity_name)`,

		// Tripwires
		`CREATE TABLE IF NOT EXISTS tripwires (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			conditions TEXT NOT NULL,
			condition_logic TEXT DEFAULT 'AND',
			endpoint_url TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			created_by TEXT,
			modified_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tripwires_org ON tripwires(org_id)`,

		// Tripwire logs
		`CREATE TABLE IF NOT EXISTS tripwire_logs (
			id TEXT PRIMARY KEY,
			tripwire_id TEXT NOT NULL,
			tripwire_name TEXT,
			org_id TEXT NOT NULL,
			record_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			event_type TEXT NOT NULL,
			status TEXT NOT NULL,
			response_code INTEGER,
			error_message TEXT,
			duration_ms INTEGER,
			executed_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tripwire_logs_tripwire ON tripwire_logs(tripwire_id)`,

		// Webhook settings
		`CREATE TABLE IF NOT EXISTS org_webhook_settings (
			id TEXT PRIMARY KEY,
			org_id TEXT UNIQUE NOT NULL,
			auth_type TEXT DEFAULT 'none',
			api_key TEXT,
			bearer_token TEXT,
			custom_header_name TEXT,
			custom_header_value TEXT,
			timeout_ms INTEGER DEFAULT 5000,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,

		// Validation rules
		`CREATE TABLE IF NOT EXISTS validation_rules (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_name TEXT NOT NULL,
			name TEXT NOT NULL,
			error_message TEXT NOT NULL,
			formula TEXT NOT NULL,
			is_active INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			created_by TEXT,
			modified_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_validation_rules_entity ON validation_rules(org_id, entity_name)`,

		// Flows
		`CREATE TABLE IF NOT EXISTS flows (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			type TEXT NOT NULL,
			trigger_type TEXT,
			trigger_entity TEXT,
			trigger_event TEXT,
			entry_criteria TEXT,
			is_active INTEGER DEFAULT 0,
			version INTEGER DEFAULT 1,
			created_by TEXT,
			modified_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flows_org ON flows(org_id)`,

		// Flow elements
		`CREATE TABLE IF NOT EXISTS flow_elements (
			id TEXT PRIMARY KEY,
			flow_id TEXT NOT NULL,
			element_type TEXT NOT NULL,
			name TEXT NOT NULL,
			label TEXT,
			config TEXT NOT NULL,
			position_x INTEGER DEFAULT 0,
			position_y INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (flow_id) REFERENCES flows(id) ON DELETE CASCADE
		)`,

		// Flow connectors
		`CREATE TABLE IF NOT EXISTS flow_connectors (
			id TEXT PRIMARY KEY,
			flow_id TEXT NOT NULL,
			source_element_id TEXT NOT NULL,
			target_element_id TEXT NOT NULL,
			condition_logic TEXT,
			label TEXT,
			sort_order INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (flow_id) REFERENCES flows(id) ON DELETE CASCADE
		)`,

		// PDF templates
		`CREATE TABLE IF NOT EXISTS pdf_templates (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			is_default INTEGER DEFAULT 0,
			is_system INTEGER DEFAULT 0,
			base_design TEXT DEFAULT 'professional',
			branding TEXT DEFAULT '{}',
			sections TEXT DEFAULT '[]',
			page_size TEXT DEFAULT 'A4',
			orientation TEXT DEFAULT 'portrait',
			margins TEXT DEFAULT '10mm,10mm,10mm,10mm',
			custom_css TEXT,
			header_html TEXT,
			footer_html TEXT,
			created_by TEXT,
			modified_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_type, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pdf_templates_org ON pdf_templates(org_id, entity_type)`,

		// Custom pages
		`CREATE TABLE IF NOT EXISTS custom_pages (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			description TEXT,
			is_active INTEGER DEFAULT 1,
			page_type TEXT DEFAULT 'standard',
			layout TEXT DEFAULT '{}',
			created_by TEXT,
			modified_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, slug)
		)`,
	}

	for i, migration := range migrations {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	log.Printf("Ran %d tenant migrations successfully", len(migrations))
	return nil
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

		// Properties table (table name is "properties" to match GetTableName("Property") convention)
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
		// Properties indexes (table name is "properties")
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
