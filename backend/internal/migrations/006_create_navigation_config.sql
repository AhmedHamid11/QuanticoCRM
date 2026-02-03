-- Migration: Create navigation configuration table
-- Stores the toolbar/navigation tab configuration

CREATE TABLE IF NOT EXISTS navigation_tabs (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    href TEXT NOT NULL,
    icon TEXT DEFAULT '',
    entity_name TEXT,  -- Optional: links to entity_defs for entity-based tabs
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_visible INTEGER DEFAULT 1,
    is_system INTEGER DEFAULT 0,  -- System tabs cannot be deleted, only hidden
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Index for ordering
CREATE INDEX IF NOT EXISTS idx_navigation_tabs_order ON navigation_tabs(sort_order);

-- Insert default navigation tabs
INSERT OR IGNORE INTO navigation_tabs (id, label, href, icon, entity_name, sort_order, is_visible, is_system) VALUES
('nav_contacts', 'Contacts', '/contacts', 'users', 'Contact', 1, 1, 1),
('nav_accounts', 'Accounts', '/accounts', 'building', 'Account', 2, 1, 1),
('nav_admin', 'Admin', '/admin', 'settings', NULL, 100, 1, 1);
