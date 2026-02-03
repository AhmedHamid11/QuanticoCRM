# Requirements: Stream Field Type

**Defined:** 2026-02-03
**Core Value:** Lightweight journaling capability embedded in any entity

## v1 Requirements

### Backend

- [ ] **BE-01**: Add `FieldTypeStream` constant to field types enum
- [ ] **BE-02**: Register Stream in `GetFieldTypes()` with appropriate params
- [ ] **BE-03**: Create two DB columns when Stream field added (entry: varchar, log: text)
- [ ] **BE-04**: On record save, append entry to log with timestamp if entry is non-empty
- [ ] **BE-05**: Clear entry field value after appending to log
- [ ] **BE-06**: Return both fields in API responses

### Frontend

- [ ] **FE-01**: Create StreamFieldEditor component for edit mode
- [ ] **FE-02**: Create StreamFieldDisplay component for read-only mode
- [ ] **FE-03**: Editor shows text input for entry + scrollable log display
- [ ] **FE-04**: Display parses log and shows entries in reverse-chronological order
- [ ] **FE-05**: Register Stream field type in field renderer mapping

## Out of Scope

| Feature | Reason |
|---------|--------|
| Edit/delete log entries | Keep append-only for simplicity and audit trail |
| Rich text formatting | Plain text sufficient for v1 |
| File attachments in entries | Scope creep, use separate attachment fields |
| Entry character limits | Not needed for v1 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BE-01 | Phase 1 | Pending |
| BE-02 | Phase 1 | Pending |
| BE-03 | Phase 1 | Pending |
| BE-04 | Phase 1 | Pending |
| BE-05 | Phase 1 | Pending |
| BE-06 | Phase 1 | Pending |
| FE-01 | Phase 1 | Pending |
| FE-02 | Phase 1 | Pending |
| FE-03 | Phase 1 | Pending |
| FE-04 | Phase 1 | Pending |
| FE-05 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 11 total
- Mapped to phases: 11
- Unmapped: 0 ✓

---
*Requirements defined: 2026-02-03*
