---
phase: quick-25
plan: "01"
subsystem: backend/search
tags: [bugfix, search, contacts, sqlite]
dependency_graph:
  requires: []
  provides: [full-name contact search]
  affects: [backend/internal/repo/contact.go]
tech_stack:
  added: []
  patterns: [SQLite string concatenation with ||]
key_files:
  modified:
    - backend/internal/repo/contact.go
decisions:
  - Used SQLite || operator for string concatenation — no additional columns or indexes needed
metrics:
  duration: "2 min"
  completed: "2026-02-20"
  tasks_completed: 1
  files_changed: 1
---

# Phase Quick-25: Fix Full-Name Search With Spaces — Summary

**One-liner:** Added SQLite `first_name || ' ' || last_name` concatenation clause to contact search so queries like "Allison Arnet" return matching results.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add concatenated full-name match to contact search query | 45744c2 | backend/internal/repo/contact.go |

## What Was Built

The contact list search query in `contact.go` previously matched only against individual columns (`first_name`, `last_name`, `email_address`). A query like "Allison Arnet" contains a space, so it matched neither `first_name` (which is just "Allison") nor `last_name` (which is just "Arnet") — returning zero results.

**Fix:** Added a fourth OR condition that concatenates `first_name || ' ' || last_name` and runs a LIKE check against the full search term. This uses SQLite's native `||` string concatenation operator.

### Before
```go
baseQuery += ` AND (c.first_name LIKE ? OR c.last_name LIKE ? OR c.email_address LIKE ?)`
args = append(args, searchTerm, searchTerm, searchTerm)
```

### After
```go
baseQuery += ` AND (c.first_name LIKE ? OR c.last_name LIKE ? OR c.email_address LIKE ? OR (c.first_name || ' ' || c.last_name) LIKE ?)`
args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
```

## Verification

- `go build ./...` passes with no errors
- Confirmed line 195 contains `(c.first_name || ' ' || c.last_name) LIKE ?`
- Confirmed line 197 appends 4 `searchTerm` values matching 4 placeholders

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- [x] `backend/internal/repo/contact.go` modified correctly
- [x] Commit `45744c2` exists
- [x] Backend compiles cleanly
