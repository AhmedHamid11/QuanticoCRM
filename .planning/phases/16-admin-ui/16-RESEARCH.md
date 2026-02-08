# Phase 16: Admin UI - Research

**Researched:** 2026-02-08
**Domain:** SvelteKit 2.x admin interface with Svelte 5 runes, card-based layouts, inline editing patterns
**Confidence:** HIGH

## Summary

Phase 16 builds admin UI on top of completed backend APIs from Phases 11-15. All endpoints exist: matching rules CRUD (`/dedup/rules`), merge preview/execute/undo/history (`/merge/*`), scan schedules/jobs (`/scan-jobs/*`), and notifications. The frontend stack is SvelteKit 2.x with Svelte 5 runes, Tailwind CSS, no additional UI libraries.

Svelte 5 introduces runes (`$state`, `$derived`, `$effect`, `$props`) replacing the old `export let` and reactive statement patterns. The project already uses these patterns extensively (see `DetailPageAlertWrapper.svelte`, admin pages). Standard patterns: card-based layouts for review queue, inline editing with expand/collapse for rule management, side-by-side field comparison for merge wizard, SSE via EventSource for real-time progress.

Key architectural constraint: All backend APIs exist and are multi-tenant aware (orgID scoped). Frontend consumes these APIs without modification. The admin routes already exist at `/admin/*` with established styling patterns (white cards with colored left borders, Tailwind utility classes, no component library).

**Primary recommendation:** Use established codebase patterns (Svelte 5 runes, Tailwind utilities, card layouts) and implement five route groups under `/admin/data-quality/*` consuming existing backend APIs.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Review Queue Layout:**
- Card-based groups — each duplicate group displayed as a card showing matched records with confidence score
- Default sort by confidence score (highest first), with entity type filter
- Inline actions on each card: Dismiss (not duplicates) and Quick Merge (auto-pick best fields). Full merge opens the merge wizard
- Checkbox selection on each card with floating bulk action bar (Merge All, Dismiss All) when items selected

**Merge Wizard Flow:**
- Single scrollable page — no multi-step wizard. All sections visible: survivor selection, field comparison, related records, confirm
- Side-by-side columns for field comparison — records in columns, fields in rows. Click/radio to select which value to keep. Differences highlighted
- Related records (tasks, notes, activities) shown as full list always visible, grouped by type with source record indicated
- After merge completes: return to review queue with merged group removed. Success toast with Undo link (30-day window)

**Rule Management UX:**
- List with inline editing — table of rules per entity type, click to expand inline for editing
- Field configuration via dropdown: select field, choose matching type (exact, fuzzy, phonetic), set weight
- "Test Rule" button runs the rule against existing data and shows sample matches with scores for threshold tuning
- Confidence thresholds configured via numeric inputs with color-coded tier labels (High/Medium/Low)

**Scan Job Dashboard:**
- Simple status table: columns for entity type, schedule, last run, status, next run. Sorted by next run time
- Running scans show inline progress bar directly in the table row with percentage and records processed
- Schedule configuration via preset dropdown options: Daily, Weekly (pick day), Monthly (pick date)
- Failed scans trigger in-app notification. Admin goes to dashboard to see details and manually trigger retry from last checkpoint

### Claude's Discretion

- Exact card component styling and spacing
- Empty state designs for review queue (no duplicates found)
- Loading states and skeleton screens
- Table pagination approach for large result sets
- Responsive behavior for smaller screens
- Toast/notification component implementation details
- Error handling patterns across all UI components

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

## Standard Stack

The project uses a minimal, modern stack with no UI component libraries.

### Core Frontend
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| SvelteKit | 2.x | App framework | Official Svelte meta-framework, provides routing, SSR, API routes |
| Svelte | 5.x | UI library | Rune-based reactivity system (latest stable) |
| Tailwind CSS | 3.4+ | Styling | Utility-first CSS, project standard (see tailwind.config.js) |
| TypeScript | 5.x | Type safety | Project uses .ts files throughout |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| EventSource | Native browser API | SSE client | Real-time progress streaming (scan jobs dashboard) |
| marked | 11.2+ | Markdown parsing | Already in package.json, use if rich text needed |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Tailwind utilities | shadcn-svelte or Flowbite | Project avoids UI libraries, custom components match existing patterns |
| Native EventSource | WebSockets | SSE simpler for one-way server→client, backend already implements SSE |
| Inline editing | Modal forms | User decisions specify inline editing for rule management |

**Installation:**
```bash
# No additional packages needed — all dependencies already installed
cd frontend && npm install
```

## Architecture Patterns

### Route Structure
```
src/routes/admin/data-quality/
├── duplicate-rules/
│   └── +page.svelte              # Rule management with inline editing
├── review-queue/
│   └── +page.svelte              # Card-based duplicate groups
├── merge/
│   ├── [groupId]/
│   │   └── +page.svelte          # Single-page merge wizard
│   └── history/
│       └── +page.svelte          # Merge history with undo
└── scan-jobs/
    └── +page.svelte              # Schedule & job management dashboard
```

### Pattern 1: Svelte 5 Runes for State Management
**What:** Replace `export let` with `$props()`, use `$state()` for reactive variables, `$derived()` for computed values
**When to use:** All new components (Svelte 5 standard)
**Example:**
```typescript
// Existing codebase pattern from DetailPageAlertWrapper.svelte
let alert = $state<PendingAlert | null>(null);
let showModal = $state(false);
let loading = $state(true);

let hasAlert = $derived(alert !== null && alert.totalMatchCount > 0);
```

### Pattern 2: Card-Based Layout with Inline Actions
**What:** White cards with shadow, hover effects, inline action buttons, checkbox selection for bulk operations
**When to use:** Review queue (duplicate groups), scan job list
**Example:**
```svelte
<!-- Existing admin page pattern -->
<div class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-blue-500">
  <div class="flex items-start justify-between">
    <div class="flex-1">
      <h3 class="text-lg font-medium text-gray-900">{title}</h3>
      <p class="mt-1 text-sm text-gray-500">{description}</p>
    </div>
    <div class="flex gap-2">
      <button class="px-3 py-1 text-sm bg-blue-500 text-white rounded">Action</button>
    </div>
  </div>
</div>
```

### Pattern 3: Inline Editing with Expand/Collapse
**What:** Table rows that expand inline to show edit form, collapse to hide. Changes saved on blur/enter, not submit button
**When to use:** Rule management table
**Example:**
```svelte
<script>
let expandedRuleId = $state<string | null>(null);
let editingRule = $state<MatchingRule | null>(null);

function toggleExpand(ruleId: string) {
  if (expandedRuleId === ruleId) {
    expandedRuleId = null;
  } else {
    expandedRuleId = ruleId;
    editingRule = rules.find(r => r.id === ruleId);
  }
}
</script>

{#each rules as rule}
<tr class="hover:bg-gray-50 cursor-pointer" onclick={() => toggleExpand(rule.id)}>
  <td>{rule.name}</td>
  <td>{rule.entityType}</td>
</tr>
{#if expandedRuleId === rule.id}
<tr>
  <td colspan="5" class="p-4 bg-gray-50">
    <!-- Inline edit form -->
  </td>
</tr>
{/if}
{/each}
```

### Pattern 4: SSE Progress Streaming
**What:** EventSource connection to `/scan-jobs/progress/stream`, listen for `progress` events, update UI reactively
**When to use:** Scan job dashboard (real-time progress bars)
**Example:**
```typescript
// Based on backend scan_job.go:396-438
onMount(() => {
  const eventSource = new EventSource('/api/v1/admin/scan-jobs/progress/stream');

  eventSource.addEventListener('progress', (event) => {
    const data = JSON.parse(event.data);
    // Update running job progress: { jobId, percentage, status }
    jobs = jobs.map(j => j.id === data.jobId ? { ...j, ...data } : j);
  });

  return () => eventSource.close();
});
```

### Pattern 5: Floating Bulk Action Bar
**What:** Fixed-position bar that appears at bottom when items selected, shows count and bulk actions
**When to use:** Review queue (bulk merge/dismiss), scan job list (bulk delete)
**Example:**
```svelte
<script>
let selectedIds = $state<Set<string>>(new Set());
let showBulkBar = $derived(selectedIds.size > 0);
</script>

{#if showBulkBar}
<div class="fixed bottom-0 left-0 right-0 bg-blue-600 text-white p-4 shadow-lg flex items-center justify-between z-50">
  <span>{selectedIds.size} selected</span>
  <div class="flex gap-4">
    <button onclick={bulkMerge}>Merge All</button>
    <button onclick={bulkDismiss}>Dismiss All</button>
  </div>
</div>
{/if}
```

### Anti-Patterns to Avoid

- **Multi-step wizard with navigation:** User decisions specify single scrollable page, all sections visible at once
- **Using Svelte 4 syntax:** Project uses Svelte 5, avoid `export let`, use `$props()` runes instead
- **Component libraries:** Project uses Tailwind utilities only, no shadcn/Flowbite
- **Modal forms for editing:** User decisions specify inline editing for rule management
- **Polling for progress:** Backend provides SSE, use EventSource instead of setInterval

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Review queue API | Custom pending alert aggregation | Backend doesn't have ListAllPending endpoint yet | Need to add to pending_alert.go repo: ListAllPending(orgID, entityType filter, pagination) |
| SSE reconnection logic | Custom retry/backoff | EventSource auto-reconnects | Native browser behavior, backend sends keepalive pings every 30s |
| Field comparison diff highlighting | Custom string diff algorithm | CSS class on mismatch | Simple equality check sufficient for field-level diffs |
| Toast notifications | Custom component | Existing `$lib/stores/toast.svelte` | Project already has toast system, use `addToast(message, type)` |
| Confidence tier colors | Hardcoded styles | Existing `$lib/api/dedup.ts:95-106` | Project defines `getBannerClass(tier)` returning Tailwind classes |

**Key insight:** Backend APIs exist for 90% of functionality. Primary frontend work is UI orchestration and state management, not business logic.

## Common Pitfalls

### Pitfall 1: Missing Review Queue Endpoint
**What goes wrong:** Frontend cannot fetch all pending alerts for review queue page
**Why it happens:** Backend has `GetPendingByRecord` (single record) but no `ListAllPending` (org-wide)
**How to avoid:** Add to `pending_alert.go` repo before starting frontend work:
```go
func (r *PendingAlertRepo) ListAllPending(ctx context.Context, orgID string, entityType string, limit, offset int) ([]PendingDuplicateAlert, int, error)
```
**Warning signs:** 404 errors when loading review queue page

### Pitfall 2: Svelte 5 Runes Confusion
**What goes wrong:** Using Svelte 4 syntax (`export let props`, reactive statements `$:`) in new components
**Why it happens:** Many online examples still show Svelte 4 patterns
**How to avoid:** Always use Svelte 5 runes in new code:
- `$props()` instead of `export let`
- `$state()` instead of `let` for reactive variables
- `$derived()` instead of `$:` for computed values
- `$effect()` instead of `$: { ... }` for side effects
**Warning signs:** TypeScript errors about `export let` in .svelte files, deprecated warnings

### Pitfall 3: Fiber Context Pooling in SSE
**What goes wrong:** Fiber reuses context objects, causing state bleed between SSE connections
**Why it happens:** Fiber's performance optimization pools fiber.Ctx instances
**How to avoid:** Backend already handles this correctly (scan_job.go:431 uses `c.Context().Done()`), but be aware when debugging connection issues
**Warning signs:** SSE events delivered to wrong org, duplicate events, ghost connections

### Pitfall 4: Inline Editing Without Save Button
**What goes wrong:** Users expect explicit Save button, confused by blur-to-save behavior
**Why it happens:** Inline editing UX pattern saves on blur/enter, not explicit button
**How to avoid:**
- Show visual feedback on save (green checkmark, toast)
- Add small "Saving..." indicator during API call
- Implement undo/cancel button visible during edit mode
**Warning signs:** User complaints about "lost changes" (actually saved on blur)

### Pitfall 5: EventSource Memory Leaks
**What goes wrong:** EventSource connections not closed when component unmounts, browser hits connection limit (6 per domain)
**Why it happens:** Forgetting cleanup in `onMount` return function
**How to avoid:**
```typescript
onMount(() => {
  const eventSource = new EventSource('/api/v1/admin/scan-jobs/progress/stream');
  // ... setup listeners
  return () => eventSource.close(); // CRITICAL
});
```
**Warning signs:** SSE stops working after navigating away/back multiple times

## Code Examples

Verified patterns from existing codebase and official docs:

### Loading Pattern with Error Handling
```typescript
// Source: frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte:48-66
async function loadData() {
  try {
    loading = true;
    const [data1, data2] = await Promise.all([
      get<Type1>('/api/endpoint1'),
      get<Type2>('/api/endpoint2')
    ]);
    stateVar1 = data1;
    stateVar2 = data2;
  } catch (e) {
    error = e instanceof Error ? e.message : 'Failed to load data';
  } finally {
    loading = false;
  }
}

onMount(() => {
  loadData();
});
```

### Svelte 5 $derived with Filtering
```typescript
// Source: frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte:19-27
let searchQuery = $state('');
let filteredItems = $derived.by(() => {
  if (!searchQuery.trim()) return items;
  const query = searchQuery.toLowerCase();
  return items.filter(item =>
    item.label.toLowerCase().includes(query) ||
    item.name.toLowerCase().includes(query)
  );
});
```

### Toast Success/Error Notifications
```typescript
// Source: existing codebase pattern
import { addToast } from '$lib/stores/toast.svelte';

async function saveChanges() {
  try {
    await post('/api/endpoint', data);
    addToast('Changes saved successfully', 'success');
  } catch (e) {
    addToast(e instanceof Error ? e.message : 'Failed to save', 'error');
  }
}
```

### Optimistic UI with Rollback
```typescript
// Source: CLAUDE.md Svelte Optimistic Update pattern
async function deleteItem(id: string) {
  const backup = [...items];
  items = items.filter(i => i.id !== id);
  try {
    await del(`/api/items/${id}`);
    addToast('Item deleted', 'success');
  } catch (err) {
    items = backup;
    addToast('Failed to delete', 'error');
  }
}
```

### SSE Connection with Keepalive
```typescript
// Source: backend/internal/handler/scan_job.go:396-438 SSE implementation pattern
onMount(() => {
  const eventSource = new EventSource('/api/v1/admin/scan-jobs/progress/stream');

  eventSource.addEventListener('progress', (event) => {
    const data = JSON.parse(event.data);
    updateJobProgress(data.jobId, data.percentage, data.status);
  });

  eventSource.onerror = (error) => {
    console.error('SSE error:', error);
    // EventSource auto-reconnects, no manual retry needed
  };

  return () => {
    eventSource.close();
  };
});
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Svelte 4 `export let` props | Svelte 5 `$props()` rune | Svelte 5 release (Oct 2024) | More explicit, better TypeScript support |
| Svelte 4 reactive `$:` statements | Svelte 5 `$derived()` and `$effect()` | Svelte 5 release | Clearer intent, fine-grained reactivity |
| Component UI libraries (Flowbite, shadcn) | Tailwind utilities only | Project decision | Smaller bundle, full control over styling |
| Multi-step wizards with nav | Single scrollable pages | Modern UX trend (2025-2026) | Reduces friction, maintains context |
| Modal dialogs for editing | Inline editing with expand | User research (2025-2026) | Less context switching, faster workflow |
| Polling for progress (setInterval) | SSE with EventSource | HTTP/2 adoption | Real-time, lower server load, auto-reconnect |

**Deprecated/outdated:**
- **Svelte 4 syntax:** Still works but generates deprecation warnings, migrate to runes
- **`class:` directive with old syntax:** Svelte 5 supports but prefer explicit conditionals for clarity
- **Stores for local component state:** Use `$state()` runes instead, stores for cross-component state only

## Open Questions

Things that couldn't be fully resolved:

1. **Review Queue Grouping Strategy**
   - What we know: Backend has `GetPendingByRecord` per-record endpoint
   - What's unclear: How to group multiple pending alerts into "duplicate groups" for card display
   - Recommendation: Add backend endpoint `ListDuplicateGroups(orgID, entityType)` that returns groups with all member records, OR fetch all pending alerts client-side and group by shared record IDs in matches

2. **Quick Merge Field Selection Strategy**
   - What we know: User decisions say "Quick Merge (auto-pick best fields)"
   - What's unclear: What algorithm determines "best" field (most complete? most recent?)
   - Recommendation: Reuse merge preview's `suggestedSurvivorID` and completeness scoring from `MergeDiscoveryService.SuggestSurvivor`

3. **Inline Edit Save Timing**
   - What we know: Inline editing should save without explicit button
   - What's unclear: Save on blur, enter key, or both? Debounce delay?
   - Recommendation: Save on explicit Enter key only for rule management (high-stakes changes), show "Press Enter to save" hint

## Sources

### Primary (HIGH confidence)
- Existing codebase patterns:
  - `frontend/src/lib/components/DetailPageAlertWrapper.svelte` - Svelte 5 runes usage
  - `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte` - Admin page patterns
  - `frontend/package.json` - Dependency versions (SvelteKit 2.0, Svelte 5.0, Tailwind 3.4)
  - `backend/internal/handler/dedup.go` - Dedup API endpoints
  - `backend/internal/handler/merge.go` - Merge API endpoints
  - `backend/internal/handler/scan_job.go` - Scan job and SSE API endpoints
  - `backend/internal/repo/pending_alert.go` - Pending alert repository (missing ListAllPending)

- Official documentation:
  - [Introducing runes - Svelte Blog](https://svelte.dev/blog/runes)
  - [Svelte 5 migration guide](https://svelte.dev/docs/svelte/v5-migration-guide)

### Secondary (MEDIUM confidence)
- [SvelteKit 2025: Modern Development Trends and Best Practices](https://zxce3.net/posts/sveltekit-2025-modern-development-trends-and-best-practices/)
- [Exploring the magic of runes in Svelte 5 - LogRocket](https://blog.logrocket.com/exploring-runes-svelte-5/)
- [Building Real-time SvelteKit Apps with Server-Sent Events](https://sveltetalk.com/posts/building-real-time-sveltekit-apps-with-server-sent-events)
- [sveltekit-sse library](https://github.com/razshare/sveltekit-sse) - Full-featured SSE implementation (project uses native EventSource instead)
- [Best Practices for Inline Editing in Table Design](https://uxdworld.com/inline-editing-in-tables-design/)
- [Cards: UI-Component Definition - Nielsen Norman Group](https://www.nngroup.com/articles/cards-component/)
- [Wizard UI Pattern: When to Use It and How to Get It Right](https://www.eleken.co/blog-posts/wizard-ui-pattern-explained)

### Tertiary (LOW confidence)
- [UI Design Trends 2026: 15 Patterns Shaping Modern Websites](https://landdding.com/blog/ui-design-trends-2026) - General trends, not Svelte-specific
- [17 Card UI Design Examples and Best Practices](https://www.eleken.co/blog-posts/card-ui-examples-and-best-practices-for-product-owners) - Generic card patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Versions verified in package.json, patterns verified in codebase
- Architecture: HIGH - SvelteKit routing, Svelte 5 runes, SSE verified in existing code
- Pitfalls: HIGH - Based on actual codebase gaps (missing ListAllPending) and Svelte 5 migration warnings

**Research date:** 2026-02-08
**Valid until:** 30 days (stable stack, mature framework)