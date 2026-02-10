---
phase: 17-core-integration
plan: 01
subsystem: salesforce-sync-foundation
tags: [database, entity-types, repository, encryption, oauth]
dependency_graph:
  requires: []
  provides: [salesforce-schema, sync-entities, salesforce-repo, token-encryption]
  affects: [17-02, 17-03, 17-04, 17-05]
tech_stack:
  added: [aes-256-gcm]
  patterns: [multi-tenant-repo, encrypted-credentials]
key_files:
  created:
    - backend/migrations/049_create_salesforce_sync_tables.sql
    - backend/internal/entity/salesforce_sync.go
    - backend/internal/repo/salesforce_sync.go
    - backend/internal/util/encryption.go
  modified:
    - backend/internal/sfid/sfid.go
decisions:
  - context: OAuth token storage
    choice: AES-256-GCM encryption with environment variable key
    rationale: Industry standard for at-rest encryption, allows key rotation
  - context: Database routing
    choice: Master DB for connections/mappings, tenant DB for sync jobs
    rationale: OAuth config is org-wide, job history is tenant-specific
  - context: SFID prefixes
    choice: 0Sf (connection), 0Sy (sync job), 0Sm (field mapping)
    rationale: Follows existing 0S* pattern, no conflicts with existing prefixes
metrics:
  duration_minutes: 2.5
  tasks_completed: 2
  files_created: 4
  files_modified: 1
  loc_added: 625
  commits: 2
completed_date: 2026-02-10
---

# Phase 17 Plan 01: Salesforce Sync Foundation Summary

**One-liner:** Database schema, entity types, repository layer, and AES-256-GCM encryption utility for Salesforce OAuth integration.

## What Was Built

Created the foundational data layer for Salesforce merge integration:

1. **Database Schema (Migration 049):**
   - `salesforce_connections` table (master DB) - stores per-org OAuth credentials with encrypted tokens
   - `sync_jobs` table (tenant DB) - tracks merge batch sync job status and progress
   - `salesforce_field_mappings` table (master DB) - maps Quantico field names to Salesforce object/field names
   - Proper indexes for query performance (org_id, status, batch_id)

2. **Entity Types:**
   - `SalesforceConnection` - OAuth credentials and config with encrypted token fields
   - `SyncJob` - batch sync job execution tracking with instruction counts
   - `SalesforceFieldMapping` - field name translation config
   - `MergeInstruction` & `MergeInstructionBatch` - batch payload structures
   - Status constants: pending, running, completed, failed
   - Trigger type constants: manual, scheduled, realtime

3. **Repository Layer (SalesforceRepo):**
   - **Connection operations (master DB):** GetConnection, UpsertConnection, UpdateTokens, DeleteConnection, SetEnabled
   - **Sync job operations (tenant DB):** CreateSyncJob, GetSyncJob, ListSyncJobs, UpdateSyncJobStatus, UpdateSyncJobProgress, UpdateSyncJobCompletion
   - **Field mapping operations (master DB):** ListFieldMappings, UpsertFieldMapping, DeleteFieldMapping
   - Multi-tenant WithDB pattern for proper database routing

4. **Encryption Utility:**
   - AES-256-GCM token encryption/decryption functions
   - Environment variable key storage (SALESFORCE_TOKEN_ENCRYPTION_KEY)
   - Secure nonce generation and proper error handling

5. **SFID Prefixes:**
   - `0Sf` - Salesforce connections
   - `0Sy` - Sync jobs
   - `0Sm` - Salesforce field mappings

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Create database migration and entity types | 05bb58a | migrations/049, entity/salesforce_sync.go, sfid/sfid.go |
| 2 | Create repository layer and encryption utility | 9814395 | repo/salesforce_sync.go, util/encryption.go |

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

### Database Routing Strategy
**Decision:** Store connections and field mappings in master DB, sync jobs in tenant DB.

**Rationale:** OAuth credentials and field mapping config are org-wide settings that need to be accessible before tenant DB connection is established. Sync job history is tenant-specific data that should live with the org's records for proper multi-tenancy isolation.

**Implementation:** Repository uses struct's `db` field for master DB operations and `WithDB(conn)` method for tenant DB operations, following the existing `ScanJobRepo` pattern.

### Token Encryption Approach
**Decision:** AES-256-GCM with base64-encoded environment variable key.

**Rationale:**
- AES-256-GCM is FIPS-approved and provides authenticated encryption
- GCM mode prevents tampering and provides integrity checking
- Environment variable storage allows key rotation without code changes
- Base64 encoding simplifies key management and transport

**Alternative considered:** Storing plaintext tokens was rejected due to security risk. Even in encrypted database, credentials should be encrypted at rest.

### SFID Prefix Selection
**Decision:** Use 0S* prefix family (0Sf, 0Sy, 0Sm).

**Rationale:** Maintains consistency with existing sync-related prefixes (0Sc for scan schedule, 0Sj for scan job). Easy to recognize as sync/Salesforce-related entities. No conflicts with existing 47 registered prefixes.

## Integration Points

### Upstream Dependencies
None - this is the foundation layer.

### Downstream Dependents
- **17-02 (OAuth Service):** Uses SalesforceRepo connection operations and encryption utility
- **17-03 (Admin UI):** Uses SalesforceRepo for configuration and field mapping management
- **17-04 (Sync Service):** Uses SalesforceRepo sync job operations and MergeInstructionBatch types
- **17-05 (Monitoring):** Uses SalesforceRepo for sync job status queries

### Database Impact
- Added 3 new tables with proper indexing
- Master DB grows with one connection record per org (small footprint)
- Tenant DB grows with sync job history (predictable, can be pruned)

## Verification Results

1. **Compilation:** `go build ./...` succeeds with zero errors
2. **Migration file:** 049 created with three CREATE TABLE statements
3. **Entity types:** All structs compile with correct JSON and db tags
4. **SFID prefixes:** Three new prefixes (0Sf, 0Sy, 0Sm) registered without conflicts
5. **Repository methods:** All CRUD operations present for three tables
6. **Encryption utility:** EncryptToken/DecryptToken functions compile successfully

## Security Considerations

### Token Encryption
- OAuth tokens encrypted at rest using AES-256-GCM
- Encryption key stored in environment variable (not in code or database)
- Nonce generated using crypto/rand for each encryption operation
- GCM mode provides authenticated encryption (prevents tampering)

### Key Management
- Key must be 32 bytes (256 bits) for AES-256
- Key should be base64-encoded in environment variable
- Production deployment requires proper key rotation strategy
- Key should be stored in secure secret management system (Railway secrets, AWS Secrets Manager, etc.)

### Database Security
- Encrypted tokens stored as BLOB type (binary)
- Even with database access, tokens cannot be decrypted without encryption key
- Client secret also encrypted for defense-in-depth

## Performance Characteristics

### Query Performance
- Index on `salesforce_connections(org_id)` for fast connection lookup
- Index on `sync_jobs(org_id, status)` for job listing and status filters
- Index on `sync_jobs(batch_id)` for batch tracking
- Index on `salesforce_field_mappings(org_id, entity_type)` for field mapping lookup

### Encryption Overhead
- AES-256-GCM encryption: ~1-2μs per token (negligible)
- Token refresh happens infrequently (every 2 hours max)
- No encryption in hot path (auth token validation uses JWT, not OAuth)

### Storage Overhead
- Encrypted tokens slightly larger than plaintext (~16 bytes GCM overhead)
- Per-org connection record: <1KB
- Per sync job record: <5KB (includes batch_payload JSON)
- Field mappings: ~100 bytes per mapping

## Next Steps

1. **Plan 17-02:** Implement OAuth 2.0 service with token refresh logic
2. **Plan 17-03:** Build admin UI for Salesforce connection configuration
3. **Plan 17-04:** Create sync service to process merge instruction batches
4. **Plan 17-05:** Add monitoring dashboard for sync job status

## Self-Check

**Files exist:**
```
✓ backend/migrations/049_create_salesforce_sync_tables.sql
✓ backend/internal/entity/salesforce_sync.go
✓ backend/internal/repo/salesforce_sync.go
✓ backend/internal/util/encryption.go
✓ backend/internal/sfid/sfid.go (modified)
```

**Commits exist:**
```
✓ 05bb58a - Task 1: Database migration and entity types
✓ 9814395 - Task 2: Repository layer and encryption utility
```

**Compilation:**
```
✓ go build ./... succeeds with zero errors
```

**SFID prefixes:**
```
✓ 0Sf (PrefixSFConnection) - no conflicts
✓ 0Sy (PrefixSyncJob) - no conflicts
✓ 0Sm (PrefixSFFieldMapping) - no conflicts
```

## Self-Check: PASSED

All files created, all commits exist, all code compiles successfully.
