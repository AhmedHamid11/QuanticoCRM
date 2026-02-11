package service

import (
	"testing"
)

// TestStripLeadingComments verifies that SQL comments before statements are properly stripped
func TestStripLeadingComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "comment before CREATE TABLE",
			input:    "-- Create contacts table\nCREATE TABLE IF NOT EXISTS contacts (\n    id TEXT PRIMARY KEY\n)",
			expected: "CREATE TABLE IF NOT EXISTS contacts (\n    id TEXT PRIMARY KEY\n)",
		},
		{
			name:     "multiple comments before SQL",
			input:    "-- Migration: Create list_views\n-- List views allow users to save filters\n\nCREATE TABLE IF NOT EXISTS list_views (\n    id TEXT PRIMARY KEY\n)",
			expected: "CREATE TABLE IF NOT EXISTS list_views (\n    id TEXT PRIMARY KEY\n)",
		},
		{
			name:     "no comments - pass through",
			input:    "CREATE TABLE IF NOT EXISTS foo (id TEXT)",
			expected: "CREATE TABLE IF NOT EXISTS foo (id TEXT)",
		},
		{
			name:     "only comments - return empty",
			input:    "-- This is just a comment\n-- Another comment",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "blank lines then comment then SQL",
			input:    "\n\n-- Comment\n\nINSERT INTO foo VALUES ('bar')",
			expected: "INSERT INTO foo VALUES ('bar')",
		},
		{
			name:     "inline comments preserved after SQL starts",
			input:    "-- Leading comment\nCREATE TABLE foo (\n    id TEXT PRIMARY KEY, -- inline comment\n    name TEXT\n)",
			expected: "CREATE TABLE foo (\n    id TEXT PRIMARY KEY, -- inline comment\n    name TEXT\n)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripLeadingComments(tt.input)
			if result != tt.expected {
				t.Errorf("stripLeadingComments(%q)\ngot:  %q\nwant: %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsMasterOnlyStatement verifies precise table name matching (not substring matching)
func TestIsMasterOnlyStatement(t *testing.T) {
	tests := []struct {
		name     string
		stmt     string
		expected bool
	}{
		// Master-only tables should be detected
		{
			name:     "CREATE TABLE organizations",
			stmt:     "CREATE TABLE IF NOT EXISTS organizations (id TEXT PRIMARY KEY)",
			expected: true,
		},
		{
			name:     "ALTER TABLE users",
			stmt:     "ALTER TABLE users ADD COLUMN email TEXT",
			expected: true,
		},
		{
			name:     "INSERT INTO sessions",
			stmt:     "INSERT INTO sessions (id, user_id) VALUES ('s1', 'u1')",
			expected: true,
		},
		{
			name:     "INSERT OR REPLACE INTO salesforce_connections",
			stmt:     "INSERT OR REPLACE INTO salesforce_connections (id) VALUES ('c1')",
			expected: true,
		},
		{
			name:     "CREATE INDEX ON users",
			stmt:     "CREATE INDEX idx_users_email ON users(email)",
			expected: true,
		},
		{
			name:     "DROP TABLE audit_logs",
			stmt:     "DROP TABLE IF EXISTS audit_logs",
			expected: true,
		},

		// Tenant tables should NOT be flagged as master-only
		{
			name:     "CREATE TABLE contacts (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS contacts (id TEXT PRIMARY KEY, org_id TEXT)",
			expected: false,
		},
		{
			name:     "CREATE TABLE list_views (contains 'users' in original comment but comment stripped)",
			stmt:     "CREATE TABLE IF NOT EXISTS list_views (id TEXT PRIMARY KEY)",
			expected: false,
		},
		{
			name:     "CREATE TABLE navigation_tabs (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS navigation_tabs (id TEXT PRIMARY KEY)",
			expected: false,
		},
		{
			name:     "INSERT with 'users' as icon value (not table name)",
			stmt:     "INSERT OR IGNORE INTO navigation_tabs (id, label, href, icon) VALUES ('nav_contacts', 'Contacts', '/contacts', 'users')",
			expected: false,
		},
		{
			name:     "CREATE TABLE with REFERENCES organizations(id) FK",
			stmt:     "CREATE TABLE IF NOT EXISTS user_org_roles (id TEXT PRIMARY KEY, org_id TEXT REFERENCES organizations(id))",
			expected: false,
		},
		{
			name:     "CREATE TABLE entity_defs (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS entity_defs (id TEXT PRIMARY KEY, org_id TEXT NOT NULL)",
			expected: false,
		},
		{
			name:     "INSERT INTO field_defs (tenant table)",
			stmt:     "INSERT OR IGNORE INTO field_defs (id, org_id, entity_name) VALUES ('f1', 'org1', 'Contact')",
			expected: false,
		},
		{
			name:     "CREATE TABLE sync_jobs (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS sync_jobs (id TEXT PRIMARY KEY, org_id TEXT NOT NULL)",
			expected: false,
		},
		{
			name:     "CREATE INDEX on tenant table",
			stmt:     "CREATE INDEX IF NOT EXISTS idx_sync_jobs_org ON sync_jobs(org_id)",
			expected: false,
		},
		{
			name:     "UPDATE field_defs (tenant table)",
			stmt:     "UPDATE field_defs SET options = '[]' WHERE entity_name = 'Quote'",
			expected: false,
		},
		{
			name:     "CREATE TABLE org_settings (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS org_settings (org_id TEXT PRIMARY KEY)",
			expected: false,
		},
		{
			name:     "CREATE TABLE bearing_configs (tenant table)",
			stmt:     "CREATE TABLE IF NOT EXISTS bearing_configs (id TEXT PRIMARY KEY)",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty statement",
			stmt:     "",
			expected: false,
		},
		{
			name:     "SELECT statement (not a DDL/DML target)",
			stmt:     "SELECT * FROM organizations WHERE id = 'foo'",
			expected: false,
		},
		{
			name:     "CREATE UNIQUE INDEX on master table",
			stmt:     "CREATE UNIQUE INDEX idx_users_email ON users(email)",
			expected: true,
		},
		{
			name:     "DELETE FROM master table",
			stmt:     "DELETE FROM sessions WHERE expired_at < '2024-01-01'",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMasterOnlyStatement(tt.stmt)
			if result != tt.expected {
				t.Errorf("isMasterOnlyStatement(%q) = %v, want %v", tt.stmt, result, tt.expected)
			}
		})
	}
}

// TestIsMasterOnlyStatementFalsePositiveRegression tests the specific cases that caused
// the metadata corruption bug: SQL comments and column values containing master table names
func TestIsMasterOnlyStatementFalsePositiveRegression(t *testing.T) {
	// These are real migration statements that were incorrectly flagged as master-only
	// by the old substring-based isMasterOnlyStatement

	tests := []struct {
		name string
		stmt string
	}{
		{
			name: "navigation_tabs INSERT with 'users' icon value",
			stmt: `INSERT OR IGNORE INTO navigation_tabs (id, label, href, icon, entity_name, sort_order, is_visible, is_system) VALUES
('nav_contacts', 'Contacts', '/contacts', 'users', 'Contact', 1, 1, 1),
('nav_accounts', 'Accounts', '/accounts', 'building', 'Account', 2, 1, 1)`,
		},
		{
			name: "field_defs INSERT with FROM organizations subquery",
			stmt: `INSERT OR IGNORE INTO field_defs (id, org_id, entity_name, name, label, type)
SELECT '0Fd' || substr(hex(randomblob(8)), 1, 16), o.id, 'Quote', 'status', 'Status', 'enum'
FROM organizations o
WHERE NOT EXISTS (SELECT 1 FROM field_defs fd WHERE fd.org_id = o.id AND fd.entity_name = 'Quote')`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isMasterOnlyStatement(tt.stmt) {
				t.Errorf("FALSE POSITIVE: isMasterOnlyStatement incorrectly flagged tenant statement as master-only:\n%s", tt.stmt)
			}
		})
	}
}

// TestEndToEndCommentStrippingAndMasterCheck verifies the full pipeline:
// raw SQL from migration file → stripLeadingComments → isMasterOnlyStatement
func TestEndToEndCommentStrippingAndMasterCheck(t *testing.T) {
	tests := []struct {
		name           string
		rawSQL         string
		shouldExecute  bool // true = should execute on tenant, false = should skip
	}{
		{
			name: "navigation_tabs CREATE TABLE with comment (was being skipped!)",
			rawSQL: `-- Migration: Create navigation configuration table
-- Stores the toolbar/navigation tab configuration

CREATE TABLE IF NOT EXISTS navigation_tabs (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    href TEXT NOT NULL
)`,
			shouldExecute: true, // This is a tenant table - MUST execute
		},
		{
			name: "list_views CREATE TABLE with 'users' in comment (was being skipped!)",
			rawSQL: `-- Migration: Create list_views table for saved filter configurations
-- List views allow users to save filter queries and column configurations

CREATE TABLE IF NOT EXISTS list_views (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL
)`,
			shouldExecute: true, // This is a tenant table - MUST execute
		},
		{
			name: "org_settings CREATE TABLE with comment",
			rawSQL: `-- Migration: Create org_settings table for per-org configuration
-- Stores settings like homepage, branding, etc.

CREATE TABLE IF NOT EXISTS org_settings (
    org_id TEXT PRIMARY KEY,
    home_page TEXT DEFAULT '/'
)`,
			shouldExecute: true, // Tenant table
		},
		{
			name: "audit_logs CREATE TABLE (master-only)",
			rawSQL: `-- Create audit logs table with hash chain for tamper detection
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL
)`,
			shouldExecute: false, // Master-only table
		},
		{
			name: "organizations CREATE TABLE (master-only)",
			rawSQL: `-- Organizations table (tenants)
CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY
)`,
			shouldExecute: false, // Master-only table
		},
		{
			name: "users CREATE TABLE (master-only)",
			rawSQL: `-- Users table (authentication)
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY
)`,
			shouldExecute: false, // Master-only table
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Strip comments (like the fixed propagator does)
			stripped := stripLeadingComments(tt.rawSQL)

			// Step 2: Check if master-only
			isMasterOnly := isMasterOnlyStatement(stripped)

			// The statement should execute if it's NOT master-only
			wouldExecute := !isMasterOnly

			if wouldExecute != tt.shouldExecute {
				t.Errorf("Statement would %s on tenant (want %s):\nRaw: %s\nStripped: %s\nisMasterOnly: %v",
					boolToAction(wouldExecute), boolToAction(tt.shouldExecute),
					tt.rawSQL, stripped, isMasterOnly)
			}
		})
	}
}

func boolToAction(b bool) string {
	if b {
		return "EXECUTE"
	}
	return "SKIP"
}
