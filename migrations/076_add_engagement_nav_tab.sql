-- Add Engagement nav tab to all existing orgs that don't already have one
-- This ensures orgs provisioned before v7.0 get access to /engagement/tasks in the top nav
INSERT OR IGNORE INTO navigation_tabs (id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at)
SELECT
    'nav_engagement_' || org_id,
    org_id,
    'Engagement',
    '/engagement/tasks',
    'engagement',
    '',
    5,
    1,
    1,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM (SELECT DISTINCT org_id FROM navigation_tabs) AS orgs
WHERE NOT EXISTS (
    SELECT 1 FROM navigation_tabs nt2
    WHERE nt2.org_id = orgs.org_id AND nt2.href = '/engagement/tasks'
);
