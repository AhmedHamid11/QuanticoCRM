-- Migration 012: Create tripwires tables
-- Tripwires are webhook triggers that fire API calls when entity records are created, updated, or deleted

-- Org-level webhook auth settings
CREATE TABLE IF NOT EXISTS org_webhook_settings (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL UNIQUE,
    auth_type TEXT NOT NULL DEFAULT 'none',  -- 'none', 'api_key', 'bearer', 'custom_header'
    api_key TEXT,
    bearer_token TEXT,
    custom_header_name TEXT,
    custom_header_value TEXT,
    timeout_ms INTEGER DEFAULT 5000,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_webhook_settings_org ON org_webhook_settings(org_id);

-- Tripwire definitions
CREATE TABLE IF NOT EXISTS tripwires (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    entity_type TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    condition_logic TEXT DEFAULT 'AND',      -- 'AND', 'OR', or future custom expressions
    conditions TEXT NOT NULL DEFAULT '[]',   -- JSON array of TripwireCondition
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_by TEXT,
    modified_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_tripwires_org ON tripwires(org_id);
CREATE INDEX IF NOT EXISTS idx_tripwires_entity ON tripwires(org_id, entity_type, enabled);
CREATE INDEX IF NOT EXISTS idx_tripwires_enabled ON tripwires(org_id, enabled);

-- Execution logs (for debugging and audit)
CREATE TABLE IF NOT EXISTS tripwire_logs (
    id TEXT PRIMARY KEY,
    tripwire_id TEXT NOT NULL,
    tripwire_name TEXT,
    org_id TEXT NOT NULL,
    record_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    event_type TEXT NOT NULL,         -- 'CREATE', 'UPDATE', 'DELETE'
    status TEXT NOT NULL,             -- 'success', 'failed', 'timeout'
    response_code INTEGER,
    error_message TEXT,
    duration_ms INTEGER,
    executed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tripwire_logs_tripwire ON tripwire_logs(tripwire_id);
CREATE INDEX IF NOT EXISTS idx_tripwire_logs_org ON tripwire_logs(org_id);
CREATE INDEX IF NOT EXISTS idx_tripwire_logs_executed ON tripwire_logs(executed_at);
CREATE INDEX IF NOT EXISTS idx_tripwire_logs_status ON tripwire_logs(org_id, status);
