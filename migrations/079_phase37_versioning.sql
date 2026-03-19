-- Phase 37: Sequence versioning tables
-- Shared by Plan 01 and Plan 02. Created by Plan 02 since Plan 01 has not run.
-- sequence_versions: immutable snapshot of steps at the time a sequence is activated.
CREATE TABLE IF NOT EXISTS sequence_versions (
    id TEXT PRIMARY KEY,
    sequence_id TEXT NOT NULL,
    version_number INTEGER NOT NULL,
    steps_snapshot_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(sequence_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_seq_versions_sequence ON sequence_versions(sequence_id);

-- version_id on enrollments: pins each enrollment to the snapshot it was enrolled on.
-- NULL means the enrollment predates versioning (backward compatible).
ALTER TABLE sequence_enrollments ADD COLUMN version_id TEXT;
