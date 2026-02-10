-- Mirror schema contracts (tenant DB - per org)
CREATE TABLE IF NOT EXISTS mirrors (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    target_entity TEXT NOT NULL,
    unique_key_field TEXT NOT NULL,
    unmapped_field_mode TEXT NOT NULL DEFAULT 'flexible',
    rate_limit INTEGER DEFAULT 500,
    is_active INTEGER DEFAULT 1,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_mirrors_org ON mirrors(org_id);
CREATE INDEX IF NOT EXISTS idx_mirrors_active ON mirrors(org_id, is_active);

-- Mirror source field definitions (tenant DB - per org)
CREATE TABLE IF NOT EXISTS mirror_source_fields (
    id TEXT PRIMARY KEY,
    mirror_id TEXT NOT NULL,
    field_name TEXT NOT NULL,
    field_type TEXT NOT NULL DEFAULT 'text',
    is_required INTEGER DEFAULT 0,
    description TEXT DEFAULT '',
    sort_order INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mirror_id) REFERENCES mirrors(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_mirror_source_fields_mirror ON mirror_source_fields(mirror_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mirror_source_fields_unique ON mirror_source_fields(mirror_id, field_name);
