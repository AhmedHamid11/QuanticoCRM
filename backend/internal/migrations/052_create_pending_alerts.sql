-- Pending duplicate alerts for async real-time detection
CREATE TABLE IF NOT EXISTS pending_duplicate_alerts (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    matches_json TEXT NOT NULL,
    total_match_count INTEGER NOT NULL,
    highest_confidence TEXT NOT NULL,
    is_block_mode INTEGER NOT NULL DEFAULT 0,
    detected_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    resolved_at TEXT,
    resolved_by_id TEXT,
    override_text TEXT,

    UNIQUE(org_id, entity_type, record_id, status)
);

CREATE INDEX IF NOT EXISTS idx_pending_alerts_record
    ON pending_duplicate_alerts(org_id, entity_type, record_id, status);
CREATE INDEX IF NOT EXISTS idx_pending_alerts_pending
    ON pending_duplicate_alerts(org_id, status, detected_at);
