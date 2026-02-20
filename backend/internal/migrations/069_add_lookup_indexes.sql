-- Add COLLATE NOCASE indexes for case-insensitive lookup resolution during imports.
-- Without these, LOWER(col) = LOWER(?) queries force full table scans.
-- COLLATE NOCASE lets SQLite use B-tree indexes for case-insensitive matching.

-- Accounts: commonly looked up by name during contact/opportunity imports
CREATE INDEX IF NOT EXISTS idx_accounts_org_name_nocase
  ON accounts(org_id, name COLLATE NOCASE);

-- Contacts: commonly looked up by last_name or email_address
CREATE INDEX IF NOT EXISTS idx_contacts_org_last_name_nocase
  ON contacts(org_id, last_name COLLATE NOCASE);

CREATE INDEX IF NOT EXISTS idx_contacts_org_email_nocase
  ON contacts(org_id, email_address COLLATE NOCASE);
