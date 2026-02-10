-- Salesforce sync integration tables for OAuth credentials, sync jobs, and field mappings

-- salesforce_connections: Stores per-org OAuth credentials (master DB)
CREATE TABLE IF NOT EXISTS salesforce_connections (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL UNIQUE,
    client_id TEXT NOT NULL,
    client_secret_encrypted BLOB NOT NULL,
    redirect_url TEXT NOT NULL,
    instance_url TEXT NOT NULL DEFAULT '',
    access_token_encrypted BLOB,
    refresh_token_encrypted BLOB,
    token_type TEXT DEFAULT 'Bearer',
    expires_at TEXT,
    is_enabled INTEGER DEFAULT 1,
    connected_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sf_conn_org ON salesforce_connections(org_id);

-- sync_jobs: Tracks status and progress of merge batch sync jobs (tenant DB)
CREATE TABLE IF NOT EXISTS sync_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    batch_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total_instructions INTEGER NOT NULL DEFAULT 0,
    delivered_instructions INTEGER NOT NULL DEFAULT 0,
    failed_instructions INTEGER NOT NULL DEFAULT 0,
    batch_payload TEXT,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT NOT NULL UNIQUE,
    trigger_type TEXT NOT NULL DEFAULT 'manual',
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_jobs_org_status ON sync_jobs(org_id, status);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_batch ON sync_jobs(batch_id);

-- salesforce_field_mappings: Maps Quantico fields to Salesforce object/field names (master DB)
CREATE TABLE IF NOT EXISTS salesforce_field_mappings (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    quantico_field TEXT NOT NULL,
    salesforce_object TEXT NOT NULL,
    salesforce_field TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, entity_type, quantico_field)
);

CREATE INDEX IF NOT EXISTS idx_sf_mapping_org_entity ON salesforce_field_mappings(org_id, entity_type);
