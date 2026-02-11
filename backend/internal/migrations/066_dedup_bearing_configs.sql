-- Migration: Deduplicate bearing_configs and add UNIQUE constraint
-- Root cause: tenant_provisioning.go created bearing_configs without the
-- UNIQUE(org_id, entity_type, source_picklist) constraint that migration 013
-- defined. Each "Repair Metadata" run inserted another duplicate bearing.

-- Step 1: Create new table with the UNIQUE constraint
CREATE TABLE IF NOT EXISTS bearing_configs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    source_picklist TEXT NOT NULL,
    display_order INTEGER DEFAULT 1,
    active INTEGER DEFAULT 1,
    confirm_backward INTEGER DEFAULT 0,
    allow_updates INTEGER DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_type, source_picklist)
);

-- Step 2: Copy one row per (org_id, entity_type, source_picklist), keeping the earliest
INSERT OR IGNORE INTO bearing_configs_new
    (id, org_id, entity_type, name, source_picklist, display_order, active, confirm_backward, allow_updates, created_at, modified_at)
SELECT id, org_id, entity_type, name, source_picklist, display_order, active, confirm_backward, allow_updates, created_at, modified_at
FROM bearing_configs
ORDER BY created_at ASC;

-- Step 3: Swap tables
DROP TABLE IF EXISTS bearing_configs;

ALTER TABLE bearing_configs_new RENAME TO bearing_configs;

-- Step 4: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_bearing_entity ON bearing_configs(org_id, entity_type, active);

CREATE INDEX IF NOT EXISTS idx_bearing_sort ON bearing_configs(org_id, entity_type, display_order);
