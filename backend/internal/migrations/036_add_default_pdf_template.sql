-- Migration: Add default Quote PDF template to existing organizations
-- This ensures all orgs have a default PDF template for quotes

-- For orgs that don't have any PDF template yet, create a default one
INSERT INTO pdf_templates (id, org_id, name, entity_type, is_default, is_system, base_design, branding, sections, page_size, orientation, margins, created_at, modified_at)
SELECT
    '0Pt' || substr(hex(randomblob(8)), 1, 12) as id,
    o.id as org_id,
    'Standard Quote' as name,
    'Quote' as entity_type,
    1 as is_default,
    1 as is_system,
    'professional' as base_design,
    '{"companyName":"","logoUrl":"","primaryColor":"#2563eb","accentColor":"#1e40af","fontFamily":"Helvetica, Arial, sans-serif"}' as branding,
    '[{"id":"header","label":"Header","enabled":true,"fields":["companyName","logo","quoteNumber","status","createdAt"]},{"id":"customer","label":"Customer Information","enabled":true,"fields":["accountName","contactName","billingAddress","shippingAddress"]},{"id":"lineItems","label":"Line Items","enabled":true,"fields":["name","description","sku","quantity","unitPrice","discountPercent","total"]},{"id":"totals","label":"Totals","enabled":true,"fields":["subtotal","discount","tax","shipping","grandTotal"]},{"id":"terms","label":"Terms & Conditions","enabled":true,"fields":["terms"]},{"id":"notes","label":"Notes","enabled":true,"fields":["notes"]},{"id":"footer","label":"Footer","enabled":true,"fields":["validUntil","thankYou"]}]' as sections,
    'A4' as page_size,
    'portrait' as orientation,
    '10mm,10mm,10mm,10mm' as margins,
    datetime('now') as created_at,
    datetime('now') as modified_at
FROM orgs o
WHERE NOT EXISTS (
    SELECT 1 FROM pdf_templates pt
    WHERE pt.org_id = o.id
    AND pt.entity_type = 'Quote'
    AND pt.is_default = 1
);
