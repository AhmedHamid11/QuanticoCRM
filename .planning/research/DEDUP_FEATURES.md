# Feature Landscape: CRM Deduplication System

**Domain:** CRM Data Quality - Duplicate Detection and Merging
**Researched:** 2026-02-05
**Overall Confidence:** HIGH (verified against Salesforce, HubSpot, Zoho, Dynamics 365, EspoCRM documentation)

---

## Executive Summary

This document maps the deduplication feature landscape for CRM systems, based on analysis of how Salesforce, HubSpot, Zoho CRM, Microsoft Dynamics 365, EspoCRM, and third-party deduplication tools handle duplicate records. Features are categorized by user expectation level and implementation complexity.

**Context (Quantico CRM):**
- Existing: Contact, Account, Lead entities with custom fields
- Existing: CSV import with field mapping, upsert mode, match field support
- Existing: Entity definition system (dynamic schemas)
- Existing: Audit logging infrastructure
- Target: Multi-entity deduplication with scoring, import integration, background jobs

**User Requirements:**
- Scoring system for duplicate confidence
- Import integration (detect during import)
- Background deduplication jobs
- Manual merge with field selection
- Works on Contact, Account, Lead, and custom entities

---

## Table Stakes (Must Have for MVP Deduplication)

Features users **expect** from any CRM deduplication system. Missing = product feels incomplete.

| Feature | Why Expected | Complexity | Dependencies | Notes |
|---------|--------------|------------|--------------|-------|
| **Configurable matching rules** | Users must define what constitutes a duplicate (email, phone, name combination) | Medium | Entity field definitions | Per-entity rules: "Match if email is identical" or "Match if first name + last name + company" |
| **Duplicate detection on record creation** | Prevent duplicates at point of entry; Salesforce/HubSpot/Zoho all do this | Medium | Matching rules | Show warning/alert before saving; optionally block creation |
| **Duplicate detection during import** | Imports are primary source of duplicates; existing import has upsert but not detection | Medium | Import handler, matching rules | Extend existing CSV import with duplicate scanning |
| **Duplicate review list** | Users need to see flagged potential duplicates in one place | Low | Matching rules | List view showing duplicate groups with match scores |
| **Manual merge UI** | Users must select which field values to keep from each duplicate | Medium | Field definitions | Side-by-side comparison; pick "keep this value" per field |
| **Survivor record selection** | One record becomes the "winner"; others are merged into it | Low | Merge UI | Usually most complete or most recent; user picks master |
| **Related record transfer** | Merge must move child records (activities, notes, related lists) to survivor | Medium | Related list configs | Tasks, calls, notes from victim record move to survivor |
| **Basic matching algorithms** | Exact match + case-insensitive match minimum viable | Low | None | "john@example.com" = "JOHN@example.com" |
| **Merge audit trail** | Record what was merged, when, by whom; critical for compliance | Low | Audit logging | Log: "Contact A merged into Contact B by User X" |
| **Undo/reverse merge** | Mistakes happen; users expect ability to undo | High | Audit logging, snapshot storage | Store pre-merge state; restore on undo |

### Implementation Notes

**Matching Rules Configuration:**
Based on Salesforce/EspoCRM patterns, rules should support:
- Field-level matching (email, phone, name fields)
- AND/OR logic ("email matches AND phone matches" vs "email OR phone")
- Fuzzy vs exact matching per field
- Per-entity configuration (Contacts may use email; Accounts may use company name + website)

**EspoCRM Reference:**
EspoCRM uses `duplicateWhereBuilderClassName` in metadata to specify custom duplicate logic. This is flexible but requires code. Quantico should offer admin UI configuration instead.

**Import Integration:**
Existing import handler has `ImportModeUpsert` with `matchField` support. Extend to:
1. Run duplicate detection on all records before insert
2. Flag duplicates in import results
3. Offer "merge with existing" option

---

## Differentiators (Competitive Advantage)

Features that distinguish excellent deduplication from basic. Not expected, but valued highly.

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| **Fuzzy matching with similarity scoring** | Catch "Jon Smith" vs "John Smith" that exact match misses | High | Matching algorithm library | Jaro-Winkler for names; Levenshtein for general strings |
| **Confidence scores (0-100)** | Let users prioritize high-confidence duplicates; reduce false positive review burden | Medium | Matching algorithm scoring | "95% match" vs "72% match" helps triage |
| **Scheduled background duplicate detection** | Find duplicates in existing data without manual search | Medium | Background job system | Daily/weekly scans; notify when new duplicates found |
| **Bulk merge** | Handle 100s of duplicate groups efficiently | Medium | Merge logic, job queue | Select multiple groups; apply same merge rules |
| **Auto-merge rules** | High-confidence duplicates (99%+) can be auto-merged | Medium | Confidence scoring, merge logic | "If score > 95% and created < 24h ago, auto-merge" |
| **Master record rules** | Automatically select survivor based on criteria | Low | Record metadata | "Keep record with most activity" or "Keep oldest" or "Keep most complete" |
| **Field-level merge strategies** | Beyond "pick one": concatenate notes, keep most recent, keep non-empty | Medium | Merge logic | Notes: concatenate; Phone: keep most recent; Email: keep first non-empty |
| **Cross-entity duplicate detection** | Lead might duplicate a Contact; detect across entity types | High | All entity schemas | Salesforce does this with Lead-to-Contact conversion dedup |
| **Real-time duplicate alert on typing** | As user types email, show "Similar record exists" | Medium | Frontend reactive query | Typeahead style; reduces duplicates before form submit |
| **Duplicate prevention policies** | Block creation entirely if duplicate found (not just warn) | Low | Matching rules | Admin config: "Block duplicates on Contact" vs "Warn only" |
| **Merge preview** | Show exactly what the merged record will look like before commit | Low | Merge logic | Side-by-side: "Before merge" vs "After merge" |
| **Integration with external dedup services** | Use specialized services (DataGroomr, Insycle) via API | High | External API integration | For customers who want enterprise-grade matching |

### Algorithm Recommendations

**For name matching (firstName, lastName, company):**
Use Jaro-Winkler distance - handles transpositions and minor variations better than Levenshtein for short strings like names. Threshold: 0.85-0.90 for "likely match".

**For email/phone matching:**
Normalize first (lowercase email, strip phone formatting), then exact match. Optionally fuzzy match on email local part only for typo detection.

**For address matching:**
Normalize street abbreviations (St/Street, Ave/Avenue), then Levenshtein with low threshold (0.90+). Address matching is complex - consider deferring to post-MVP.

**Scoring formula recommendation:**
```
totalScore = SUM(fieldWeight * fieldMatchScore) / SUM(fieldWeight)

Example:
- email exact match: weight=40, score=100 -> contributes 4000
- firstName fuzzy match (0.92): weight=20, score=92 -> contributes 1840
- lastName fuzzy match (0.88): weight=20, score=88 -> contributes 1760
- phone exact match: weight=20, score=100 -> contributes 2000

totalScore = (4000 + 1840 + 1760 + 2000) / 100 = 96%
```

---

## Anti-Features (Explicitly Skip)

Features to deliberately **NOT** build. Common mistakes or scope creep in dedup systems.

| Anti-Feature | Why Avoid | What To Do Instead |
|--------------|-----------|-------------------|
| **ML-based duplicate detection** | Requires training data; complex to tune; overkill for CRM-scale data | Use proven algorithms (Jaro-Winkler, Levenshtein) with configurable thresholds |
| **Real-time dedup on every field keystroke** | Performance killer; annoying UX; unnecessary | Debounced check on field blur or before submit |
| **Automatic merge without review** | Too risky for CRM data; customers will lose important info | Auto-merge only with explicit admin opt-in and very high threshold (99%+) |
| **Complex cross-entity matching UI** | Lead-Contact-Account matching is confusing; edge case for MVP | Start with same-entity matching; cross-entity as future enhancement |
| **Phonetic matching (Soundex, Metaphone)** | Language-dependent; complex to implement correctly; marginal value | Jaro-Winkler handles most name variations adequately |
| **Address standardization via external service** | Adds external dependency; cost per record; edge case | Simple address normalization (abbreviations) is sufficient for MVP |
| **Merge chains (A->B, B->C, therefore A->C)** | Complex to track; confusing to users; rare case | Each merge is independent; if C has duplicates, show them separately |
| **Deduplication-as-a-service API** | Scope creep; not core CRM value | Focus on built-in dedup; integrations can come later |

---

## User Workflows

How users actually interact with deduplication in practice.

### Workflow 1: Prevent Duplicates During Data Entry

**Trigger:** User creates new Contact/Lead/Account
**Steps:**
1. User fills in form (name, email, phone, company)
2. On field blur (email, phone) or before save, system checks for duplicates
3. If potential duplicates found:
   - Show banner: "1 similar contact found" with link to view
   - User can: Ignore and save anyway, View potential duplicate, Cancel and edit existing
4. If blocked by admin policy, save button is disabled with explanation

**UX Considerations:**
- Don't interrupt typing - check on blur or submit
- Show duplicate matches inline, not in modal (less disruptive)
- Make "view duplicate" easy (new tab or side panel)

### Workflow 2: Handle Duplicates During Import

**Trigger:** User imports CSV file
**Steps:**
1. User uploads CSV and maps columns (existing flow)
2. System runs duplicate detection on all rows vs existing records
3. Import preview shows:
   - Rows that will create new records
   - Rows that match existing records (with match score)
   - Rows that are duplicates of each other within file
4. User chooses action for duplicates:
   - Skip duplicate rows
   - Update existing records (upsert)
   - Import anyway (create duplicates)
   - Review and merge individually
5. Import proceeds based on selection

**Builds on existing:** Current import has `upsert` mode with `matchField`. Extend with:
- Multi-field matching
- Fuzzy matching option
- Duplicate detection results in preview

### Workflow 3: Review and Merge Existing Duplicates

**Trigger:** Admin runs duplicate scan or views duplicate report
**Steps:**
1. Navigate to Admin > Data Quality > Duplicates (or entity list > Actions > Find Duplicates)
2. View list of duplicate groups, sorted by confidence score
3. Click a group to see side-by-side comparison of duplicate records
4. For each field, select which value to keep (radio buttons)
   - Or use "auto-select" rules: most recent, most complete, master record
5. Preview merged record
6. Click Merge - survivor record is updated, victim record(s) deleted
7. Related records (tasks, notes) transferred to survivor
8. Merge logged to audit trail

**UX Considerations:**
- Default selections based on merge rules (don't require all manual picks)
- Show field differences highlighted
- Show related record counts ("5 tasks will be transferred")
- Confirm before destructive action

### Workflow 4: Bulk Cleanup Campaign

**Trigger:** Quarterly data cleanup initiative
**Steps:**
1. Admin schedules background duplicate scan for entity type
2. Scan runs overnight, detects duplicate groups
3. Results emailed to admin: "Found 247 potential duplicate Contacts"
4. Admin reviews in batch mode:
   - Sort by score (highest first)
   - Apply master record rules to auto-select field values
   - Select multiple groups, click "Merge All Selected"
5. System processes merges in background
6. Summary report: "Merged 215 duplicates, 32 skipped (insufficient confidence)"

**UX Considerations:**
- Background jobs should not block UI
- Progress indicator for large batch merges
- Export duplicate report to CSV for offline review

### Workflow 5: Undo a Bad Merge

**Trigger:** User realizes they merged wrong records
**Steps:**
1. Navigate to merged record's audit history
2. Find merge event: "Merged from Contact B on 2026-02-05"
3. Click "Undo Merge"
4. System confirms: "This will restore Contact B as a separate record"
5. Both records restored to pre-merge state
6. Related records re-linked to original parents

**Dependencies:**
- Must snapshot both records before merge
- Must track which related records were transferred
- Complex for multi-way merges (A + B + C -> A)

---

## Competitive Analysis

### Salesforce Duplicate Management

**Native features (included):**
- Matching rules: Compare fields like email, phone, name
- Duplicate rules: Define action (alert, block, report)
- Duplicate jobs: Scan existing data (Performance/Unlimited editions only)
- Compare and Merge: Manual merge UI

**Limitations:**
- Cannot auto-merge natively
- Limited to standard and custom objects
- Duplicate jobs only in expensive editions

**Third-party tools (DataGroomr, Plauti Deduplicate):**
- AI/ML-powered matching
- Bulk merge with undo
- Scheduled scans
- Tag-based mass merging

**Takeaway:** Salesforce native is prevention-focused (block/warn). Cleanup requires AppExchange tools. Quantico opportunity: include cleanup in core.

### HubSpot Duplicate Management

**Native features:**
- Auto-dedup on identical email (Contacts) or domain (Companies)
- Unique property enforcement (up to 10 properties)
- Duplicate Manager tool (Pro/Enterprise only)
- Bulk duplicate management (Data Hub subscription)

**Limitations:**
- Very limited native automation
- No fuzzy matching
- Requires paid subscription for bulk features

**Third-party tools (Insycle, Dedupely, Koalify):**
- Bulk merge with master selection rules
- Scheduled automation
- Multi-criteria matching
- Merge undo

**Takeaway:** HubSpot native dedup is basic - just email/domain exact match. Quantico opportunity: better out-of-box experience than HubSpot free tier.

### Zoho CRM Duplicate Management

**Native features:**
- Per-record "Find and Merge Duplicates" button
- Module-wide deduplication (select criteria fields)
- Auto-merge for exact matches
- Manual conflict resolution for partial matches

**Approach:**
- More proactive about merging than Salesforce/HubSpot
- Auto-merge exact, manual merge for conflicts
- Good balance of automation and control

**Takeaway:** Zoho's approach is user-friendly. Quantico should follow similar pattern: auto-merge obvious duplicates, flag ambiguous ones.

### Microsoft Dynamics 365

**Native features:**
- Duplicate detection rules (field-based matching)
- Duplicate detection jobs (bulk scan)
- Block or warn on duplicate creation
- Merge records UI

**Advanced tools (Inogic DeDupeD):**
- Scheduled detection jobs (daily/weekly/monthly)
- Master record selection rules
- Merge undo
- Cross-entity deduplication

**Takeaway:** Dynamics has mature native dedup. Good reference for enterprise features.

### EspoCRM (Reference Implementation)

**Native features:**
- Duplicate check on record creation
- Configurable via `duplicateWhereBuilderClassName` in metadata
- Default checks: name and/or email
- Manual merge of selected records

**Implementation:**
```javascript
// In entityDefs metadata:
"duplicateWhereBuilderClassName": "Espo\\Custom\\Classes\\DuplicateWhereBuilders\\Lead"
```

**Import handling:**
- Import can skip duplicates or update existing
- Duplicate detection runs during import
- Post-import report shows duplicates found

**Takeaway:** EspoCRM approach is developer-focused (requires code). Quantico should offer admin UI configuration.

---

## Feature Prioritization for Quantico CRM

Based on user requirements and competitive analysis:

### Phase 1: Core Deduplication (MVP)

**Must build:**
1. Matching rule configuration (admin UI)
2. Duplicate detection on record save (warn)
3. Import duplicate detection (extend existing)
4. Duplicate review list
5. Manual merge UI with field selection
6. Related record transfer
7. Merge audit logging

**Complexity:** Medium-High
**Duration estimate:** 2-3 sprints

### Phase 2: Automation and Scoring

**Should build:**
1. Fuzzy matching (Jaro-Winkler for names)
2. Confidence scoring (0-100)
3. Master record auto-selection rules
4. Bulk merge capability
5. Merge undo/rollback

**Complexity:** Medium
**Duration estimate:** 1-2 sprints

### Phase 3: Background Processing

**Nice to have:**
1. Scheduled duplicate detection jobs
2. Email notifications for new duplicates
3. Auto-merge for high-confidence matches
4. Duplicate prevention policies (block mode)

**Complexity:** Medium
**Duration estimate:** 1 sprint

---

## Sources

### Official Documentation
- [EspoCRM Duplicate Checking](https://docs.espocrm.com/development/duplicate-check/)
- [Zoho CRM Auto-Merge Duplicates](https://help.zoho.com/portal/en/kb/crm/manage-crm-data/duplication-management/articles/auto-merge-duplicates)
- [HubSpot Deduplicate Records](https://knowledge.hubspot.com/records/deduplication-of-records)
- [Microsoft Dynamics Duplicate Detection](https://learn.microsoft.com/en-us/power-platform/admin/run-bulk-system-jobs-detect-duplicate-records)

### Industry Analysis
- [Salesforce Ben: Duplicate Management Guide](https://www.salesforceben.com/salesforce-duplicate-rules/)
- [Databar: CRM Deduplication Complete Guide](https://databar.ai/blog/article/crm-deduplication-complete-guide-to-finding-merging-duplicate-records)
- [RT Dynamic: CRM Deduplication Guide 2025](https://www.rtdynamic.com/blog/crm-deduplication-guide-2025/)
- [Insycle: Deduplication Best Practices](https://support.insycle.com/hc/en-us/articles/6584810088855-Deduplication-Best-Practices)
- [Insycle: Picking the Right Master Record](https://blog.insycle.com/picking-master-record-crm-deduplication)

### Algorithms
- [Tilores: Fuzzy Matching Algorithms](https://tilores.io/fuzzy-matching-algorithms)
- [Flagright: Jaro-Winkler vs Levenshtein](https://www.flagright.com/post/jaro-winkler-vs-levenshtein-choosing-the-right-algorithm-for-aml-screening)
- [Data Ladder: Fuzzy Matching 101](https://dataladder.com/fuzzy-matching-101/)

### Third-Party Tools (for reference)
- [DataGroomr Salesforce Deduplication 2025](https://datagroomr.com/salesforce-deduplication-in-2025/)
- [Inogic DeDupeD for Dynamics 365](https://www.inogic.com/product/productivity-apps/dedupe-find-clean-merge-duplicate-dynamics-365-crm-data/)
- [Dedupely CRM Deduplication](https://dedupe.ly/)
