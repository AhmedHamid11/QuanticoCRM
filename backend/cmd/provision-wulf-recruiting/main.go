package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"

	"github.com/fastcrm/backend/internal/service"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Command line flags
	orgID := flag.String("org-id", "", "Organization ID to provision (required)")
	dbPath := flag.String("db", "", "Path to SQLite database (defaults to fastcrm.db in current dir)")
	createTables := flag.Bool("create-tables", true, "Create data tables for recruiting entities")
	withSampleData := flag.Bool("sample-data", true, "Include sample data from Wulf Recruiting spreadsheets")
	flag.Parse()

	if *orgID == "" {
		log.Fatal("Error: --org-id is required")
	}

	// Determine database path
	dbFile := *dbPath
	if dbFile == "" {
		dbFile = "fastcrm.db"
		// Check common locations
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
	}

	log.Printf("Using database: %s", dbFile)
	log.Printf("Provisioning org: %s", *orgID)

	// Connect to database
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create recruiting data tables if requested
	if *createTables {
		log.Println("Creating recruiting data tables...")
		if err := createRecruitingTables(ctx, db); err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		log.Println("Tables created successfully")
	}

	// Run provisioning
	provSvc := service.NewProvisioningService(db)

	if *withSampleData {
		log.Println("Running full provisioning with sample data...")
		if err := provSvc.ProvisionWulfRecruitingComplete(ctx, *orgID); err != nil {
			log.Fatalf("Failed to provision: %v", err)
		}
	} else {
		log.Println("Running metadata-only provisioning...")
		if err := provSvc.ProvisionWulfRecruiting(ctx, *orgID); err != nil {
			log.Fatalf("Failed to provision metadata: %v", err)
		}
		provSvc.ProvisionNavigation(ctx, *orgID)
	}

	log.Println("Wulf Recruiting provisioning completed successfully!")
	log.Println("")
	log.Println("Entities created:")
	log.Println("  - Client (recruiting client companies)")
	log.Println("  - ClientContact (contacts at client companies)")
	log.Println("  - Candidate (people being recruited)")
	log.Println("  - JobOpening (job orders/JO's)")
	log.Println("  - Submittal (pipeline tracking)")
	log.Println("  - Activity (activity log)")
	log.Println("  - Invoice (billing)")
}

func createRecruitingTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		// Clients table
		`CREATE TABLE IF NOT EXISTS clients (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			industry TEXT,
			website TEXT,
			phone_number TEXT,
			email_address TEXT,
			address_street TEXT,
			address_city TEXT,
			address_state TEXT,
			address_country TEXT,
			address_postal_code TEXT,
			contract_terms TEXT,
			contract_signed_date TEXT,
			client_since TEXT,
			status TEXT DEFAULT 'Active',
			account_manager TEXT,
			notes TEXT,
			openings_summary TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,

		// Client Contacts table
		`CREATE TABLE IF NOT EXISTS client_contacts (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			client_name TEXT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			role TEXT,
			email TEXT,
			phone TEXT,
			is_primary INTEGER DEFAULT 0,
			notes TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}',
			FOREIGN KEY (client_id) REFERENCES clients(id)
		)`,

		// Candidates table
		`CREATE TABLE IF NOT EXISTS candidates (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email TEXT,
			phone TEXT,
			phone_type TEXT,
			address_city TEXT,
			address_state TEXT,
			address_country TEXT DEFAULT 'US',
			willing_to_relocate INTEGER DEFAULT 0,
			relocation_areas TEXT,
			geo_range TEXT,
			current_salary TEXT,
			current_bonus TEXT,
			salary_expectations TEXT,
			current_employer TEXT,
			current_title TEXT,
			position_type TEXT,
			industry_experience TEXT,
			years_experience INTEGER,
			status TEXT DEFAULT 'Active',
			is_placeable INTEGER DEFAULT 1,
			resume_url TEXT,
			notes TEXT,
			source TEXT,
			source_date TEXT,
			last_contacted_date TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,

		// Job Openings table
		`CREATE TABLE IF NOT EXISTS job_openings (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			jo_number TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			client_id TEXT,
			client_name TEXT,
			hiring_manager TEXT,
			city TEXT,
			state TEXT,
			country TEXT DEFAULT 'US',
			work_type TEXT DEFAULT 'On-site',
			salary_range TEXT,
			bonus_info TEXT,
			category TEXT DEFAULT 'B',
			status TEXT DEFAULT 'Open',
			date_posted TEXT,
			date_filled TEXT,
			owner TEXT,
			submittals_total INTEGER DEFAULT 0,
			notes TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}',
			FOREIGN KEY (client_id) REFERENCES clients(id)
		)`,

		// Submittals table (Pipeline)
		`CREATE TABLE IF NOT EXISTS submittals (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			candidate_id TEXT NOT NULL,
			candidate_name TEXT,
			job_opening_id TEXT NOT NULL,
			job_opening_title TEXT,
			jo_number TEXT,
			client_name TEXT,
			recruiter TEXT,
			stage TEXT DEFAULT 'Submitted',
			submitted_date TEXT,
			pi_1_date TEXT,
			pi_2_date TEXT,
			pi_3_date TEXT,
			onsite_1_date TEXT,
			onsite_2_date TEXT,
			offer_date TEXT,
			accepted_date TEXT,
			start_date TEXT,
			final_salary TEXT,
			commission_amount REAL,
			pipeline_days INTEGER,
			feedback TEXT,
			invoice_date TEXT,
			invoice_due_date TEXT,
			paid_date TEXT,
			paid_status TEXT,
			recruiter_payout REAL,
			notes TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}',
			FOREIGN KEY (candidate_id) REFERENCES candidates(id),
			FOREIGN KEY (job_opening_id) REFERENCES job_openings(id)
		)`,

		// Activities table
		`CREATE TABLE IF NOT EXISTS activities (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			parent_type TEXT NOT NULL,
			parent_id TEXT NOT NULL,
			parent_name TEXT,
			activity_type TEXT NOT NULL,
			subject TEXT,
			description TEXT,
			activity_date TEXT NOT NULL,
			created_by TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}'
		)`,

		// Invoices table
		`CREATE TABLE IF NOT EXISTS invoices (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			invoice_number TEXT NOT NULL,
			client_id TEXT,
			client_name TEXT,
			candidate_id TEXT,
			candidate_name TEXT,
			job_opening_id TEXT,
			position_title TEXT,
			hired_date TEXT,
			invoice_date TEXT,
			due_date TEXT,
			paid_date TEXT,
			base_salary REAL,
			fee_percentage REAL,
			fee_amount REAL,
			status TEXT DEFAULT 'Draft',
			recruiter_payout REAL,
			payout_date TEXT,
			notes TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0,
			custom_fields TEXT DEFAULT '{}',
			FOREIGN KEY (client_id) REFERENCES clients(id),
			FOREIGN KEY (candidate_id) REFERENCES candidates(id),
			FOREIGN KEY (job_opening_id) REFERENCES job_openings(id)
		)`,
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_clients_org ON clients(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_clients_name ON clients(org_id, name)",
		"CREATE INDEX IF NOT EXISTS idx_clients_status ON clients(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_client_contacts_org ON client_contacts(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_client_contacts_client ON client_contacts(client_id)",
		"CREATE INDEX IF NOT EXISTS idx_candidates_org ON candidates(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_candidates_name ON candidates(org_id, last_name, first_name)",
		"CREATE INDEX IF NOT EXISTS idx_candidates_status ON candidates(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_candidates_placeable ON candidates(org_id, is_placeable)",
		"CREATE INDEX IF NOT EXISTS idx_job_openings_org ON job_openings(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_job_openings_jo_number ON job_openings(org_id, jo_number)",
		"CREATE INDEX IF NOT EXISTS idx_job_openings_client ON job_openings(client_id)",
		"CREATE INDEX IF NOT EXISTS idx_job_openings_status ON job_openings(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_job_openings_category ON job_openings(org_id, category)",
		"CREATE INDEX IF NOT EXISTS idx_submittals_org ON submittals(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_submittals_candidate ON submittals(candidate_id)",
		"CREATE INDEX IF NOT EXISTS idx_submittals_job ON submittals(job_opening_id)",
		"CREATE INDEX IF NOT EXISTS idx_submittals_stage ON submittals(org_id, stage)",
		"CREATE INDEX IF NOT EXISTS idx_submittals_recruiter ON submittals(org_id, recruiter)",
		"CREATE INDEX IF NOT EXISTS idx_activities_org ON activities(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_activities_parent ON activities(parent_type, parent_id)",
		"CREATE INDEX IF NOT EXISTS idx_activities_date ON activities(org_id, activity_date)",
		"CREATE INDEX IF NOT EXISTS idx_invoices_org ON invoices(org_id)",
		"CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(org_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_invoices_client ON invoices(client_id)",
	}

	// Execute table creation
	for _, sql := range tables {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return err
		}
	}

	// Execute index creation
	for _, sql := range indexes {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			log.Printf("Warning: index creation failed (may already exist): %v", err)
		}
	}

	return nil
}
