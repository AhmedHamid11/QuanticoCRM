-- Migration: Bump platform version to trigger dedup migrations propagation
-- This ensures all tenant databases get migrations 050-053 applied

-- Insert new platform version to trigger propagation
INSERT OR REPLACE INTO platform_versions (version, description, released_at)
VALUES ('v3.0.0', 'Deduplication System - matching rules, similarity algorithms, blocking strategies, real-time detection', CURRENT_TIMESTAMP);
