# Quantico CRM - Stream Field Type

## What This Is

Adding a new "Stream" field type to Quantico CRM that functions like a Twitter/journal feed. When a user creates a Stream field, it automatically creates two underlying database columns: an entry field (for new input) and a log field (for timestamped history). Each new entry gets appended to the log with a timestamp prefix.

## Core Value

Users can track chronological notes/updates on any record without creating separate activity records - a lightweight journaling capability embedded in any entity.

## Requirements

### Validated

- ✓ Existing field type system with 18 types — existing
- ✓ Dynamic field creation via Entity Manager — existing
- ✓ Frontend field renderers for various types — existing
- ✓ Backend field validation service — existing

### Active

- [ ] Add `stream` field type constant to backend
- [ ] Create two DB columns when Stream field created (`fieldName` for entry, `fieldName_log` for history)
- [ ] Backend logic to append entry to log with timestamp on save
- [ ] Frontend Stream field editor component (entry input + log display)
- [ ] Frontend Stream field read-only renderer (log display)
- [ ] Clear entry field after successful append
- [ ] Handle empty entries gracefully

### Out of Scope

- Edit/delete individual log entries — keep simple, append-only
- Rich text in entries — plain text only for v1
- @mentions or hashtags — not a social feature
- Separate timestamp format configuration — use ISO format

## Context

The existing field type system is well-structured:
- `backend/internal/entity/metadata.go` defines FieldType constants and GetFieldTypes()
- `backend/internal/repo/metadata.go` handles field CRUD and DB column creation
- `backend/internal/service/provisioning.go` provisions default fields
- `frontend/src/lib/components/fields/` has field renderers by type

Stream field is a "compound" field similar to how `address` creates multiple columns - it just needs entry + log columns.

## Constraints

- **Tech stack**: Go/Fiber backend, SvelteKit frontend, SQLite (Turso)
- **Pattern**: Follow existing field type patterns (e.g., address, rollup)
- **UI**: Use existing Tailwind component patterns

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single field creates two columns | Keeps API simple while storing both entry and log | — Pending |
| Append-only log | Simpler than edit/delete, audit-friendly | — Pending |
| Timestamp format: ISO 8601 | Universal, sortable, parseable | — Pending |

---
*Last updated: 2026-02-03 after initialization*
