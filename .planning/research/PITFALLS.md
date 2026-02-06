# Pitfalls Research: Deduplication System

**Domain:** CRM Deduplication for Multi-Tenant System
**Researched:** 2026-02-05
**Overall Confidence:** HIGH (verified against multiple CRM vendor experiences, industry documentation)

---

## Executive Summary

Adding deduplication to an existing CRM introduces unique risks that differ from greenfield implementations. The primary dangers are data loss during merge operations, performance degradation at scale, and cross-tenant data leakage in multi-tenant systems. This document catalogs pitfalls specific to Quantico CRM's architecture (per-tenant Turso databases, dynamic entity system, existing audit logging).

**Most Critical Pitfalls:**
1. Related record orphaning during merge (data loss)
2. Quadratic complexity in duplicate detection (performance)
3. Tenant isolation bypass in shared detection logic (security)
4. Irreversible merges without undo capability (user trust)

---

## Data Integrity Pitfalls

### CRITICAL: Related Record Orphaning

**What goes wrong:** When merging Record A into Record B (keeping B), foreign key references pointing to A are not updated, creating orphaned records. In CRM context: contacts linked to deleted account disappear, activities linked to merged lead vanish, quotes lose their associated opportunity.

**Why it happens in existing systems:**
- Dynamic entity system means relationships aren't known at compile time
- Lookup fields are discovered at runtime, not statically analyzed
- Existing code wasn't designed with merge operations in mind
- SQLite (Turso) doesn't have CASCADE UPDATE by default

**Warning signs:**
- After merge, related lists show fewer records than expected
- Activity timelines have gaps
- Reports show mismatched totals (10 contacts, but accounts show 8 total)
- "Record not found" errors when clicking on linked records

**Consequences:**
- Permanent data loss (activities, notes, attachments)
- Customer complaints about missing information
- Audit trail inconsistencies (references to deleted records)
- Reporting accuracy degradation

**Prevention:**
1. **Discover ALL lookup relationships before merge** - Query the dynamic field system to find every field that references the source entity type
2. **Update foreign keys BEFORE deleting source** - Use transaction to ensure atomicity
3. **Add referential integrity tests** - Test that after merge, zero records reference the deleted ID
4. **Create merge audit record** - Track what was merged, what was updated, enabling investigation

**Phase assignment:** Must be solved in Phase 1 (Merge Engine) before any merge capability ships.

**Implementation pattern:**
```sql
-- Before deleting record A, update all references to point to B
-- For each lookup field that references the entity type:
UPDATE tasks SET contact_id = :target_id WHERE contact_id = :source_id;
UPDATE activities SET contact_id = :target_id WHERE contact_id = :source_id;
-- ... for all discovered lookup fields

-- Verify no orphans would be created
SELECT COUNT(*) FROM tasks WHERE contact_id = :source_id;
-- Must be 0 before proceeding with delete
```

---

### CRITICAL: Master Record Selection Errors

**What goes wrong:** Wrong record selected as the master/survivor, causing newer or more accurate data to be overwritten by stale data. User selects record with outdated phone number as master, losing the current phone number forever.

**Why it happens:**
- UI doesn't clearly show which record is "winning"
- Automatic selection based on "most recently updated" isn't always correct (an old record might have been updated for a trivial reason)
- Users don't understand the consequence of their selection
- Bulk merge operations don't allow per-record review

**Warning signs:**
- Users complaining "the merge picked the wrong data"
- Customers receiving communications at old addresses
- Sales reps losing track of recent conversations

**Consequences:**
- Customer relationship damage
- Lost business opportunities
- User distrust of the dedup system
- Manual data re-entry required

**Prevention:**
1. **Field-by-field selection UI** - Never just pick "record A" or "record B"; let users choose best value for each field
2. **Visual diff display** - Side-by-side comparison showing exactly what will happen
3. **Preview before merge** - Show the final merged record before committing
4. **Recency indicators** - Show when each field was last updated, not just record-level timestamps
5. **Confidence scoring for auto-selection** - When automating, prefer: non-null > null, longer > shorter (for addresses), more recent > older

**Phase assignment:** Phase 2 (Manual Merge UI) - Critical for user trust.

---

### HIGH: Activity and Note Merging Confusion

**What goes wrong:** After merging two contacts, the activity timeline is confusing. Notes, calls, and emails from both records appear in a jumbled order. Some activities might appear duplicated. Users can't tell which activities came from which original record.

**Why it happens:**
- Activities don't have clear "source record" attribution after merge
- Timestamp sorting interleaves activities from both records
- Email threads might reference both old and new record IDs
- Notes from the deleted record don't indicate their origin

**Warning signs:**
- Users asking "where did this note come from?"
- Activity timelines that don't make chronological sense
- Duplicate meeting records appearing

**Prevention:**
1. **Add origin attribution to merged activities** - "Originally from [Record Name] (merged)"
2. **Preserve both creation and merge timestamps** - So users can filter "activities from before merge"
3. **Consider activity deduplication** - Same meeting might exist on both records
4. **Create merge activity** - "Merged from [Record Name] on [Date]" as first activity post-merge

**Phase assignment:** Phase 2 (Merge Engine enhancements)

---

### MEDIUM: Audit Trail Discontinuity

**What goes wrong:** After merge, the audit trail for the surviving record doesn't include the history of the deleted record. Compliance officers can't see who modified the now-deleted record. SOC 2 auditors flag incomplete audit trails.

**Why it happens:**
- Audit logs reference record IDs; deleted record's audit entries become orphaned
- Current audit system (hash chain) wasn't designed for record merges
- Record deletion removes the context needed to understand historical audits

**Warning signs:**
- Audit queries for merged record missing historical entries
- "Gaps" in modification history
- Compliance questions about incomplete records

**Prevention:**
1. **Never delete audit entries** - They're already immutable (hash chain)
2. **Add merge event to audit log** - "Record [A] merged into [B] by [User]"
3. **Include source record ID in merge audit entry** - Enables joining old audit entries
4. **Consider audit entry relinking** - Update foreign key in audit entries (if possible without breaking hash chain)

**Phase assignment:** Phase 1 - Audit logging integration must be in initial merge design.

---

### MEDIUM: Import Duplicate Detection Bypasses

**What goes wrong:** Dedup rules are correctly applied during import, but users find workarounds: importing via API directly, using browser console to POST records, importing in small batches that don't trigger detection. Result: duplicates enter the system despite dedup feature.

**Why it happens:**
- Dedup only runs on designated import paths
- Direct API creation doesn't invoke dedup
- Detection thresholds tuned for one import size fail at another

**Warning signs:**
- Duplicate counts increasing despite import dedup feature
- Users reporting "it let me import them anyway"
- API-created records showing as duplicates of UI-imported records

**Prevention:**
1. **Hook dedup into record creation, not just import** - Or clearly document that dedup is import-only
2. **Rate-limit direct API creation** - Make bulk import the only efficient path
3. **Provide duplicate report on all sources** - Background scan catches what import missed
4. **Log dedup decisions** - Know when something was flagged vs. allowed

**Phase assignment:** Phase 3 - Import Integration, with consideration for API hooks

---

## Performance Pitfalls

### CRITICAL: Quadratic Detection Complexity

**What goes wrong:** Duplicate detection compares every record to every other record, resulting in O(n^2) complexity. With 10,000 contacts, that's 100 million comparisons. System becomes unresponsive during scans, database locks up, users can't work.

**Why it happens:**
- Naive implementation: `for each record: compare to all other records`
- No blocking/indexing strategy to reduce comparison set
- Fuzzy matching algorithms (Levenshtein) are expensive per comparison
- Background job monopolizes database connection

**Warning signs:**
- "Scanning for duplicates" progress bar stuck at 10%
- Database CPU at 100% during scans
- Normal operations slowing down when dedup job runs
- Scans completing for small orgs but timing out for large ones

**Consequences:**
- Feature unusable for larger tenants
- Other users affected during scan
- Customer complaints about slow performance
- Background jobs failing and restarting repeatedly

**Prevention:**
1. **Implement blocking strategy** - Only compare records that share a blocking key (same email domain, same first 3 letters of name, same company). Reduces comparisons from n^2 to near-linear.
2. **Pre-compute comparison indexes** - On record save, compute normalized keys (lowercase email, phonetic name) that enable fast lookup
3. **Limit comparison window** - "Compare this record to records created in last 30 days" rather than entire database
4. **Use appropriate algorithms** - Jaro-Winkler faster than Levenshtein for names; exact match on normalized email before fuzzy matching
5. **Batch and yield** - Process 100 records, yield, process next 100. Never hold lock for full scan.

**Scaling reference (from research):**
- 10,000 records: Naive = 100M comparisons, Blocked = ~500K comparisons
- 100,000 records: Naive = 10B comparisons (hours), Blocked = ~5M comparisons (minutes)

**Phase assignment:** Phase 1 - Detection Engine must use blocking from day one.

**Implementation pattern:**
```go
// Blocking strategy: only compare records sharing a blocking key
func detectDuplicates(newRecord Record, allRecords []Record) []DuplicateCandidate {
    // Compute blocking keys for new record
    keys := computeBlockingKeys(newRecord)
    // keys might be: [email_domain:gmail.com, name_prefix:joh, company:acme]

    // Only fetch records matching at least one blocking key
    candidates := repo.FindByBlockingKeys(keys)

    // Now compare against reduced set
    for _, candidate := range candidates {
        score := computeSimilarity(newRecord, candidate)
        if score >= threshold {
            // Potential duplicate
        }
    }
}
```

---

### HIGH: Real-Time Detection Blocking User Operations

**What goes wrong:** Import with 1,000 records triggers synchronous duplicate detection. User waits 30 seconds while server processes. Timeout occurs, request fails, user retries, server gets overwhelmed.

**Why it happens:**
- Detection runs in the request path, not async
- Large imports processed as single transaction
- No streaming/chunking of results

**Warning signs:**
- Import timeouts for files over 100 records
- Browser showing "waiting for server" for extended periods
- Users complaining imports are "slow"

**Prevention:**
1. **Async detection for large imports** - Return immediately with job ID, poll for results
2. **Streaming detection** - Process records in batches of 50, return progressive results
3. **Optimistic import with post-hoc review** - Import immediately, queue detection job, notify user of results
4. **Set expectations** - Show "Scanning for duplicates..." progress indicator

**Phase assignment:** Phase 3 - Import Integration architecture decision

---

### MEDIUM: Merge Operations Blocking Table

**What goes wrong:** Merge operation updates many related records in a single transaction. Transaction holds write lock on multiple tables. Other users' operations queue behind the merge. System appears "frozen" to other users.

**Why it happens in SQLite/Turso:**
- SQLite uses database-level write locks
- Large transaction = long lock hold time
- Many foreign key updates in one merge operation
- No row-level locking available

**Warning signs:**
- "Database is locked" errors during merge
- Other users' saves failing while merge runs
- Merge of record with many related records taking much longer

**Prevention:**
1. **Chunk related record updates** - Update 50 records, commit, update next 50
2. **Use queued updates** - Mark records for update, process in background
3. **Implement merge lock indicator** - Show other users "Merge in progress, saves may be delayed"
4. **Prioritize user operations** - If conflict, let user save win, retry merge update

**Phase assignment:** Phase 2 - Merge Engine optimization

---

### MEDIUM: Index Bloat from Normalized Fields

**What goes wrong:** To enable efficient duplicate detection, normalized/blocking key columns are added to every record. These require indexes. With many entities and fields, index overhead grows, slowing writes.

**Why it happens:**
- Each blocking key strategy needs its own column + index
- Dynamic entity system means many entity tables
- Updates to source fields require updating normalized columns

**Warning signs:**
- Record create/update latency increasing over time
- Database size growing faster than data volume
- Vacuum operations taking longer

**Prevention:**
1. **Be selective with blocking keys** - Not every field needs a blocking index
2. **Use composite blocking keys** - One index on (email_domain, name_prefix) instead of two
3. **Consider separate blocking table** - Don't add columns to main entity table
4. **Lazy recomputation** - Compute blocking keys on background, not synchronously

**Phase assignment:** Phase 1 - Schema design decision

---

## UX Pitfalls

### CRITICAL: Irreversible Merge Without Undo

**What goes wrong:** User merges wrong records. Data is lost. No undo. User has to manually recreate the deleted record and remember what data was on it. User loses trust in the system, avoids using dedup feature.

**Why it happens:**
- Merge is implemented as "delete source, update target" with no recovery path
- Soft-delete doesn't preserve enough information to reconstruct
- Backup/restore is too coarse-grained

**Warning signs (from HubSpot/Salesforce community forums):**
- "Can I undo a merge?" questions
- Users requesting backup before merge
- Users refusing to merge because they're afraid

**Consequences:**
- Feature abandonment
- Support burden
- Data loss incidents
- Customer escalations

**Prevention:**
1. **Pre-merge snapshot** - Store complete copy of both records before merge
2. **Merge archive table** - Enable "unmerge" operation (split back to two records)
3. **Time-limited undo** - "Undo merge" available for 30 days
4. **Confirmation friction** - Multi-step confirmation for destructive merges
5. **Preview output** - Show exactly what merged record will look like

**Implementation approach:**
```sql
CREATE TABLE merge_archive (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_record_json TEXT NOT NULL,  -- Complete snapshot
    target_record_json TEXT NOT NULL,
    merged_at TEXT NOT NULL,
    merged_by TEXT NOT NULL,
    expires_at TEXT NOT NULL,  -- 30 days from merge
    is_unmerged INTEGER DEFAULT 0
);
```

**Phase assignment:** Phase 2 - Must ship with merge UI.

---

### HIGH: False Positive Overload

**What goes wrong:** Detection algorithm is too aggressive. Marks "John Smith" at company A as duplicate of "John Smith" at company B. Users see thousands of "duplicates" that aren't actually duplicates. Users stop reviewing, ignore the feature, or waste time on false positives.

**Why it happens:**
- Threshold set too low (80% match = duplicate)
- Common names trigger false positives
- Single-field matching (email only, name only) insufficient
- No negative signals (different company = probably not duplicate)

**Warning signs:**
- "90% of suggested duplicates aren't duplicates"
- Users clicking "Not duplicate" constantly
- Users avoiding the dedup review screen

**Prevention:**
1. **Multi-field scoring** - Require matches on multiple fields, not just one
2. **Negative signals** - Different company, different domain = reduce score
3. **Tiered confidence** - HIGH (95%+) auto-merge eligible, MEDIUM (85-95%) review required, LOW (<85%) don't show
4. **User feedback loop** - Track "Not duplicate" clicks, adjust algorithm
5. **Entity-specific rules** - Contacts need name+email match; accounts need name+domain match

**Scoring example:**
```
Contact duplicate scoring:
- Email exact match: +50 points
- Email domain match: +10 points
- First name fuzzy match (>90%): +15 points
- Last name fuzzy match (>90%): +15 points
- Phone exact match: +20 points
- Company exact match: +15 points
- Company different: -20 points (negative signal)

Threshold: 70 points = potential duplicate
```

**Phase assignment:** Phase 1 - Scoring Engine must be tunable from day one.

---

### HIGH: Bulk Merge Disasters

**What goes wrong:** Admin selects 100 duplicate pairs, clicks "Merge All," accidentally merges records that shouldn't be merged. Or: merge of one pair fails, but 50 others already committed. Inconsistent state.

**Why it happens:**
- Bulk operations skip per-record confirmation
- Transaction size too large (all-or-nothing fails)
- No individual item status in bulk operation
- No ability to exclude specific items from bulk merge

**Warning signs:**
- Users asking "can I merge these 50 but not those 10?"
- Users preferring to merge one-by-one (ignoring bulk feature)
- Partial merge failures leaving confusing state

**Prevention:**
1. **Require explicit selection** - Checkboxes, not "merge all visible"
2. **Individual transaction per merge** - Report success/failure per item
3. **Pre-merge validation** - Check all selected items, report issues before starting any
4. **Dry run option** - "Preview bulk merge results" before committing
5. **Maximum batch size** - Limit to 25 at a time, require re-review for next batch

**Phase assignment:** Phase 4 - Bulk Operations (if included)

---

### MEDIUM: Confusing Duplicate Notifications

**What goes wrong:** User creates a contact. System shows "Possible duplicate found!" User doesn't know what to do: continue anyway? Go review? Where? Confusion leads to user ignoring the warning and creating duplicate anyway.

**Why it happens:**
- Warning without actionable path
- Modal interruption without context
- No clear explanation of what matched

**Warning signs:**
- Users clicking through duplicate warnings
- Users reporting "I saw a warning but didn't know what it meant"
- Duplicate creation rates not decreasing despite warnings

**Prevention:**
1. **Inline comparison** - Show the potential duplicate right in the warning
2. **Clear actions** - "View existing record" / "Merge into existing" / "Create anyway (not a duplicate)"
3. **Explain the match** - "Same email address" / "Similar name and company"
4. **Non-blocking option** - Allow continuation with clear consequences stated

**Phase assignment:** Phase 3 - Import Integration UX

---

## Multi-Tenant Pitfalls

### CRITICAL: Cross-Tenant Data Exposure

**What goes wrong:** Duplicate detection algorithm has bug that compares records across tenants. Admin at Org A sees "potential duplicate" that's actually from Org B. Data breach.

**Why it happens in Quantico architecture:**
- Per-tenant databases SHOULD isolate by default
- BUT: if detection engine ever queries master DB or cross-org
- OR: if caching layer doesn't respect tenant boundaries
- OR: if blocking keys are stored in shared table

**Warning signs:**
- Duplicate suggestions showing unfamiliar record names
- Users seeing records they definitely didn't create
- Record counts in duplicate scan not matching expected org size

**Consequences:**
- GDPR breach notification requirement (72 hours)
- SOC 2 audit failure
- Customer trust destruction
- Legal liability

**Prevention:**
1. **ALWAYS use tenant database connection** - Never query across tenants
2. **No shared tables for dedup data** - Blocking keys, duplicate pairs, etc. all per-tenant
3. **Add tenant ID assertions** - `assert record.org_id == current_org_id` before any operation
4. **Isolation tests** - Specific tests that create duplicate data across orgs, verify no cross-contamination
5. **Code review checklist** - Every DB query reviewed for tenant scoping

**Test case:**
```go
func TestNoCrossTenantDuplicateDetection(t *testing.T) {
    // Create identical contacts in two different orgs
    orgA := createOrg("A")
    orgB := createOrg("B")

    contactA := createContact(orgA, "john@example.com")
    contactB := createContact(orgB, "john@example.com")

    // Scan for duplicates in org A
    duplicates := detectDuplicates(orgA, contactA)

    // Must NOT find contactB from orgB
    for _, dup := range duplicates {
        assert(dup.OrgID == orgA.ID, "Cross-tenant leak detected!")
    }
}
```

**Phase assignment:** Phase 1 - Must be validated before any detection code ships.

---

### HIGH: Tenant Database Lock Contention

**What goes wrong:** One large tenant runs full duplicate scan. Their database is locked/slow. This is fine (expected). But: if there's any shared resource (job queue, logging, metrics), large tenant's load affects smaller tenants' experience.

**Why it happens:**
- Background job runner is shared
- Job status updates write to master DB
- Metrics/logging system overloaded by scan progress events
- Connection pool exhausted by long-running scan

**Warning signs:**
- Small tenants reporting slowness during certain hours
- Correlation between large tenant activity and system-wide latency
- Job queue backing up across all tenants

**Prevention:**
1. **Per-tenant job queue isolation** - Or at least fair scheduling
2. **Rate limit background jobs** - Max 1 scan per tenant at a time
3. **Separate monitoring from operational DB** - Scan progress doesn't hit tenant DB
4. **Circuit breaker on long jobs** - Auto-pause after X minutes, require manual continuation

**Phase assignment:** Phase 5 - Background Scanning scalability

---

### MEDIUM: Inconsistent Feature Availability

**What goes wrong:** Dedup feature deployed to new tenants but not old tenants. Or: migration adds dedup tables to some tenant DBs but fails silently for others. Users on different orgs have different experiences.

**Why it happens:**
- Rolling migrations across many tenant databases
- Silent migration failures
- Feature flags not consistently applied

**Warning signs:**
- "I don't see the dedup button" from some users
- Migration logs showing partial success
- Different orgs at different schema versions

**Prevention:**
1. **Schema version tracking per tenant** - Already exists in Quantico
2. **Migration verification job** - Periodically check all tenants have expected tables
3. **Feature availability check** - UI queries backend "is dedup enabled for this org?"
4. **Admin visibility** - Dashboard showing which orgs have which features

**Phase assignment:** All phases - Part of deployment checklist

---

## Prevention Strategies Summary

| Pitfall | Strategy | Phase |
|---------|----------|-------|
| Related record orphaning | Discover all lookups, update before delete, verify zero orphans | Phase 1 |
| Master record selection errors | Field-by-field selection UI, visual diff, preview | Phase 2 |
| Quadratic detection complexity | Blocking keys, limit comparison window, batch processing | Phase 1 |
| Irreversible merges | Pre-merge snapshot, merge archive, time-limited undo | Phase 2 |
| False positive overload | Multi-field scoring, negative signals, confidence tiers | Phase 1 |
| Cross-tenant data exposure | Always use tenant DB, no shared tables, isolation tests | Phase 1 |
| Bulk merge disasters | Individual transactions, pre-validation, dry run | Phase 4 |
| Real-time blocking imports | Async detection, streaming results, progress indicators | Phase 3 |
| Audit trail discontinuity | Never delete audit entries, add merge event, include source ID | Phase 1 |
| Import detection bypasses | Hook into record creation, background scan catch-up | Phase 3 |

---

## Phase-Specific Warning Checklist

### Phase 1: Detection Engine
- [ ] Blocking strategy implemented (not O(n^2))
- [ ] Tenant isolation verified in all queries
- [ ] Scoring algorithm is tunable
- [ ] Blocking keys indexed efficiently
- [ ] All lookup relationships discoverable for merge

### Phase 2: Merge Engine & UI
- [ ] Pre-merge snapshot saved
- [ ] Undo capability implemented
- [ ] Field-by-field selection UI
- [ ] Related records updated before source deleted
- [ ] Audit log includes merge event
- [ ] Merge lock prevents concurrent merges of same record

### Phase 3: Import Integration
- [ ] Async processing for large imports
- [ ] Clear duplicate notification with actions
- [ ] Progress indication during detection
- [ ] Fallback to background scan if sync times out

### Phase 4: Bulk Operations
- [ ] Individual transaction per merge
- [ ] Pre-validation before bulk start
- [ ] Maximum batch size enforced
- [ ] Per-item success/failure reporting

### Phase 5: Background Scanning
- [ ] Fair scheduling across tenants
- [ ] Long-running job limits
- [ ] Database lock management (chunked transactions)
- [ ] Scan progress doesn't impact operational DB

---

## Sources

### CRM Vendor Documentation and Community (HIGH confidence)
- [Insycle: Data Retention When Merging Duplicates](https://blog.insycle.com/data-retention-merging-duplicates)
- [Insycle: CRM Deduplication Master Record Selection](https://blog.insycle.com/picking-master-record-crm-deduplication)
- [HubSpot Community: Reverse Merged Deals Discussion](https://community.hubspot.com/t5/CRM/Reverse-back-merged-deals/m-p/1138996)
- [Airbyte: Maintain Data Consistency When Merging CRM Records](https://airbyte.com/data-engineering-resources/maintain-data-consistency-when-merging-crm-records)
- [Dynamics 365: Entity Relationship Cascading Behavior](https://learn.microsoft.com/en-us/power-apps/developer/data-platform/configure-entity-relationship-cascading-behavior)
- [CiviCRM: Foreign Keys Aren't Optional](https://civicrm.org/blog/jamie/foreign-keys-arent-optional)

### Algorithm and Performance (HIGH confidence)
- [QuestDB: Solving Duplicate Data with Performant Deduplication](https://questdb.com/blog/solving-duplicate-data-performant-deduplication/)
- [Robin Linacre: Super-fast Deduplication Using Splink and DuckDB](https://www.robinlinacre.com/fast_deduplication/)
- [arXiv: Scalable Blocking for Very Large Databases](https://arxiv.org/abs/2008.08285)
- [LeadAngel: How Fuzzy Matching Reduces Lead Duplication](https://www.leadangel.com/blog/operations/how-fuzzy-matching-reduces-lead-duplication-and-enhances-crm-data-quality/)
- [Medium: Why Fuzzy Matching Isn't Enough](https://medium.com/@williamflaiz/why-fuzzy-matching-isnt-enough-and-what-actually-finds-your-hidden-duplicates-7ddfdc5c26de)

### Multi-Tenant Security (HIGH confidence)
- [AWS: SaaS Tenant Isolation Strategies](https://d1.awsstatic.com/whitepapers/saas-tenant-isolation-strategies.pdf)
- [AWS: Tenant Isolation - SaaS Architecture Fundamentals](https://docs.aws.amazon.com/whitepapers/latest/saas-architecture-fundamentals/tenant-isolation.html)
- [Propelius: Tenant Data Isolation Patterns and Anti-Patterns](https://propelius.ai/blogs/tenant-data-isolation-patterns-and-anti-patterns)
- [ComplyDog: Multi-Tenant SaaS Privacy - Data Isolation Architecture](https://complydog.com/blog/multi-tenant-saas-privacy-data-isolation-compliance-architecture)

### Integration Issues (MEDIUM confidence)
- [HubSpot + Salesforce Duplicate Merge Problems](https://blog.insycle.com/hubspot-salesforce-integration-syncing-merging-duplicates)
- [New Breed: Prevent and Fix Duplicates in HubSpot and Salesforce](https://www.newbreedrevenue.com/blog/prevent-and-fix-duplicates-in-hubspot-and-salesforce)
- [Syncari: Preventing Orphan Records Across Systems](https://syncari.com/orphaned-records/)

---

*Researched for Quantico CRM v3.0 Deduplication System milestone*
