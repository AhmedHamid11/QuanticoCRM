-- Add edit_in_list column to related_list_configs table
-- When enabled, clicking "New" in a related list shows an inline editable row
-- instead of navigating to a separate create page

ALTER TABLE related_list_configs ADD COLUMN edit_in_list INTEGER DEFAULT 0;
