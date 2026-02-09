-- Add family_id and is_revoked columns for token rotation security
-- family_id groups refresh tokens so all can be revoked on reuse detection
-- is_revoked marks individual sessions as revoked
ALTER TABLE sessions ADD COLUMN family_id TEXT;
ALTER TABLE sessions ADD COLUMN is_revoked INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sessions_family ON sessions(family_id);
