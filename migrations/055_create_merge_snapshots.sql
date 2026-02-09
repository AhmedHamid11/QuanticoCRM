-- Merge snapshots table for undo capability
-- Stores pre-merge state of all affected records and FK changes
CREATE TABLE IF NOT EXISTS merge_snapshots (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    survivor_id TEXT NOT NULL,
    survivor_before TEXT NOT NULL,
    duplicate_ids TEXT NOT NULL,
    duplicate_snapshots TEXT NOT NULL,
    related_record_fks TEXT NOT NULL,
    merged_by_id TEXT NOT NULL,
    consumed_at TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_merge_snapshots_org ON merge_snapshots(org_id);
CREATE INDEX IF NOT EXISTS idx_merge_snapshots_survivor ON merge_snapshots(org_id, survivor_id);
CREATE INDEX IF NOT EXISTS idx_merge_snapshots_expires ON merge_snapshots(expires_at);
