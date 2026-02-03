-- Migration: Add lookup field support
-- Enhances field_defs to support configurable lookup relationships

-- Add additional columns for lookup configuration
ALTER TABLE field_defs ADD COLUMN link_type TEXT DEFAULT 'belongsTo';  -- belongsTo, hasMany, hasOne
ALTER TABLE field_defs ADD COLUMN link_foreign_key TEXT;  -- The FK column name on the related table
ALTER TABLE field_defs ADD COLUMN link_display_field TEXT DEFAULT 'name';  -- Field to display from linked record

-- Insert Account entity definition
INSERT OR IGNORE INTO entity_defs (id, name, label, label_plural, icon, color, is_custom, is_customizable)
VALUES ('account', 'Account', 'Account', 'Accounts', 'building', '#10b981', 0, 1);

-- Insert Account field definitions
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, sort_order) VALUES
('account_name', 'Account', 'name', 'Account Name', 'varchar', 1, 1),
('account_website', 'Account', 'website', 'Website', 'url', 0, 2),
('account_email', 'Account', 'emailAddress', 'Email', 'email', 0, 3),
('account_phone', 'Account', 'phoneNumber', 'Phone', 'phone', 0, 4),
('account_type', 'Account', 'type', 'Type', 'enum', 0, 5),
('account_industry', 'Account', 'industry', 'Industry', 'enum', 0, 6),
('account_sic_code', 'Account', 'sicCode', 'SIC Code', 'varchar', 0, 7),
('account_billing_street', 'Account', 'billingAddressStreet', 'Billing Street', 'varchar', 0, 8),
('account_billing_city', 'Account', 'billingAddressCity', 'Billing City', 'varchar', 0, 9),
('account_billing_state', 'Account', 'billingAddressState', 'Billing State', 'varchar', 0, 10),
('account_billing_country', 'Account', 'billingAddressCountry', 'Billing Country', 'varchar', 0, 11),
('account_billing_postal', 'Account', 'billingAddressPostalCode', 'Billing Postal Code', 'varchar', 0, 12),
('account_shipping_street', 'Account', 'shippingAddressStreet', 'Shipping Street', 'varchar', 0, 13),
('account_shipping_city', 'Account', 'shippingAddressCity', 'Shipping City', 'varchar', 0, 14),
('account_shipping_state', 'Account', 'shippingAddressState', 'Shipping State', 'varchar', 0, 15),
('account_shipping_country', 'Account', 'shippingAddressCountry', 'Shipping Country', 'varchar', 0, 16),
('account_shipping_postal', 'Account', 'shippingAddressPostalCode', 'Shipping Postal Code', 'varchar', 0, 17),
('account_description', 'Account', 'description', 'Description', 'text', 0, 18);

-- Add enum options for Account type and industry
UPDATE field_defs SET options = '["", "Customer", "Partner", "Investor", "Reseller", "Competitor", "Other"]'
WHERE id = 'account_type';

UPDATE field_defs SET options = '["", "Technology", "Finance", "Healthcare", "Manufacturing", "Retail", "Education", "Government", "Non-Profit", "Other"]'
WHERE id = 'account_industry';

-- Add accountId lookup field to Contact
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, sort_order, link_entity, link_type, link_display_field)
VALUES ('contact_account', 'Contact', 'accountId', 'Account', 'link', 0, 13, 'Account', 'belongsTo', 'name');

-- Create a relationship registry table for tracking all relationships
CREATE TABLE IF NOT EXISTS relationship_defs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    from_entity TEXT NOT NULL,
    to_entity TEXT NOT NULL,
    from_field TEXT NOT NULL,
    to_field TEXT,  -- NULL for belongsTo, populated for hasMany
    relationship_type TEXT NOT NULL,  -- belongsTo, hasMany, hasOne, manyToMany
    is_custom INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (from_entity) REFERENCES entity_defs(name) ON DELETE CASCADE,
    FOREIGN KEY (to_entity) REFERENCES entity_defs(name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_relationship_from ON relationship_defs(from_entity);
CREATE INDEX IF NOT EXISTS idx_relationship_to ON relationship_defs(to_entity);

-- Insert Contact -> Account relationship
INSERT OR IGNORE INTO relationship_defs (id, name, from_entity, to_entity, from_field, relationship_type)
VALUES ('rel_contact_account', 'contactAccount', 'Contact', 'Account', 'accountId', 'belongsTo');

-- Insert reverse relationship: Account has many Contacts
INSERT OR IGNORE INTO relationship_defs (id, name, from_entity, to_entity, from_field, to_field, relationship_type)
VALUES ('rel_account_contacts', 'accountContacts', 'Account', 'Contact', 'id', 'accountId', 'hasMany');
