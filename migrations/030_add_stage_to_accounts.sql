-- Migration: Add stage column to accounts table
-- This column is used for workflow/pipeline stages

ALTER TABLE accounts ADD COLUMN stage TEXT DEFAULT '';
