-- Migration 078: SFDC Mirror Extensions
-- Adds sequence enrollment triggers, SFDC activity writebacks, mirror source watermarks,
-- and upsert_mode column to mirrors table.

-- sequence_enrollment_triggers: stores field-match rules that auto-enroll contacts
-- into sequences when a Mirror ingest promotes a matching record.
CREATE TABLE IF NOT EXISTS sequence_enrollment_triggers (
    id              TEXT NOT NULL PRIMARY KEY,
    sequence_id     TEXT NOT NULL,
    org_id          TEXT NOT NULL,
    target_entity   TEXT NOT NULL,
    field_name      TEXT NOT NULL,
    operator        TEXT NOT NULL,  -- "eq" | "neq"
    value           TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_set_org_entity ON sequence_enrollment_triggers (org_id, target_entity);
CREATE INDEX IF NOT EXISTS idx_set_sequence_id ON sequence_enrollment_triggers (sequence_id);

-- sfdc_activity_writebacks: queues SFDC Task write-backs for completed sequence steps.
-- step_execution_id is unique so each step execution produces at most one writeback.
CREATE TABLE IF NOT EXISTS sfdc_activity_writebacks (
    id                  TEXT NOT NULL PRIMARY KEY,
    org_id              TEXT NOT NULL,
    step_execution_id   TEXT NOT NULL,
    enrollment_id       TEXT NOT NULL,
    contact_id          TEXT NOT NULL,
    sfdc_contact_id     TEXT,
    status              TEXT NOT NULL DEFAULT 'pending',
    sfdc_task_id        TEXT,
    batch_job_id        TEXT,
    error_message       TEXT,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (step_execution_id)
);

CREATE INDEX IF NOT EXISTS idx_saw_org_status ON sfdc_activity_writebacks (org_id, status);
CREATE INDEX IF NOT EXISTS idx_saw_step_execution_id ON sfdc_activity_writebacks (step_execution_id);

-- mirror_source_watermarks: tracks the last successful ingest timestamp per mirror.
-- Used by n8n to fetch only changed SFDC records since the last sync.
CREATE TABLE IF NOT EXISTS mirror_source_watermarks (
    id                  TEXT NOT NULL PRIMARY KEY,
    org_id              TEXT NOT NULL,
    mirror_id           TEXT NOT NULL,
    last_ingest_at      TEXT,
    last_ingest_count   INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (org_id, mirror_id)
);

CREATE INDEX IF NOT EXISTS idx_msw_org_id ON mirror_source_watermarks (org_id);

-- Add upsert_mode to mirrors: when true, promotes records using INSERT OR REPLACE
-- so updated SFDC records overwrite existing rows instead of being skipped.
ALTER TABLE mirrors ADD COLUMN upsert_mode INTEGER NOT NULL DEFAULT 0;
