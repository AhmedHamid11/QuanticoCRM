package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/turso"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// TenantProvisioningService handles creating new tenant databases
type TenantProvisioningService struct {
	masterDBConn        db.DBConn // Master DB with retry logic (for metadata)
	tursoClient         *turso.Client
	provisioningService *ProvisioningService
	localMode           bool
}

// TenantDatabase holds the connection info for a provisioned tenant database
type TenantDatabase struct {
	Name     string
	URL      string
	Token    string
	Database *sql.DB
}

// NewTenantProvisioningService creates a new tenant provisioning service
func NewTenantProvisioningService(masterDBConn db.DBConn) *TenantProvisioningService {
	// Check if we're in local mode (no Turso Platform API credentials)
	// For tenant provisioning, we need TURSO_API_TOKEN (or TURSO_AUTH_TOKEN) and TURSO_ORG to create databases
	apiToken := os.Getenv("TURSO_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("TURSO_AUTH_TOKEN")
	}
	localMode := apiToken == "" || os.Getenv("TURSO_ORG") == ""

	var tursoClient *turso.Client
	if !localMode {
		var err error
		tursoClient, err = turso.NewClient()
		if err != nil {
			log.Printf("Warning: Failed to create Turso client, falling back to local mode: %v", err)
			localMode = true
		}
	}

	return &TenantProvisioningService{
		masterDBConn:        masterDBConn,
		tursoClient:         tursoClient,
		provisioningService: NewProvisioningService(nil), // Will set DB per-tenant
		localMode:           localMode,
	}
}

// ProvisionTenant creates a new tenant database and provisions default metadata
// Returns the database credentials that should be stored in the org record
func (s *TenantProvisioningService) ProvisionTenant(ctx context.Context, orgID, orgSlug string) (*TenantDatabase, error) {
	if s.localMode {
		return s.provisionLocalTenant(ctx, orgID)
	}

	return s.provisionTursoTenant(ctx, orgID, orgSlug)
}

// provisionLocalTenant handles tenant provisioning in local/development mode
// In local mode, all orgs share the same database (existing behavior)
func (s *TenantProvisioningService) provisionLocalTenant(ctx context.Context, orgID string) (*TenantDatabase, error) {
	log.Printf("[TenantProvisioning] Local mode: using shared database for org %s", orgID)

	// In local mode, provision metadata to the master/shared database
	s.provisioningService.SetDB(s.masterDBConn)
	if err := s.provisioningService.ProvisionDefaultMetadata(ctx, orgID); err != nil {
		return nil, fmt.Errorf("failed to provision metadata: %w", err)
	}

	// For local mode, we need a *sql.DB for the return value
	// The DBConn might be a TursoDB or a raw *sql.DB
	var rawDB *sql.DB
	if tursoDB, ok := s.masterDBConn.(*db.TursoDB); ok {
		rawDB = tursoDB.GetDB()
	} else if sqlDB, ok := s.masterDBConn.(*sql.DB); ok {
		rawDB = sqlDB
	}

	return &TenantDatabase{
		Name:     "local",
		URL:      "",
		Token:    "",
		Database: rawDB,
	}, nil
}

// provisionTursoTenant creates a new Turso database for the tenant
func (s *TenantProvisioningService) provisionTursoTenant(ctx context.Context, orgID, orgSlug string) (*TenantDatabase, error) {
	// Generate database name from org slug (must be unique and URL-safe)
	dbName := s.generateDBName(orgSlug, orgID)
	log.Printf("[TenantProvisioning] Creating Turso database: %s for org %s", dbName, orgID)

	// Create the database via Turso API
	db, err := s.tursoClient.CreateDatabase(ctx, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Turso database: %w", err)
	}

	// Get the database URL
	dbURL := s.tursoClient.GetDatabaseURL(db)
	log.Printf("[TenantProvisioning] Database created: %s", dbURL)

	// Create an auth token for this database (never expires)
	token, err := s.tursoClient.CreateAuthToken(ctx, dbName, "never")
	if err != nil {
		// Try to clean up the database we just created
		_ = s.tursoClient.DeleteDatabase(ctx, dbName)
		return nil, fmt.Errorf("failed to create auth token: %w", err)
	}

	// Connect to the new database
	connStr := dbURL + "?authToken=" + token
	tenantDB, err := sql.Open("libsql", connStr)
	if err != nil {
		_ = s.tursoClient.DeleteDatabase(ctx, dbName)
		return nil, fmt.Errorf("failed to connect to new database: %w", err)
	}

	// Verify connection
	if err := tenantDB.PingContext(ctx); err != nil {
		tenantDB.Close()
		_ = s.tursoClient.DeleteDatabase(ctx, dbName)
		return nil, fmt.Errorf("failed to ping new database: %w", err)
	}

	// Run migrations on the new database
	if err := s.runMigrations(ctx, tenantDB); err != nil {
		tenantDB.Close()
		_ = s.tursoClient.DeleteDatabase(ctx, dbName)
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Provision all org data to the TENANT database.
	// API handlers read metadata via getMetadataRepo(c) which uses the tenant DB,
	// so entity_defs, field_defs, layout_defs must live in the tenant DB.
	s.provisioningService.SetDB(tenantDB)
	if err := s.provisioningService.ProvisionMetadataOnly(ctx, orgID); err != nil {
		tenantDB.Close()
		_ = s.tursoClient.DeleteDatabase(ctx, dbName)
		return nil, fmt.Errorf("failed to provision metadata: %w", err)
	}

	// Navigation and sample data also go to the tenant DB
	s.provisioningService.ProvisionNavigation(ctx, orgID)
	s.provisioningService.ProvisionSampleData(ctx, orgID)

	log.Printf("[TenantProvisioning] Successfully provisioned tenant database for org %s", orgID)

	return &TenantDatabase{
		Name:     dbName,
		URL:      dbURL,
		Token:    token,
		Database: tenantDB,
	}, nil
}

// generateDBName creates a unique, URL-safe database name
func (s *TenantProvisioningService) generateDBName(slug, orgID string) string {
	// Use slug if available, otherwise use org ID
	name := slug
	if name == "" {
		name = orgID
	}

	// Make it URL-safe: lowercase, replace spaces with dashes, remove special chars
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Add timestamp suffix for uniqueness
	suffix := fmt.Sprintf("-%d", time.Now().Unix()%100000)

	// Turso has a 64 char limit on database names
	if len(name) > 50 {
		name = name[:50]
	}

	return "org-" + name + suffix
}

// runMigrations runs all tenant schema migrations on a new database
func (s *TenantProvisioningService) runMigrations(ctx context.Context, db *sql.DB) error {
	log.Println("[TenantProvisioning] Running tenant database migrations...")

	// These are the tenant-specific tables (not master DB tables like organizations, users)
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

		// Quote line items table
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

		// Tripwires (webhooks)
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

		// Bearing configs (stage indicators)
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
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pdf_templates_org ON pdf_templates(org_id, entity_type)`,

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
	}

	// Execute each migration
	for i, migration := range migrations {
		_, err := db.ExecContext(ctx, migration)
		if err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	// Run schema upgrade migrations (add columns if they don't exist)
	// These use ALTER TABLE which errors if column exists, so we ignore errors
	upgradeMigrations := []string{
		// Fix list_views schema - add columns that might be missing
		`ALTER TABLE list_views ADD COLUMN filter_query TEXT DEFAULT ''`,
		`ALTER TABLE list_views ADD COLUMN sort_by TEXT DEFAULT ''`,
		`ALTER TABLE list_views ADD COLUMN sort_dir TEXT DEFAULT 'desc'`,
		`ALTER TABLE list_views ADD COLUMN created_by_id TEXT`,
		// Fix entity_defs schema - add display_field and search_fields columns
		`ALTER TABLE entity_defs ADD COLUMN display_field TEXT DEFAULT 'name'`,
		`ALTER TABLE entity_defs ADD COLUMN search_fields TEXT DEFAULT '["name"]'`,
		// Fix field_defs schema - add columns that might be missing
		`ALTER TABLE field_defs ADD COLUMN rollup_query TEXT`,
		`ALTER TABLE field_defs ADD COLUMN rollup_result_type TEXT`,
		`ALTER TABLE field_defs ADD COLUMN rollup_decimal_places INTEGER DEFAULT 2`,
		`ALTER TABLE field_defs ADD COLUMN default_to_today INTEGER DEFAULT 0`,
		`ALTER TABLE field_defs ADD COLUMN variant TEXT DEFAULT 'info'`,
		`ALTER TABLE field_defs ADD COLUMN content TEXT DEFAULT ''`,
	}

	for _, migration := range upgradeMigrations {
		// Ignore errors - column might already exist
		db.ExecContext(ctx, migration)
	}

	log.Println("[TenantProvisioning] Migrations completed successfully")
	return nil
}

// DeleteTenant deletes a tenant's database (for cleanup/testing)
func (s *TenantProvisioningService) DeleteTenant(ctx context.Context, dbName string) error {
	if s.localMode {
		log.Printf("[TenantProvisioning] Local mode: skipping database deletion for %s", dbName)
		return nil
	}

	if s.tursoClient == nil {
		return fmt.Errorf("Turso client not initialized")
	}

	return s.tursoClient.DeleteDatabase(ctx, dbName)
}

// IsLocalMode returns whether we're in local development mode
func (s *TenantProvisioningService) IsLocalMode() bool {
	return s.localMode
}
