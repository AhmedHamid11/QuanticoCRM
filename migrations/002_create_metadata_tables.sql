-- Migration: Create metadata tables for admin panel
-- Stores entity definitions, custom fields, and layouts

-- Entity definitions (scopes)
CREATE TABLE IF NOT EXISTS entity_defs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL,
    label_plural TEXT NOT NULL,
    icon TEXT DEFAULT 'folder',
    color TEXT DEFAULT '#6366f1',
    is_custom INTEGER DEFAULT 0,
    is_customizable INTEGER DEFAULT 1,
    has_stream INTEGER DEFAULT 0,
    has_activities INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Field definitions for each entity
CREATE TABLE IF NOT EXISTS field_defs (
    id TEXT PRIMARY KEY,
    entity_name TEXT NOT NULL,
    name TEXT NOT NULL,
    label TEXT NOT NULL,
    type TEXT NOT NULL,
    is_required INTEGER DEFAULT 0,
    is_read_only INTEGER DEFAULT 0,
    is_audited INTEGER DEFAULT 0,
    is_custom INTEGER DEFAULT 0,
    default_value TEXT,
    options TEXT,  -- JSON array for enum/multiEnum
    max_length INTEGER,
    min_value REAL,
    max_value REAL,
    pattern TEXT,
    tooltip TEXT,
    link_entity TEXT,  -- For link fields
    sort_order INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(entity_name, name),
    FOREIGN KEY (entity_name) REFERENCES entity_defs(name) ON DELETE CASCADE
);

-- Layout definitions
CREATE TABLE IF NOT EXISTS layout_defs (
    id TEXT PRIMARY KEY,
    entity_name TEXT NOT NULL,
    layout_type TEXT NOT NULL,  -- 'list', 'detail', 'filters', 'massUpdate'
    layout_data TEXT NOT NULL,  -- JSON layout configuration
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(entity_name, layout_type),
    FOREIGN KEY (entity_name) REFERENCES entity_defs(name) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_field_defs_entity ON field_defs(entity_name);
CREATE INDEX IF NOT EXISTS idx_layout_defs_entity ON layout_defs(entity_name);

-- Insert default Contact entity definition
INSERT OR IGNORE INTO entity_defs (id, name, label, label_plural, icon, color, is_custom, is_customizable)
VALUES ('contact', 'Contact', 'Contact', 'Contacts', 'user', '#3b82f6', 0, 1);

-- Insert default Contact field definitions
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, sort_order) VALUES
('contact_salutation', 'Contact', 'salutationName', 'Salutation', 'enum', 0, 1),
('contact_first_name', 'Contact', 'firstName', 'First Name', 'varchar', 0, 2),
('contact_last_name', 'Contact', 'lastName', 'Last Name', 'varchar', 1, 3),
('contact_email', 'Contact', 'emailAddress', 'Email', 'email', 0, 4),
('contact_phone', 'Contact', 'phoneNumber', 'Phone', 'phone', 0, 5),
('contact_do_not_call', 'Contact', 'doNotCall', 'Do Not Call', 'bool', 0, 6),
('contact_description', 'Contact', 'description', 'Description', 'text', 0, 7),
('contact_address_street', 'Contact', 'addressStreet', 'Street', 'varchar', 0, 8),
('contact_address_city', 'Contact', 'addressCity', 'City', 'varchar', 0, 9),
('contact_address_state', 'Contact', 'addressState', 'State', 'varchar', 0, 10),
('contact_address_country', 'Contact', 'addressCountry', 'Country', 'varchar', 0, 11),
('contact_address_postal', 'Contact', 'addressPostalCode', 'Postal Code', 'varchar', 0, 12);

-- Update salutation field with options
UPDATE field_defs SET options = '["", "Mr.", "Ms.", "Mrs.", "Dr."]' WHERE id = 'contact_salutation';
