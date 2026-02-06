# Quantico CRM Roadmap

## Milestones

- **v1.0 Platform Update System** - Phases 01-05 (shipped 2026-02-01) -> [archive](milestones/v1.0-ROADMAP.md)
- **v2.0 Security Hardening** - Phases 06-10 (shipped 2026-02-04) -> [archive](milestones/v2.0-ROADMAP.md)
- **v3.0 Deduplication System** - Phases 11-16 (in progress)

---

## v3.0 Deduplication System

**Milestone Goal:** Comprehensive, entity-agnostic deduplication system with scoring-based matching, import integration, manual merge with undo, and background scanning.

### Phase 11: Detection Foundation
**Goal**: Core deduplication infrastructure with similarity algorithms and configurable rules
**Depends on**: Phase 10 (v2.0 complete)
**Requirements**: DETECT-01, DETECT-03, DETECT-04, DETECT-05, DETECT-06, DETECT-07, DETECT-08
**Success Criteria** (what must be TRUE):
  1. Admin can create matching rules specifying fields, logic, and fuzzy vs exact matching per entity
  2. Jaro-Winkler algorithm returns similarity scores for name comparisons (threshold 0.88)
  3. Email normalization produces consistent lowercase, trimmed values for exact matching
  4. Phone numbers normalize to E.164 format for comparison
  5. Blocking queries reduce candidate set using Soundex and prefix strategies
**Plans**: TBD

Plans:
- [ ] 11-01: Database schema and entity types
- [ ] 11-02: Similarity service (Jaro-Winkler, email, phone normalization)
- [ ] 11-03: Rule management API and caching

### Phase 12: Real-Time Detection
**Goal**: Prevent new duplicates by detecting matches during record creation
**Depends on**: Phase 11
**Requirements**: DETECT-02, DETECT-09
**Success Criteria** (what must be TRUE):
  1. When creating a record, system shows duplicate warning with match scores before save
  2. User can choose warn mode (proceed anyway) or block mode (must resolve first)
  3. Confidence levels display as High/Medium/Low tiers (>=95%, >=85%, >=70%)
**Plans**: TBD

Plans:
- [ ] 12-01: Hook into GenericEntityHandler.Create() for duplicate check
- [ ] 12-02: Duplicate warning UI with confidence tiers

### Phase 13: Merge Engine
**Goal**: Complete merge capability with field selection, related record transfer, audit logging, and undo
**Depends on**: Phase 11
**Requirements**: MERGE-01, MERGE-02, MERGE-03, MERGE-04, MERGE-05, MERGE-06, MERGE-07, MERGE-08, MERGE-09, MERGE-10, MERGE-11
**Success Criteria** (what must be TRUE):
  1. User can merge 2+ duplicate records, selecting survivor and field values via side-by-side UI
  2. All related records (Tasks, Activities, Notes, any lookup references) transfer to survivor automatically
  3. Merge executes atomically (all or nothing) with full audit log of who/when/what
  4. Merge preview shows before/after comparison, related record counts, and data loss warnings
  5. User can undo merge within 30 days, restoring source records and re-pointing related records
**Plans**: TBD

Plans:
- [ ] 13-01: Merge execution with atomic transactions
- [ ] 13-02: Related record discovery and transfer
- [ ] 13-03: Merge audit logging and snapshots
- [ ] 13-04: Undo capability with restoration
- [ ] 13-05: Multi-record merge (3+ records)

### Phase 14: Import Integration
**Goal**: Extend CSV import to detect and handle duplicates during import
**Depends on**: Phase 11
**Requirements**: IMPORT-01, IMPORT-02, IMPORT-03, IMPORT-04, IMPORT-05
**Success Criteria** (what must be TRUE):
  1. CSV import analyze step shows potential duplicates with match scores
  2. User can choose skip/update/import/merge for each duplicate row
  3. Import detects duplicates within the file (rows duplicating each other)
  4. Import blocks proceeding until all duplicate decisions are made
**Plans**: TBD

Plans:
- [ ] 14-01: Duplicate detection during import analyze
- [ ] 14-02: Within-file duplicate detection
- [ ] 14-03: Import duplicate resolution UI

### Phase 15: Background Scanning
**Goal**: Scheduled duplicate scans with job management and notifications
**Depends on**: Phase 11, Phase 13
**Requirements**: BACKGROUND-01, BACKGROUND-02, BACKGROUND-03, BACKGROUND-04, BACKGROUND-05, BACKGROUND-06
**Success Criteria** (what must be TRUE):
  1. Admin can schedule duplicate scans per entity type (daily/weekly/monthly)
  2. Scans use cursor-based chunking with checkpoint progress to avoid timeouts
  3. Job status shows pending/running/completed/failed with progress percentage
  4. Admin receives email notification when scan completes with summary and link to review queue
  5. Failed jobs can be retried from last checkpoint
**Plans**: TBD

Plans:
- [ ] 15-01: Job infrastructure with scheduling
- [ ] 15-02: Chunked scanning with checkpoints
- [ ] 15-03: Job notifications and retry logic

### Phase 16: Admin UI
**Goal**: Complete admin interface for duplicate rule management, review queue, and merge wizard
**Depends on**: Phase 11, Phase 12, Phase 13, Phase 15
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-06
**Success Criteria** (what must be TRUE):
  1. Admin can manage matching rules in Settings > Data Quality > Duplicate Rules
  2. Duplicate review queue shows all detected duplicates grouped by entity, sorted by confidence
  3. Merge wizard guides user through survivor selection, field selection, related preview, and confirmation
  4. Admin can bulk merge multiple duplicate groups with progress indicator
  5. Merge history shows recent merges with undo option (if within 30 days)
  6. Admin can view and manage scheduled scan jobs
**Plans**: TBD

Plans:
- [ ] 16-01: Duplicate rule management UI
- [ ] 16-02: Duplicate review queue
- [ ] 16-03: Merge wizard
- [ ] 16-04: Bulk merge and merge history UI
- [ ] 16-05: Scan job management UI

---

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 01-05 | v1.0 | 9/9 | Complete | 2026-02-01 |
| 06-10 | v2.0 | 22/22 | Complete | 2026-02-04 |
| 11. Detection Foundation | v3.0 | 0/3 | Not started | - |
| 12. Real-Time Detection | v3.0 | 0/2 | Not started | - |
| 13. Merge Engine | v3.0 | 0/5 | Not started | - |
| 14. Import Integration | v3.0 | 0/3 | Not started | - |
| 15. Background Scanning | v3.0 | 0/3 | Not started | - |
| 16. Admin UI | v3.0 | 0/5 | Not started | - |

**v3.0 Total:** 0/21 plans

---

*Last updated: 2026-02-06 - v3.0 roadmap created*
