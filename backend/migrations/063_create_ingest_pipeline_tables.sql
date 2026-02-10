-- Migration 063: Create ingest pipeline tables
-- Phase 21 Plan 01: Ingest Pipeline & Delta Engine - Data Layer

-- Ingest jobs track async processing status
CREATE TABLE IF NOT EXISTS ingest_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    mirror_id TEXT NOT NULL,
    key_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'accepted',
    records_received INTEGER NOT NULL DEFAULT 0,
    records_processed INTEGER NOT NULL DEFAULT 0,
    records_promoted INTEGER NOT NULL DEFAULT 0,
    records_skipped INTEGER NOT NULL DEFAULT 0,
    records_failed INTEGER NOT NULL DEFAULT 0,
    errors TEXT DEFAULT '[]',
    warnings TEXT DEFAULT '[]',
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ingest_jobs_org ON ingest_jobs(org_id);
CREATE INDEX IF NOT EXISTS idx_ingest_jobs_mirror ON ingest_jobs(org_id, mirror_id);
CREATE INDEX IF NOT EXISTS idx_ingest_jobs_status ON ingest_jobs(org_id, status);

-- Delta key index for deduplication
CREATE TABLE IF NOT EXISTS ingest_delta_keys (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    mirror_id TEXT NOT NULL,
    unique_key TEXT NOT NULL,
    record_id TEXT,
    ingested_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(mirror_id, unique_key)
);
CREATE INDEX IF NOT EXISTS idx_delta_keys_mirror ON ingest_delta_keys(mirror_id);
CREATE INDEX IF NOT EXISTS idx_delta_keys_lookup ON ingest_delta_keys(mirror_id, unique_key);

-- Add map_field column to mirror_source_fields for field mapping
-- This extends the existing mirror_source_fields table from migration 062
ALTER TABLE mirror_source_fields ADD COLUMN map_field TEXT;
