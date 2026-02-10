-- Add org_id to metadata tables for multi-tenant isolation
-- This ensures custom entities, fields, layouts, and navigation are org-specific
-- Using Quantico's org_id (00DKFBKQG1G000E4VG) as default for existing data

-- 1. Recreate entity_defs with org_id
CREATE TABLE entity_defs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL DEFAULT '00DKFBKQG1G000E4VG',
    name TEXT NOT NULL,
    label TEXT NOT NULL,
    label_plural TEXT NOT NULL,
    icon TEXT DEFAULT '',
    color TEXT DEFAULT '',
    is_custom INTEGER DEFAULT 0,
    is_customizable INTEGER DEFAULT 1,
    has_stream INTEGER DEFAULT 0,
    has_activities INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    UNIQUE(org_id, name)
);

INSERT INTO entity_defs_new (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, created_at, modified_at)
SELECT id, '00DKFBKQG1G000E4VG', name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, created_at, modified_at FROM entity_defs;

DROP TABLE entity_defs;
ALTER TABLE entity_defs_new RENAME TO entity_defs;
CREATE INDEX idx_entity_defs_org ON entity_defs(org_id);

-- 2. Recreate field_defs with org_id
CREATE TABLE field_defs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL DEFAULT '00DKFBKQG1G000E4VG',
    entity_name TEXT NOT NULL,
    name TEXT NOT NULL,
    label TEXT NOT NULL,
    type TEXT NOT NULL,
    is_required INTEGER DEFAULT 0,
    is_read_only INTEGER DEFAULT 0,
    is_audited INTEGER DEFAULT 0,
    is_custom INTEGER DEFAULT 0,
    default_value TEXT,
    options TEXT,
    max_length INTEGER,
    min_value REAL,
    max_value REAL,
    pattern TEXT,
    tooltip TEXT,
    link_entity TEXT,
    link_type TEXT,
    link_foreign_key TEXT,
    link_display_field TEXT,
    sort_order INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    rollup_query TEXT,
    rollup_result_type TEXT,
    rollup_decimal_places INTEGER DEFAULT 2,
    UNIQUE(org_id, entity_name, name)
);

INSERT INTO field_defs_new (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, default_value, options, max_length, min_value, max_value, pattern, tooltip, link_entity, link_type, link_foreign_key, link_display_field, sort_order, created_at, modified_at, rollup_query, rollup_result_type, rollup_decimal_places)
SELECT id, '00DKFBKQG1G000E4VG', entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, default_value, options, max_length, min_value, max_value, pattern, tooltip, link_entity, link_type, link_foreign_key, link_display_field, sort_order, created_at, modified_at, rollup_query, rollup_result_type, rollup_decimal_places FROM field_defs;

DROP TABLE field_defs;
ALTER TABLE field_defs_new RENAME TO field_defs;
CREATE INDEX idx_field_defs_org_entity ON field_defs(org_id, entity_name);

-- 3. Recreate layout_defs with org_id
CREATE TABLE layout_defs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL DEFAULT '00DKFBKQG1G000E4VG',
    entity_name TEXT NOT NULL,
    layout_type TEXT NOT NULL,
    layout_data TEXT NOT NULL,
    created_at TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    UNIQUE(org_id, entity_name, layout_type)
);

INSERT INTO layout_defs_new (id, org_id, entity_name, layout_type, layout_data, created_at, modified_at)
SELECT id, '00DKFBKQG1G000E4VG', entity_name, layout_type, layout_data, created_at, modified_at FROM layout_defs;

DROP TABLE layout_defs;
ALTER TABLE layout_defs_new RENAME TO layout_defs;
CREATE INDEX idx_layout_defs_org_entity ON layout_defs(org_id, entity_name);

-- 4. Recreate navigation_tabs with org_id
CREATE TABLE navigation_tabs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL DEFAULT '00DKFBKQG1G000E4VG',
    label TEXT NOT NULL,
    href TEXT NOT NULL,
    icon TEXT DEFAULT '',
    entity_name TEXT,
    sort_order INTEGER DEFAULT 0,
    is_visible INTEGER DEFAULT 1,
    is_system INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    UNIQUE(org_id, href)
);

INSERT INTO navigation_tabs_new (id, org_id, label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at)
SELECT id, '00DKFBKQG1G000E4VG', label, href, icon, entity_name, sort_order, is_visible, is_system, created_at, modified_at FROM navigation_tabs;

DROP TABLE navigation_tabs;
ALTER TABLE navigation_tabs_new RENAME TO navigation_tabs;
CREATE INDEX idx_navigation_tabs_org ON navigation_tabs(org_id, sort_order);

-- 5. Recreate relationship_defs with org_id
CREATE TABLE relationship_defs_new (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL DEFAULT '00DKFBKQG1G000E4VG',
    name TEXT NOT NULL,
    from_entity TEXT NOT NULL,
    to_entity TEXT NOT NULL,
    from_field TEXT NOT NULL,
    to_field TEXT,
    relationship_type TEXT NOT NULL,
    is_custom INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    UNIQUE(org_id, name)
);

INSERT INTO relationship_defs_new (id, org_id, name, from_entity, to_entity, from_field, to_field, relationship_type, is_custom, created_at, modified_at)
SELECT id, '00DKFBKQG1G000E4VG', name, from_entity, to_entity, from_field, to_field, relationship_type, is_custom, created_at, modified_at FROM relationship_defs;

DROP TABLE relationship_defs;
ALTER TABLE relationship_defs_new RENAME TO relationship_defs;
CREATE INDEX idx_relationship_defs_org ON relationship_defs(org_id);
CREATE INDEX idx_relationship_defs_from ON relationship_defs(org_id, from_entity);
CREATE INDEX idx_relationship_defs_to ON relationship_defs(org_id, to_entity);
