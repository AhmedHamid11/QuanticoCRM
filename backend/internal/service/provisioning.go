package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/sfid"
)

// ProvisioningService handles provisioning default metadata for new organizations
type ProvisioningService struct {
	db             db.DBConn
	SkipSampleData bool // Set to true to skip sample data creation (useful for tests)
}

// NewProvisioningService creates a new ProvisioningService
func NewProvisioningService(dbConn db.DBConn) *ProvisioningService {
	return &ProvisioningService{db: dbConn, SkipSampleData: false}
}

// SetDB allows changing the database connection (used for tenant provisioning)
func (s *ProvisioningService) SetDB(dbConn db.DBConn) {
	s.db = dbConn
}

// ProvisionMetadataOnly creates default entities, fields, layouts, and navigation for a new org (no sample data)
// This is used in production mode where metadata goes to master DB and data goes to tenant DB
func (s *ProvisioningService) ProvisionMetadataOnly(ctx context.Context, orgID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Starting metadata-only provisioning for org %s", orgID)
	return s.provisionMetadata(ctx, orgID, now)
}

// ProvisionSampleData creates sample accounts, contacts, tasks, quotes for a new org
// This is used in production mode where data goes to tenant DB
func (s *ProvisioningService) ProvisionSampleData(ctx context.Context, orgID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Creating sample data for org %s", orgID)
	s.createSampleData(ctx, orgID, now)
}

// ProvisionNavigation creates default navigation tabs for a new org
// This should be called against the tenant DB where navigation_tabs are queried from
func (s *ProvisioningService) ProvisionNavigation(ctx context.Context, orgID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Creating navigation tabs for org %s", orgID)
	// Create navigation tabs (order: Home, Account, Contact, Quotes, Tasks)
	// isSystem=true for standard tabs that shouldn't be deleted
	tabs := []struct{ label, href, entity string; order int }{
		{"Home", "/", "", 0},
		{"Accounts", "/accounts", "Account", 1},
		{"Contacts", "/contacts", "Contact", 2},
		{"Quotes", "/quotes", "Quote", 3},
		{"Tasks", "/tasks", "Task", 4},
	}
	for _, tab := range tabs {
		if err := s.createNavTabWithError(ctx, orgID, tab.label, tab.href, tab.entity, tab.order, true, now); err != nil {
			return fmt.Errorf("failed to create navigation tab %s: %w", tab.label, err)
		}
	}
	return nil
}

// ProvisionDefaultMetadata creates default entities, fields, layouts, navigation, and sample data for a new org
// This is the original function used in local mode where everything goes to the same database
func (s *ProvisioningService) ProvisionDefaultMetadata(ctx context.Context, orgID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Starting full provisioning for org %s", orgID)

	if err := s.provisionMetadata(ctx, orgID, now); err != nil {
		return err
	}

	// Create navigation tabs (in local mode, same DB as metadata)
	if err := s.ProvisionNavigation(ctx, orgID); err != nil {
		return err
	}

	// Create sample data (unless skipped for tests)
	if !s.SkipSampleData {
		s.createSampleData(ctx, orgID, now)
	}

	log.Printf("[Provisioning] Completed full provisioning for org %s", orgID)
	return nil
}

// ensureMetadataTables checks if metadata tables (entity_defs, field_defs, layout_defs) exist
// and creates them if they don't. This handles migration gaps like when migration 002 was added
// after an organization was provisioned. For existing tables with wrong schema, it recreates them.
func (s *ProvisioningService) ensureMetadataTables(ctx context.Context) error {
	log.Printf("[Provisioning] Checking metadata table schema...")

	// Check if entity_defs table exists
	var tableName string
	err := s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='entity_defs' LIMIT 1").Scan(&tableName)

	tableExists := err == nil

	// Always ensure navigation_tabs exists with correct schema, regardless of entity_defs status
	// This fixes: (1) table doesn't exist, (2) table exists but has wrong schema (missing href/org_id columns)
	if err := s.ensureNavigationTabsTable(ctx); err != nil {
		log.Printf("[Provisioning] Warning: failed to ensure navigation_tabs table: %v", err)
	}

	if tableExists {
		// Verify the schema is correct by checking for UNIQUE constraint on (org_id, name)
		var uniqueConstraintExists int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='index' AND tbl_name='entity_defs'
			AND sql LIKE '%org_id%name%'
		`).Scan(&uniqueConstraintExists)

		if err == nil && uniqueConstraintExists > 0 {
			log.Printf("[Provisioning] entity_defs table exists with correct schema, skipping creation")
			return nil
		}

		// Schema is wrong or missing constraints - recreate it
		log.Printf("[Provisioning] entity_defs exists but schema is incorrect, recreating...")
		if err := s.dropAndRecreateMetadataTables(ctx); err != nil {
			return fmt.Errorf("failed to fix metadata tables: %w", err)
		}
		return nil
	}

	// Table doesn't exist, create all metadata tables with full schema
	// This includes all columns from migrations 002, 019, and 039
	log.Printf("[Provisioning] Creating metadata tables with full schema...")

	// Create entity_defs table with all columns (migration 002 + 019 + 039)
	_, err = s.db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS entity_defs (
				id TEXT PRIMARY KEY,
				org_id TEXT NOT NULL,
				name TEXT NOT NULL,
				label TEXT NOT NULL,
				label_plural TEXT NOT NULL,
				icon TEXT DEFAULT 'folder',
				color TEXT DEFAULT '#6366f1',
				is_custom INTEGER DEFAULT 0,
				is_customizable INTEGER DEFAULT 1,
				has_stream INTEGER DEFAULT 0,
				has_activities INTEGER DEFAULT 0,
				display_field TEXT DEFAULT 'name',
				search_fields TEXT DEFAULT '["name"]',
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				modified_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(org_id, name)
			)
	`)
	if err != nil {
		return fmt.Errorf("failed to create entity_defs table: %w", err)
	}
	log.Printf("[Provisioning] Created entity_defs table")

	// Create field_defs table with all columns (migration 002 + 019)
	_, err = s.db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS field_defs (
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
				default_value TEXT,
				options TEXT,
				max_length INTEGER,
				min_value REAL,
				max_value REAL,
				pattern TEXT,
				tooltip TEXT,
				link_entity TEXT,
				link_type TEXT,
				link_foreign_key TEXT,
				link_display_field TEXT,
				sort_order INTEGER DEFAULT 0,
				rollup_query TEXT,
				rollup_result_type TEXT,
				rollup_decimal_places INTEGER DEFAULT 2,
				default_to_today INTEGER DEFAULT 0,
				variant TEXT DEFAULT 'info',
				content TEXT DEFAULT '',
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				modified_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(org_id, entity_name, name)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create field_defs table: %w", err)
		}
		log.Printf("[Provisioning] Created field_defs table")

	// Create layout_defs table with all columns (migration 002 + 019)
	_, err = s.db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS layout_defs (
				id TEXT PRIMARY KEY,
				org_id TEXT NOT NULL,
				entity_name TEXT NOT NULL,
				layout_type TEXT NOT NULL,
				layout_data TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				modified_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(org_id, entity_name, layout_type)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create layout_defs table: %w", err)
		}
		log.Printf("[Provisioning] Created layout_defs table")

	// Create navigation_tabs table (needed for org navigation display)
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS navigation_tabs (
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
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create navigation_tabs table: %w", err)
	}
	log.Printf("[Provisioning] Created navigation_tabs table")

	// Create indexes
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_entity_defs_org ON entity_defs(org_id)`)
		if err != nil {
			return fmt.Errorf("failed to create entity_defs org index: %w", err)
		}
		_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_field_defs_org_entity ON field_defs(org_id, entity_name)`)
		if err != nil {
			return fmt.Errorf("failed to create field_defs org index: %w", err)
		}
		_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_layout_defs_org_entity ON layout_defs(org_id, entity_name)`)
		if err != nil {
			return fmt.Errorf("failed to create layout_defs org index: %w", err)
		}
		_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_navigation_org ON navigation_tabs(org_id)`)
		if err != nil {
			return fmt.Errorf("failed to create navigation_tabs org index: %w", err)
		}
		log.Printf("[Provisioning] Created metadata table indexes")

	return nil
}

// ensureNavigationTabsTable ensures navigation_tabs table exists with the correct schema.
// If the table exists but has wrong columns (e.g., missing href or org_id), it drops and recreates it.
func (s *ProvisioningService) ensureNavigationTabsTable(ctx context.Context) error {
	// Check if navigation_tabs table exists
	var tblName string
	err := s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='navigation_tabs' LIMIT 1").Scan(&tblName)
	tableExists := err == nil

	if tableExists {
		// Verify schema has required columns (href, org_id) by checking table info
		var hrefExists bool
		rows, err := s.db.QueryContext(ctx, "PRAGMA table_info(navigation_tabs)")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cid int
				var name, colType string
				var notNull, pk int
				var dfltValue *string
				if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err == nil {
					if name == "href" {
						hrefExists = true
					}
				}
			}
		}

		if hrefExists {
			log.Printf("[Provisioning] navigation_tabs table exists with correct schema")
			return nil
		}

		// Schema is wrong - drop and recreate
		log.Printf("[Provisioning] navigation_tabs table has wrong schema (missing href), recreating...")
		if _, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS navigation_tabs"); err != nil {
			return fmt.Errorf("failed to drop old navigation_tabs: %w", err)
		}
	}

	// Create navigation_tabs with correct schema
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS navigation_tabs (
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
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create navigation_tabs table: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_navigation_org ON navigation_tabs(org_id)`)
	log.Printf("[Provisioning] Created navigation_tabs table with correct schema")
	return nil
}

// dropAndRecreateMetadataTables safely drops and recreates metadata tables with correct schema
// This fixes schema mismatches in existing databases
func (s *ProvisioningService) dropAndRecreateMetadataTables(ctx context.Context) error {
	log.Printf("[Provisioning] Safely dropping and recreating metadata tables...")

	// Disable foreign key constraints temporarily
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}
	defer func() {
		s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	// Drop tables in reverse dependency order
	tables := []string{"layout_defs", "field_defs", "entity_defs"}
	for _, table := range tables {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
		log.Printf("[Provisioning] Dropped table %s", table)
	}

	// Recreate entity_defs table with correct schema
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE entity_defs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			label TEXT NOT NULL,
			label_plural TEXT NOT NULL,
			icon TEXT DEFAULT 'folder',
			color TEXT DEFAULT '#6366f1',
			is_custom INTEGER DEFAULT 0,
			is_customizable INTEGER DEFAULT 1,
			has_stream INTEGER DEFAULT 0,
			has_activities INTEGER DEFAULT 0,
			display_field TEXT DEFAULT 'name',
			search_fields TEXT DEFAULT '["name"]',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			modified_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(org_id, name)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to recreate entity_defs: %w", err)
	}
	log.Printf("[Provisioning] Recreated entity_defs table")

	// Recreate field_defs table
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE field_defs (
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
			default_value TEXT,
			options TEXT,
			max_length INTEGER,
			min_value REAL,
			max_value REAL,
			pattern TEXT,
			tooltip TEXT,
			link_entity TEXT,
			link_type TEXT,
			link_foreign_key TEXT,
			link_display_field TEXT,
			sort_order INTEGER DEFAULT 0,
			rollup_query TEXT,
			rollup_result_type TEXT,
			rollup_decimal_places INTEGER DEFAULT 2,
			default_to_today INTEGER DEFAULT 0,
			variant TEXT DEFAULT 'info',
			content TEXT DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			modified_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(org_id, entity_name, name)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to recreate field_defs: %w", err)
	}
	log.Printf("[Provisioning] Recreated field_defs table")

	// Recreate layout_defs table
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE layout_defs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_name TEXT NOT NULL,
			layout_type TEXT NOT NULL,
			layout_data TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			modified_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(org_id, entity_name, layout_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to recreate layout_defs: %w", err)
	}
	log.Printf("[Provisioning] Recreated layout_defs table")

	// Recreate navigation_tabs table
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS navigation_tabs (
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
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to recreate navigation_tabs: %w", err)
	}
	log.Printf("[Provisioning] Recreated navigation_tabs table")

	// Recreate indexes
	indexes := []struct {
		name string
		stmt string
	}{
		{"idx_entity_defs_org", "CREATE INDEX IF NOT EXISTS idx_entity_defs_org ON entity_defs(org_id)"},
		{"idx_field_defs_org_entity", "CREATE INDEX IF NOT EXISTS idx_field_defs_org_entity ON field_defs(org_id, entity_name)"},
		{"idx_layout_defs_org_entity", "CREATE INDEX IF NOT EXISTS idx_layout_defs_org_entity ON layout_defs(org_id, entity_name)"},
		{"idx_navigation_org", "CREATE INDEX IF NOT EXISTS idx_navigation_org ON navigation_tabs(org_id)"},
	}

	for _, idx := range indexes {
		if _, err := s.db.ExecContext(ctx, idx.stmt); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
	}
	log.Printf("[Provisioning] Recreated all metadata indexes")

	return nil
}

// provisionMetadata creates entities, fields, layouts, navigation for a new org
func (s *ProvisioningService) provisionMetadata(ctx context.Context, orgID, now string) error {
	log.Printf("[Provisioning] Creating metadata for org %s", orgID)

	// Ensure metadata tables exist (fixes migration gaps like Guardare)
	if err := s.ensureMetadataTables(ctx); err != nil {
		return fmt.Errorf("failed to ensure metadata tables: %w", err)
	}

	// Create default entities
	entities := []struct{ name, plural string }{
		{"Contact", "Contacts"},
		{"Account", "Accounts"},
		{"Task", "Tasks"},
		{"Quote", "Quotes"},
		{"QuoteLineItem", "Quote Line Items"},
	}
	for _, e := range entities {
		if err := s.createEntity(ctx, orgID, e.name, e.plural, now); err != nil {
			log.Printf("[Provisioning] Warning: failed to create %s entity for org %s: %v", e.name, orgID, err)
		} else {
			log.Printf("[Provisioning] Created entity %s for org %s", e.name, orgID)
		}
	}

	// --- Contact fields (all standard columns) ---
	// Field names must match the JSON keys returned by the API (from entity struct)
	s.createField(ctx, orgID, "Contact", "salutationName", "Salutation", "enum", false, 1, now)
	s.createField(ctx, orgID, "Contact", "firstName", "First Name", "varchar", true, 2, now)
	s.createField(ctx, orgID, "Contact", "lastName", "Last Name", "varchar", true, 3, now)
	s.createField(ctx, orgID, "Contact", "emailAddress", "Email", "email", false, 4, now)
	s.createField(ctx, orgID, "Contact", "phoneNumber", "Phone", "phone", false, 5, now)
	s.createField(ctx, orgID, "Contact", "phoneNumberType", "Phone Type", "enum", false, 6, now)
	s.createField(ctx, orgID, "Contact", "doNotCall", "Do Not Call", "bool", false, 7, now)
	s.createLinkField(ctx, orgID, "Contact", "accountId", "Account", "Account", 8, now)
	s.createField(ctx, orgID, "Contact", "description", "Description", "text", false, 10, now)
	s.createField(ctx, orgID, "Contact", "addressStreet", "Street", "varchar", false, 20, now)
	s.createField(ctx, orgID, "Contact", "addressCity", "City", "varchar", false, 21, now)
	s.createField(ctx, orgID, "Contact", "addressState", "State", "varchar", false, 22, now)
	s.createField(ctx, orgID, "Contact", "addressCountry", "Country", "varchar", false, 23, now)
	s.createField(ctx, orgID, "Contact", "addressPostalCode", "Postal Code", "varchar", false, 24, now)
	s.createField(ctx, orgID, "Contact", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Contact", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// --- Account fields (all standard columns) ---
	// Field names must match JSON keys from API
	s.createField(ctx, orgID, "Account", "name", "Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Account", "website", "Website", "url", false, 2, now)
	s.createField(ctx, orgID, "Account", "emailAddress", "Email", "email", false, 3, now)
	s.createField(ctx, orgID, "Account", "phoneNumber", "Phone", "phone", false, 4, now)
	s.createField(ctx, orgID, "Account", "type", "Type", "enum", false, 5, now)
	s.createField(ctx, orgID, "Account", "industry", "Industry", "varchar", false, 6, now)
	s.createField(ctx, orgID, "Account", "sicCode", "SIC Code", "varchar", false, 7, now)
	s.createField(ctx, orgID, "Account", "stage", "Stage", "varchar", false, 8, now)
	s.createField(ctx, orgID, "Account", "description", "Description", "text", false, 10, now)
	s.createField(ctx, orgID, "Account", "billingAddressStreet", "Billing Street", "varchar", false, 20, now)
	s.createField(ctx, orgID, "Account", "billingAddressCity", "Billing City", "varchar", false, 21, now)
	s.createField(ctx, orgID, "Account", "billingAddressState", "Billing State", "varchar", false, 22, now)
	s.createField(ctx, orgID, "Account", "billingAddressCountry", "Billing Country", "varchar", false, 23, now)
	s.createField(ctx, orgID, "Account", "billingAddressPostalCode", "Billing Postal Code", "varchar", false, 24, now)
	s.createField(ctx, orgID, "Account", "shippingAddressStreet", "Shipping Street", "varchar", false, 30, now)
	s.createField(ctx, orgID, "Account", "shippingAddressCity", "Shipping City", "varchar", false, 31, now)
	s.createField(ctx, orgID, "Account", "shippingAddressState", "Shipping State", "varchar", false, 32, now)
	s.createField(ctx, orgID, "Account", "shippingAddressCountry", "Shipping Country", "varchar", false, 33, now)
	s.createField(ctx, orgID, "Account", "shippingAddressPostalCode", "Shipping Postal Code", "varchar", false, 34, now)
	s.createField(ctx, orgID, "Account", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Account", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// --- Task fields (all standard columns) ---
	s.createField(ctx, orgID, "Task", "subject", "Subject", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Task", "status", "Status", "enum", false, 2, now)
	s.createField(ctx, orgID, "Task", "priority", "Priority", "enum", false, 3, now)
	s.createField(ctx, orgID, "Task", "type", "Type", "enum", false, 4, now)
	s.createField(ctx, orgID, "Task", "dueDate", "Due Date", "date", false, 5, now)
	s.createField(ctx, orgID, "Task", "description", "Description", "text", false, 6, now)
	s.createField(ctx, orgID, "Task", "parentId", "Related To", "varchar", false, 10, now)
	s.createField(ctx, orgID, "Task", "parentType", "Related Type", "varchar", false, 11, now)
	s.createField(ctx, orgID, "Task", "parentName", "Related Name", "varchar", false, 12, now)
	s.createField(ctx, orgID, "Task", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Task", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// --- Quote fields (all standard columns) ---
	s.createField(ctx, orgID, "Quote", "name", "Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Quote", "quoteNumber", "Quote Number", "varchar", false, 2, now)
	s.createEnumField(ctx, orgID, "Quote", "status", "Status", []string{"Draft", "Needs Review", "Approved", "Sent", "Accepted", "Declined", "Expired"}, 3, now)
	s.createLinkField(ctx, orgID, "Quote", "accountId", "Account", "Account", 4, now)
	s.createLinkField(ctx, orgID, "Quote", "contactId", "Contact", "Contact", 5, now)
	s.createField(ctx, orgID, "Quote", "validUntil", "Valid Until", "date", false, 6, now)
	s.createField(ctx, orgID, "Quote", "subtotal", "Subtotal", "float", false, 10, now)
	s.createField(ctx, orgID, "Quote", "discountPercent", "Discount %", "float", false, 11, now)
	s.createField(ctx, orgID, "Quote", "discountAmount", "Discount Amount", "float", false, 12, now)
	s.createField(ctx, orgID, "Quote", "taxPercent", "Tax %", "float", false, 13, now)
	s.createField(ctx, orgID, "Quote", "taxAmount", "Tax Amount", "float", false, 14, now)
	s.createField(ctx, orgID, "Quote", "shippingAmount", "Shipping", "float", false, 15, now)
	s.createField(ctx, orgID, "Quote", "grandTotal", "Grand Total", "float", false, 16, now)
	s.createField(ctx, orgID, "Quote", "currency", "Currency", "varchar", false, 17, now)
	s.createField(ctx, orgID, "Quote", "description", "Description", "text", false, 20, now)
	s.createField(ctx, orgID, "Quote", "terms", "Terms", "text", false, 21, now)
	s.createField(ctx, orgID, "Quote", "notes", "Notes", "text", false, 22, now)
	s.createField(ctx, orgID, "Quote", "billingAddressStreet", "Billing Street", "varchar", false, 30, now)
	s.createField(ctx, orgID, "Quote", "billingAddressCity", "Billing City", "varchar", false, 31, now)
	s.createField(ctx, orgID, "Quote", "billingAddressState", "Billing State", "varchar", false, 32, now)
	s.createField(ctx, orgID, "Quote", "billingAddressCountry", "Billing Country", "varchar", false, 33, now)
	s.createField(ctx, orgID, "Quote", "billingAddressPostalCode", "Billing Postal Code", "varchar", false, 34, now)
	s.createField(ctx, orgID, "Quote", "shippingAddressStreet", "Shipping Street", "varchar", false, 40, now)
	s.createField(ctx, orgID, "Quote", "shippingAddressCity", "Shipping City", "varchar", false, 41, now)
	s.createField(ctx, orgID, "Quote", "shippingAddressState", "Shipping State", "varchar", false, 42, now)
	s.createField(ctx, orgID, "Quote", "shippingAddressCountry", "Shipping Country", "varchar", false, 43, now)
	s.createField(ctx, orgID, "Quote", "shippingAddressPostalCode", "Shipping Postal Code", "varchar", false, 44, now)
	s.createField(ctx, orgID, "Quote", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Quote", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// --- QuoteLineItem fields (special entity - appears on Quote, not standalone) ---
	s.createField(ctx, orgID, "QuoteLineItem", "name", "Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "QuoteLineItem", "description", "Description", "text", false, 2, now)
	s.createField(ctx, orgID, "QuoteLineItem", "sku", "SKU", "varchar", false, 3, now)
	s.createField(ctx, orgID, "QuoteLineItem", "quantity", "Quantity", "float", true, 4, now)
	s.createField(ctx, orgID, "QuoteLineItem", "unitPrice", "Unit Price", "currency", true, 5, now)
	s.createField(ctx, orgID, "QuoteLineItem", "discountPercent", "Discount %", "float", false, 6, now)
	s.createField(ctx, orgID, "QuoteLineItem", "discountAmount", "Discount Amount", "currency", false, 7, now)
	s.createField(ctx, orgID, "QuoteLineItem", "taxPercent", "Tax %", "float", false, 8, now)
	s.createField(ctx, orgID, "QuoteLineItem", "total", "Total", "currency", false, 9, now)
	s.createLinkField(ctx, orgID, "QuoteLineItem", "quoteId", "Quote", "Quote", 10, now)
	s.createField(ctx, orgID, "QuoteLineItem", "sortOrder", "Sort Order", "int", false, 11, now)
	s.createField(ctx, orgID, "QuoteLineItem", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "QuoteLineItem", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// Create navigation tabs (in metadata provisioning for tenant DB)
	// This ensures navigation tabs are always available when metadata is provisioned
	if err := s.ProvisionNavigation(ctx, orgID); err != nil {
		return fmt.Errorf("failed to provision navigation: %w", err)
	}

	// Create list layouts (all relevant fields)
	// Field names must match JSON keys from API
	s.createLayout(ctx, orgID, "Contact", "list",
		`["salutationName","firstName","lastName","emailAddress","phoneNumber","accountId","addressCity","createdAt"]`, now)
	s.createLayout(ctx, orgID, "Account", "list",
		`["name","type","industry","phoneNumber","emailAddress","website","billingAddressCity","createdAt"]`, now)
	s.createLayout(ctx, orgID, "Task", "list",
		`["subject","status","priority","type","dueDate","parentName","createdAt"]`, now)
	s.createLayout(ctx, orgID, "Quote", "list",
		`["name","quoteNumber","status","accountId","contactId","grandTotal","validUntil","createdAt"]`, now)

	// Create detail layouts (all fields organized into sections)
	// Field names must match JSON keys from API
	s.createLayout(ctx, orgID, "Contact", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"salutationName"},{"field":"accountId"}],
			[{"field":"firstName"},{"field":"lastName"}],
			[{"field":"emailAddress"},{"field":"phoneNumber"}],
			[{"field":"phoneNumberType"},{"field":"doNotCall"}]
		]},
		{"label":"Address","rows":[
			[{"field":"addressStreet"}],
			[{"field":"addressCity"},{"field":"addressState"}],
			[{"field":"addressPostalCode"},{"field":"addressCountry"}]
		]},
		{"label":"Description","rows":[
			[{"field":"description"}]
		]}
	]`, now)

	s.createLayout(ctx, orgID, "Account", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"name"},{"field":"website"}],
			[{"field":"emailAddress"},{"field":"phoneNumber"}],
			[{"field":"type"},{"field":"industry"}],
			[{"field":"sicCode"},{"field":"stage"}]
		]},
		{"label":"Billing Address","rows":[
			[{"field":"billingAddressStreet"}],
			[{"field":"billingAddressCity"},{"field":"billingAddressState"}],
			[{"field":"billingAddressPostalCode"},{"field":"billingAddressCountry"}]
		]},
		{"label":"Shipping Address","rows":[
			[{"field":"shippingAddressStreet"}],
			[{"field":"shippingAddressCity"},{"field":"shippingAddressState"}],
			[{"field":"shippingAddressPostalCode"},{"field":"shippingAddressCountry"}]
		]},
		{"label":"Description","rows":[
			[{"field":"description"}]
		]}
	]`, now)

	s.createLayout(ctx, orgID, "Task", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"subject"}],
			[{"field":"status"},{"field":"priority"}],
			[{"field":"type"},{"field":"dueDate"}],
			[{"field":"parentName"},{"field":"parentType"}]
		]},
		{"label":"Description","rows":[
			[{"field":"description"}]
		]}
	]`, now)

	s.createLayout(ctx, orgID, "Quote", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"name"},{"field":"quoteNumber"}],
			[{"field":"status"},{"field":"validUntil"}],
			[{"field":"accountId"},{"field":"contactId"}],
			[{"field":"currency"}]
		]},
		{"label":"Totals","rows":[
			[{"field":"subtotal"},{"field":"discountPercent"}],
			[{"field":"discountAmount"},{"field":"taxPercent"}],
			[{"field":"taxAmount"},{"field":"shippingAmount"}],
			[{"field":"grandTotal"}]
		]},
		{"label":"Billing Address","rows":[
			[{"field":"billingAddressStreet"}],
			[{"field":"billingAddressCity"},{"field":"billingAddressState"}],
			[{"field":"billingAddressPostalCode"},{"field":"billingAddressCountry"}]
		]},
		{"label":"Shipping Address","rows":[
			[{"field":"shippingAddressStreet"}],
			[{"field":"shippingAddressCity"},{"field":"shippingAddressState"}],
			[{"field":"shippingAddressPostalCode"},{"field":"shippingAddressCountry"}]
		]},
		{"label":"Additional Info","rows":[
			[{"field":"description"}],
			[{"field":"terms"}],
			[{"field":"notes"}]
		]}
	]`, now)

	// --- Create bearings (stage progress indicators) ---
	s.createBearing(ctx, orgID, "Quote", "Quote Status", "status", 1, now)

	// Verify layouts were created
	var layoutCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layout_defs WHERE org_id = ?", orgID).Scan(&layoutCount)
	if err != nil {
		log.Printf("[Provisioning] Warning: failed to verify layout count: %v", err)
	} else {
		log.Printf("[Provisioning] Verified %d layouts created for org %s", layoutCount, orgID)
	}

	// Create default matching rules for duplicate detection
	s.createDefaultMatchingRules(ctx, orgID, now)

	log.Printf("[Provisioning] Completed metadata provisioning for org %s", orgID)
	return nil
}

// createDefaultMatchingRules creates example matching rules so users can see how they work
func (s *ProvisioningService) createDefaultMatchingRules(ctx context.Context, orgID, now string) {
	// Contact matching rule: Email + Name
	contactRuleID := sfid.New("0Mr")
	contactFieldConfigs := `[
		{"fieldName":"emailAddress","weight":60,"algorithm":"email","threshold":0.95,"exactMatchBoost":true},
		{"fieldName":"lastName","weight":25,"algorithm":"jaro_winkler","threshold":0.88},
		{"fieldName":"firstName","weight":15,"algorithm":"jaro_winkler","threshold":0.85}
	]`
	contactMergeFields := `["firstName","lastName","emailAddress","phoneNumber","accountId","description"]`

	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO matching_rules (id, org_id, name, description, entity_type, is_enabled, priority,
			threshold, high_confidence_threshold, medium_confidence_threshold, blocking_strategy,
			field_configs, merge_display_fields, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, 0.70, 0.95, 0.85, 'multi', ?, ?, ?, ?)
	`, contactRuleID, orgID, "Contact Email Match",
		"Finds duplicate contacts by matching email address (60%) and name similarity (40%). High confidence when email matches exactly.",
		"Contact", contactFieldConfigs, contactMergeFields, now, now)
	if err != nil {
		log.Printf("[Provisioning] Warning: failed to create Contact matching rule: %v", err)
	} else {
		log.Printf("[Provisioning] Created Contact matching rule for org %s", orgID)
	}

	// Account matching rule: Name + Website
	accountRuleID := sfid.New("0Mr")
	accountFieldConfigs := `[
		{"fieldName":"name","weight":50,"algorithm":"jaro_winkler","threshold":0.90},
		{"fieldName":"website","weight":30,"algorithm":"exact","threshold":1.0,"exactMatchBoost":true},
		{"fieldName":"emailAddress","weight":20,"algorithm":"email","threshold":0.95}
	]`
	accountMergeFields := `["name","website","emailAddress","phoneNumber","industry","description"]`

	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO matching_rules (id, org_id, name, description, entity_type, is_enabled, priority,
			threshold, high_confidence_threshold, medium_confidence_threshold, blocking_strategy,
			field_configs, merge_display_fields, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, 0.75, 0.95, 0.85, 'prefix', ?, ?, ?, ?)
	`, accountRuleID, orgID, "Account Name Match",
		"Finds duplicate accounts by matching company name (50%), website (30%), and email domain (20%). Uses name prefix blocking for performance.",
		"Account", accountFieldConfigs, accountMergeFields, now, now)
	if err != nil {
		log.Printf("[Provisioning] Warning: failed to create Account matching rule: %v", err)
	} else {
		log.Printf("[Provisioning] Created Account matching rule for org %s", orgID)
	}
}

// createSampleData inserts sample records so new orgs aren't empty
func (s *ProvisioningService) createSampleData(ctx context.Context, orgID, now string) {
	// --- 10 Sample Accounts ---
	accounts := []struct {
		name, website, email, phone, typ, industry, stage string
		street, city, state, country, postal              string
		description                                       string
	}{
		{"Acme Corporation", "https://acme.com", "info@acme.com", "(555) 100-1000", "Customer", "Manufacturing", "Active",
			"100 Industrial Way", "Chicago", "IL", "United States", "60601", "Large manufacturing client with multiple locations."},
		{"TechStart Inc", "https://techstart.io", "hello@techstart.io", "(555) 100-2000", "Prospect", "Technology", "Negotiation",
			"200 Innovation Drive", "San Francisco", "CA", "United States", "94105", "Fast-growing SaaS startup in Series B."},
		{"Global Finance Ltd", "https://globalfinance.com", "contact@globalfinance.com", "(555) 100-3000", "Customer", "Finance", "Active",
			"300 Wall Street", "New York", "NY", "United States", "10005", "Investment banking and wealth management firm."},
		{"HealthCare Plus", "https://healthcareplus.org", "admin@healthcareplus.org", "(555) 100-4000", "Partner", "Healthcare", "Active",
			"400 Medical Center Blvd", "Boston", "MA", "United States", "02115", "Regional healthcare provider network."},
		{"Green Energy Solutions", "https://greenenergy.co", "sales@greenenergy.co", "(555) 100-5000", "Prospect", "Energy", "Qualification",
			"500 Renewable Road", "Austin", "TX", "United States", "78701", "Solar and wind energy installation company."},
		{"Retail Giants Co", "https://retailgiants.com", "support@retailgiants.com", "(555) 100-6000", "Customer", "Retail", "Active",
			"600 Shopping Plaza", "Atlanta", "GA", "United States", "30301", "National retail chain with 500+ locations."},
		{"EduTech Academy", "https://edutech.edu", "info@edutech.edu", "(555) 100-7000", "Prospect", "Education", "Discovery",
			"700 Campus Drive", "Seattle", "WA", "United States", "98101", "Online learning platform for K-12."},
		{"Construction Pro LLC", "https://constructionpro.net", "bids@constructionpro.net", "(555) 100-8000", "Customer", "Construction", "Active",
			"800 Builder Lane", "Denver", "CO", "United States", "80201", "Commercial construction and project management."},
		{"Media Dynamics", "https://mediadynamics.tv", "press@mediadynamics.tv", "(555) 100-9000", "Partner", "Media", "Active",
			"900 Broadcast Ave", "Los Angeles", "CA", "United States", "90028", "Digital media and content production studio."},
		{"FoodService Express", "https://foodserviceexpress.com", "orders@foodserviceexpress.com", "(555) 100-0000", "Prospect", "Food & Beverage", "Proposal",
			"1000 Culinary Court", "Miami", "FL", "United States", "33101", "Restaurant supply chain and distribution."},
	}

	accountIDs := make([]string, len(accounts))
	for i, a := range accounts {
		accountIDs[i] = sfid.NewAccount()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO accounts (id, org_id, name, website, email_address, phone_number, type, industry, stage,
				billing_address_street, billing_address_city, billing_address_state, billing_address_country, billing_address_postal_code,
				description, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, accountIDs[i], orgID, a.name, a.website, a.email, a.phone, a.typ, a.industry, a.stage,
			a.street, a.city, a.state, a.country, a.postal, a.description, now, now)
		if err != nil {
			log.Printf("Warning: failed to create account %s: %v", a.name, err)
		}
	}

	// --- 10 Sample Contacts ---
	contacts := []struct {
		salutation, first, last, email, phone, phoneType string
		street, city, state, country, postal             string
		accountIdx                                       int
		description                                      string
	}{
		{"Mr.", "John", "Anderson", "j.anderson@acme.com", "(555) 200-1001", "Office", "100 Industrial Way", "Chicago", "IL", "United States", "60601", 0, "Primary contact for Acme. Decision maker."},
		{"Ms.", "Sarah", "Chen", "sarah.chen@techstart.io", "(555) 200-2001", "Mobile", "200 Innovation Drive", "San Francisco", "CA", "United States", "94105", 1, "CEO and founder. Very responsive."},
		{"Dr.", "Michael", "Roberts", "m.roberts@globalfinance.com", "(555) 200-3001", "Office", "300 Wall Street", "New York", "NY", "United States", "10005", 2, "Chief Investment Officer."},
		{"Mrs.", "Emily", "Thompson", "e.thompson@healthcareplus.org", "(555) 200-4001", "Mobile", "400 Medical Center Blvd", "Boston", "MA", "United States", "02115", 3, "VP of Operations. Handles vendor relationships."},
		{"Mr.", "David", "Martinez", "d.martinez@greenenergy.co", "(555) 200-5001", "Office", "500 Renewable Road", "Austin", "TX", "United States", "78701", 4, "Sales Director. Interested in expansion."},
		{"Ms.", "Jennifer", "Wilson", "j.wilson@retailgiants.com", "(555) 200-6001", "Mobile", "600 Shopping Plaza", "Atlanta", "GA", "United States", "30301", 5, "Procurement Manager. Budget authority up to $500K."},
		{"Mr.", "Robert", "Taylor", "r.taylor@edutech.edu", "(555) 200-7001", "Office", "700 Campus Drive", "Seattle", "WA", "United States", "98101", 6, "Head of Technology. Evaluating platforms."},
		{"Mrs.", "Lisa", "Brown", "l.brown@constructionpro.net", "(555) 200-8001", "Mobile", "800 Builder Lane", "Denver", "CO", "United States", "80201", 7, "Project Manager. Manages large accounts."},
		{"Mr.", "James", "Garcia", "j.garcia@mediadynamics.tv", "(555) 200-9001", "Office", "900 Broadcast Ave", "Los Angeles", "CA", "United States", "90028", 8, "Creative Director. Partnership opportunities."},
		{"Ms.", "Amanda", "Lee", "a.lee@foodserviceexpress.com", "(555) 200-0001", "Mobile", "1000 Culinary Court", "Miami", "FL", "United States", "33101", 9, "Supply Chain Manager. Looking for efficiency."},
	}

	contactIDs := make([]string, len(contacts))
	for i, c := range contacts {
		contactIDs[i] = sfid.NewContact()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO contacts (id, org_id, salutation_name, first_name, last_name,
				email_address, phone_number, phone_number_type, do_not_call,
				description, address_street, address_city, address_state, address_country, address_postal_code,
				account_id, account_name, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, contactIDs[i], orgID, c.salutation, c.first, c.last, c.email, c.phone, c.phoneType,
			c.description, c.street, c.city, c.state, c.country, c.postal,
			accountIDs[c.accountIdx], accounts[c.accountIdx].name, now, now)
		if err != nil {
			log.Printf("Warning: failed to create contact %s %s: %v", c.first, c.last, err)
		}
	}

	// --- 10 Sample Tasks ---
	tasks := []struct {
		subject, description, status, priority, taskType string
		daysFromNow                                       int
		accountIdx                                        int
	}{
		{"Follow up on proposal", "Send follow-up email regarding the Q1 proposal we submitted last week.", "Open", "High", "Email", 1, 1},
		{"Schedule demo call", "Arrange product demonstration for the technical team.", "Open", "Normal", "Call", 3, 4},
		{"Review contract terms", "Legal review of the updated service agreement.", "In Progress", "High", "Todo", 2, 2},
		{"Prepare quarterly report", "Compile sales metrics and account health for QBR.", "Open", "Normal", "Todo", 7, 0},
		{"Site visit planning", "Coordinate logistics for on-site assessment next month.", "Open", "Low", "Meeting", 14, 7},
		{"Send pricing update", "Notify customer of new pricing effective next quarter.", "Completed", "Normal", "Email", -5, 5},
		{"Technical support call", "Address integration issues reported by IT team.", "In Progress", "Urgent", "Call", 0, 3},
		{"Partnership discussion", "Explore co-marketing opportunities.", "Open", "Normal", "Meeting", 5, 8},
		{"Invoice follow-up", "Check on outstanding payment from last month.", "Open", "High", "Call", 2, 0},
		{"Onboarding kickoff", "Schedule kickoff meeting for new implementation.", "Open", "Normal", "Meeting", 10, 9},
	}

	for _, t := range tasks {
		taskID := sfid.New("0Ts")
		dueDate := time.Now().UTC().AddDate(0, 0, t.daysFromNow).Format("2006-01-02")
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tasks (id, org_id, subject, description, status, priority, type, due_date,
				parent_id, parent_type, parent_name, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'Account', ?, ?, ?, 0, '{}')
		`, taskID, orgID, t.subject, t.description, t.status, t.priority, t.taskType, dueDate,
			accountIDs[t.accountIdx], accounts[t.accountIdx].name, now, now)
		if err != nil {
			log.Printf("Warning: failed to create task %s: %v", t.subject, err)
		}
	}

	// --- 10 Sample Quotes ---
	type lineItemData struct {
		name, desc, sku    string
		qty, price         float64
		discountPct        float64
		discountAmt        float64
		taxPct             float64
	}
	quotes := []struct {
		name, status      string
		accountIdx        int
		contactIdx        int
		subtotal          float64
		discountPct       float64
		taxPct            float64
		shipping          float64
		validDaysFromNow  int
		description       string
		lineItems         []lineItemData
	}{
		{"Enterprise Software License", "Accepted", 0, 0, 75000.00, 10, 8.25, 0, 30,
			"Annual enterprise license agreement with premium support.",
			[]lineItemData{
				{"Enterprise License", "Annual software license - 500 seats", "ENT-LIC-500", 1, 50000, 0, 0, 0},
				{"Premium Support", "24/7 support with 2hr SLA", "SUP-PREM-24", 1, 25000, 0, 0, 0},
			}},
		{"Cloud Migration Project", "Sent", 1, 1, 45000.00, 5, 0, 0, 14,
			"Phase 1 cloud migration including assessment and planning.",
			[]lineItemData{
				{"Migration Assessment", "Infrastructure and application audit", "SVC-MIG-ASSESS", 1, 15000, 0, 0, 0},
				{"Migration Services", "Lift and shift to AWS", "SVC-MIG-HOUR", 200, 150, 0, 0, 0},
			}},
		{"Financial Analytics Platform", "Draft", 2, 2, 120000.00, 15, 8.875, 500, 45,
			"Custom analytics dashboard with real-time market data feeds.",
			[]lineItemData{
				{"Platform License", "Analytics platform - unlimited users", "FIN-ANLYTC-UNL", 1, 80000, 10, 0, 8.875},
				{"Data Feeds", "Real-time market data - 12 months", "DATA-MKT-MO", 12, 2500, 0, 0, 8.875},
				{"Implementation", "Setup and configuration", "SVC-IMPL-HR", 40, 250, 5, 0, 8.875},
			}},
		{"Healthcare IT Upgrade", "Accepted", 3, 3, 85000.00, 0, 0, 2500, 60,
			"EHR system upgrade and staff training program.",
			[]lineItemData{
				{"EHR Upgrade", "System upgrade to v12", "HC-EHR-UPG", 1, 60000, 0, 0, 0},
				{"Training", "On-site training sessions", "SVC-TRAIN-DAY", 50, 500, 0, 0, 0},
			}},
		{"Solar Panel Installation", "Proposal", 4, 4, 32000.00, 8, 6.25, 1500, 21,
			"Commercial rooftop solar installation - 50kW system.",
			[]lineItemData{
				{"Solar Panels", "400W commercial panels", "SOLAR-400W", 125, 200, 5, 0, 6.25},
				{"Installation", "Labor and mounting hardware", "SVC-INSTALL", 1, 7000, 0, 500, 6.25},
			}},
		{"POS System Rollout", "Sent", 5, 5, 250000.00, 12, 7.5, 5000, 30,
			"Point of sale system for 100 retail locations.",
			[]lineItemData{
				{"POS Hardware", "Terminal + printer + scanner", "POS-HW-FULL", 100, 1500, 10, 0, 7.5},
				{"POS Software", "Cloud POS subscription - annual", "POS-SW-YR", 100, 1000, 15, 0, 7.5},
			}},
		{"LMS Implementation", "Draft", 6, 6, 28000.00, 0, 10.25, 0, 14,
			"Learning management system setup for online courses.",
			[]lineItemData{
				{"LMS Platform", "Annual subscription", "LMS-SUB-YR", 1, 18000, 0, 0, 10.25},
				{"Content Migration", "Transfer existing courses", "SVC-MIGRATE", 1, 5000, 0, 0, 10.25},
				{"Custom Branding", "Theme and logo customization", "SVC-BRAND", 1, 5000, 0, 0, 10.25},
			}},
		{"Project Management Software", "Accepted", 7, 7, 15600.00, 5, 4.55, 0, 90,
			"Project management tool for construction teams.",
			[]lineItemData{
				{"PM Software", "Annual license - 50 users", "PM-LIC-USER", 50, 300, 0, 0, 4.55},
				{"Mobile App", "Field worker mobile access", "PM-MOBILE", 50, 12, 0, 0, 4.55},
			}},
		{"Video Production Package", "Rejected", 8, 8, 95000.00, 0, 9.5, 0, -30,
			"Full video production for marketing campaign.",
			[]lineItemData{
				{"Video Production", "4 promotional videos", "VID-PROD", 4, 15000, 0, 0, 9.5},
				{"Animation", "Motion graphics package", "VID-ANIM-PKG", 1, 20000, 0, 0, 9.5},
				{"Editing", "Post-production hours", "VID-EDIT-HR", 50, 300, 0, 0, 9.5},
			}},
		{"Supply Chain Software", "Draft", 9, 9, 42000.00, 7, 7, 750, 45,
			"Inventory and logistics management platform.",
			[]lineItemData{
				{"SCM Platform", "Supply chain management - annual", "SCM-ENT-YR", 1, 30000, 5, 0, 7},
				{"Integration", "ERP integration services", "SVC-ERP-INT", 60, 200, 0, 0, 7},
			}},
	}

	for i, q := range quotes {
		quoteID := sfid.New("0Qt")
		quoteNumber := fmt.Sprintf("Q-2024-%04d", i+1)
		validUntil := time.Now().UTC().AddDate(0, 0, q.validDaysFromNow).Format("2006-01-02")

		discountAmt := q.subtotal * (q.discountPct / 100)
		afterDiscount := q.subtotal - discountAmt
		taxAmt := afterDiscount * (q.taxPct / 100)
		grandTotal := afterDiscount + taxAmt + q.shipping

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO quotes (id, org_id, name, quote_number, status,
				account_id, account_name, contact_id, contact_name,
				valid_until, subtotal, discount_percent, discount_amount, tax_percent, tax_amount,
				shipping_amount, grand_total, currency, description,
				created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'USD', ?, ?, ?, 0, '{}')
		`, quoteID, orgID, q.name, quoteNumber, q.status,
			accountIDs[q.accountIdx], accounts[q.accountIdx].name,
			contactIDs[q.contactIdx], contacts[q.contactIdx].first+" "+contacts[q.contactIdx].last,
			validUntil, q.subtotal, q.discountPct, discountAmt, q.taxPct, taxAmt,
			q.shipping, grandTotal, q.description, now, now)
		if err != nil {
			log.Printf("Warning: failed to create quote %s: %v", q.name, err)
			continue
		}

		// Create line items for this quote
		for j, li := range q.lineItems {
			lineItemID := sfid.New("0Ql")
			// Calculate line total with discounts
			lineTotal := li.qty * li.price
			if li.discountPct > 0 {
				lineTotal -= lineTotal * (li.discountPct / 100)
			} else if li.discountAmt > 0 {
				lineTotal -= li.discountAmt
			}
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO quote_line_items (id, org_id, quote_id, name, description, sku, quantity, unit_price,
					discount_percent, discount_amount, tax_percent, total, sort_order, created_at, modified_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, lineItemID, orgID, quoteID, li.name, li.desc, li.sku, li.qty, li.price,
				li.discountPct, li.discountAmt, li.taxPct, lineTotal, j, now, now)
			if err != nil {
				log.Printf("Warning: failed to create line item %s: %v", li.name, err)
			}
		}
	}

	// --- Default Quote PDF Template ---
	pdfTemplateID := sfid.New("0Pt")
	defaultBranding := `{"companyName":"","logoUrl":"","primaryColor":"#2563eb","accentColor":"#1e40af","fontFamily":"Helvetica, Arial, sans-serif"}`
	defaultSections := `[
		{"id":"header","label":"Header","enabled":true,"fields":["companyName","logo","quoteNumber","status","createdAt"]},
		{"id":"customer","label":"Customer Information","enabled":true,"fields":["accountName","contactName","billingAddress","shippingAddress"]},
		{"id":"lineItems","label":"Line Items","enabled":true,"fields":["name","description","sku","quantity","unitPrice","discountPercent","total"]},
		{"id":"totals","label":"Totals","enabled":true,"fields":["subtotal","discount","tax","shipping","grandTotal"]},
		{"id":"terms","label":"Terms & Conditions","enabled":true,"fields":["terms"]},
		{"id":"notes","label":"Notes","enabled":true,"fields":["notes"]},
		{"id":"footer","label":"Footer","enabled":true,"fields":["validUntil","thankYou"]}
	]`
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pdf_templates (id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at)
		VALUES (?, ?, ?, 'Quote', 1, 1, 'professional', ?, ?, 'A4', 'portrait', '10mm,10mm,10mm,10mm', ?, ?)
	`, pdfTemplateID, orgID, "Standard Quote", defaultBranding, defaultSections, now, now)
	if err != nil {
		log.Printf("Warning: failed to create default PDF template: %v", err)
	}

	log.Printf("Created sample data for org %s: 10 accounts, 10 contacts, 10 tasks, 10 quotes, 1 PDF template", orgID)
}

func (s *ProvisioningService) createEntity(ctx context.Context, orgID, name, plural, now string) error {
	id := sfid.New("0Et")

	// Get entity-specific display and search fields
	displayField, searchFields := getEntityLookupConfig(name)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entity_defs (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, display_field, search_fields, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, '', '', 0, 1, 0, 0, ?, ?, ?, ?)
	`, id, orgID, name, name, plural, displayField, searchFields, now, now)
	return err
}

// getEntityLookupConfig returns the display_field and search_fields for a given entity
// These are used by the lookup API to build search queries and display names
func getEntityLookupConfig(entityName string) (displayField string, searchFields string) {
	switch entityName {
	case "Contact":
		return "first_name || ' ' || last_name", `["first_name", "last_name", "email_address"]`
	case "Account":
		return "name", `["name", "email_address", "website"]`
	case "Task":
		// Tasks use 'subject' not 'name' for their title
		return "subject", `["subject"]`
	case "Quote":
		return "name", `["name", "quote_number"]`
	case "QuoteLineItem":
		return "name", `["name", "sku"]`
	default:
		// Default for custom entities
		return "name", `["name"]`
	}
}

func (s *ProvisioningService) createField(ctx context.Context, orgID, entity, name, label, typ string, required bool, order int, now string) {
	id := sfid.New("0Fd")
	reqInt := 0
	if required {
		reqInt = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?, ?)
	`, id, orgID, entity, name, label, typ, reqInt, order, now, now)
	if err != nil {
		log.Printf("Warning: failed to create field %s.%s: %v", entity, name, err)
	}
}

// createLinkField creates a lookup/link field with proper link_entity for related list discovery
func (s *ProvisioningService) createLinkField(ctx context.Context, orgID, entity, name, label, linkEntity string, order int, now string) {
	id := sfid.New("0Fd")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, link_entity, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, 'link', 0, 0, 0, 0, ?, ?, ?, ?)
	`, id, orgID, entity, name, label, order, linkEntity, now, now)
	if err != nil {
		log.Printf("Warning: failed to create link field %s.%s: %v", entity, name, err)
	}
}

func (s *ProvisioningService) createNavTabWithError(ctx context.Context, orgID, label, href, entity string, order int, isSystem bool, now string) error {
	id := sfid.New("0Nt")
	isSystemVal := 0
	if isSystem {
		isSystemVal = 1
	}
	// Use INSERT OR REPLACE to fix stale/broken tab data during reprovision
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO navigation_tabs (id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at)
		VALUES (?, ?, ?, ?, '', ?, ?, 1, ?, ?, ?)
	`, id, orgID, label, href, entity, order, isSystemVal, now, now)
	if err != nil {
		return fmt.Errorf("failed to create nav tab %s for org %s: %w", label, orgID, err)
	}
	return nil
}

func (s *ProvisioningService) createNavTab(ctx context.Context, orgID, label, href, entity string, order int, isSystem bool, now string) {
	if err := s.createNavTabWithError(ctx, orgID, label, href, entity, order, isSystem, now); err != nil {
		log.Printf("[Provisioning] Warning: %v", err)
	}
}

func (s *ProvisioningService) createLayout(ctx context.Context, orgID, entity, layoutType, layoutData, now string) {
	id := sfid.New("0Ly")
	// Use INSERT OR REPLACE to handle potential duplicates (e.g., if provisioning runs twice)
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO layout_defs (id, org_id, entity_name, layout_type, layout_data, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, orgID, entity, layoutType, layoutData, now, now)
	if err != nil {
		log.Printf("[Provisioning] ERROR: failed to create %s layout for %s (org=%s): %v", layoutType, entity, orgID, err)
	} else {
		log.Printf("[Provisioning] Created %s layout for %s (org=%s)", layoutType, entity, orgID)
	}
}

// createEnumField creates an enum field with options
func (s *ProvisioningService) createEnumField(ctx context.Context, orgID, entity, name, label string, options []string, order int, now string) {
	id := sfid.New("0Fd")
	// Convert options slice to JSON array string
	optionsJSON := "["
	for i, opt := range options {
		if i > 0 {
			optionsJSON += ","
		}
		optionsJSON += fmt.Sprintf("%q", opt)
	}
	optionsJSON += "]"

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, options, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, 'enum', 0, 0, 0, 0, ?, ?, ?, ?)
	`, id, orgID, entity, name, label, order, optionsJSON, now, now)
	if err != nil {
		log.Printf("Warning: failed to create enum field %s.%s: %v", entity, name, err)
	}
}

// createBearing creates a bearing (stage progress indicator) for an entity
func (s *ProvisioningService) createBearing(ctx context.Context, orgID, entityType, name, sourcePicklist string, displayOrder int, now string) {
	id := sfid.New("0Br")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO bearing_configs (id, org_id, entity_type, name, source_picklist, display_order, active, confirm_backward, allow_updates, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, 0, 1, ?, ?)
	`, id, orgID, entityType, name, sourcePicklist, displayOrder, now, now)
	if err != nil {
		log.Printf("Warning: failed to create bearing %s for %s: %v", name, entityType, err)
	}
}
