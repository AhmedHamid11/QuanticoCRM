-- Add tenant database columns to organizations for per-org Turso databases
-- These columns store the connection info for each org's dedicated database
-- NOTE: Columns already exist in production, migration just needs to be recorded

-- Migration is a no-op - columns were added manually before migration tracking was set up
-- The columns are: database_url, database_token, database_name
SELECT 1;
