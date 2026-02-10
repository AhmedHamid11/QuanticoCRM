-- Migration: Create platform version tracking infrastructure
-- Adds platform_versions table and current_version column to organizations

-- Create platform_versions table to track platform release history
CREATE TABLE IF NOT EXISTS platform_versions (
    version TEXT PRIMARY KEY,           -- e.g., "v0.1.0" (with v prefix)
    description TEXT NOT NULL,          -- What changed in this version
    released_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add current_version column to organizations table
-- SQLite doesn't support IF NOT EXISTS for ALTER TABLE, but migration system ensures this runs once
ALTER TABLE organizations ADD COLUMN current_version TEXT DEFAULT 'v0.1.0';

-- Create index for efficient version queries
CREATE INDEX IF NOT EXISTS idx_organizations_version ON organizations(current_version);

-- Seed initial platform version
INSERT OR REPLACE INTO platform_versions (version, description, released_at)
VALUES ('v0.1.0', 'Initial platform version', CURRENT_TIMESTAMP);
