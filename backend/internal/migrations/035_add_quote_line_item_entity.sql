-- Migration: Add QuoteLineItem as a proper entity with field definitions
-- This allows QuoteLineItem fields to be configurable via the entity system

-- For databases WITH org_id column (production/multi-tenant):
-- Insert QuoteLineItem entity definition for all existing orgs that have Quote
INSERT OR IGNORE INTO entity_defs (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, created_at, modified_at)
SELECT
    '0Et' || substr(hex(randomblob(8)), 1, 15) as id,
    org_id,
    'QuoteLineItem' as name,
    'Quote Line Item' as label,
    'Quote Line Items' as label_plural,
    'list' as icon,
    '#8b5cf6' as color,
    0 as is_custom,
    1 as is_customizable,
    0 as has_stream,
    0 as has_activities,
    datetime('now') as created_at,
    datetime('now') as modified_at
FROM entity_defs WHERE name = 'Quote' AND org_id IS NOT NULL;

-- For databases WITHOUT org_id column (older dev databases):
INSERT OR IGNORE INTO entity_defs (id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, created_at, modified_at)
SELECT
    '0EtQLI' || substr(hex(randomblob(6)), 1, 12) as id,
    'QuoteLineItem' as name,
    'Quote Line Item' as label,
    'Quote Line Items' as label_plural,
    'list' as icon,
    '#8b5cf6' as color,
    0 as is_custom,
    1 as is_customizable,
    0 as has_stream,
    0 as has_activities,
    datetime('now') as created_at,
    datetime('now') as modified_at
WHERE NOT EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem');

-- Insert QuoteLineItem field definitions
-- For multi-tenant (with org_id):
INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'name', 'Name', 'varchar', 1, 0, 0, 0, 1, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'description', 'Description', 'text', 0, 0, 0, 0, 2, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'sku', 'SKU', 'varchar', 0, 0, 0, 0, 3, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'quantity', 'Quantity', 'float', 1, 0, 0, 0, 4, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'unitPrice', 'Unit Price', 'currency', 1, 0, 0, 0, 5, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'discountPercent', 'Discount %', 'float', 0, 0, 0, 0, 6, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'discountAmount', 'Discount Amount', 'currency', 0, 0, 0, 0, 7, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'taxPercent', 'Tax %', 'float', 0, 0, 0, 0, 8, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'total', 'Total', 'currency', 0, 1, 0, 0, 9, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, link_entity, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'quoteId', 'Quote', 'link', 1, 0, 0, 0, 10, 'Quote', datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'sortOrder', 'Sort Order', 'int', 0, 0, 0, 0, 11, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'createdAt', 'Created At', 'datetime', 0, 1, 0, 0, 100, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 15), org_id, 'QuoteLineItem', 'modifiedAt', 'Modified At', 'datetime', 0, 1, 0, 0, 101, datetime('now'), datetime('now')
FROM entity_defs WHERE name = 'QuoteLineItem' AND org_id IS NOT NULL;

-- For single-tenant (without org_id - dev databases):
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI01', 'QuoteLineItem', 'name', 'Name', 'varchar', 1, 0, 0, 0, 1, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'name');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI02', 'QuoteLineItem', 'description', 'Description', 'text', 0, 0, 0, 0, 2, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'description');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI03', 'QuoteLineItem', 'sku', 'SKU', 'varchar', 0, 0, 0, 0, 3, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'sku');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI04', 'QuoteLineItem', 'quantity', 'Quantity', 'float', 1, 0, 0, 0, 4, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'quantity');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI05', 'QuoteLineItem', 'unitPrice', 'Unit Price', 'currency', 1, 0, 0, 0, 5, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'unitPrice');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI06', 'QuoteLineItem', 'discountPercent', 'Discount %', 'float', 0, 0, 0, 0, 6, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'discountPercent');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI07', 'QuoteLineItem', 'discountAmount', 'Discount Amount', 'currency', 0, 0, 0, 0, 7, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'discountAmount');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI08', 'QuoteLineItem', 'taxPercent', 'Tax %', 'float', 0, 0, 0, 0, 8, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'taxPercent');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI09', 'QuoteLineItem', 'total', 'Total', 'currency', 0, 1, 0, 0, 9, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'total');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, link_entity, created_at, modified_at)
SELECT '0FdQLI10', 'QuoteLineItem', 'quoteId', 'Quote', 'link', 1, 0, 0, 0, 10, 'Quote', datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'quoteId');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI11', 'QuoteLineItem', 'sortOrder', 'Sort Order', 'int', 0, 0, 0, 0, 11, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'sortOrder');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI12', 'QuoteLineItem', 'createdAt', 'Created At', 'datetime', 0, 1, 0, 0, 100, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'createdAt');

INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, is_read_only, is_audited, is_custom, sort_order, created_at, modified_at)
SELECT '0FdQLI13', 'QuoteLineItem', 'modifiedAt', 'Modified At', 'datetime', 0, 1, 0, 0, 101, datetime('now'), datetime('now')
WHERE EXISTS (SELECT 1 FROM entity_defs WHERE name = 'QuoteLineItem') AND NOT EXISTS (SELECT 1 FROM field_defs WHERE entity_name = 'QuoteLineItem' AND name = 'modifiedAt');
