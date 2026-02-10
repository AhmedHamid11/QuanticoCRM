CREATE TABLE IF NOT EXISTS ingest_api_keys (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    rate_limit INTEGER DEFAULT 500,
    created_by TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ingest_api_keys_org ON ingest_api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_ingest_api_keys_hash ON ingest_api_keys(key_hash);
