-- Migration 079: Phase 37 schema additions — call disposition shortcut column,
-- sequence versioning tables, and version_id on enrollments.
-- All tables are tenant-scoped — do NOT add any to masterOnlyTableNames.

-- Add disposition shortcut column to sequence_step_executions.
-- This stores the disposition string directly on the execution row for fast reads,
-- alongside the full call_dispositions record.
ALTER TABLE sequence_step_executions ADD COLUMN disposition TEXT;

-- sequence_versions: Immutable snapshot of a sequence's steps at activation time.
-- Allows enrolled contacts to continue on the version they were enrolled on
-- even after the sequence is edited and re-activated.
CREATE TABLE IF NOT EXISTS sequence_versions (
    id TEXT PRIMARY KEY,
    sequence_id TEXT NOT NULL,
    version_number INTEGER NOT NULL,
    steps_snapshot_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(sequence_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_seq_versions_sequence ON sequence_versions(sequence_id);

-- Add version_id to sequence_enrollments so each enrolled contact tracks
-- which version of the sequence steps they are following.
-- Nullable: NULL means the enrollment predates versioning (Phase 37).
ALTER TABLE sequence_enrollments ADD COLUMN version_id TEXT;
