-- Add session timeout settings to org_settings
ALTER TABLE org_settings ADD COLUMN idle_timeout_minutes INTEGER NOT NULL DEFAULT 30;
ALTER TABLE org_settings ADD COLUMN absolute_timeout_minutes INTEGER NOT NULL DEFAULT 1440;

-- Bounds: idle 15-60 min, absolute 480-4320 min (8h-72h)
-- Enforced in application code, not database constraints
