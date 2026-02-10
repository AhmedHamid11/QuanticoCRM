-- Migration: Create contacts table
-- Based on EspoCRM Contact entity definition

CREATE TABLE IF NOT EXISTS contacts (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    salutation_name TEXT DEFAULT '',
    first_name TEXT DEFAULT '',
    last_name TEXT NOT NULL,
    email_address TEXT DEFAULT '',
    phone_number TEXT DEFAULT '',
    phone_number_type TEXT DEFAULT 'Mobile',
    do_not_call INTEGER DEFAULT 0,
    description TEXT DEFAULT '',
    address_street TEXT DEFAULT '',
    address_city TEXT DEFAULT '',
    address_state TEXT DEFAULT '',
    address_country TEXT DEFAULT '',
    address_postal_code TEXT DEFAULT '',
    account_id TEXT,
    assigned_user_id TEXT,
    created_by_id TEXT,
    modified_by_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted INTEGER DEFAULT 0
);

-- Indexes based on EspoCRM entity definition
CREATE INDEX IF NOT EXISTS idx_contacts_org_id ON contacts(org_id);
CREATE INDEX IF NOT EXISTS idx_contacts_created_at ON contacts(created_at, deleted);
CREATE INDEX IF NOT EXISTS idx_contacts_first_name ON contacts(first_name, deleted);
CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(first_name, last_name);
CREATE INDEX IF NOT EXISTS idx_contacts_assigned_user ON contacts(assigned_user_id, deleted);
CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts(account_id, deleted);
CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(email_address);

-- Unique index for ordering
CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_created_at_id ON contacts(created_at, id);
