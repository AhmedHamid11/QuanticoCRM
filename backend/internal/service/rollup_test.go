package service

import (
	"strings"
	"testing"
)

func TestValidateRollupQuery_Security(t *testing.T) {
	svc := &RollupService{}

	tests := []struct {
		name      string
		query     string
		wantError bool
		errorMsg  string
	}{
		// SHOULD FAIL - Accessing users table
		{
			name:      "accessing users table",
			query:     "SELECT COUNT(*) FROM users",
			wantError: true,
			errorMsg:  "cannot access system table: users",
		},
		// SHOULD FAIL - Accessing sessions table
		{
			name:      "accessing sessions table",
			query:     "SELECT COUNT(*) FROM sessions",
			wantError: true,
			errorMsg:  "cannot access system table: sessions",
		},
		// SHOULD FAIL - Accessing api_tokens table
		{
			name:      "accessing api_tokens table",
			query:     "SELECT COUNT(*) FROM api_tokens",
			wantError: true,
			errorMsg:  "cannot access system table: api_tokens",
		},
		// SHOULD FAIL - Accessing invitations table
		{
			name:      "accessing invitations table",
			query:     "SELECT COUNT(*) FROM invitations",
			wantError: true,
			errorMsg:  "cannot access system table: invitations",
		},
		// SHOULD FAIL - Accessing sqlite_master
		{
			name:      "accessing sqlite_master",
			query:     "SELECT COUNT(*) FROM sqlite_master",
			wantError: true,
			errorMsg:  "cannot access system table: sqlite_master",
		},
		// SHOULD FAIL - INSERT attempt (caught by SELECT check first)
		{
			name:      "INSERT keyword blocked",
			query:     "INSERT INTO Contact (name) VALUES ('test')",
			wantError: true,
			errorMsg:  "must be a SELECT statement",
		},
		// SHOULD FAIL - DELETE attempt (caught by SELECT check first)
		{
			name:      "DELETE keyword blocked",
			query:     "DELETE FROM Contact",
			wantError: true,
			errorMsg:  "must be a SELECT statement",
		},
		// SHOULD FAIL - DROP attempt (caught by SELECT check first)
		{
			name:      "DROP keyword blocked",
			query:     "DROP TABLE Contact",
			wantError: true,
			errorMsg:  "must be a SELECT statement",
		},
		// SHOULD FAIL - UPDATE in subquery blocked
		{
			name:      "UPDATE in subquery blocked",
			query:     "SELECT * FROM (UPDATE Contact SET name='hack')",
			wantError: true,
			errorMsg:  "forbidden keyword: UPDATE",
		},
		// SHOULD FAIL - CTE blocked (starts with WITH, not SELECT)
		{
			name:      "CTE blocked",
			query:     "WITH active AS (SELECT * FROM Contact) SELECT COUNT(*) FROM active",
			wantError: true,
			errorMsg:  "must be a SELECT statement",
		},
		// SHOULD FAIL - Subquery in FROM blocked
		{
			name:      "subquery in FROM blocked",
			query:     "SELECT * FROM (SELECT * FROM Contact) AS sub",
			wantError: true,
			errorMsg:  "cannot use subqueries in FROM clause",
		},
		// SHOULD PASS - Simple count
		{
			name:      "valid simple count",
			query:     "SELECT COUNT(*) FROM Contact",
			wantError: false,
		},
		// SHOULD PASS - Count with WHERE using {{id}}
		{
			name:      "valid count with record id",
			query:     "SELECT COUNT(*) FROM Contact WHERE account_id = '{{id}}'",
			wantError: false,
		},
		// SHOULD PASS - SUM rollup
		{
			name:      "valid sum rollup",
			query:     "SELECT SUM(amount) FROM Opportunity WHERE account_id = '{{id}}'",
			wantError: false,
		},
		// SHOULD PASS - With ORDER BY
		{
			name:      "valid with ORDER BY",
			query:     "SELECT name FROM Contact WHERE account_id = '{{id}}' ORDER BY created_at DESC LIMIT 1",
			wantError: false,
		},
		// SHOULD PASS - With JOIN
		{
			name:      "valid with JOIN",
			query:     "SELECT COUNT(*) FROM Contact c LEFT JOIN Task t ON t.related_id = c.id WHERE c.account_id = '{{id}}'",
			wantError: false,
		},
		// SHOULD PASS - With table alias
		{
			name:      "valid with table alias",
			query:     "SELECT COUNT(*) FROM Contact c WHERE c.account_id = '{{id}}'",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateRollupQuery(tt.query)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestInjectOrgFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple query without WHERE",
			input:    "SELECT COUNT(*) FROM Contact",
			expected: "SELECT COUNT(*) FROM Contact WHERE Contact.org_id = ?",
		},
		{
			name:     "query with existing WHERE",
			input:    "SELECT COUNT(*) FROM Contact WHERE account_id = '{{id}}'",
			expected: "SELECT COUNT(*) FROM Contact WHERE Contact.org_id = ? AND account_id = '{{id}}'",
		},
		{
			name:     "query with ORDER BY",
			input:    "SELECT name FROM Contact ORDER BY created_at",
			expected: "SELECT name FROM Contact WHERE Contact.org_id = ? ORDER BY created_at",
		},
		{
			name:     "query with GROUP BY",
			input:    "SELECT status, COUNT(*) FROM Contact GROUP BY status",
			expected: "SELECT status, COUNT(*) FROM Contact WHERE Contact.org_id = ? GROUP BY status",
		},
		{
			name:     "query with LIMIT",
			input:    "SELECT name FROM Contact LIMIT 10",
			expected: "SELECT name FROM Contact WHERE Contact.org_id = ? LIMIT 10",
		},
		{
			name:     "query with WHERE and ORDER BY",
			input:    "SELECT name FROM Contact WHERE status = 'active' ORDER BY name",
			expected: "SELECT name FROM Contact WHERE Contact.org_id = ? AND status = 'active' ORDER BY name",
		},
		{
			name:     "query with table alias",
			input:    "SELECT COUNT(*) FROM Contact c WHERE c.status = 'active'",
			expected: "SELECT COUNT(*) FROM Contact c WHERE Contact.org_id = ? AND c.status = 'active'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectOrgFilter(tt.input)
			if result != tt.expected {
				t.Errorf("injectOrgFilter(%q)\n  got:      %q\n  expected: %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractTableNames(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple FROM",
			query:    "SELECT * FROM Contact",
			expected: []string{"Contact"},
		},
		{
			name:     "FROM with alias",
			query:    "SELECT * FROM Contact c",
			expected: []string{"Contact"},
		},
		{
			name:     "FROM with JOIN",
			query:    "SELECT * FROM Contact c LEFT JOIN Task t ON t.related_id = c.id",
			expected: []string{"Contact", "Task"},
		},
		{
			name:     "multiple JOINs",
			query:    "SELECT * FROM Account a JOIN Contact c ON c.account_id = a.id JOIN Opportunity o ON o.account_id = a.id",
			expected: []string{"Account", "Contact", "Opportunity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTableNames(tt.query)

			if len(result) != len(tt.expected) {
				t.Errorf("extractTableNames(%q) returned %d tables, expected %d: got %v, expected %v",
					tt.query, len(result), len(tt.expected), result, tt.expected)
				return
			}

			// Check each expected table is present (order may vary)
			for _, exp := range tt.expected {
				found := false
				for _, got := range result {
					if got == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("extractTableNames(%q) missing expected table %q, got %v", tt.query, exp, result)
				}
			}
		})
	}
}

func TestExtractIDColumn(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple WHERE clause",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}}",
			expected: "account_id",
			wantErr:  false,
		},
		{
			name:     "with spaces around equals",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id  =  {{id}}",
			expected: "account_id",
			wantErr:  false,
		},
		{
			name:     "no spaces around equals",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id={{id}}",
			expected: "account_id",
			wantErr:  false,
		},
		{
			name:     "different column name",
			query:    "SELECT SUM(amount) FROM Opportunity WHERE parent_account_id = {{id}}",
			expected: "parent_account_id",
			wantErr:  false,
		},
		{
			name:    "no id placeholder",
			query:   "SELECT COUNT(*) FROM Contact",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractIDColumn(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if result != tt.expected {
					t.Errorf("extractIDColumn(%q) = %q, expected %q", tt.query, result, tt.expected)
				}
			}
		})
	}
}

func TestTransformToBatchQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		groupCol string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple COUNT query",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}}",
			groupCol: "account_id",
			expected: "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN ({{ids}}) GROUP BY account_id",
			wantErr:  false,
		},
		{
			name:     "SUM query",
			query:    "SELECT SUM(amount) FROM Opportunity WHERE account_id = {{id}}",
			groupCol: "account_id",
			expected: "SELECT account_id, SUM(amount) FROM Opportunity WHERE account_id IN ({{ids}}) GROUP BY account_id",
			wantErr:  false,
		},
		{
			name:     "with additional WHERE conditions",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}} AND status = 'active'",
			groupCol: "account_id",
			expected: "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN ({{ids}}) AND status = 'active' GROUP BY account_id",
			wantErr:  false,
		},
		{
			name:     "with ORDER BY",
			query:    "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}} ORDER BY created_at",
			groupCol: "account_id",
			expected: "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN ({{ids}}) GROUP BY account_id ORDER BY created_at",
			wantErr:  false,
		},
		{
			name:     "with LIMIT",
			query:    "SELECT MAX(created_at) FROM Task WHERE related_id = {{id}} LIMIT 1",
			groupCol: "related_id",
			expected: "SELECT related_id, MAX(created_at) FROM Task WHERE related_id IN ({{ids}}) GROUP BY related_id LIMIT 1",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformToBatchQuery(tt.query, tt.groupCol)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if result != tt.expected {
					t.Errorf("transformToBatchQuery(%q, %q)\n  got:      %q\n  expected: %q", tt.query, tt.groupCol, result, tt.expected)
				}
			}
		})
	}
}

func TestInjectOrgFilterBatch(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		numIDs   int
		expected string
	}{
		{
			name:     "simple batch query with 3 IDs",
			query:    "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN ({{ids}}) GROUP BY account_id",
			numIDs:   3,
			expected: "SELECT account_id, COUNT(*) FROM Contact WHERE Contact.org_id = ? AND account_id IN (?,?,?) GROUP BY account_id",
		},
		{
			name:     "batch query with 1 ID",
			query:    "SELECT account_id, SUM(amount) FROM Opportunity WHERE account_id IN ({{ids}}) GROUP BY account_id",
			numIDs:   1,
			expected: "SELECT account_id, SUM(amount) FROM Opportunity WHERE Opportunity.org_id = ? AND account_id IN (?) GROUP BY account_id",
		},
		{
			name:     "batch query with 5 IDs",
			query:    "SELECT related_id, COUNT(*) FROM Task WHERE related_id IN ({{ids}}) GROUP BY related_id",
			numIDs:   5,
			expected: "SELECT related_id, COUNT(*) FROM Task WHERE Task.org_id = ? AND related_id IN (?,?,?,?,?) GROUP BY related_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectOrgFilterBatch(tt.query, tt.numIDs)
			if result != tt.expected {
				t.Errorf("injectOrgFilterBatch(%q, %d)\n  got:      %q\n  expected: %q", tt.query, tt.numIDs, result, tt.expected)
			}
		})
	}
}
