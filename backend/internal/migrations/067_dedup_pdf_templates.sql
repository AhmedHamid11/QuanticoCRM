-- Migration: Deduplicate pdf_templates and add UNIQUE constraint
-- Same root cause as bearing_configs: tenant_provisioning.go created the table
-- without a UNIQUE constraint, so each reprovision inserted duplicate templates.
-- NOTE: Only copy columns from migration 033 schema (master DB).
-- Tenant DBs may have extra columns (custom_css, header_html, etc.) added by
-- tenant_provisioning.go, but the master DB does not.

-- Step 1: Create new table with UNIQUE constraint (base columns only)
CREATE TABLE IF NOT EXISTS pdf_templates_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL DEFAULT 'Quote',
    is_default INTEGER DEFAULT 0,
    is_system INTEGER DEFAULT 0,
    base_design TEXT NOT NULL DEFAULT 'professional',
    branding TEXT NOT NULL DEFAULT '{}',
    sections TEXT NOT NULL DEFAULT '[]',
    page_size TEXT DEFAULT 'A4',
    orientation TEXT DEFAULT 'portrait',
    margins TEXT DEFAULT '10mm,10mm,10mm,10mm',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, entity_type, name)
);

-- Step 2: Copy one row per (org_id, entity_type, name), keeping the earliest
INSERT OR IGNORE INTO pdf_templates_new
    (id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at)
SELECT id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at
FROM pdf_templates
ORDER BY created_at ASC;

-- Step 3: Swap tables
DROP TABLE IF EXISTS pdf_templates;

ALTER TABLE pdf_templates_new RENAME TO pdf_templates;

-- Step 4: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_pdf_templates_org ON pdf_templates(org_id);

CREATE INDEX IF NOT EXISTS idx_pdf_templates_entity ON pdf_templates(org_id, entity_type);

CREATE INDEX IF NOT EXISTS idx_pdf_templates_default ON pdf_templates(org_id, entity_type, is_default);
