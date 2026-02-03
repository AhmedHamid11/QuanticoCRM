CREATE TABLE IF NOT EXISTS quotes (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    quote_number TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'Draft',
    account_id TEXT,
    account_name TEXT DEFAULT '',
    contact_id TEXT,
    contact_name TEXT DEFAULT '',
    valid_until TEXT DEFAULT '',
    subtotal REAL DEFAULT 0,
    discount_percent REAL DEFAULT 0,
    discount_amount REAL DEFAULT 0,
    tax_percent REAL DEFAULT 0,
    tax_amount REAL DEFAULT 0,
    shipping_amount REAL DEFAULT 0,
    grand_total REAL DEFAULT 0,
    currency TEXT DEFAULT 'USD',
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
    terms TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    assigned_user_id TEXT,
    created_by_id TEXT,
    modified_by_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted INTEGER DEFAULT 0,
    custom_fields TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_quotes_org_id ON quotes(org_id);
CREATE INDEX IF NOT EXISTS idx_quotes_status ON quotes(org_id, status, deleted);
CREATE INDEX IF NOT EXISTS idx_quotes_account ON quotes(account_id, deleted);
CREATE INDEX IF NOT EXISTS idx_quotes_contact ON quotes(contact_id, deleted);
CREATE INDEX IF NOT EXISTS idx_quotes_quote_number ON quotes(org_id, quote_number);
CREATE INDEX IF NOT EXISTS idx_quotes_created_at ON quotes(created_at, deleted);
CREATE UNIQUE INDEX IF NOT EXISTS idx_quotes_created_at_id ON quotes(created_at, id);

CREATE TABLE IF NOT EXISTS quote_line_items (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    quote_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    description TEXT DEFAULT '',
    sku TEXT DEFAULT '',
    quantity REAL DEFAULT 1,
    unit_price REAL DEFAULT 0,
    discount_percent REAL DEFAULT 0,
    discount_amount REAL DEFAULT 0,
    tax_percent REAL DEFAULT 0,
    total REAL DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (quote_id) REFERENCES quotes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_quote_line_items_quote ON quote_line_items(quote_id);
CREATE INDEX IF NOT EXISTS idx_quote_line_items_org ON quote_line_items(org_id);
