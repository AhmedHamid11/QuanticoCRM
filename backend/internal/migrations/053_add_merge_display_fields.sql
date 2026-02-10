-- Add merge_display_fields column to matching_rules and pending_duplicate_alerts
-- This controls which fields appear on the merge screen in the duplicate modal

-- Add to matching_rules (where the config is defined)
ALTER TABLE matching_rules ADD COLUMN merge_display_fields TEXT;
-- Example value: '["firstName","lastName","emailAddress","phoneNumber","accountName","description"]'

-- Add to pending_duplicate_alerts (copied from rule when alert is created)
ALTER TABLE pending_duplicate_alerts ADD COLUMN merge_display_fields TEXT;
