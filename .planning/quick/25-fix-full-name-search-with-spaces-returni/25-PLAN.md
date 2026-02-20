---
phase: quick-25
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/repo/contact.go
autonomous: true
requirements: [QUICK-25]
must_haves:
  truths:
    - "Searching 'Allison Arnet' returns contacts where first_name='Allison' and last_name='Arnet'"
    - "Searching by first name only still works"
    - "Searching by last name only still works"
    - "Searching by email still works"
  artifacts:
    - path: "backend/internal/repo/contact.go"
      provides: "Contact search with full-name concatenation"
      contains: "first_name || ' ' || last_name"
  key_links:
    - from: "backend/internal/repo/contact.go"
      to: "SQLite query"
      via: "concatenated name LIKE clause"
      pattern: "first_name \\|\\| ' ' \\|\\| last_name"
---

<objective>
Fix contact search so that queries containing spaces (e.g., "Allison Arnet") match across first_name + last_name concatenation.

Purpose: Currently, searching a full name like "Allison Arnet" returns no results because the LIKE check runs against each column individually — no single column contains the full string with a space. Adding a concatenated `(first_name || ' ' || last_name) LIKE ?` clause fixes this.

Output: Updated search query in `contact.go` that matches full names spanning first + last name columns.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@backend/internal/repo/contact.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add concatenated full-name match to contact search query</name>
  <files>backend/internal/repo/contact.go</files>
  <action>
In `backend/internal/repo/contact.go`, find the search block at lines 194-198:

```go
if params.Search != "" {
    baseQuery += ` AND (c.first_name LIKE ? OR c.last_name LIKE ? OR c.email_address LIKE ?)`
    searchTerm := "%" + params.Search + "%"
    args = append(args, searchTerm, searchTerm, searchTerm)
}
```

Replace with:

```go
if params.Search != "" {
    baseQuery += ` AND (c.first_name LIKE ? OR c.last_name LIKE ? OR c.email_address LIKE ? OR (c.first_name || ' ' || c.last_name) LIKE ?)`
    searchTerm := "%" + params.Search + "%"
    args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
}
```

Key changes:
- Add a fourth OR condition: `(c.first_name || ' ' || c.last_name) LIKE ?`
- Add a fourth `searchTerm` argument to `args`
- SQLite `||` is the string concatenation operator — this produces "Allison Arnet" which then matches the LIKE pattern

No other files need changes. Accounts, tasks, quotes, etc. all use a single `name` column and are not affected.
  </action>
  <verify>
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` — compiles without errors
2. Grep the file to confirm the new clause exists: search for `first_name || ' ' || last_name` in contact.go
3. Verify the args line now appends 4 searchTerm values (count the commas)
  </verify>
  <done>
Contact search query includes concatenated full-name match. Searching "Allison Arnet" will now match contacts where first_name="Allison" and last_name contains "Arnet". Single-field searches (first name only, last name only, email only) continue to work unchanged.
  </done>
</task>

</tasks>

<verification>
- `go build ./...` passes from backend directory
- The search SQL in contact.go contains the concatenated name clause
- The args slice appends exactly 4 searchTerm values for the 4 placeholders
</verification>

<success_criteria>
- Contact search with space-separated full names returns matching results
- Existing single-column searches (first name, last name, email) unaffected
- Backend compiles cleanly
</success_criteria>

<output>
After completion, create `.planning/quick/25-fix-full-name-search-with-spaces-returni/25-SUMMARY.md`
</output>
