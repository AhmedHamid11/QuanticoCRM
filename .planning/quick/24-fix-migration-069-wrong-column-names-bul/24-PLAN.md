---
phase: quick-24
plan: "01"
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/migrations/069_add_lookup_indexes.sql
  - backend/internal/repo/pending_alert.go
autonomous: true
requirements: [QUICK-24]
status: complete
commit: "56511b2"
must_haves:
  truths:
    - "Migration 069 references only columns that actually exist in the schema"
    - "Bulk dismiss/resolve does not fail with UNIQUE constraint violation"
    - "COLLATE NOCASE indexes exist for accounts.name, contacts.last_name, contacts.email_address"
  artifacts:
    - path: "backend/internal/migrations/069_add_lookup_indexes.sql"
      provides: "Corrected COLLATE NOCASE indexes for case-insensitive lookups"
      contains: "contacts.last_name COLLATE NOCASE"
    - path: "backend/internal/repo/pending_alert.go"
      provides: "BulkResolve with delete-before-update pattern"
      contains: "DELETE FROM pending_duplicate_alerts"
  key_links:
    - from: "backend/internal/repo/pending_alert.go"
      to: "pending_duplicate_alerts table"
      via: "delete-before-update in BulkResolve"
      pattern: "DELETE FROM pending_duplicate_alerts WHERE.*org_id.*status"
---

<objective>
Fix two bugs: (1) migration 069 referenced non-existent columns and a non-existent table, (2) BulkResolve hit UNIQUE constraint violations when dismissing duplicate alerts.

Purpose: Migration 069 was creating indexes on `contacts.name` and `contacts.email` (which don't exist — the actual columns are `last_name` and `email_address`) and on a `leads` table that doesn't exist. BulkResolve was failing because updating pending alerts to 'dismissed' status violated the UNIQUE constraint on `(org_id, entity_type, record_id, status)` when previously dismissed records already existed.

Output: Corrected migration file and robust BulkResolve method.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@backend/internal/migrations/069_add_lookup_indexes.sql
@backend/internal/repo/pending_alert.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix migration 069 column names and remove leads indexes</name>
  <files>backend/internal/migrations/069_add_lookup_indexes.sql</files>
  <action>
    Fix the COLLATE NOCASE index migration to reference actual column names:

    1. `contacts.name` does not exist — contacts uses `first_name` and `last_name`. Change the index to use `contacts.last_name`.
    2. `contacts.email` does not exist — the actual column is `email_address`. Change the index to use `contacts.email_address`.
    3. Remove all `leads` indexes entirely — no leads table exists in the schema.
    4. Keep the `accounts.name` index (accounts DO have a `name` column).

    Final migration should create exactly 3 indexes:
    - `idx_accounts_org_name_nocase` on `accounts(org_id, name COLLATE NOCASE)`
    - `idx_contacts_org_last_name_nocase` on `contacts(org_id, last_name COLLATE NOCASE)`
    - `idx_contacts_org_email_nocase` on `contacts(org_id, email_address COLLATE NOCASE)`
  </action>
  <verify>Review migration file has correct column names matching the actual schema. No references to `contacts.name`, `contacts.email`, or `leads` table.</verify>
  <done>Migration 069 creates indexes only on columns and tables that exist in the schema.</done>
</task>

<task type="auto">
  <name>Task 2: Fix BulkResolve UNIQUE constraint violation</name>
  <files>backend/internal/repo/pending_alert.go</files>
  <action>
    Add delete-before-update pattern to `BulkResolve` method to avoid UNIQUE constraint violation on `(org_id, entity_type, record_id, status)`.

    Before the UPDATE statement, add a DELETE that removes any previously resolved/dismissed alerts that already have the target status. This matches the pattern already used in the single-record `Resolve` method (lines 145-151).

    The delete query should:
    1. Filter by `org_id = ?` and `status = ?` (the target status)
    2. Optionally filter by `entity_type = ?` if entityType is not empty (matching the UPDATE's behavior)
    3. Execute before the UPDATE to clear the way for status changes

    This prevents the constraint violation when a record was previously dismissed, then re-pended, and is being dismissed again.
  </action>
  <verify>Review that BulkResolve has delete-before-update pattern. The DELETE WHERE clause should match the same scope as the UPDATE (org_id, status, optional entity_type).</verify>
  <done>BulkResolve can dismiss/resolve alerts without UNIQUE constraint violations, even when records were previously dismissed.</done>
</task>

</tasks>

<verification>
- Migration 069 references only existing columns: `accounts.name`, `contacts.last_name`, `contacts.email_address`
- No references to `leads` table or `contacts.name` or `contacts.email`
- BulkResolve in pending_alert.go has delete-before-update pattern before the UPDATE statement
- Delete scope matches update scope (org_id + status + optional entity_type)
</verification>

<success_criteria>
- Migration 069 creates valid COLLATE NOCASE indexes on real columns only
- Bulk dismiss operations complete without UNIQUE constraint errors
- Both fixes committed in 56511b2
</success_criteria>

<output>
After completion, create `.planning/quick/24-fix-migration-069-wrong-column-names-bul/24-SUMMARY.md`
</output>
