# Research Summary: Deduplication System

**Project:** Quantico CRM v3.0 - Deduplication Milestone
**Domain:** CRM Data Quality - Duplicate Detection and Merging
**Researched:** 2026-02-05
**Confidence:** HIGH

## Executive Summary

CRM deduplication is a well-understood problem with established patterns from Salesforce, HubSpot, Zoho, and Dynamics 365. The key insight from research is that effective dedup requires a two-stage approach: **blocking** (SQL-based candidate selection to avoid O(n^2) comparisons) followed by **scoring** (detailed Go-side fuzzy matching). Turso's built-in SQLean Fuzzy extension provides `fuzzy_soundex()` and `fuzzy_jaro_winkler()` directly in SQL, eliminating the need to pull all data into Go for initial filtering.

The recommended approach for Quantico CRM is: use `github.com/adrg/strutil` for Go-side string similarity (Jaro-Winkler for names, Levenshtein for emails), `github.com/nyaruka/phonenumbers` for phone normalization to E.164 format, and in-process goroutine workers for background scanning (no Redis dependency needed). The system must integrate with existing handlers (`generic_entity.go`, `import.go`) as a pre-save validation step, following the pattern established by `ValidationService`.

The primary risks are: **data loss during merge** (orphaned related records if lookup fields aren't updated), **performance degradation** (naive O(n^2) comparison kills large tenants), and **cross-tenant data leakage** (if detection logic ever queries across org boundaries). All three are preventable with proper architecture - the research provides specific patterns for each.

## Key Findings

### Recommended Stack

From STACK.md: Two Go libraries plus Turso's built-in fuzzy matching cover all needs.

**Core technologies:**
- `github.com/adrg/strutil` v0.3.1: String similarity metrics (Jaro-Winkler, Levenshtein, Jaccard) - cleanest API of Go options, actively maintained
- `github.com/nyaruka/phonenumbers` v1.6.8: Phone normalization - Google libphonenumber port, production-tested, E.164 format
- Turso SQLean Fuzzy (built-in): `fuzzy_soundex()`, `fuzzy_leven()`, `fuzzy_jaro_winkler()` for SQL-level blocking
- In-process goroutine workers: Simpler than Redis queue, sufficient for per-tenant DB architecture

**Algorithm selection:**
| Field Type | Algorithm | Threshold |
|------------|-----------|-----------|
| Names | Jaro-Winkler | 0.85+ |
| Emails | Exact match on domain, Levenshtein on local part | 2 edits max |
| Company names | Jaro-Winkler + token sort | 0.80+ |
| Phones | E.164 normalization then exact match | - |

### Expected Features

From DEDUP_FEATURES.md: Clear tier structure based on competitive analysis of Salesforce, HubSpot, Zoho, Dynamics 365, and EspoCRM.

**Must have (table stakes):**
- Configurable matching rules per entity (admin UI, not code)
- Duplicate detection on record creation (warn before save)
- Duplicate detection during CSV import (extend existing import)
- Duplicate review list with match scores
- Manual merge UI with field-by-field selection
- Related record transfer (activities, notes to survivor)
- Merge audit logging

**Should have (differentiators):**
- Fuzzy matching with Jaro-Winkler scoring
- Confidence scores (0-100)
- Master record auto-selection rules
- Merge undo/rollback (30-day window)
- Bulk merge with per-item status

**Defer (v2+):**
- Cross-entity detection (Lead-Contact matching)
- ML-based matching
- External dedup service integration
- Address standardization via external API

### Architecture Approach

From ARCHITECTURE.md: Follows existing handler -> service -> repo pattern. The dedup service integrates as a sibling to ValidationService, hooking into `GenericEntityHandler.Create()` before save.

**Major components:**
1. `internal/service/dedup.go` - Core detection engine (rule loading, candidate selection, scoring)
2. `internal/service/similarity.go` - String similarity algorithms (Jaro-Winkler, Levenshtein wrappers)
3. `internal/handler/dedup.go` - Admin API for rules, scans, merge operations
4. `internal/repo/dedup.go` - CRUD for rules, duplicate groups, merge logs

**New database tables:**
- `dedup_rules` - Per-entity matching configuration
- `duplicate_groups` - Detected duplicate pairs with scores
- `merge_logs` - Audit trail with full record snapshots for undo
- `dedup_scan_jobs` - Background job tracking

**All tables in tenant DB** (not master) - ensures tenant isolation.

**Data flow (real-time):**
1. Record create/update triggers dedup check
2. Load active rules for entity type (cached)
3. SQL blocking query finds candidates (Soundex, prefix match)
4. Go-side scoring of candidates (Jaro-Winkler)
5. Return matches above threshold (409 Conflict) or proceed

### Critical Pitfalls

From PITFALLS.md: Five must-avoid pitfalls, all preventable with proper design.

1. **Related record orphaning** - Merge deletes source record but lookup fields still reference it. **Prevention:** Discover ALL lookup relationships from metadata, update foreign keys BEFORE delete, verify zero orphans in transaction.

2. **Quadratic detection complexity** - Naive O(n^2) comparison kills performance at scale. **Prevention:** Use blocking strategy (Soundex, prefix, email domain) to reduce candidates from n to ~50-100 before scoring.

3. **Irreversible merges** - User merges wrong records, no undo, data lost forever. **Prevention:** Store full snapshots of both records before merge in `merge_logs`, enable time-limited undo (30 days).

4. **Cross-tenant data exposure** - Detection bug compares records across orgs. **Prevention:** ALL queries use tenant DB connection (never master), add `org_id` assertions, write isolation tests.

5. **False positive overload** - Too many "duplicates" that aren't actually duplicates. **Prevention:** Multi-field weighted scoring, negative signals (different company = reduce score), tiered confidence (HIGH/MEDIUM/LOW).

## Implications for Roadmap

Based on research dependencies and pitfall assignments, recommended 6-phase structure:

### Phase 1: Foundation and Detection Engine
**Rationale:** Core infrastructure required by all subsequent phases. Blocking strategy must be correct from day one to avoid performance trap.
**Delivers:** Database schema, entity types, repository, similarity service, rule loading with caching
**Addresses:** Configurable matching rules (table stakes)
**Avoids:** Quadratic complexity (blocking strategy baked in), cross-tenant exposure (tenant DB only)
**Complexity:** Medium

### Phase 2: Real-Time Detection Integration
**Rationale:** Prevents new duplicates from entering system. Must work before merge can be useful.
**Delivers:** Hook into `GenericEntityHandler.Create()`, duplicate warning on save
**Addresses:** Duplicate detection on record creation (table stakes)
**Uses:** Dedup service from Phase 1, strutil for scoring
**Complexity:** Medium

### Phase 3: Manual Merge Engine
**Rationale:** Most complex phase, highest risk for data loss. Requires Phase 1 complete. Critical for user trust.
**Delivers:** Merge execution, related record re-pointing, field-by-field selection, merge audit logging with snapshots, undo capability
**Addresses:** Manual merge UI, related record transfer, merge undo (table stakes + differentiator)
**Avoids:** Related record orphaning, irreversible merges
**Complexity:** High

### Phase 4: Import Integration
**Rationale:** Imports are the primary source of duplicates. Extends existing import handler.
**Delivers:** Duplicate detection during CSV import, preview with match scores, skip/merge/import options
**Addresses:** Duplicate detection during import (table stakes)
**Uses:** Detection engine from Phase 1, extends `import.go`
**Complexity:** Medium

### Phase 5: Background Scanning
**Rationale:** Finds duplicates in existing data. Can operate independently once detection engine works.
**Delivers:** Scheduled duplicate scans, job status tracking, email notifications
**Addresses:** Scheduled background detection (differentiator)
**Avoids:** Long-running job lock contention (chunked transactions)
**Complexity:** Medium

### Phase 6: UI Components
**Rationale:** Frontend last - backend must be complete first.
**Delivers:** Admin UI for rule management, duplicate review queue, merge wizard
**Addresses:** Duplicate review list, merge preview (table stakes)
**Complexity:** Medium

### Phase Ordering Rationale

- **Phases 1-2-3 are sequential:** Detection engine (1) required before real-time hooks (2), both required before merge (3) can work
- **Phase 4 can parallel with Phase 3:** Import integration uses detection engine but not merge
- **Phase 5 after Phase 3:** Background scanning finds duplicates that need merge capability to resolve
- **Phase 6 last:** All backend APIs must exist before building UI

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Merge Engine):** Complex related record discovery, need to map all lookup fields dynamically. Research existing `RelatedListConfig` and `metadata.go` patterns.
- **Phase 5 (Background Scanning):** Job scheduling patterns, need to evaluate goroutine pool sizing for tenant fairness.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Foundation):** Well-documented handler/service/repo pattern, strutil API is simple
- **Phase 2 (Real-Time Integration):** Follows existing ValidationService hook pattern exactly
- **Phase 4 (Import Integration):** Extends existing import handler, pattern established
- **Phase 6 (UI Components):** Standard Svelte admin patterns, no special research needed

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official docs for strutil, phonenumbers, SQLean Fuzzy. All actively maintained. |
| Features | HIGH | Verified against Salesforce, HubSpot, Zoho, Dynamics 365 documentation |
| Architecture | HIGH | Based on existing Quantico codebase analysis + established CRM patterns |
| Pitfalls | HIGH | Multiple vendor experiences, CRM community forums, documented case studies |

**Overall confidence:** HIGH

### Gaps to Address

- **Expression indexes in Turso:** STACK.md notes uncertainty whether Turso supports indexes on expressions like `fuzzy_soundex(first_name)`. If not, compute and store Soundex in separate column. **Validate during Phase 1.**

- **Bulk merge transaction strategy:** Should bulk merge use one transaction per merge or batch commits? Need to balance atomicity vs lock contention. **Decide during Phase 3 planning.**

- **Merge undo complexity for multi-way merges:** Undo is straightforward for 2-record merge. For 3+ records merged together, restoration is more complex. **Document limitation or design solution in Phase 3.**

## Sources

### Primary (HIGH confidence)
- [strutil GitHub](https://github.com/adrg/strutil) - String similarity API, algorithm details
- [phonenumbers GitHub](https://github.com/nyaruka/phonenumbers) - Phone normalization, E.164 format
- [Turso SQLite Extensions](https://docs.turso.tech/features/sqlite-extensions) - Built-in SQLean Fuzzy
- [SQLean Fuzzy Documentation](https://github.com/nalgeon/sqlean/blob/main/docs/fuzzy.md) - Fuzzy function reference
- [EspoCRM Duplicate Checking](https://docs.espocrm.com/development/duplicate-check/) - Reference implementation
- [Microsoft Dynamics Duplicate Detection](https://learn.microsoft.com/en-us/power-platform/admin/run-bulk-system-jobs-detect-duplicate-records) - Enterprise patterns

### Secondary (MEDIUM confidence)
- [Insycle: Data Retention When Merging](https://blog.insycle.com/data-retention-merging-duplicates) - Merge pitfalls
- [Insycle: Master Record Selection](https://blog.insycle.com/picking-master-record-crm-deduplication) - Survivorship rules
- [Tilores: Fuzzy Matching Algorithms](https://tilores.io/fuzzy-matching-algorithms) - Algorithm selection guidance
- [AWS: SaaS Tenant Isolation Strategies](https://d1.awsstatic.com/whitepapers/saas-tenant-isolation-strategies.pdf) - Multi-tenant security

### Tertiary (LOW confidence)
- Community forums (HubSpot, Salesforce) - User experience anecdotes
- Medium articles on dedup - General patterns, needs validation

---
*Research completed: 2026-02-05*
*Ready for roadmap: yes*
