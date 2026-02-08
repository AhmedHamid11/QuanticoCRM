# Requirements: v3.0 Deduplication System

**Milestone:** v3.0
**Created:** 2026-02-05
**Status:** Approved

---

## Overview

Comprehensive, entity-agnostic deduplication system with scoring-based matching, import integration, manual merge with undo, and background scanning.

**User decisions:**
- Matching criteria: Full profile scoring (all available fields)
- Import behavior: Block with review (require user decision)
- Merge rules: Manual field selection per conflict
- Entity scope: Generic system for all entities

---

## v3.0 Requirements

### Detection & Matching

- [ ] **DETECT-01**: Admin can configure matching rules per entity type via UI
  - Fields to match (email, phone, name, company, custom fields)
  - AND/OR logic between fields
  - Fuzzy vs exact matching per field
  - Enable/disable rules

- [x] **DETECT-02**: System detects duplicates on record creation with configurable response
  - Warn mode: Show duplicate warning, allow save
  - Block mode: Prevent save until duplicates resolved

- [ ] **DETECT-03**: System uses Jaro-Winkler algorithm for fuzzy name matching (threshold 0.88)

- [ ] **DETECT-04**: System uses exact match for email after normalization (lowercase, trim)

- [ ] **DETECT-05**: System uses E.164 normalization for phone matching via libphonenumber

- [ ] **DETECT-06**: System calculates confidence scores (0-100) using weighted field scoring
  - Email weight: 40
  - Phone weight: 20
  - Last name weight: 20
  - First name weight: 15
  - Company weight: 5

- [ ] **DETECT-07**: System applies negative signals to reduce false positives
  - Different company: -30% score
  - Different email domain: -20% score

- [ ] **DETECT-08**: System uses SQL blocking strategy to limit candidate comparisons
  - Soundex blocking for names
  - Email domain blocking
  - First 3 characters of last name

- [x] **DETECT-09**: User sees tiered confidence levels in duplicate review
  - High confidence: >= 95%
  - Medium confidence: >= 85%
  - Low confidence: >= 70%
  - Below 70%: not shown as duplicate

### Import Integration

- [x] **IMPORT-01**: CSV import runs duplicate detection during analyze step

- [x] **IMPORT-02**: Import preview shows potential duplicates with match scores

- [x] **IMPORT-03**: User can choose action for each duplicate row:
  - Skip (don't import)
  - Update existing (upsert)
  - Import anyway (create duplicate)
  - Merge with existing

- [x] **IMPORT-04**: Import detects duplicates within the file (rows that duplicate each other)

- [x] **IMPORT-05**: Import blocks proceeding until duplicate decisions are made

### Merge Capabilities

- [x] **MERGE-01**: User can merge two or more duplicate records into one

- [x] **MERGE-02**: User selects survivor record (which record ID to keep)

- [x] **MERGE-03**: User selects field values to keep via side-by-side UI
  - Radio buttons per field
  - Default selection based on rules (most complete, most recent)

- [x] **MERGE-04**: Merge transfers all related records to survivor
  - Tasks, Activities, Notes
  - Any record with lookup field pointing to source

- [x] **MERGE-05**: System discovers related records dynamically from entity metadata
  - Uses existing lookup field definitions
  - Updates all foreign keys before delete

- [x] **MERGE-06**: Merge executes as atomic transaction (all or nothing)

- [x] **MERGE-07**: Merge creates audit log entry with:
  - Who merged
  - When merged
  - Which records merged
  - Which field values chosen

- [x] **MERGE-08**: System stores pre-merge snapshots for undo capability
  - Full source record data
  - Master record pre-merge state
  - Related record FK changes

- [x] **MERGE-09**: User can undo merge within 30 days
  - Restores source record
  - Restores master to pre-merge state
  - Re-points related records

- [x] **MERGE-10**: Multi-record merge (3+) executes as sequential pair merges with grouped undo

- [x] **MERGE-11**: Merge preview shows:
  - Before/after comparison
  - Related record counts that will transfer
  - Warnings for any data loss

### Background Processing

- [x] **BACKGROUND-01**: Admin can schedule duplicate scan jobs per entity type
  - Daily, weekly, or monthly frequency
  - Specific time of day

- [x] **BACKGROUND-02**: Background scan uses cursor-based chunking (500 records per chunk)
  - Avoids Turso 5-second transaction timeout
  - Checkpoints progress after each chunk

- [x] **BACKGROUND-03**: Background jobs track status:
  - Pending, Running, Completed, Failed
  - Progress percentage
  - Records scanned / duplicates found

- [x] **BACKGROUND-04**: Admin receives in-app notification when scan completes
  - Summary: X duplicates found across Y records
  - Link to duplicate review queue

- [x] **BACKGROUND-05**: Background jobs use per-tenant rate limiting
  - Max 2 concurrent jobs per tenant
  - Prevents one tenant blocking others

- [x] **BACKGROUND-06**: Failed jobs can be retried
  - Resume from last checkpoint
  - Max 3 retry attempts

### Admin UI

- [x] **UI-01**: Admin can manage matching rules in Settings > Data Quality > Duplicate Rules

- [x] **UI-02**: Admin can view duplicate review queue showing all detected duplicates
  - Grouped by entity type
  - Sorted by confidence score (highest first)
  - Filter by entity, confidence level

- [x] **UI-03**: Merge wizard guides user through:
  - Survivor selection
  - Field value selection
  - Related record preview
  - Confirmation

- [x] **UI-04**: Admin can bulk merge multiple duplicate groups
  - Select groups to merge
  - Apply same rules to all
  - Progress indicator for batch

- [x] **UI-05**: Admin can view merge history with undo option
  - List of recent merges
  - Undo button for each (if within 30 days)

- [x] **UI-06**: Admin can view and manage scheduled scan jobs
  - Create/edit/delete schedules
  - View job history
  - Trigger manual scan

---

## Future Requirements (Deferred)

- Cross-entity duplicate detection (Lead-to-Contact matching)
- ML-based matching algorithms
- External dedup service integration (DataGroomr, Insycle)
- Address standardization via external API
- Auto-merge without review for high-confidence matches
- Duplicate prevention policies that block creation entirely

---

## Out of Scope

| Feature | Reason |
|---------|--------|
| Real-time keystroke duplicate checking | Performance impact, poor UX |
| Phonetic matching (Soundex for matching) | Jaro-Winkler sufficient, Soundex only for blocking |
| Merge chains (transitive merge tracking) | Complex edge case, low value |
| Deduplication-as-a-service API | Not core CRM value |

---

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| DETECT-01 | Phase 11 | Complete |
| DETECT-02 | Phase 12 | Complete |
| DETECT-03 | Phase 11 | Complete |
| DETECT-04 | Phase 11 | Complete |
| DETECT-05 | Phase 11 | Complete |
| DETECT-06 | Phase 11 | Complete |
| DETECT-07 | Phase 11 | Partial |
| DETECT-08 | Phase 11 | Complete |
| DETECT-09 | Phase 12 | Complete |
| IMPORT-01 | Phase 14 | Complete |
| IMPORT-02 | Phase 14 | Complete |
| IMPORT-03 | Phase 14 | Complete |
| IMPORT-04 | Phase 14 | Complete |
| IMPORT-05 | Phase 14 | Complete |
| MERGE-01 | Phase 13 | Complete |
| MERGE-02 | Phase 13 | Complete |
| MERGE-03 | Phase 13 | Complete |
| MERGE-04 | Phase 13 | Complete |
| MERGE-05 | Phase 13 | Complete |
| MERGE-06 | Phase 13 | Complete |
| MERGE-07 | Phase 13 | Complete |
| MERGE-08 | Phase 13 | Complete |
| MERGE-09 | Phase 13 | Complete |
| MERGE-10 | Phase 13 | Complete |
| MERGE-11 | Phase 13 | Complete |
| BACKGROUND-01 | Phase 15 | Complete |
| BACKGROUND-02 | Phase 15 | Complete |
| BACKGROUND-03 | Phase 15 | Complete |
| BACKGROUND-04 | Phase 15 | Complete |
| BACKGROUND-05 | Phase 15 | Complete |
| BACKGROUND-06 | Phase 15 | Complete |
| UI-01 | Phase 16 | Complete |
| UI-02 | Phase 16 | Complete |
| UI-03 | Phase 16 | Complete |
| UI-04 | Phase 16 | Complete |
| UI-05 | Phase 16 | Complete |
| UI-06 | Phase 16 | Complete |

---

*37 requirements across 5 categories*
*Last updated: 2026-02-08 - Phase 14 requirements complete*
