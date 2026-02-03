-- Migration: Create accounts table
-- Standard Account entity for CRM

CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    website TEXT DEFAULT '',
    email_address TEXT DEFAULT '',
    phone_number TEXT DEFAULT '',
    type TEXT DEFAULT '',
    industry TEXT DEFAULT '',
    sic_code TEXT DEFAULT '',
    billing_address_street TEXT DEFAULT '',
    billing_address_city TEXT DEFAULT '',
    billing_address_state TEXT DEFAULT '',
    billing_address_country TEXT DEFAULT '',
    billing_address_postal_code TEXT DEFAULT '',
    shipping_address_street TEXT DEFAULT '',
    shipping_address_city TEXT DEFAULT '',
    shipping_address_state TEXT DEFAULT '',
    shipping_address_country TEXT DEFAULT '',
    shipping_address_postal_code TEXT DEFAULT '',
    description TEXT DEFAULT '',
    assigned_user_id TEXT,
    created_by_id TEXT,
    modified_by_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}'
);

-- Indexes for accounts
CREATE INDEX IF NOT EXISTS idx_accounts_org_id ON accounts(org_id);
CREATE INDEX IF NOT EXISTS idx_accounts_name ON accounts(name, deleted);
CREATE INDEX IF NOT EXISTS idx_accounts_created_at ON accounts(created_at, deleted);
CREATE INDEX IF NOT EXISTS idx_accounts_assigned_user ON accounts(assigned_user_id, deleted);
CREATE INDEX IF NOT EXISTS idx_accounts_type ON accounts(type, deleted);
CREATE INDEX IF NOT EXISTS idx_accounts_industry ON accounts(industry, deleted);

-- Unique index for ordering
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_created_at_id ON accounts(created_at, id);

-- Add foreign key constraint for contacts.account_id (SQLite supports this)
-- Note: SQLite doesn't enforce FK by default, but we define the relationship
