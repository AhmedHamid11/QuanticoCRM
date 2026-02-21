-- Add archive columns needed by the merge system.
-- These were supposed to be in 056_add_archive_columns.sql but that migration was a stub.

ALTER TABLE contacts ADD COLUMN archived_at TEXT;
ALTER TABLE contacts ADD COLUMN archived_reason TEXT;
ALTER TABLE contacts ADD COLUMN survivor_id TEXT;

ALTER TABLE accounts ADD COLUMN archived_at TEXT;
ALTER TABLE accounts ADD COLUMN archived_reason TEXT;
ALTER TABLE accounts ADD COLUMN survivor_id TEXT;

ALTER TABLE tasks ADD COLUMN archived_at TEXT;
ALTER TABLE tasks ADD COLUMN archived_reason TEXT;
ALTER TABLE tasks ADD COLUMN survivor_id TEXT;

ALTER TABLE quotes ADD COLUMN archived_at TEXT;
ALTER TABLE quotes ADD COLUMN archived_reason TEXT;
ALTER TABLE quotes ADD COLUMN survivor_id TEXT;
