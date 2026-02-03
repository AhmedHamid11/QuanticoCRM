-- Migration run tracking for multi-tenant updates
CREATE TABLE IF NOT EXISTS migration_runs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    org_name TEXT NOT NULL,
    from_version TEXT NOT NULL,
    to_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'success', 'failed')),
    error_message TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_migration_runs_org ON migration_runs(org_id);
CREATE INDEX IF NOT EXISTS idx_migration_runs_status ON migration_runs(status);
CREATE INDEX IF NOT EXISTS idx_migration_runs_started ON migration_runs(started_at DESC);
