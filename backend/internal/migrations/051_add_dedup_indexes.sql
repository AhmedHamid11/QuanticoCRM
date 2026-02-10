-- Pre-computed blocking index columns for duplicate detection performance
-- These columns store Soundex, prefix, and normalized values updated on record save
-- Allows fast candidate retrieval using index scans instead of full table comparisons

-- Add blocking index columns to contacts table
ALTER TABLE contacts ADD COLUMN dedup_last_name_soundex TEXT;
ALTER TABLE contacts ADD COLUMN dedup_last_name_prefix TEXT;      -- First 3 chars, lowercase
ALTER TABLE contacts ADD COLUMN dedup_email_domain TEXT;          -- Domain portion, lowercase
ALTER TABLE contacts ADD COLUMN dedup_phone_e164 TEXT;            -- E.164 normalized phone

-- Create indexes for fast blocking key lookups
CREATE INDEX IF NOT EXISTS idx_contacts_dedup_soundex ON contacts(org_id, dedup_last_name_soundex);
CREATE INDEX IF NOT EXISTS idx_contacts_dedup_prefix ON contacts(org_id, dedup_last_name_prefix);
CREATE INDEX IF NOT EXISTS idx_contacts_dedup_domain ON contacts(org_id, dedup_email_domain);
CREATE INDEX IF NOT EXISTS idx_contacts_dedup_phone ON contacts(org_id, dedup_phone_e164);

-- Note: This migration adds columns to contacts only (primary dedup entity)
-- When dedup is enabled for other entities (Lead, Account, custom), similar columns
-- will be added via provisioning system or entity-specific migrations
