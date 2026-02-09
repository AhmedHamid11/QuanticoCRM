-- Add token family tracking for refresh token rotation security
ALTER TABLE sessions ADD COLUMN family_id TEXT;
ALTER TABLE sessions ADD COLUMN is_revoked INTEGER DEFAULT 0;

-- Add session timeout tracking
ALTER TABLE sessions ADD COLUMN last_activity_at TEXT;
ALTER TABLE sessions ADD COLUMN idle_timeout_minutes INTEGER;
ALTER TABLE sessions ADD COLUMN absolute_timeout_minutes INTEGER;

-- Indexes for token family lookups
CREATE INDEX IF NOT EXISTS idx_sessions_family ON sessions(family_id);
