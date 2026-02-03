-- Migration: Add Quote status options and bearing for existing orgs

-- First, ensure the Quote status field exists in field_defs for all orgs that have the quotes table
-- This handles orgs created before Quote was added to provisioning
INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, options, created_at, modified_at)
SELECT
    '0Fd' || substr(hex(randomblob(8)), 1, 16) as id,
    o.id as org_id,
    'Quote' as entity_name,
    'status' as name,
    'Status' as label,
    'enum' as type,
    0 as is_required,
    0 as is_read_only,
    0 as is_audited,
    0 as is_custom,
    3 as sort_order,
    '["Draft","Needs Review","Approved","Sent","Accepted","Declined","Expired"]' as options,
    datetime('now') as created_at,
    datetime('now') as modified_at
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM field_defs fd
    WHERE fd.org_id = o.id
      AND fd.entity_name = 'Quote'
      AND fd.name = 'status'
);

-- Update Quote status field to have options (for existing orgs where it was created without options)
UPDATE field_defs
SET options = '["Draft","Needs Review","Approved","Sent","Accepted","Declined","Expired"]'
WHERE entity_name = 'Quote'
  AND name = 'status'
  AND (options IS NULL OR options = '' OR options = '[]');

-- Create Quote Status bearing for all orgs that don't have one
INSERT OR IGNORE INTO bearing_configs (id, org_id, entity_type, name, source_picklist, display_order, active, confirm_backward, allow_updates, created_at, modified_at)
SELECT
    '0Br' || substr(hex(randomblob(8)), 1, 16) as id,
    o.id as org_id,
    'Quote' as entity_type,
    'Quote Status' as name,
    'status' as source_picklist,
    1 as display_order,
    1 as active,
    0 as confirm_backward,
    1 as allow_updates,
    datetime('now') as created_at,
    datetime('now') as modified_at
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM bearing_configs bc
    WHERE bc.org_id = o.id
      AND bc.entity_type = 'Quote'
      AND bc.source_picklist = 'status'
);
