-- Migration: Add custom_fields column to contacts table
-- This stores custom field values as JSON

ALTER TABLE contacts ADD COLUMN custom_fields TEXT DEFAULT '{}';
