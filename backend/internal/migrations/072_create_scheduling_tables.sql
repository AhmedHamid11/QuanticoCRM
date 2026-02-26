-- Migration 064: Create scheduling tables for lightweight Calendly-like scheduling tool

-- Google Calendar connections (per-user, tenant table)
CREATE TABLE IF NOT EXISTS google_calendar_connections (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    access_token_encrypted BLOB,
    refresh_token_encrypted BLOB,
    token_expiry DATETIME,
    calendar_id TEXT DEFAULT 'primary',
    connected_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, user_id)
);

-- Scheduling pages (each user can have multiple booking pages)
CREATE TABLE IF NOT EXISTS scheduling_pages (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    duration_minutes INTEGER NOT NULL DEFAULT 30,
    availability_json TEXT NOT NULL DEFAULT '{}',
    timezone TEXT NOT NULL DEFAULT 'America/New_York',
    is_active INTEGER NOT NULL DEFAULT 1,
    buffer_minutes INTEGER NOT NULL DEFAULT 0,
    max_days_ahead INTEGER NOT NULL DEFAULT 30,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(slug)
);

-- Bookings made by external visitors
CREATE TABLE IF NOT EXISTS scheduling_bookings (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    scheduling_page_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    guest_name TEXT NOT NULL,
    guest_email TEXT NOT NULL,
    guest_notes TEXT DEFAULT '',
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'confirmed',
    google_event_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scheduling_page_id) REFERENCES scheduling_pages(id)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_scheduling_pages_org_user ON scheduling_pages(org_id, user_id);
CREATE INDEX IF NOT EXISTS idx_scheduling_pages_slug ON scheduling_pages(slug);
CREATE INDEX IF NOT EXISTS idx_scheduling_bookings_page ON scheduling_bookings(scheduling_page_id);
CREATE INDEX IF NOT EXISTS idx_scheduling_bookings_user_range ON scheduling_bookings(user_id, org_id, start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_gcal_connections_org_user ON google_calendar_connections(org_id, user_id);
