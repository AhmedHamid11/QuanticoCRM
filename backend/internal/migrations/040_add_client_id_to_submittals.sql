-- Add client_id column to submittals table for direct Client relationship
-- This enables the "Placements" related list on Client detail pages

-- Add client_id column
ALTER TABLE submittals ADD COLUMN client_id TEXT;

-- Add index for efficient queries
CREATE INDEX IF NOT EXISTS idx_submittals_client ON submittals(client_id);

-- Backfill client_id from job_openings relationship
UPDATE submittals
SET client_id = (
    SELECT j.client_id
    FROM job_openings j
    WHERE j.id = submittals.job_opening_id
)
WHERE client_id IS NULL AND job_opening_id IS NOT NULL;

-- Add field definition for clientId as a link field (for orgs that have Submittal entity)
INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, sort_order, link_entity, created_at, modified_at)
SELECT
    'fld_submittal_clientid_' || org_id,
    org_id,
    'Submittal',
    'clientId',
    'Client',
    'link',
    0,
    3,
    'Client',
    datetime('now'),
    datetime('now')
FROM entity_defs
WHERE name = 'Submittal';
