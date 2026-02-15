-- Import dedup decision tracking tables
-- Stores import job metadata and dedup decisions so Salesforce can retrieve them via API

CREATE TABLE IF NOT EXISTS import_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    external_id_field TEXT,
    total_rows INTEGER DEFAULT 0,
    created_count INTEGER DEFAULT 0,
    updated_count INTEGER DEFAULT 0,
    skipped_count INTEGER DEFAULT 0,
    merged_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_org ON import_jobs(org_id);
CREATE INDEX IF NOT EXISTS idx_import_jobs_org_entity ON import_jobs(org_id, entity_type);

CREATE TABLE IF NOT EXISTS import_dedup_decisions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    import_job_id TEXT NOT NULL,
    decision_type TEXT NOT NULL,
    action TEXT NOT NULL,
    kept_external_id TEXT,
    discarded_external_id TEXT,
    match_field TEXT,
    match_value TEXT,
    matched_record_id TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_import_dedup_job ON import_dedup_decisions(import_job_id);
CREATE INDEX IF NOT EXISTS idx_import_dedup_org ON import_dedup_decisions(org_id);
