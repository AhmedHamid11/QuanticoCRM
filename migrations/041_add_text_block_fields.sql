-- Migration: Add textBlock field support
-- Adds variant and content columns to field_defs for textBlock field type

-- Add variant column for textBlock style (info, warning, error, success)
ALTER TABLE field_defs ADD COLUMN variant TEXT DEFAULT 'info';

-- Add content column for textBlock message text (supports {{fieldName}} placeholders)
ALTER TABLE field_defs ADD COLUMN content TEXT DEFAULT '';
