-- Add session timeout columns for idle and absolute timeouts
-- These enable configurable per-org session timeout policies

ALTER TABLE sessions ADD COLUMN last_activity_at TEXT;
ALTER TABLE sessions ADD COLUMN idle_timeout_minutes INTEGER DEFAULT 30;
ALTER TABLE sessions ADD COLUMN absolute_timeout_minutes INTEGER DEFAULT 1440;

-- Create index for activity-based queries
CREATE INDEX IF NOT EXISTS idx_sessions_last_activity ON sessions(last_activity_at);

-- Update existing sessions with current timestamp as last activity
UPDATE sessions SET last_activity_at = CURRENT_TIMESTAMP WHERE last_activity_at IS NULL;
