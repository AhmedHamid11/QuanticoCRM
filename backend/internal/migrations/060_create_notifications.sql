-- Notifications table for in-app notifications
-- One record per user per notification event
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,                -- 'scan_complete', 'scan_failed'
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    link_url TEXT,                     -- URL to navigate when clicked
    is_read INTEGER NOT NULL DEFAULT 0,
    is_dismissed INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT                    -- Auto-dismiss after 30 days
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(org_id, user_id, is_dismissed, created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_expiry ON notifications(expires_at);
