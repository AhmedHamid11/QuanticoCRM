-- Migration: Add allow_updates column to bearing_configs
-- This controls whether clicking a bearing stage updates the field value
-- Default is true (1) for backwards compatibility

ALTER TABLE bearing_configs ADD COLUMN allow_updates INTEGER DEFAULT 1;
