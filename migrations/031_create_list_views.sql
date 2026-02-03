-- Migration: Create list_views table for saved filter configurations
-- List views allow users to save filter queries and column configurations

CREATE TABLE IF NOT EXISTS list_views (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_name TEXT NOT NULL,
    name TEXT NOT NULL,
    filter_query TEXT DEFAULT '',
    columns TEXT DEFAULT '[]',
    sort_by TEXT DEFAULT 'created_at',
    sort_dir TEXT DEFAULT 'desc',
    is_default INTEGER DEFAULT 0,
    is_system INTEGER DEFAULT 0,
    created_by_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_name, name)
);

CREATE INDEX IF NOT EXISTS idx_list_views_org_entity ON list_views(org_id, entity_name);
CREATE INDEX IF NOT EXISTS idx_list_views_default ON list_views(org_id, entity_name, is_default);
