-- Migration 076: Create Gmail OAuth tokens and Salesforce CDC cursors for Sales Engagement Module (v7.0)
-- All tables are tenant-scoped (per-user-per-org) — do NOT add any to masterOnlyTableNames

-- gmail_oauth_tokens: Per-user Gmail OAuth credentials in tenant DB
CREATE TABLE IF NOT EXISTS gmail_oauth_tokens (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    access_token_encrypted BLOB,
    refresh_token_encrypted BLOB,
    token_expiry DATETIME,
    gmail_address TEXT,
    dns_spf_valid INTEGER NOT NULL DEFAULT 0,
    dns_dkim_valid INTEGER NOT NULL DEFAULT 0,
    dns_dmarc_valid INTEGER NOT NULL DEFAULT 0,
    connected_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_gmail_oauth_org_user ON gmail_oauth_tokens(org_id, user_id);

-- sfdc_cdc_cursors: Salesforce Change Data Capture replay cursors in tenant DB
CREATE TABLE IF NOT EXISTS sfdc_cdc_cursors (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    object_type TEXT NOT NULL,
    last_event_replay_id TEXT,
    last_polled_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, object_type)
);

CREATE INDEX IF NOT EXISTS idx_sfdc_cdc_org_object ON sfdc_cdc_cursors(org_id, object_type);
