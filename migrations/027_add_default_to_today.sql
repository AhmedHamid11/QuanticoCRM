-- Add default_to_today column to field_defs table
-- This allows date/datetime fields to automatically default to today's date when creating new records

ALTER TABLE field_defs ADD COLUMN default_to_today INTEGER DEFAULT 0;
