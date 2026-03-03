-- Add missing columns to sessions table that Go code expects but migration 017 didn't create
-- These columns are required by CreateSessionWithFamily() in repo/auth.go

ALTER TABLE sessions ADD COLUMN family_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN is_revoked INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN last_activity_at TEXT DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE sessions ADD COLUMN idle_timeout_minutes INTEGER DEFAULT 30;
ALTER TABLE sessions ADD COLUMN absolute_timeout_minutes INTEGER DEFAULT 1440;
