-- Migration: Bump platform version to trigger mirror migrations propagation
-- This ensures all tenant databases get migrations 063-064 applied

-- Insert new platform version to trigger propagation
INSERT OR REPLACE INTO platform_versions (version, description, released_at)
VALUES ('v5.0.0', 'Mirror Ingest Layer - schema contracts, ingest pipeline, delta keys, field mapping', CURRENT_TIMESTAMP);
