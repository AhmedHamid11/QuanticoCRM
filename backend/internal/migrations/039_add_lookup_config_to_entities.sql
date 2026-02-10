-- Migration: Add lookup configuration columns to entity_defs
-- These columns allow dynamic lookup behavior for any entity

-- Add display_field column (SQL expression for display name in lookups)
ALTER TABLE entity_defs ADD COLUMN display_field TEXT DEFAULT 'name';

-- Add search_fields column (JSON array of field names to search)
ALTER TABLE entity_defs ADD COLUMN search_fields TEXT DEFAULT '["name"]';

-- Update Account with proper lookup config
UPDATE entity_defs SET
  display_field = 'name',
  search_fields = '["name", "email_address", "website"]'
WHERE name = 'Account';

-- Update Contact with name concatenation for display
UPDATE entity_defs SET
  display_field = 'first_name || '' '' || last_name',
  search_fields = '["first_name", "last_name", "email_address"]'
WHERE name = 'Contact';

-- Update Task
UPDATE entity_defs SET
  display_field = 'name',
  search_fields = '["name"]'
WHERE name = 'Task';

-- Update Quote
UPDATE entity_defs SET
  display_field = 'name',
  search_fields = '["name"]'
WHERE name = 'Quote';
