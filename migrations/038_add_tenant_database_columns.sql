-- Add tenant database columns to organizations for per-org Turso databases
-- These columns store the connection info for each org's dedicated database

-- Add database URL column (Turso libsql URL)
ALTER TABLE organizations ADD COLUMN database_url TEXT DEFAULT '';

-- Add database auth token column (Turso JWT token)
ALTER TABLE organizations ADD COLUMN database_token TEXT DEFAULT '';

-- Add database name column (for Turso API management)
ALTER TABLE organizations ADD COLUMN database_name TEXT DEFAULT '';
