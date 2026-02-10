-- Migration: Create validation rules table
-- Validation rules evaluate BEFORE save operations and can block invalid changes

CREATE TABLE IF NOT EXISTS validation_rules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    entity_type TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    trigger_on_create INTEGER DEFAULT 0,
    trigger_on_update INTEGER DEFAULT 1,
    trigger_on_delete INTEGER DEFAULT 0,
    condition_logic TEXT DEFAULT 'AND',  -- 'AND' or 'OR'
    conditions TEXT NOT NULL DEFAULT '[]',  -- JSON array of ValidationCondition
    actions TEXT NOT NULL DEFAULT '[]',  -- JSON array of ValidationAction
    error_message TEXT,  -- Default error message for the rule
    priority INTEGER DEFAULT 100,  -- Lower number = higher priority
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    modified_by TEXT
);

-- Index for efficient rule lookup by entity type
CREATE INDEX IF NOT EXISTS idx_validation_rules_entity ON validation_rules(org_id, entity_type, enabled);

-- Index for priority ordering
CREATE INDEX IF NOT EXISTS idx_validation_rules_priority ON validation_rules(org_id, entity_type, priority);
