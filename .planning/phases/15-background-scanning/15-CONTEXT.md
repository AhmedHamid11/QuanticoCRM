# Phase 15: Background Scanning - Context

**Gathered:** 2026-02-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Scheduled duplicate scans with job management, chunked processing, checkpoint recovery, and in-app notifications. Admin configures per-entity scan schedules, monitors progress in real time, and reviews results through the central review queue. Scan job management UI is Phase 16.

</domain>

<decisions>
## Implementation Decisions

### Scheduling & triggers
- Preset interval options: daily, weekly, monthly (no cron expressions)
- Admin picks time of day alongside frequency (e.g., weekly on Mondays at 3 AM)
- Manual "Run Now" button to trigger scans on demand
- Scans configured per entity type (each entity gets its own schedule)
- If a scan is already running for an entity when next scheduled run triggers, skip the new run and log that it was skipped

### Progress & monitoring
- Live progress bar that updates in real time as chunks complete, showing records processed count
- Dedicated scan jobs page (Settings > Data Quality > Scan Jobs) showing active, scheduled, and historical runs
- Small header indicator in the app when a scan is actively running
- Historical runs show summary row (date, entity type, duration, records scanned, duplicates found, status)
- Each historical run supports drill-down to see the full duplicate list from that scan

### Results & notifications
- In-app notification only (no email) when scan completes
- Simple notification message: "Contact scan complete" with link to results (no duplicate count in notification)
- Always notify, even when zero duplicates found — confirms the scan ran
- Scan results feed into the central duplicate review queue (same queue Phase 16 will build UI for)
- Failure notification: same in-app mechanism, different message: "Contact scan failed at 45% — click to retry"

### Failure & recovery
- Auto-retry the failed chunk once; if it fails again, save checkpoint and mark job as failed
- Retry resumes from last checkpoint (does not restart from scratch)
- Partial results from failed scans are visible immediately in the review queue — admin can act on them

### Claude's Discretion
- Chunk size for cursor-based processing
- Checkpoint storage format and granularity
- Progress bar update mechanism (polling interval vs SSE)
- Header indicator design
- Notification persistence and dismissal behavior

</decisions>

<specifics>
## Specific Ideas

No specific references — open to standard approaches for job management and progress tracking.

</specifics>

<deferred>
## Deferred Ideas

- **Salesforce external ID tracking on merge results** — User wants to use Quantico as a deduplication tool for 3rd parties (e.g., Salesforce). Records would have "Disposed External ID" and "Survived External ID" fields so merged results can be exported back to Salesforce for semi-manual merge there. This is a new integration capability — its own phase.

</deferred>

---

*Phase: 15-background-scanning*
*Context gathered: 2026-02-08*
