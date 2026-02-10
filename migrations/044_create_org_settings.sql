-- Migration: Create org_settings table for per-org configuration
-- Stores settings like homepage, branding, etc.

CREATE TABLE IF NOT EXISTS org_settings (
    org_id TEXT PRIMARY KEY,
    home_page TEXT DEFAULT '/',
    settings_json TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);
