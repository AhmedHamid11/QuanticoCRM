-- Scan jobs table for execution tracking
-- Tracks individual scan runs with status and progress
CREATE TABLE IF NOT EXISTS scan_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    schedule_id TEXT,                  -- NULL for manual "Run Now" jobs
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'completed', 'failed', 'cancelled'
    trigger_type TEXT NOT NULL DEFAULT 'scheduled', -- 'scheduled', 'manual'
    total_records INTEGER NOT NULL DEFAULT 0,
    processed_records INTEGER NOT NULL DEFAULT 0,
    duplicates_found INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_scan_jobs_org_status ON scan_jobs(org_id, status);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_org_entity ON scan_jobs(org_id, entity_type, created_at);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_schedule ON scan_jobs(schedule_id, created_at);
