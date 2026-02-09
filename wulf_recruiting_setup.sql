-- ============================================================================
-- WULF RECRUITING CRM SETUP
-- ============================================================================
-- This script provisions a complete recruiting CRM database for Wulf Recruiting.
-- Based on requirements from Excel spreadsheets and Word doc.
--
-- Entities:
--   1. Client - Recruiting client companies
--   2. ClientContact - Contacts at client companies
--   3. Candidate - People being recruited
--   4. JobOpening - Job orders (JO's)
--   5. Submittal - Pipeline tracking (candidate -> job opening)
--   6. Activity - Activity log for candidates
--   7. Invoice - Billing/payment tracking
--
-- Run this against a fresh org database after creating the org.
-- ============================================================================

-- Set org ID (replace with actual org ID when running)
-- You'll need to replace 'WULF_ORG_ID' with the actual org ID

-- ============================================================================
-- PART 1: CREATE DATA TABLES
-- ============================================================================

-- Clients table (recruiting client companies)
CREATE TABLE IF NOT EXISTS clients (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    industry TEXT,
    website TEXT,
    phone_number TEXT,
    email_address TEXT,

    -- Address
    address_street TEXT,
    address_city TEXT,
    address_state TEXT,
    address_country TEXT,
    address_postal_code TEXT,

    -- Contract info
    contract_terms TEXT,  -- e.g., "25%"
    contract_signed_date TEXT,
    client_since TEXT,

    -- Status
    status TEXT DEFAULT 'Active',  -- Active, Inactive, Prospect
    account_manager TEXT,

    -- Notes
    notes TEXT,
    openings_summary TEXT,  -- What types of openings they typically have

    -- Metadata
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_clients_org ON clients(org_id);
CREATE INDEX IF NOT EXISTS idx_clients_name ON clients(org_id, name);
CREATE INDEX IF NOT EXISTS idx_clients_status ON clients(org_id, status);

-- Client Contacts table (contacts at client companies)
CREATE TABLE IF NOT EXISTS client_contacts (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_name TEXT,

    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    role TEXT,  -- Job title/role at company
    email TEXT,
    phone TEXT,
    is_primary INTEGER DEFAULT 0,
    notes TEXT,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}',

    FOREIGN KEY (client_id) REFERENCES clients(id)
);

CREATE INDEX IF NOT EXISTS idx_client_contacts_org ON client_contacts(org_id);
CREATE INDEX IF NOT EXISTS idx_client_contacts_client ON client_contacts(client_id);

-- Candidates table (people being recruited)
CREATE TABLE IF NOT EXISTS candidates (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    -- Basic info
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    phone_type TEXT,  -- Mobile, Home, Work

    -- Current location
    address_city TEXT,
    address_state TEXT,
    address_country TEXT DEFAULT 'US',

    -- Relocation preferences
    willing_to_relocate INTEGER DEFAULT 0,
    relocation_areas TEXT,  -- JSON array or comma-separated
    geo_range TEXT,  -- Geographic range for job search

    -- Compensation
    current_salary TEXT,  -- Text to allow ranges like "$80-$90K"
    current_bonus TEXT,
    salary_expectations TEXT,  -- Text field for ranges

    -- Professional info
    current_employer TEXT,
    current_title TEXT,
    position_type TEXT,  -- What type of positions they're looking for
    industry_experience TEXT,
    years_experience INTEGER,

    -- Status
    status TEXT DEFAULT 'Active',  -- Active, Placed, Inactive, Do Not Contact
    is_placeable INTEGER DEFAULT 1,  -- Currently available for placement

    -- Resume/attachments stored as URLs or file paths
    resume_url TEXT,

    -- Notes
    notes TEXT,

    -- Source tracking
    source TEXT,  -- Where candidate came from
    source_date TEXT,

    -- Last contact
    last_contacted_date TEXT,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_candidates_org ON candidates(org_id);
CREATE INDEX IF NOT EXISTS idx_candidates_name ON candidates(org_id, last_name, first_name);
CREATE INDEX IF NOT EXISTS idx_candidates_status ON candidates(org_id, status);
CREATE INDEX IF NOT EXISTS idx_candidates_placeable ON candidates(org_id, is_placeable);

-- Job Openings table (JO's)
CREATE TABLE IF NOT EXISTS job_openings (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    -- JO identification
    jo_number TEXT NOT NULL,  -- e.g., "1251"

    -- Job details
    title TEXT NOT NULL,
    description TEXT,

    -- Client relationship
    client_id TEXT,
    client_name TEXT,
    hiring_manager TEXT,

    -- Location
    city TEXT,
    state TEXT,
    country TEXT DEFAULT 'US',
    work_type TEXT DEFAULT 'On-site',  -- Remote, On-site, Hybrid

    -- Compensation
    salary_range TEXT,  -- Text field: "$140-$160K + bonus (TBD)"
    bonus_info TEXT,    -- Text field: "up to 25% bonus, uncapped"

    -- Urgency/Priority
    category TEXT DEFAULT 'B',  -- AA, A, B, C, D (priority levels)

    -- Status
    status TEXT DEFAULT 'Open',  -- Open, On Hold, Filled, Cancelled
    date_posted TEXT,
    date_filled TEXT,

    -- Ownership
    owner TEXT,  -- Who's working this JO (recruiter name)

    -- Metrics
    submittals_total INTEGER DEFAULT 0,

    -- Notes
    notes TEXT,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}',

    FOREIGN KEY (client_id) REFERENCES clients(id)
);

CREATE INDEX IF NOT EXISTS idx_job_openings_org ON job_openings(org_id);
CREATE INDEX IF NOT EXISTS idx_job_openings_jo_number ON job_openings(org_id, jo_number);
CREATE INDEX IF NOT EXISTS idx_job_openings_client ON job_openings(client_id);
CREATE INDEX IF NOT EXISTS idx_job_openings_status ON job_openings(org_id, status);
CREATE INDEX IF NOT EXISTS idx_job_openings_category ON job_openings(org_id, category);

-- Submittals table (Pipeline - links candidates to job openings)
CREATE TABLE IF NOT EXISTS submittals (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    -- Relationships
    candidate_id TEXT NOT NULL,
    candidate_name TEXT,  -- Denormalized for display
    job_opening_id TEXT NOT NULL,
    job_opening_title TEXT,
    client_id TEXT,       -- Foreign key to clients
    client_name TEXT,     -- Denormalized for display
    jo_number TEXT,

    -- Recruiter
    recruiter TEXT,  -- Who submitted this candidate

    -- Pipeline stage
    stage TEXT DEFAULT 'Submitted',  -- Submitted, PI_1, PI_2, PI_3, Onsite_1, Onsite_2, Offer, Accepted, Started, Placed

    -- Stage dates (all optional, filled as candidate progresses)
    submitted_date TEXT,
    pi_1_date TEXT,      -- Phone Interview 1
    pi_2_date TEXT,      -- Phone Interview 2
    pi_3_date TEXT,      -- Phone Interview 3
    onsite_1_date TEXT,  -- On-site Interview 1
    onsite_2_date TEXT,  -- On-site Interview 2
    offer_date TEXT,
    accepted_date TEXT,
    start_date TEXT,

    -- Compensation (final)
    final_salary TEXT,

    -- Commission/Fee tracking
    commission_amount REAL,

    -- Pipeline metrics
    pipeline_days INTEGER,  -- Days from submittal to placement

    -- Feedback
    feedback TEXT,

    -- Invoice tracking (for placed candidates)
    invoice_date TEXT,
    invoice_due_date TEXT,
    paid_date TEXT,
    paid_status TEXT,  -- Pending, Paid, Overdue

    -- Recruiter payout
    recruiter_payout REAL,

    notes TEXT,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}',

    FOREIGN KEY (candidate_id) REFERENCES candidates(id),
    FOREIGN KEY (job_opening_id) REFERENCES job_openings(id),
    FOREIGN KEY (client_id) REFERENCES clients(id)
);

CREATE INDEX IF NOT EXISTS idx_submittals_org ON submittals(org_id);
CREATE INDEX IF NOT EXISTS idx_submittals_candidate ON submittals(candidate_id);
CREATE INDEX IF NOT EXISTS idx_submittals_job ON submittals(job_opening_id);
CREATE INDEX IF NOT EXISTS idx_submittals_client ON submittals(client_id);
CREATE INDEX IF NOT EXISTS idx_submittals_stage ON submittals(org_id, stage);
CREATE INDEX IF NOT EXISTS idx_submittals_recruiter ON submittals(org_id, recruiter);

-- Activity log table (for tracking candidate interactions)
CREATE TABLE IF NOT EXISTS activities (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    -- What this activity is about
    parent_type TEXT NOT NULL,  -- Candidate, Client, JobOpening, Submittal
    parent_id TEXT NOT NULL,
    parent_name TEXT,

    -- Activity details
    activity_type TEXT NOT NULL,  -- Call, Email, Note, Meeting, LinkedIn, Text
    subject TEXT,
    description TEXT,

    -- Date/time
    activity_date TEXT NOT NULL,

    -- Who did this
    created_by TEXT,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_activities_org ON activities(org_id);
CREATE INDEX IF NOT EXISTS idx_activities_parent ON activities(parent_type, parent_id);
CREATE INDEX IF NOT EXISTS idx_activities_date ON activities(org_id, activity_date);

-- Invoices table (for billing tracking)
CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    invoice_number TEXT NOT NULL,

    -- Related records
    client_id TEXT,
    client_name TEXT,
    candidate_id TEXT,
    candidate_name TEXT,
    job_opening_id TEXT,
    position_title TEXT,

    -- Dates
    hired_date TEXT,
    invoice_date TEXT,
    due_date TEXT,
    paid_date TEXT,

    -- Amounts
    base_salary REAL,
    fee_percentage REAL,  -- e.g., 25 for 25%
    fee_amount REAL,

    -- Status
    status TEXT DEFAULT 'Draft',  -- Draft, Sent, Paid, Overdue, Cancelled

    -- Payouts
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
);

CREATE INDEX IF NOT EXISTS idx_invoices_org ON invoices(org_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(org_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_client ON invoices(client_id);

-- ============================================================================
-- PART 2: RUN THE PROVISIONING
-- ============================================================================
--
-- After running this SQL to create the tables, use the Go provisioning service
-- to create the metadata and sample data:
--
-- Option 1: Via API (after creating org through normal signup):
--   POST /api/v1/admin/provision-recruiting
--
-- Option 2: Via Go code:
--   provisioningService.ProvisionWulfRecruitingComplete(ctx, orgID)
--
-- Option 3: Run the standalone provisioning command:
--   cd backend && go run cmd/provision-recruiting/main.go --org-id=YOUR_ORG_ID
--
-- ============================================================================

-- Example: You can verify tables were created with:
-- SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%client%';
-- SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%candidate%';
-- SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%job%';
-- SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%submittal%';

