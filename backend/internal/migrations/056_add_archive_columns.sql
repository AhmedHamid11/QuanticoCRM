-- Archive columns are added dynamically to entity tables at merge time.
-- This migration documents the expected schema and adds columns to known standard tables.
-- The merge service will ALTER TABLE to add these columns if missing before archiving.

-- For each entity table (contacts, accounts, tasks, quotes, etc.):
-- ALTER TABLE {table} ADD COLUMN archived_at TEXT DEFAULT NULL;
-- ALTER TABLE {table} ADD COLUMN archived_reason TEXT DEFAULT NULL;
-- ALTER TABLE {table} ADD COLUMN survivor_id TEXT DEFAULT NULL;

-- Pre-add to known standard entity tables that are most likely to be merged
-- Using separate statements since SQLite doesn't support IF NOT EXISTS for ALTER TABLE

-- We use a defensive approach: try to add, ignore error if column already exists
-- The Go code handles this via SyncFieldColumns pattern
SELECT 1; -- Placeholder, actual column additions happen in Go merge service
