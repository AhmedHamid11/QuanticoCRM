-- Migration: Create custom pages table
-- Database-driven pages with embeddable components (iframes, text, entity lists, etc.)

CREATE TABLE IF NOT EXISTS custom_pages (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    slug TEXT NOT NULL,                    -- URL-friendly identifier (e.g., "dashboard", "reports")
    title TEXT NOT NULL,                   -- Display title
    description TEXT,                      -- Optional description
    icon TEXT DEFAULT 'file',              -- Icon for navigation
    is_enabled INTEGER DEFAULT 1,          -- Whether the page is accessible
    is_public INTEGER DEFAULT 0,           -- Whether non-admins can view (when enabled)
    layout TEXT DEFAULT 'single',          -- 'single' (full width) or 'grid' (multi-column)
    components TEXT NOT NULL DEFAULT '[]', -- JSON array of page components
    sort_order INTEGER DEFAULT 0,          -- For ordering in lists
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_by TEXT,
    modified_by TEXT,
    UNIQUE(org_id, slug)
);

-- Index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_custom_pages_org_slug ON custom_pages(org_id, slug);
CREATE INDEX IF NOT EXISTS idx_custom_pages_org_enabled ON custom_pages(org_id, is_enabled);

-- Component JSON structure reference (not enforced in SQLite):
-- [
--   {
--     "id": "unique-component-id",
--     "type": "iframe|text|markdown|entity_list|html|link_group|stats",
--     "title": "Component Title",
--     "width": "full|1/2|1/3|2/3",
--     "order": 1,
--     "config": {
--       // Type-specific configuration
--     }
--   }
-- ]
--
-- Component type configs:
--
-- iframe:
--   { "url": "https://example.com", "height": 400, "sandbox": "allow-scripts allow-same-origin" }
--
-- text/markdown:
--   { "content": "# Hello\nThis is **markdown**" }
--
-- html:
--   { "content": "<div class='custom'>HTML content</div>" }
--
-- entity_list:
--   { "entity": "contacts", "filters": {...}, "columns": [...], "pageSize": 10 }
--
-- link_group:
--   { "links": [{ "label": "Link 1", "href": "/path", "icon": "icon-name" }] }
--
-- stats:
--   { "items": [{ "label": "Total", "value": "{{count:contacts}}", "icon": "users" }] }
