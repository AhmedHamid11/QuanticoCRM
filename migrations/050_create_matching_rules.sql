-- Matching rules for duplicate detection
-- Stores configurable rules for identifying duplicates within or across entities
-- Each rule defines field comparison configs, thresholds, and blocking strategies

CREATE TABLE IF NOT EXISTS matching_rules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    entity_type TEXT NOT NULL,            -- Source entity: "Contact", "Lead", "Account", etc.
    target_entity_type TEXT,              -- For cross-entity rules (e.g., Contact-Lead dedup)
    is_enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 0,           -- Lower = higher priority (first matching rule wins)
    threshold REAL NOT NULL DEFAULT 0.70, -- Minimum score to be considered a match
    high_confidence_threshold REAL DEFAULT 0.95,   -- Auto-merge safe
    medium_confidence_threshold REAL DEFAULT 0.85, -- Needs review
    blocking_strategy TEXT NOT NULL DEFAULT '',       -- Deprecated: engine auto-detects blocking keys

    -- field_configs: JSON array of FieldConfig
    -- Example: [
    --   {
    --     "fieldName": "lastName",
    --     "targetFieldName": "lastName",  // optional for cross-entity
    --     "weight": 40.0,                 // 0-100 contribution to total score
    --     "algorithm": "jaro_winkler",    // exact, jaro_winkler, email, phone, phonetic
    --     "threshold": 0.88,              // per-field threshold
    --     "exactMatchBoost": true         // auto-high confidence on exact match
    --   },
    --   {
    --     "fieldName": "email",
    --     "weight": 60.0,
    --     "algorithm": "email"
    --   }
    -- ]
    field_configs TEXT NOT NULL,

    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(org_id, entity_type, name)
);

-- Index for listing rules by org and entity
CREATE INDEX IF NOT EXISTS idx_matching_rules_org ON matching_rules(org_id, entity_type, is_enabled);

-- Index for priority-based rule selection
CREATE INDEX IF NOT EXISTS idx_matching_rules_priority ON matching_rules(org_id, is_enabled, priority);
