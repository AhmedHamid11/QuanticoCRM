-- Migration: Create bearing_configs table
-- Bearings are visual stage progress indicators (like Salesforce Path component)
-- They display the current position within a workflow based on a picklist field

CREATE TABLE IF NOT EXISTS bearing_configs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    source_picklist TEXT NOT NULL,  -- The picklist field that drives the stages
    display_order INTEGER DEFAULT 1, -- Sort position (1-12 max per entity)
    active INTEGER DEFAULT 1,        -- Toggle to show/hide
    confirm_backward INTEGER DEFAULT 0, -- Confirm before backward movement
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_type, source_picklist)
);

-- Index for efficient lookup by entity type
CREATE INDEX IF NOT EXISTS idx_bearing_entity ON bearing_configs(org_id, entity_type, active);

-- Index for sorting
CREATE INDEX IF NOT EXISTS idx_bearing_sort ON bearing_configs(org_id, entity_type, display_order);
