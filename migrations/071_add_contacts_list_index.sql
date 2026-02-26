-- Composite index for the default contacts list query:
-- WHERE org_id = ? AND deleted = 0 ORDER BY created_at DESC LIMIT 20
-- Without this, SQLite scans all org contacts and sorts in memory.
CREATE INDEX IF NOT EXISTS idx_contacts_org_deleted_created
  ON contacts(org_id, deleted, created_at DESC);
