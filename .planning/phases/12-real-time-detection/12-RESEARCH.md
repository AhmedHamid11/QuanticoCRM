# Phase 12: Real-Time Detection - Research

**Researched:** 2026-02-06
**Domain:** Real-time duplicate detection during record creation/edit, async notifications, UI patterns
**Confidence:** HIGH

## Summary

This research covers implementing real-time duplicate detection that triggers during record creation and editing, then surfaces results to users via notifications. The key architectural decision from CONTEXT.md is **optimistic save with async detection**: records save immediately while duplicate checking runs in a background goroutine, with results surfacing as a banner notification on the detail page.

The Phase 11 foundation provides: detection engine (`detector.go`), blocking strategies (`blocker.go`), scoring (`scorer.go`), and matching rules API. Phase 12 builds the integration layer that hooks record saves to async detection, stores pending duplicate alerts, and provides the frontend components (DuplicateWarningModal, DuplicateAlertBanner).

**Key insight:** The optimistic save pattern requires a notification/alert storage table to persist detection results between the async job and the user viewing the detail page. This is distinct from the `duplicate_pairs` table (Phase 15 batch scan results) - Phase 12 needs a `pending_duplicate_alerts` table for real-time detection results.

**Primary recommendation:** Add a `pending_duplicate_alerts` table to store async detection results. Hook into record create/update handlers to spawn goroutines using `c.Context()` for proper context handling. Frontend loads pending alerts when rendering detail pages.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go goroutines | N/A | Async background detection | Native Go concurrency, no external deps |
| github.com/gofiber/fiber/v2 | 2.x | HTTP framework with context pooling | Already in project, handles async context properly |
| SvelteKit 2.x | 2.x | Frontend framework with reactive state | Already in project, supports optimistic UI patterns |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| (existing) dedup package | N/A | Detection engine from Phase 11 | All duplicate checking |
| (existing) sfid package | N/A | ID generation | New alert records |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Goroutines | External job queue (Redis) | Added complexity; goroutines sufficient for real-time single-record checks |
| In-DB notifications | In-memory pub/sub | Persistence needed; user may not view record immediately after save |
| Modal on save | Toast notification | User decision from CONTEXT.md: modal for warnings |

**Installation:**
No new dependencies required - uses existing Go stdlib and project libraries.

## Architecture Patterns

### Recommended Project Structure
```
backend/internal/
├── dedup/
│   ├── detector.go          # [EXISTS] Main detection orchestrator
│   ├── realtime.go           # NEW: Real-time detection coordinator
│   └── ...
├── entity/
│   ├── dedup.go              # [EXISTS] MatchingRule, MatchResult
│   └── pending_alert.go      # NEW: PendingDuplicateAlert entity
├── repo/
│   ├── matching_rule.go      # [EXISTS]
│   └── pending_alert.go      # NEW: Alert CRUD
├── handler/
│   ├── contact.go            # [MODIFY] Add async detection hook
│   └── dedup.go              # [MODIFY] Add alert endpoints
└── migrations/
    └── 052_create_pending_alerts.sql  # NEW

frontend/src/
├── lib/
│   ├── components/
│   │   ├── DuplicateWarningModal.svelte  # NEW: Modal for warnings
│   │   └── DuplicateAlertBanner.svelte   # NEW: Detail page banner
│   └── api/dedup.ts                      # NEW: Dedup API utilities
└── routes/
    └── contacts/
        └── [id]/+page.svelte             # [MODIFY] Add alert banner
```

### Pattern 1: Optimistic Save with Async Detection
**What:** Record saves synchronously (fast UX), detection runs in background goroutine
**When to use:** All record create/update operations where matching rules exist
**Example:**
```go
// Source: Fiber docs + CONTEXT.md decision
func (h *ContactHandler) Create(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)
    userID := c.Locals("userID").(string)

    // Parse and validate input...
    var input ContactCreateInput
    if err := c.BodyParser(&input); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
    }

    // Save record immediately (optimistic)
    contact, err := h.contactRepo.Create(ctx, orgID, input)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    // Spawn async detection with derived context
    // CRITICAL: Get context BEFORE returning, use WithTimeout for cleanup
    asyncCtx := context.WithTimeout(context.Background(), 30*time.Second)
    recordData := contact.ToMap() // Convert to map[string]interface{}

    go func() {
        defer asyncCtx.Done()
        h.runAsyncDuplicateCheck(asyncCtx, orgID, userID, "Contact", contact.ID, recordData)
    }()

    // Return immediately - user sees success
    return c.Status(201).JSON(contact)
}

func (h *ContactHandler) runAsyncDuplicateCheck(ctx context.Context, orgID, userID, entityType, recordID string, recordData map[string]interface{}) {
    matches, err := h.detector.CheckForDuplicates(ctx, h.db, orgID, entityType, recordData, recordID)
    if err != nil {
        log.Printf("Async dedup check failed for %s/%s: %v", entityType, recordID, err)
        return
    }

    if len(matches) == 0 {
        return // Silent success
    }

    // Store alert for user to see on detail page
    alert := entity.PendingDuplicateAlert{
        OrgID:      orgID,
        EntityType: entityType,
        RecordID:   recordID,
        Matches:    matches, // Top N matches
        DetectedAt: time.Now(),
        Status:     "pending",
    }

    if err := h.alertRepo.Create(ctx, alert); err != nil {
        log.Printf("Failed to store duplicate alert: %v", err)
    }
}
```

### Pattern 2: Pending Alert Table Schema
**What:** Dedicated table for real-time detection results that haven't been reviewed
**When to use:** Store async detection results for display on detail page
**Example:**
```sql
-- Source: Notification system design patterns
CREATE TABLE IF NOT EXISTS pending_duplicate_alerts (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    record_id TEXT NOT NULL,           -- The newly created/edited record
    matches_json TEXT NOT NULL,         -- JSON array of DuplicateMatch (top 3)
    total_match_count INTEGER NOT NULL, -- Total matches found (for "X more" indicator)
    highest_confidence TEXT NOT NULL,   -- "high", "medium", "low"
    detected_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- "pending", "dismissed", "merged", "created_anyway"
    resolved_at TEXT,
    resolved_by_id TEXT,
    override_text TEXT,                 -- If user typed "DUPLICATE" for block mode override

    UNIQUE(org_id, entity_type, record_id, status)  -- One pending alert per record
);

CREATE INDEX IF NOT EXISTS idx_pending_alerts_record
    ON pending_duplicate_alerts(org_id, entity_type, record_id, status);
CREATE INDEX IF NOT EXISTS idx_pending_alerts_user
    ON pending_duplicate_alerts(org_id, status, detected_at);
```

### Pattern 3: Alert-Aware Detail Page
**What:** Detail page checks for pending alerts and shows banner
**When to use:** All entity detail pages
**Example:**
```svelte
<!-- Source: Quantico existing component patterns -->
<script lang="ts">
    import { onMount } from 'svelte';
    import { get } from '$lib/utils/api';
    import DuplicateAlertBanner from '$lib/components/DuplicateAlertBanner.svelte';

    interface PendingAlert {
        id: string;
        matches: DuplicateMatch[];
        totalMatchCount: number;
        highestConfidence: string;
        detectedAt: string;
    }

    let pendingAlert = $state<PendingAlert | null>(null);

    async function loadPendingAlert() {
        try {
            const alert = await get<PendingAlert>(
                `/dedup/Contact/${contactId}/pending-alert`
            );
            pendingAlert = alert;
        } catch {
            // No alert = no duplicates found, this is fine
            pendingAlert = null;
        }
    }

    onMount(() => {
        loadPendingAlert();
    });
</script>

{#if pendingAlert}
    <DuplicateAlertBanner
        alert={pendingAlert}
        onDismiss={() => dismissAlert(pendingAlert.id)}
        onViewMatches={() => openDuplicateModal(pendingAlert)}
    />
{/if}
```

### Pattern 4: Duplicate Warning Modal with Actions
**What:** Modal showing matched records with action buttons
**When to use:** User clicks "View Matches" or opens alert
**Example:**
```svelte
<!-- Source: FieldFormModal.svelte pattern + CONTEXT.md decisions -->
<script lang="ts">
    interface Props {
        alert: PendingAlert;
        userCanMerge: boolean;  // Based on dedup permission
        isBlockMode: boolean;   // From matching rule config
        onClose: () => void;
        onCreateAnyway: (overrideText?: string) => void;
        onMerge: (survivorId: string) => void;
        onViewExisting: (recordId: string) => void;
    }

    let { alert, userCanMerge, isBlockMode, onClose, onCreateAnyway, onMerge, onViewExisting }: Props = $props();
    let overrideText = $state('');
    let showAllMatches = $state(false);

    function getConfidenceBadgeClass(tier: string): string {
        switch (tier) {
            case 'high': return 'bg-red-100 text-red-800 border-red-200';
            case 'medium': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
            case 'low': return 'bg-blue-100 text-blue-800 border-blue-200';
            default: return 'bg-gray-100 text-gray-800';
        }
    }

    function formatConfidence(score: number): string {
        return `${Math.round(score * 100)}%`;
    }
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
    <div class="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div class="px-6 py-4 border-b border-gray-200 bg-yellow-50">
            <h2 class="text-lg font-medium text-yellow-800">
                Potential Duplicates Found
            </h2>
            <p class="text-sm text-yellow-700 mt-1">
                {alert.totalMatchCount} potential match{alert.totalMatchCount !== 1 ? 'es' : ''} found
            </p>
        </div>

        <div class="px-6 py-4 space-y-4">
            {#each alert.matches.slice(0, showAllMatches ? undefined : 3) as match (match.recordId)}
                <div class="border rounded-lg p-4">
                    <!-- Confidence badge -->
                    <div class="flex items-center justify-between mb-3">
                        <span class="text-sm font-medium text-gray-700">Match ID: {match.recordId}</span>
                        <span class="px-2 py-1 text-xs font-medium rounded border {getConfidenceBadgeClass(match.matchResult.confidenceTier)}">
                            {match.matchResult.confidenceTier.toUpperCase()} - {formatConfidence(match.matchResult.score)}
                        </span>
                    </div>

                    <!-- Matching fields with highlights -->
                    <div class="grid grid-cols-2 gap-2 text-sm">
                        {#each Object.entries(match.matchResult.fieldScores) as [field, score]}
                            <div class="flex justify-between {score >= 0.85 ? 'bg-yellow-50 p-1 rounded' : ''}">
                                <span class="text-gray-600">{field}:</span>
                                <span class="font-medium">{formatConfidence(score)}</span>
                            </div>
                        {/each}
                    </div>

                    <!-- Quick actions for this match -->
                    <div class="mt-3 flex gap-2">
                        <button
                            onclick={() => onViewExisting(match.recordId)}
                            class="text-sm text-blue-600 hover:underline"
                        >
                            View Record
                        </button>
                    </div>
                </div>
            {/each}

            {#if alert.totalMatchCount > 3 && !showAllMatches}
                <button
                    onclick={() => showAllMatches = true}
                    class="text-sm text-blue-600 hover:underline"
                >
                    Show {alert.totalMatchCount - 3} more matches...
                </button>
            {/if}
        </div>

        <!-- Actions footer -->
        <div class="px-6 py-4 border-t border-gray-200 bg-gray-50">
            {#if isBlockMode}
                <p class="text-sm text-gray-600 mb-3">
                    Block mode is enabled. Type "DUPLICATE" to proceed anyway.
                </p>
                <input
                    type="text"
                    bind:value={overrideText}
                    placeholder='Type "DUPLICATE" to override'
                    class="w-full px-3 py-2 border border-gray-300 rounded-md mb-3"
                />
            {/if}

            <div class="flex justify-end gap-3">
                <button
                    onclick={onClose}
                    class="px-4 py-2 text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
                >
                    Cancel
                </button>

                {#if userCanMerge}
                    <button
                        onclick={() => onMerge(alert.matches[0].recordId)}
                        class="px-4 py-2 text-white bg-green-600 rounded-md hover:bg-green-700"
                    >
                        Merge with Top Match
                    </button>
                {/if}

                <button
                    onclick={() => onCreateAnyway(isBlockMode ? overrideText : undefined)}
                    disabled={isBlockMode && overrideText !== 'DUPLICATE'}
                    class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    Create Anyway
                </button>
            </div>
        </div>
    </div>
</div>
```

### Anti-Patterns to Avoid
- **Blocking save for detection:** Violates CONTEXT.md decision; detection must be async
- **Not handling goroutine panics:** Wrap async detection in recover() to prevent server crashes
- **Using request context in goroutine:** Fiber pools contexts; use derived context from c.Context()
- **Storing all matches in alert:** Per CONTEXT.md, store top 3 + count; full list can be re-fetched
- **No timeout on async detection:** Always use context.WithTimeout to prevent goroutine leaks

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Async context in Fiber | Copy context locals manually | `context.WithTimeout(context.Background())` with values | Fiber docs recommend this for async work |
| Duplicate detection | Query + compare in handler | `detector.CheckForDuplicates()` from Phase 11 | Already optimized with blocking strategies |
| ID generation | UUID or timestamp | `sfid.New("alert")` | Consistent with project patterns |
| Modal UI | Custom overlay + focus trap | Existing modal pattern from FieldFormModal | Consistent UX, accessibility handled |

**Key insight:** Phase 11 detection engine is complete and tested. Phase 12 is purely integration and UI - do not duplicate detection logic.

## Common Pitfalls

### Pitfall 1: Goroutine Context Leaks
**What goes wrong:** Using Fiber's request context directly in goroutines causes race conditions or panics
**Why it happens:** Fiber pools and reuses context objects after handler returns
**How to avoid:** Create new context with `context.Background()` and copy needed values before spawning goroutine
**Warning signs:** Intermittent panics in production, nil pointer errors in async code

### Pitfall 2: Alert Duplication on Rapid Edits
**What goes wrong:** Multiple pending alerts for same record if user edits quickly
**Why it happens:** Previous alert not dismissed, new async check creates another
**How to avoid:** Use UNIQUE constraint on (org_id, entity_type, record_id, status='pending'); upsert on alert create
**Warning signs:** Multiple alert banners, confusing UX

### Pitfall 3: Missing Permission Checks in Modal
**What goes wrong:** Users without merge permission see merge options
**Why it happens:** Permission check only on backend, frontend shows all buttons
**How to avoid:** Pass `userCanMerge` prop to modal based on permission check; backend also enforces
**Warning signs:** 403 errors when users click Merge

### Pitfall 4: No Timeout on Async Detection
**What goes wrong:** Goroutines hang forever if database is slow
**Why it happens:** No timeout set on async context
**How to avoid:** Always use `context.WithTimeout(context.Background(), 30*time.Second)`
**Warning signs:** Goroutine count growing over time, memory leaks

### Pitfall 5: UI Flicker on Alert Load
**What goes wrong:** Banner appears/disappears as alert loads
**Why it happens:** Initial state is null, then loads, causing re-render
**How to avoid:** Track loading state, only show banner after load completes with result
**Warning signs:** Banner flashes briefly on all detail pages

## Code Examples

Verified patterns from official sources and project codebase:

### PendingDuplicateAlert Entity
```go
// Source: entity/dedup.go pattern + notification system design
package entity

import "time"

// PendingDuplicateAlert stores async detection results for user review
type PendingDuplicateAlert struct {
    ID               string         `json:"id" db:"id"`
    OrgID            string         `json:"orgId" db:"org_id"`
    EntityType       string         `json:"entityType" db:"entity_type"`
    RecordID         string         `json:"recordId" db:"record_id"`
    Matches          []DuplicateMatch `json:"matches" db:"-"`     // Top 3 matches
    MatchesJSON      string         `json:"-" db:"matches_json"`
    TotalMatchCount  int            `json:"totalMatchCount" db:"total_match_count"`
    HighestConfidence string        `json:"highestConfidence" db:"highest_confidence"`
    Status           string         `json:"status" db:"status"` // pending, dismissed, merged, created_anyway
    DetectedAt       time.Time      `json:"detectedAt" db:"detected_at"`
    ResolvedAt       *time.Time     `json:"resolvedAt,omitempty" db:"resolved_at"`
    ResolvedByID     *string        `json:"resolvedById,omitempty" db:"resolved_by_id"`
    OverrideText     *string        `json:"overrideText,omitempty" db:"override_text"`
}

// DuplicateMatch reused from Phase 11 detector
type DuplicateMatch struct {
    RecordID    string       `json:"recordId"`
    MatchResult *MatchResult `json:"matchResult"`
}

// AlertStatus constants
const (
    AlertStatusPending      = "pending"
    AlertStatusDismissed    = "dismissed"
    AlertStatusMerged       = "merged"
    AlertStatusCreatedAnyway = "created_anyway"
)
```

### Async Detection Hook in Handler
```go
// Source: Fiber docs, Go concurrency patterns
func (h *ContactHandler) afterCreate(c *fiber.Ctx, contact *entity.Contact) {
    orgID := c.Locals("orgID").(string)
    userID, _ := c.Locals("userID").(string)

    // Check if any matching rules exist (quick bailout)
    rules, _ := h.ruleRepo.ListEnabledRules(c.Context(), orgID, "Contact")
    if len(rules) == 0 {
        return
    }

    // Prepare data for async check
    recordData := map[string]interface{}{
        "id":           contact.ID,
        "firstName":    contact.FirstName,
        "lastName":     contact.LastName,
        "emailAddress": contact.EmailAddress,
        "phoneNumber":  contact.PhoneNumber,
        // ... other fields
    }

    // Spawn async detection
    asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

    go func() {
        defer cancel()
        defer func() {
            if r := recover(); r != nil {
                log.Printf("Panic in async dedup check: %v", r)
            }
        }()

        h.runAsyncDuplicateCheck(asyncCtx, orgID, userID, "Contact", contact.ID, recordData)
    }()
}
```

### Alert Banner Component
```svelte
<!-- Source: Toast.svelte pattern + CONTEXT.md decisions -->
<script lang="ts">
    interface Props {
        alert: {
            id: string;
            totalMatchCount: number;
            highestConfidence: string;
        };
        onDismiss: () => void;
        onViewMatches: () => void;
    }

    let { alert, onDismiss, onViewMatches }: Props = $props();

    function getBannerClass(confidence: string): string {
        switch (confidence) {
            case 'high': return 'bg-red-50 border-red-200 text-red-800';
            case 'medium': return 'bg-yellow-50 border-yellow-200 text-yellow-800';
            case 'low': return 'bg-blue-50 border-blue-200 text-blue-800';
            default: return 'bg-gray-50 border-gray-200 text-gray-800';
        }
    }
</script>

<div class="rounded-lg border p-4 mb-4 {getBannerClass(alert.highestConfidence)}">
    <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <span class="font-medium">
                {alert.totalMatchCount} potential duplicate{alert.totalMatchCount !== 1 ? 's' : ''} found
            </span>
        </div>
        <div class="flex items-center gap-2">
            <button
                onclick={onViewMatches}
                class="text-sm font-medium underline hover:no-underline"
            >
                View Matches
            </button>
            <button
                onclick={onDismiss}
                class="p-1 rounded hover:bg-black/10"
                aria-label="Dismiss"
            >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
            </button>
        </div>
    </div>
</div>
```

### API Endpoints for Alerts
```go
// Source: handler/dedup.go pattern
// Additional routes to add to DedupHandler.RegisterRoutes

// Get pending alert for a specific record
app.Get("/dedup/:entity/:id/pending-alert", h.GetPendingAlert)

// Resolve alert (dismiss, created_anyway, etc.)
app.Post("/dedup/:entity/:id/resolve-alert", h.ResolveAlert)

// Handler implementations
func (h *DedupHandler) GetPendingAlert(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)
    entityType := c.Params("entity")
    recordID := c.Params("id")

    alert, err := h.alertRepo.GetPendingByRecord(c.Context(), orgID, entityType, recordID)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }
    if alert == nil {
        return c.Status(404).JSON(fiber.Map{"error": "No pending alert"})
    }

    return c.JSON(alert)
}

func (h *DedupHandler) ResolveAlert(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)
    userID := c.Locals("userID").(string)
    entityType := c.Params("entity")
    recordID := c.Params("id")

    var input struct {
        Status       string `json:"status"`       // "dismissed", "created_anyway"
        OverrideText string `json:"overrideText"` // For block mode
    }
    if err := c.BodyParser(&input); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
    }

    err := h.alertRepo.Resolve(c.Context(), orgID, entityType, recordID, input.Status, userID, input.OverrideText)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.SendStatus(204)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sync check before save (blocking) | Optimistic save + async notification | 2024+ | Faster UX, no save delay |
| Single warn/block toggle | Per-entity, per-rule configuration | 2025+ | Flexible business rules |
| In-memory detection results | Persisted pending alerts | Standard | Results survive page reload |
| All-or-nothing merge | Quick merge with survivor selection | 2023+ | Simpler first-pass resolution |

**Deprecated/outdated:**
- Blocking duplicate checks on every save: Creates poor UX for high-volume data entry
- Pop-up alerts without persistence: User loses context if they navigate away

## Open Questions

Things that couldn't be fully resolved:

1. **Key Fields for Edit Detection**
   - What we know: CONTEXT.md says "edits when key fields change" trigger detection
   - What's unclear: Which fields are "key fields"? Same as fields in matching rule configs?
   - Recommendation: Use fields from active matching rules' fieldConfigs for the entity

2. **Alert Cleanup Policy**
   - What we know: Pending alerts should be cleaned up eventually
   - What's unclear: After how long? After record is edited again?
   - Recommendation: Clean up resolved alerts after 30 days; re-run detection on edit replaces existing pending alert

3. **Merge Integration with Phase 13**
   - What we know: Phase 12 provides "Merge" button in modal, Phase 13 implements actual merge
   - What's unclear: Does clicking "Merge" in Phase 12 just navigate to merge UI, or initiate merge?
   - Recommendation: Phase 12 modal navigates to merge UI (Phase 13) with pre-selected records; actual merge logic is Phase 13

## Sources

### Primary (HIGH confidence)
- [Fiber Go Context docs](https://docs.gofiber.io/next/guide/go-context/) - Async context handling patterns
- [gofiber/fiber#2988](https://github.com/gofiber/fiber/issues/2988) - Goroutine handling in Fiber
- Phase 11 RESEARCH.md - Detection engine architecture
- Project codebase - handler/dedup.go, entity/dedup.go patterns

### Secondary (MEDIUM confidence)
- [Go Background Job Processing](https://oneuptime.com/blog/post/2026-01-30-go-background-job-processing/view) - Goroutine patterns
- [Optimistikit](https://github.com/paoloricciuti/optimistikit) - SvelteKit optimistic UI library
- [Notification Database Design](https://tannguyenit95.medium.com/designing-a-notification-system-1da83ca971bc) - Alert table patterns

### Tertiary (LOW confidence)
- [CRM Deduplication UX 2026](https://www.inogic.com/blog/2026/01/how-to-identify-duplicates-in-dynamics-365-crm-step-by-step-guide-2026/) - General UX patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Uses existing project patterns and Go stdlib
- Architecture: HIGH - Based on existing Phase 11 foundation and verified Fiber patterns
- Pitfalls: HIGH - Documented in Fiber issues and Go concurrency guides

**Research date:** 2026-02-06
**Valid until:** 2026-03-06 (30 days - stable patterns, architecture is application-specific)
