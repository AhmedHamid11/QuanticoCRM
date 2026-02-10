-- Migration 062: Add API usage tracking for Salesforce rate limiting
-- Table: api_usage_log (tenant DB) - tracks API calls per org over rolling 24-hour windows

CREATE TABLE IF NOT EXISTS api_usage_log (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    api_calls INTEGER NOT NULL DEFAULT 1,
    job_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_usage_org_time ON api_usage_log(org_id, timestamp);

-- Add api_calls_made column to sync_jobs for per-job tracking
-- NOTE: SQLite doesn't support IF NOT EXISTS for ALTER TABLE
-- The migration_propagator should handle "duplicate column" errors gracefully
ALTER TABLE sync_jobs ADD COLUMN api_calls_made INTEGER NOT NULL DEFAULT 0;
