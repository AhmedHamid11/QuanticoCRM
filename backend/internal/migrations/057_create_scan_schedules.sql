-- Scan schedules table for admin-configured background scans
-- One schedule per entity type per org
CREATE TABLE IF NOT EXISTS scan_schedules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    frequency TEXT NOT NULL,           -- 'daily', 'weekly', 'monthly'
    day_of_week INTEGER,               -- 0=Sunday..6=Saturday (for weekly)
    day_of_month INTEGER,              -- 1-28 (for monthly, cap at 28 to avoid edge cases)
    hour INTEGER NOT NULL DEFAULT 3,   -- 0-23, hour of day (UTC)
    minute INTEGER NOT NULL DEFAULT 0, -- 0-59
    is_enabled INTEGER NOT NULL DEFAULT 1,
    last_run_at TEXT,                  -- ISO timestamp of last execution
    next_run_at TEXT,                  -- ISO timestamp of next scheduled run
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_type)        -- One schedule per entity per org
);

CREATE INDEX IF NOT EXISTS idx_scan_schedules_org ON scan_schedules(org_id, is_enabled);
CREATE INDEX IF NOT EXISTS idx_scan_schedules_next_run ON scan_schedules(next_run_at, is_enabled);
