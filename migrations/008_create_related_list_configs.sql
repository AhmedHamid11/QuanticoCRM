-- Migration: Create related_list_configs table
-- Stores configuration for related lists (Salesforce-style) on entity detail pages

CREATE TABLE IF NOT EXISTS related_list_configs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    related_entity TEXT NOT NULL,
    lookup_field TEXT NOT NULL,
    label TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    is_multi_lookup INTEGER DEFAULT 0,
    display_fields TEXT NOT NULL DEFAULT '[]', -- JSON array of FieldConfig
    sort_order INTEGER DEFAULT 0,
    default_sort TEXT,
    default_sort_dir TEXT DEFAULT 'desc',
    page_size INTEGER DEFAULT 5,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_type, related_entity, lookup_field)
);

-- Index for efficient lookup by entity type
CREATE INDEX IF NOT EXISTS idx_related_list_entity ON related_list_configs(org_id, entity_type, enabled);

-- Index for sorting
CREATE INDEX IF NOT EXISTS idx_related_list_sort ON related_list_configs(org_id, entity_type, sort_order);
