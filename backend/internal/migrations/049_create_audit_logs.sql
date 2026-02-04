-- Create audit logs table with hash chain for tamper detection
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    actor_id TEXT,
    actor_email TEXT,
    target_id TEXT,
    target_type TEXT,
    ip_address TEXT,
    user_agent TEXT,
    details TEXT,
    success INTEGER NOT NULL DEFAULT 1,
    error_msg TEXT,
    prev_hash TEXT NOT NULL,
    entry_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for timeline queries (most common: show recent audit events)
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_created ON audit_logs(org_id, created_at DESC);

-- Index for filtering by event type
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_type ON audit_logs(org_id, event_type);

-- Index for filtering by actor (who did what)
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(org_id, actor_id);

-- Index for chain verification (traverse hash chain)
CREATE INDEX IF NOT EXISTS idx_audit_logs_prev_hash ON audit_logs(prev_hash);
