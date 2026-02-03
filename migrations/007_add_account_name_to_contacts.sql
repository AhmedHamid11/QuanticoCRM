-- Migration: Add account_name column to contacts table
-- Stores the denormalized account name for display purposes

ALTER TABLE contacts ADD COLUMN account_name TEXT DEFAULT '';
