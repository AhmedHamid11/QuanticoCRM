# Domain Pitfalls: Salesforce Integration

**Domain:** Salesforce API Integration for CRM Sync
**Researched:** 2026-02-09
**Confidence:** MEDIUM to HIGH (verified with official docs, community sources, and Salesforce developer documentation)

---

## Executive Summary

Salesforce integration introduces seven critical pitfall categories that can cause data loss, sync failures, or production outages if not addressed upfront. The most dangerous pitfalls involve **OAuth token management** (tokens expire without warning), **record ID format mismatches** (15 vs 18 characters causing deduplication failures), and **Flow partial failures** (merge instructions lost when staging object fills). Quantico's high-volume sync (hundreds of thousands of records) amplifies these risks, especially around rate limiting, governor limits, and data integrity during failures.

**Key findings:**
- OAuth refresh tokens can be silently revoked by admin policy changes or password resets
- 15-character IDs are case-sensitive; Excel/databases treat them as identical, breaking deduplication
- Salesforce enforces 25 concurrent long-running requests and 15,000 batches per 24 hours
- Field API names are infrastructure; changing them breaks all integrations silently
- Long Text Area fields have 131,072 character limit; JSON payloads must be validated client-side
- Sandbox-to-production failures often caused by missing dependencies or version mismatches
- Flow scheduled paths have built-in retry (4 retries over 3h45m), but custom fault paths disable this

---

## Critical Pitfalls

### Pitfall 1: OAuth Token Expiration Without Refresh Logic

**What goes wrong:** Access tokens expire (typically 2 hours), and your integration stops syncing silently. Users expect data to flow, but Quantico CRM stops receiving/sending updates with no visible errors.

**Why it happens:**
- Salesforce access tokens expire after 15 minutes to 2 hours depending on session policies
- If admin sets token policy to "Immediately expire refresh token", refresh tokens stop working entirely
- Password changes revoke ALL access and refresh tokens for that user
- Spring 2024 update introduced rotating refresh tokens—old refresh token becomes invalid after use, and apps must store the new one
- Concurrent refresh token requests can cause race conditions where both requests invalidate each other

**Consequences:**
- Silent sync failures (no error visible to users)
- Data drift between Quantico and Salesforce
- Customer reports "my changes aren't syncing" hours or days after token expiration
- Manual re-authentication required, disrupting workflow

**Prevention:**
1. **Always request `refresh_token` or `offline_access` scope** during OAuth flow
2. **Store new refresh tokens immediately**: Spring 2024+ returns new refresh_token with every refresh; old token gets revoked
3. **Implement proactive token refresh**: Refresh access token 5-10 minutes before expiration, not reactively after 401 error
4. **Handle refresh token concurrency**: Use a mutex/lock to prevent simultaneous refresh requests
5. **Detect policy overrides**: Log when refresh fails with `invalid_grant` error; prompt user to re-authenticate
6. **Monitor for password changes**: When refresh fails, detect if it's due to password change (specific error code) and notify user to re-authorize

**Detection warning signs:**
- `invalid_grant` error when attempting token refresh
- 401 Unauthorized responses after previously successful API calls
- Refresh token returns error "expired access/refresh token"
- Users report sync stopped working with no recent changes

**Validation rules:**
- Test refresh flow every 15 minutes in staging
- Simulate admin policy override (set "Immediately expire refresh token" in sandbox and verify error handling)
- Test concurrent refresh requests (2+ threads attempting refresh simultaneously)
- Test password change scenario: change user password, verify old tokens rejected, verify re-auth prompt appears

**Confidence:** HIGH (verified with [Salesforce OAuth Refresh Token Flow documentation](https://help.salesforce.com/s/articleView?id=xcloud.remoteaccess_oauth_refresh_token_flow.htm&language=en_US&type=5), [Nango invalid_grant blog](https://nango.dev/blog/salesforce-oauth-refresh-token-invalid-grant), [Paragon expired token solutions](https://www.useparagon.com/integration-error-solutions/salesforce-invalid-grant-expired-access-refresh-token))

---

### Pitfall 2: Rate Limiting Without Exponential Backoff + Jitter

**What goes wrong:** Salesforce returns HTTP 429 "Too Many Requests", and your sync retry logic causes a thundering herd problem—all retry attempts hit at the same time, causing more 429s and cascading failures.

**Why it happens:**
- Salesforce enforces rate limits per user, per hour (session ID-based) or per user, per application, per hour (OAuth-based)
- Marketing Cloud API enforces strict rate limiting with 429 responses
- Multiple Quantico workers/processes retry simultaneously after 429, creating synchronized bursts
- Retry-After header in 429 response tells you how long to wait, but many libraries ignore it
- API calls longer than 20 seconds count against concurrent request limit (25 max in production, 5 in dev orgs)

**Consequences:**
- Sync completely stalls for extended periods
- Cascading failures: more retries → more 429s → longer recovery time
- Salesforce may temporarily block your IP or OAuth app
- Data sync delays compound (hours or days behind)

**Prevention:**
1. **Implement exponential backoff with jitter**: First retry at 2s + random(0-1s), second at 4s + random(0-2s), third at 8s + random(0-4s), etc.
2. **Respect Retry-After header**: Always check 429 response for `Retry-After` header and wait that duration before retrying
3. **Use per-user OAuth tokens, not shared session IDs**: OAuth provides per-user, per-app quota instead of shared per-user quota
4. **Monitor response codes**: Track 429 rates and adjust request volume dynamically
5. **Cache access tokens**: Don't request new access tokens more than once per 20 minutes
6. **Circuit breaker pattern**: After N consecutive 429s, pause all requests for M seconds before resuming

**Detection warning signs:**
- Increasing 429 error rate in logs
- API response times degrading (approaching 20-second threshold)
- "Concurrent API Request Limit Exceeded" errors
- Sync queue backing up during peak hours

**Architecture choice:**
- Use distributed rate limiter (Redis-based) if running multiple Quantico backend instances
- Implement job queue with rate-limited workers (e.g., 80% of known limit to leave headroom)

**Confidence:** HIGH (verified with [Salesforce Marketing Cloud rate limiting best practices](https://developer.salesforce.com/docs/marketing/marketing-cloud/guide/rate-limiting-best-practices.html), [Docebo 429 handling guide](https://help.docebo.com/hc/en-us/articles/31803763436946-Best-practices-for-handling-API-rate-limits-and-429-errors), [API Status Check rate limit guide](https://apistatuscheck.com/blog/how-to-handle-api-rate-limits))

---

### Pitfall 3: 15-Character vs 18-Character ID Confusion

**What goes wrong:** Quantico stores 15-character Salesforce IDs. Two different Salesforce records with IDs differing only by case (e.g., `0013000000AbCdE` vs `0013000000aBcDe`) are treated as identical in Quantico's database, causing data overwrites, failed lookups, and incorrect merges.

**Why it happens:**
- Salesforce UI and exports often show 15-character IDs (case-sensitive)
- 18-character IDs have 3-character checksum suffix that makes them case-insensitive
- Excel, SQL databases, and many programming languages default to case-insensitive string comparison
- Case-insensitive systems can't distinguish `AbC` from `abc`, treating different records as duplicates

**Consequences:**
- Deduplication logic fails: two records collapse into one
- Lookups return wrong record or fail to find existing record
- Merge instructions target wrong Salesforce record
- Data corruption: Record A's data overwrites Record B's data

**Prevention:**
1. **Always use 18-character IDs in Quantico database**: Validate and convert on ingestion
2. **Use Salesforce CASESAFEID() formula** to convert 15-char to 18-char in Salesforce-side exports/queries
3. **Validate ID format on API boundaries**: Reject 15-character IDs from Quantico frontend, require 18-character
4. **Add DB constraint**: Quantico schema should enforce `salesforce_id` column is exactly 18 characters
5. **Test with case-differing IDs**: Create test data with IDs like `001xx00000AbCdE` and `001xx00000aBcDe` to verify deduplication works

**Detection warning signs:**
- Unexpected record merges reported by users
- "Duplicate record" errors in Salesforce API responses using 15-char ID
- Lookup failures where ID is valid in Salesforce but not found in Quantico
- Two Salesforce records mapping to one Quantico record

**Validation rules:**
- Add API validation: `if len(salesforce_id) != 18: reject`
- Add database migration: `ALTER TABLE ... ADD CONSTRAINT CHECK (length(salesforce_id) = 18)`
- Frontend validation: Ensure ID fields accept exactly 18 characters

**Confidence:** HIGH (verified with [Nick Frates 15 vs 18 guide](https://www.nickfrates.com/blog/salesforce-15-vs-18-digit-id-differences-conversion-guide), [MatchMyEmail Salesforce ID guide](https://www.matchmyemail.com/salesforce-15-to-18/), [Medium Salesforce ID article](https://medium.com/@shirley_peng/salesforce-record-id-15-vs-18-characters-why-you-see-both-fe9be3ce9ec2))

---

### Pitfall 4: Field Mapping Using Display Labels Instead of API Names

**What goes wrong:** Quantico maps fields using user-facing labels like "Customer Name". Admin renames label to "Client Name" in Salesforce. All Quantico sync jobs break silently because field lookups fail.

**Why it happens:**
- Field labels are user-facing and changeable; field API names are infrastructure and permanent
- Changing API names breaks every Flow, report, email template, code reference, and integration
- Most failures are silent: records aren't created, emails aren't sent, but no error message appears
- Field API names are case-sensitive; `Account_Name__c` ≠ `account_name__c`
- Custom fields have `__c` suffix; relationships have `__r` suffix; these are easy to forget or mix up

**Consequences:**
- Silent sync failures: Quantico sends data to wrong field or null field
- Data loss: Field mappings fail, data discarded without error
- Admin confusion: "I only changed the label, why did sync break?"
- Integration maintenance burden: Every label change requires developer intervention

**Prevention:**
1. **Always use field API names in Quantico code and configs**: Never reference labels
2. **Store API names in Quantico database**: Map Quantico fields to Salesforce API names (e.g., `customer_name` → `Account_Name__c`)
3. **Never allow API name changes**: Document that changing API names is forbidden; only labels can change
4. **Fetch field metadata on sync startup**: Call Salesforce Describe API to validate all mapped fields still exist
5. **Case-sensitive comparisons**: Treat `Account_Name__c` and `account_name__c` as different fields
6. **Suffix validation**: Custom fields must end in `__c`, relationships in `__r`, external IDs as configured

**Detection warning signs:**
- "Field does not exist" errors in Salesforce API responses
- Sync succeeds but data not appearing in expected Salesforce fields
- Admin reports "field rename broke sync" after changing label only
- Two mapping options appear in dropdown with slightly different names (e.g., one has extra "of")

**Validation rules:**
- API name regex validation: `^[A-Za-z][A-Za-z0-9_]*(__c|__r)?$`
- Pre-flight check: Before sync, call `/services/data/v61.0/sobjects/{object}/describe` and verify all mapped fields exist
- Config validation: Reject field mappings using labels; require API names

**Confidence:** MEDIUM to HIGH (verified with [Pardot API Name Trap article](https://www.salesforceben.com/mapping-pardot-and-salesforce-custom-fields-api-name/), [Medium API Name Disaster](https://medium.com/@jcarmona86/salesforce-field-rename-api-name-disaster-broke-integrations-342f0e4473e4), [Salesforce Help API vs Label](https://help.salesforce.com/s/articleView?id=000387274&language=en_US&type=1))

---

### Pitfall 5: Long Text Area JSON Payload Size Exceeded

**What goes wrong:** Quantico sends JSON merge instructions exceeding 131,072 characters to Salesforce Long Text Area field. Salesforce silently truncates the payload, Flow parses invalid JSON, merge instructions are lost, and records aren't merged.

**Why it happens:**
- Salesforce Long Text Area fields have strict 131,072 character limit
- Large merges (hundreds of field updates across dozens of records) generate large JSON payloads
- Salesforce doesn't return validation error on truncation—it just cuts off the string
- Flow JSON parsing fails on truncated JSON with cryptic error
- Developers assume "Long Text Area = unlimited" without checking limits

**Consequences:**
- Merge instructions lost: Records not merged, data inconsistency
- Flow errors: JSON parsing fails, entire batch rejected
- Silent failures: No error returned to Quantico, looks like success
- Data corruption: Partial JSON parsed successfully, executing incomplete merge instructions

**Prevention:**
1. **Validate payload size client-side**: Before sending to Salesforce, check `len(json_payload) <= 131072`
2. **Split large payloads**: If merge instructions exceed limit, split into multiple staging records
3. **Compress JSON**: Remove whitespace, use short field names in JSON (e.g., `rec_id` not `record_identifier`)
4. **Estimate worst-case payload size**: Calculate max payload for largest expected merge (e.g., 200 records × 50 fields × 50 chars avg)
5. **Add server-side validation in Salesforce**: Use Salesforce validation rule or Apex trigger to reject records with JSON exceeding limit
6. **Monitor payload sizes**: Log JSON payload sizes, alert when approaching 120,000 characters (safety margin)

**Detection warning signs:**
- Flow errors: "Unexpected character at position 131072" or "Unexpected end of JSON input"
- Merge instructions partially executed (some records merged, others not)
- Staging records created but no corresponding merge activity in Salesforce
- Error emails from Flow with JSON parsing failure

**Validation rules:**
- Quantico backend: `if len(merge_json) > 131072: split_payload()`
- Salesforce validation rule: `LEN(Merge_Instructions__c) > 131072` → error
- Unit test: Generate merge payload with 500 records, verify splitting logic activates

**Confidence:** MEDIUM (Long Text Area limit of 131,072 chars confirmed in search results from [Salesforce FAQ](https://salesforcefaqs.com/create-text-area-long-field-type-in-salesforce/), but specific JSON truncation behavior needs validation in Salesforce testing)

---

## Moderate Pitfalls

### Pitfall 6: Sandbox vs Production Environment Differences

**What goes wrong:** Sync works perfectly in Salesforce sandbox. Deploy to production, and sync fails due to missing custom fields, outdated platform version, or different duplicate rules.

**Why it happens:**
- Sandboxes are static snapshots, updated only on manual refresh
- Sandbox refresh can take days or weeks for complex orgs, causing staleness
- Production and sandbox may run different Salesforce platform versions during release windows
- Dependencies (custom objects, Apex classes, fields) missing in production but present in sandbox
- Data masking in sandbox means test data doesn't match production patterns

**Consequences:**
- Production deployment failures
- Emergency rollbacks
- Data sync broken for customers during business hours
- Extended downtime while troubleshooting environment-specific issues

**Prevention:**
1. **Use Full Copy sandbox for final testing**: Developer sandboxes lack data; Full Copy mirrors production
2. **Verify platform versions match**: Check Salesforce release version in sandbox vs production before deploying
3. **Deploy dependencies first**: Use metadata API or change sets to deploy custom fields, objects, Apex classes before deploying sync logic
4. **Test with production-like data volumes**: Sandbox with 100 records won't catch issues that appear with 1M records
5. **Automated dependency detection**: Use Salesforce CLI to scan metadata dependencies and verify they exist in production
6. **Scheduled sandbox refresh**: Refresh sandbox monthly to keep it aligned with production

**Detection warning signs:**
- "Field does not exist" errors in production but not sandbox
- Deployment validation errors: "This component is newer than the installed version"
- Performance issues in production not seen in sandbox (data volume differences)
- Duplicate rules behave differently (different matching rules in prod vs sandbox)

**Testing strategy:**
- Pre-deployment checklist: Run `sfdx force:source:deploy --checkonly` to validate without deploying
- Test in multiple sandbox types: Developer → Partial Copy → Full Copy → Production
- Automated tests must pass in Full Copy sandbox before production deployment

**Confidence:** MEDIUM to HIGH (verified with [Flosum Sandbox vs Production guide](https://www.flosum.com/blog/the-difference-between-salesforce-production-sandbox), [Salesforce Help performance differences](https://help.salesforce.com/s/articleView?id=000381735&language=en_US&type=1), [Salto sandbox strategies](https://www.salto.io/guides/salesforce-sandbox-strategies))

---

### Pitfall 7: Salesforce Flow Fault Path Disabling Built-in Retry

**What goes wrong:** You add a custom fault path to handle Flow errors gracefully. Salesforce's built-in retry logic (4 retries over 3h45m for scheduled paths) stops working. Transient errors that would have auto-resolved now fail permanently.

**Why it happens:**
- Salesforce Flow scheduled paths have built-in retry logic for failures
- If you connect a fault path to an element, Salesforce assumes you're handling errors manually and disables auto-retry
- Developers add fault paths to log errors without realizing retry is disabled
- Transient network errors or temporary Salesforce unavailability would auto-resolve, but now fail immediately

**Consequences:**
- Increased failure rate: Transient errors become permanent failures
- Manual intervention required: Errors that would auto-resolve need manual retry
- Lost merge instructions: Flow fails once, no retry, staging record orphaned
- User receives 5 error emails (initial failure + 4 retries) when built-in retry is active, but only 1 email (failure) when fault path is present

**Prevention:**
1. **Only add fault paths when you need custom error handling**: Don't add them "just in case"
2. **Implement manual retry logic in fault path**: If adding fault path, include loop element to retry N times with delay
3. **Use scheduled path retry for transient errors**: Let Salesforce handle network timeouts, temporary unavailability
4. **Document retry behavior**: Add comments in Flow explaining when retry is active vs disabled
5. **Test fault path behavior**: Simulate transient error (e.g., temporarily disable Apex class), verify retry happens or doesn't happen as expected

**Detection warning signs:**
- Error emails decrease from 5 to 1 after adding fault path (retry disabled)
- Transient errors now require manual intervention
- Flow success rate decreases after adding "error handling improvements"

**Testing strategy:**
- Simulate transient error in scheduled path without fault path → verify 4 retries over 3h45m
- Add fault path, simulate same error → verify no retries, fault path executes once
- If fault path includes manual retry, verify retry attempts logged

**Confidence:** HIGH (verified with [Salesforce Time retry logic article](https://salesforcetime.com/2025/07/03/retry-logic-in-record-triggered-flow-scheduled-paths/), [Nick Frates fault path guide](https://www.nickfrates.com/blog/salesforce-flow-error-handling-101-fault-paths-explained), [Salesforce Help fault path docs](https://help.salesforce.com/s/articleView?id=sf.flow_build_logic_fault.htm&language=en_US&type=5))

---

### Pitfall 8: Governor Limits on Bulk Operations

**What goes wrong:** Quantico sends 500 records in a single API call. Salesforce rejects the batch because it exceeds governor limits (CPU time, heap size, SOQL queries, or DML statements).

**Why it happens:**
- Salesforce enforces strict governor limits to prevent runaway processes
- Bulk API and Bulk API 2.0 share a 15,000 batch limit per 24 hours
- CPU time limited to 60,000 milliseconds for Bulk API operations
- Concurrent job limit: only 3 Bulk API jobs can run simultaneously
- Large batches trigger complex workflows, triggers, or validation rules that exceed limits

**Consequences:**
- Batch rejected entirely (all-or-nothing failure)
- Sync queue backs up as batches retry
- Hit 15,000 batch/day limit, blocking all sync for 24 hours
- Performance degradation as concurrent job limit reached

**Prevention:**
1. **Use Bulk API 2.0 for large data sets**: More efficient than REST API for >2,000 records
2. **Batch size tuning**: Start with 200 records/batch, monitor performance, adjust down if errors occur
3. **Monitor batch consumption**: Track batches used per day, alert at 80% of 15,000 limit
4. **Respect concurrent job limit**: Don't start more than 3 Bulk API jobs simultaneously
5. **Optimize Salesforce-side automation**: Review triggers, workflows, Process Builders for inefficiency
6. **Use asynchronous patterns**: Don't wait for batch to complete; poll status endpoint

**Detection warning signs:**
- "Concurrent API Batch Limit Exceeded" errors
- "CPU time limit exceeded" in batch failure logs
- Batch processing time increasing (approaching 10-minute API timeout)
- 15,000 batch limit reached before end of day

**Architecture choice:**
- Implement job queue with concurrency limit (max 3 concurrent Bulk API jobs)
- Dynamic batch sizing: Reduce batch size if failures increase, increase if success rate high

**Confidence:** HIGH (verified with [Salesforce Bulk API limits cheatsheet](https://developer.salesforce.com/docs/atlas.en-us.salesforce_app_limits_cheatsheet.meta/salesforce_app_limits_cheatsheet/salesforce_app_limits_platform_bulkapi.htm), [Flosum Bulk API limits guide](https://www.flosum.com/blog/maximizing-salesforce-bulk-api-limits-5-tips-for-enterprise-users), [Salesforce Bulk API PDF](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/api_asynch.pdf))

---

### Pitfall 9: Null Value Handling Inconsistencies

**What goes wrong:** Quantico sends `"field": null` in JSON to Salesforce API. Salesforce interprets this as "don't update field" instead of "set field to null". Field retains old value, causing data staleness.

**Why it happens:**
- Salesforce API distinguishes between absent field (don't update), `null` (clear field), and empty string `""` (set to empty)
- REST API may ignore `null` values depending on endpoint and API version
- Data Loader with Bulk API requires "Insert null values" checkbox to be enabled
- Salesforce formulas treat `null` and empty string differently: concatenating `null` produces string `"null"`

**Consequences:**
- Fields not cleared when they should be
- Data staleness: Old values persist when they should be deleted
- Inconsistent behavior: Some fields clear, others don't
- Hard-to-debug sync issues: Records look correct in Quantico but wrong in Salesforce

**Prevention:**
1. **Explicitly set fields to null using `fieldsToNull` parameter**: In SOAP API, use `<fieldsToNull>` element; in REST API, send `"Field__c": null` explicitly
2. **Distinguish between "don't send" and "send null"**: Quantico should track field state: unset (omit from JSON), null (send `null`), or value (send value)
3. **Test null handling**: Create test case where field has value, update to null, verify field cleared in Salesforce
4. **Use isEmpty() and isBlank() correctly in Apex**: `isEmpty()` checks for empty string, `isBlank()` checks for null or empty
5. **Document Salesforce's null behavior**: Add comments explaining `null` vs `""` vs omitted field

**Detection warning signs:**
- Fields not clearing when "Clear field" action performed in Quantico
- Concatenation producing string `"null"` in Salesforce formulas
- Unexpected values persisting after update
- "Set to blank" operations in Quantico not reflected in Salesforce

**Validation rules:**
- Unit test: Update field to `null`, verify Salesforce field is empty
- Integration test: Send `{"Field__c": null}` to Salesforce REST API, query record, verify field is null

**Confidence:** MEDIUM (verified with [Salesforce null handling guide](https://matheus.dev/how-salesforce-handles-null-and-empty-strings/), [Salesforce Help Data Loader behavior](https://help.salesforce.com/s/articleView?id=000385696&language=en_US&type=1), [Trailblazer Community null vs empty string](https://trailhead.salesforce.com/trailblazer-community/feed/0D54S00000A83hySAB))

---

### Pitfall 10: Invocable Apex JSON Parsing Errors

**What goes wrong:** Quantico sends JSON payload to Salesforce Flow via Invocable Apex. Apex class uses custom wrapper class with `@InvocableVariable`. Flow fails with "Invalid type" error because Flow doesn't support custom classes.

**Why it happens:**
- Flow only accepts primitive types or standard SObject types as `@InvocableVariable` inputs
- Custom wrapper classes are invalid for `@InvocableVariable` (despite compiling successfully)
- Developers assume JSON deserialization works like REST API, but Flow has stricter type rules
- Maximum 20 fields per Apex Action Element to avoid iteration limit during JSON parsing
- `@InvocableMethod` returns text by default; type conversion must happen in Flow

**Consequences:**
- Flow fails at runtime with "Invalid type for InvocableVariable"
- JSON parsing errors in Apex due to malformed input
- Flow can't handle complex nested JSON structures
- Hard 20-field limit forces payload splitting

**Prevention:**
1. **Use only primitive types or SObject types in `@InvocableVariable`**: String, Integer, Boolean, Date, DateTime, Account, Contact, etc.
2. **Parse JSON in Apex, not Flow**: Accept JSON as String parameter, deserialize in Apex, return results as standard types
3. **Limit to 20 fields per Apex Action**: If more fields needed, split into multiple invocable methods
4. **Type conversion in Flow**: `@InvocableMethod` returns String; use Flow assignment to convert to Date, Number, etc.
5. **Test with Agentforce compatibility**: Use `@JsonAccess(serializable='always' deserializable='always')` for classes intended for JSON serialization
6. **Separate structures into different classes**: Don't nest custom classes; Flow won't recognize variables in nested structures

**Detection warning signs:**
- Flow error: "The type X is not valid for InvocableVariable"
- JSON parsing errors in Apex at runtime (not compile time)
- Flow can't see variables in custom class
- "Too many SOQL queries" error when processing JSON with >20 fields

**Validation rules:**
- Code review: Reject `@InvocableVariable` with custom class types
- Unit test: Call invocable method from Flow (not just Apex), verify it works
- 75% code coverage minimum for Apex classes handling JSON

**Confidence:** MEDIUM to HIGH (verified with [Salesforce LLM Apex mistakes](https://salesforcediaries.com/2026/01/16/llm-mistakes-in-apex-lwc-salesforce-code-generation-rules/), [Medium JSON Parser in Flow](https://munawirrahman.medium.com/json-parser-in-salesforce-flow-380b46c86030), [Salesforce Developer JSONParser docs](https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_class_System_JsonParser.htm))

---

## Minor Pitfalls

### Pitfall 11: Staging Object Record Limit Exceeded

**What goes wrong:** Quantico creates staging records faster than Salesforce Flow can process them. Staging object fills up, hits storage limit, new sync requests fail with "Storage limit exceeded" error.

**Why it happens:**
- No automatic cleanup for custom staging objects
- Flow processes staging records slower than Quantico creates them (especially during bulk sync)
- Salesforce data storage limits: Enterprise Edition gets 1GB + 20MB per user license
- Large JSON payloads in Long Text Area fields consume storage quickly

**Consequences:**
- New sync requests rejected
- Data sync stops until storage freed
- Manual intervention required: bulk delete staging records
- Customers can't sync data during peak usage

**Prevention:**
1. **Scheduled cleanup job**: Delete staging records after successful processing (mark as "Processed", delete after 7 days)
2. **Flow cleanup step**: Add "Delete record" element at end of Flow after merge succeeds
3. **Monitor staging record count**: Alert when >10,000 unprocessed records accumulate
4. **Storage monitoring**: Use Salesforce storage reports to track usage, alert at 80%
5. **Batch processing limits**: Don't allow more than N staging records to be created per hour
6. **Archive instead of delete**: Move old staging records to Big Object for long-term storage

**Detection warning signs:**
- "Insufficient storage" errors when creating staging records
- Staging object row count increasing monotonically (no cleanup happening)
- Storage usage alerts from Salesforce
- Sync queue backing up during bulk operations

**Prevention strategy:**
- Implement waterfall cleanup: Delete records >7 days old (success or failure), archive records >30 days old
- Flow final step: "Delete staging record" after merge completes successfully
- Apex scheduled job: Weekly cleanup of orphaned staging records (failed to process)

**Confidence:** MEDIUM (verified with [Salesforce Ben custom staging article](https://www.salesforceben.com/how-to-use-custom-staging-to-handle-email-volume-limits-in-salesforce/), [Flosum storage management](https://www.flosum.com/blog/salesforce-storage-limits), [Xappex data storage optimization](https://www.xappex.com/blog/salesforce-data-storage/))

---

### Pitfall 12: Duplicate Detection Bypassed by External ID

**What goes wrong:** Quantico uses Salesforce External ID field for upsert operations. Duplicate records created because Salesforce's duplicate rules don't fire on API upserts with External ID.

**Why it happens:**
- Duplicate rules apply to UI and some API operations, but not all
- External ID upsert bypasses duplicate detection by default (performance optimization)
- `allowSave` setting in duplicate rule may allow API operations even when duplicates detected
- Duplicate rules limited to 5 per object; complex deduplication logic may not fit

**Consequences:**
- Duplicate Contact/Account/Lead records in Salesforce
- Data quality degradation
- Merge complexity: Manual merge of duplicates required
- Reporting inaccuracies due to duplicate records

**Prevention:**
1. **Enable duplicate rules on API operations**: Set `allowSave=false` for API in duplicate rule config
2. **Data profiling before sync**: Identify duplicates in Quantico before sending to Salesforce
3. **Use External ID from trusted source**: External IDs should come from authoritative system (e.g., DUNS Number for accounts)
4. **Pre-flight duplicate check**: Before creating record, query Salesforce for potential duplicates using matching logic
5. **Apex trigger validation**: Add custom duplicate detection in before-insert trigger if duplicate rules insufficient

**Detection warning signs:**
- Multiple Salesforce records with same email/phone/name
- Duplicate rule alerts in UI but duplicates still created via API
- Users report "same contact appearing twice"

**Validation rules:**
- Test duplicate creation via API: Attempt to create duplicate using External ID, verify rejection or merge
- Review duplicate rules: Ensure `allowSave=false` for API operations
- Data profiling: Run duplicate detection on Quantico data before first sync

**Confidence:** MEDIUM (verified with [Salesforce Ben duplicate rules guide](https://www.salesforceben.com/salesforce-duplicate-rules/), [Xappex duplicate management](https://www.xappex.com/blog/salesforce-duplicate-management/), [Salesforce Help duplicate detection process](https://help.salesforce.com/s/articleView?id=sales.duplicate_detection_and_handling.htm&language=en_US&type=5))

---

### Pitfall 13: Concurrent Request Limits (Long-Running API Calls)

**What goes wrong:** Quantico makes 30 simultaneous API calls to Salesforce. 25 succeed, 5 fail with "ConcurrentPerOrgLongTxn Limit exceeded" error. Sync partially completes, causing data inconsistency.

**Why it happens:**
- Salesforce limits long-running API requests (>20 seconds) to 25 concurrent in production, 5 in dev orgs
- No limit on requests <20 seconds
- Large batch operations, complex SOQL queries, or trigger-heavy objects cause requests to exceed 20 seconds
- Quantico's parallel sync workers all hit Salesforce simultaneously

**Consequences:**
- Partial sync failures
- Data inconsistency between Quantico and Salesforce
- Error rate increases during bulk operations
- Unpredictable failures: same request succeeds when retried alone but fails in batch

**Prevention:**
1. **Concurrency limit in Quantico**: Max 20 concurrent API requests (safety margin below 25 limit)
2. **Monitor request duration**: Log API call duration, alert when requests approach 20 seconds
3. **Optimize queries**: Use selective SOQL filters, indexed fields, avoid complex joins
4. **Use Bulk API for large operations**: Bulk API doesn't count against concurrent request limit for queries
5. **Backpressure mechanism**: If concurrent limit errors increase, reduce parallelism dynamically

**Detection warning signs:**
- "ConcurrentPerOrgLongTxn Limit exceeded" errors
- API requests timing out (approaching 10-minute max)
- Errors correlate with bulk sync operations

**Architecture choice:**
- Implement semaphore/job queue with max 20 concurrent workers
- Use exponential backoff on concurrent limit errors (treat like rate limiting)

**Confidence:** MEDIUM to HIGH (verified with [Salesforce API limits cheatsheet](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_app_limits_cheatsheet.pdf), [Coefficient rate limits guide](https://coefficient.io/salesforce-api/salesforce-api-rate-limits), [Salesforce Help concurrent API errors](https://help.salesforce.com/s/articleView?id=000389067&language=en_US&type=1))

---

### Pitfall 14: Record Locking and Concurrent Updates

**What goes wrong:** Quantico and Salesforce user both update the same Contact record simultaneously. One update succeeds, the other fails with "UNABLE_TO_LOCK_ROW: unable to obtain exclusive access to this record" error. Quantico retries, but Salesforce user's changes are overwritten.

**Why it happens:**
- Salesforce uses row-level locking to prevent concurrent updates
- Updating child record in master-detail relationship locks parent record
- Multiple triggers, workflows, or processes acting on same record cause lock contention
- Quantico has no visibility into Salesforce user activity; both attempt updates simultaneously

**Consequences:**
- Update failures requiring retry
- Lost updates: Last write wins, earlier changes lost
- User frustration: "My changes disappeared"
- Increased error rate during peak usage

**Prevention:**
1. **Retry logic with exponential backoff**: Treat lock errors as transient, retry after delay
2. **Optimistic concurrency**: Use `SystemModstamp` field to detect if record changed since last read
3. **Stagger scheduled jobs**: Avoid multiple processes updating same records simultaneously
4. **Efficient data modeling**: Minimize master-detail relationships that cause parent locking
5. **Granular locking**: Update only changed fields, not entire record

**Detection warning signs:**
- "UNABLE_TO_LOCK_ROW" errors in logs
- Errors increase during business hours (when users active)
- Specific records fail repeatedly (high-contention records)

**Validation rules:**
- Test concurrent update: Two threads update same record simultaneously, verify one retries successfully
- Optimistic concurrency check: Verify `SystemModstamp` before update, reject if changed

**Confidence:** MEDIUM to HIGH (verified with [Salesforce Ben record locking tips](https://www.salesforceben.com/salesforce-record-locking-tips-for-developers-how-to-avoid-concurrency-issues/), [LinkedIn record locking article](https://www.linkedin.com/pulse/understanding-salesforce-record-locking-preventing-them-david-masri), [Salesforce Developer record-level locking docs](https://developer.salesforce.com/docs/atlas.en-us.draes.meta/draes/draes_object_relationships_record_level_locking.htm))

---

### Pitfall 15: SOQL Relationship Query Depth Limits

**What goes wrong:** Quantico needs to fetch Contact, related Account, related Account's Parent Account, and Parent Account's Owner. Single SOQL query fails with "Maximum relationship depth exceeded" error.

**Why it happens:**
- Parent-to-child (subqueries) limited to 1 level deep
- Child-to-parent limited to 5 levels deep
- No more than 55 child-to-parent relationships in single query
- Maximum 20 parent-to-child relationships in single query
- Developers assume unlimited nesting like SQL joins

**Consequences:**
- Query failures requiring multiple round-trips
- Performance degradation (N+1 query problem)
- Increased API call count (approaching daily limit)

**Prevention:**
1. **Limit child-to-parent queries to 5 levels**: Contact → Account → Parent Account → Parent Account → Parent Account → Owner (5 levels max)
2. **Avoid subqueries beyond 1 level**: Can't query Account → Contacts → Opportunities in single query
3. **Denormalize data when possible**: Store frequently-accessed parent fields directly on child object
4. **Use multiple queries for deep relationships**: Query Account with parent, then separately query Contacts
5. **Optimize with indexed fields**: Use fields marked as External ID or unique for faster queries

**Detection warning signs:**
- "Exceeded max relationship depth" errors
- SOQL queries failing with "Too many levels of nested relationships"
- Performance degradation as relationship traversal increases

**Validation rules:**
- Code review: Count relationship levels in SOQL, flag queries with >4 levels
- Unit test: Test query with maximum expected relationship depth

**Confidence:** HIGH (verified with [Salesforce SOQL relationship query limits](https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql_relationships_query_limits.htm), [Salesforce Ben SOQL limits](https://www.salesforceben.com/salesforce-soql-queries-and-limits/), [Salesforce SOQL PDF Spring '26](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_soql_sosl.pdf))

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| OAuth Implementation | Token refresh failure after admin policy change | Test with "Immediately expire refresh token" policy enabled in sandbox |
| Initial Sync (Bulk Import) | 15,000 batch/day limit exceeded during first sync | Spread initial sync over multiple days; use progressive batching |
| Field Mapping Configuration | Using labels instead of API names | Validate all mappings against Salesforce Describe API before first sync |
| Real-time Sync | Rate limiting during peak usage | Implement exponential backoff + jitter; monitor 429 error rates |
| Merge Instructions | JSON payload exceeds 131,072 chars | Calculate worst-case payload size; implement splitting logic |
| Staging Record Processing | Staging object fills during bulk operations | Implement Flow cleanup step; monitor staging record count |
| Production Deployment | Missing dependencies in production | Use `sfdx force:source:deploy --checkonly` before deploying |
| Error Handling | Fault paths disabling built-in retry | Document when fault paths should/shouldn't be used; test retry behavior |

---

## Testing Strategy

### Unit Tests (Quantico Backend)

| Test Case | Purpose | Pass Criteria |
|-----------|---------|---------------|
| 18-character ID validation | Prevent 15-char ID storage | Reject IDs with length ≠ 18 |
| JSON payload size check | Prevent Long Text Area overflow | Split payloads >131,072 chars |
| OAuth refresh token flow | Handle token expiration | Refresh succeeds, new token stored |
| Exponential backoff on 429 | Prevent thundering herd | Retry delays: 2s, 4s, 8s, 16s with jitter |
| Null value handling | Clear fields correctly | `null` sent as `{"Field__c": null}` |
| Concurrent request limiting | Prevent ConcurrentPerOrgLongTxn errors | Max 20 parallel API calls |

### Integration Tests (Quantico + Salesforce Sandbox)

| Test Case | Purpose | Pass Criteria |
|-----------|---------|---------------|
| Field mapping with API names | Ensure sync survives label changes | Rename label in Salesforce, sync still works |
| Duplicate ID deduplication | Prevent case-insensitive ID collisions | Two IDs differing by case treated as separate |
| Large batch processing | Test governor limits | 200-record batch succeeds without CPU/heap errors |
| Staging record cleanup | Prevent storage overflow | Staging records deleted after processing |
| Record locking retry | Handle concurrent updates | Retry succeeds after UNABLE_TO_LOCK_ROW error |
| Sandbox vs production parity | Catch environment differences | All tests pass in both sandbox and production |

### Load Tests (Quantico + Salesforce Production)

| Test Case | Purpose | Pass Criteria |
|-----------|---------|---------------|
| 15,000 batches/day limit | Verify batch consumption | Sync completes without hitting limit |
| Concurrent job limit (3 max) | Test Bulk API concurrency | No "Concurrent job limit" errors |
| Rate limiting under load | Verify backoff strategy works at scale | 429 errors <1% of requests |
| Long-running request handling | Test 20-second threshold | Requests >20s count against 25 concurrent limit |

---

## Data Integrity Safeguards

### Scenario 1: Staging Object Full

**Problem:** Flow can't create new staging record because storage limit reached.

**Detection:**
- Quantico API call to create staging record fails with "Insufficient storage" error
- Salesforce storage monitoring alert triggers

**Recovery:**
1. Queue failed sync requests in Quantico (don't drop)
2. Trigger emergency cleanup: Delete oldest processed staging records
3. Retry sync requests from queue
4. Alert admin to review staging record retention policy

**Validation:**
- Test: Fill staging object to storage limit, attempt new sync, verify queueing and recovery

---

### Scenario 2: Flow Fails After Staging Record Created

**Problem:** Staging record created in Salesforce, but Flow fails before processing merge instructions.

**Detection:**
- Staging record exists with `Status__c = "Pending"` for >1 hour
- No corresponding merge activity in Salesforce audit logs

**Recovery:**
1. Scheduled Apex job: Query staging records with `Status__c = "Pending" AND CreatedDate < (NOW - 1 hour)`
2. Retry Flow for orphaned staging records
3. After 3 retry attempts, mark as "Failed" and alert admin
4. Quantico polls for failed staging records, notifies user

**Validation:**
- Test: Create staging record, manually fail Flow (e.g., disable Apex class), verify retry job runs

---

### Scenario 3: Partial Merge (Some Records Merged, Others Failed)

**Problem:** Merge instructions contain 100 records. 95 merge successfully, 5 fail due to validation errors. Quantico doesn't know which succeeded.

**Detection:**
- Flow error log: "Failed to update record 001xx000000AbC"
- Staging record marked as "Partial Success"

**Recovery:**
1. Flow should log success/failure per record in custom object (Merge_Result__c)
2. Quantico polls Merge_Result__c, updates sync status per record
3. Failed records flagged in Quantico UI for manual review
4. Retry logic for failed records only (don't re-merge successful ones)

**Validation:**
- Test: Merge instructions with intentionally invalid record (e.g., required field missing), verify partial success handling

---

## Sources

### OAuth Pitfalls
- [Salesforce OAuth 2.0 Refresh Token Flow](https://help.salesforce.com/s/articleView?id=xcloud.remoteaccess_oauth_refresh_token_flow.htm&language=en_US&type=5)
- [Salesforce OAuth refresh token invalid_grant — Nango Blog](https://nango.dev/blog/salesforce-oauth-refresh-token-invalid-grant)
- [Salesforce invalid_grant expired token solutions — Paragon](https://www.useparagon.com/integration-error-solutions/salesforce-invalid-grant-expired-access-refresh-token)
- [Jotform: Understanding Refresh Tokens in Salesforce](https://www.jotform.com/help/understanding-refresh-tokens-in-salesforce/)

### Rate Limiting
- [Salesforce Marketing Cloud Rate Limiting Best Practices](https://developer.salesforce.com/docs/marketing/marketing-cloud/guide/rate-limiting-best-practices.html)
- [Docebo: Best practices for handling API rate limits and 429 errors](https://help.docebo.com/hc/en-us/articles/31803763436946-Best-practices-for-handling-API-rate-limits-and-429-errors)
- [API Status Check: How to Handle API Rate Limits (2026 Guide)](https://apistatuscheck.com/blog/how-to-handle-api-rate-limits)
- [Coefficient: Salesforce API Rate Limits](https://coefficient.io/salesforce-api/salesforce-api-rate-limits)

### Record IDs
- [Nick Frates: Salesforce 15 vs 18 Digit ID Guide](https://www.nickfrates.com/blog/salesforce-15-vs-18-digit-id-differences-conversion-guide)
- [MatchMyEmail: Everything About 15 & 18-Character Salesforce IDs](https://www.matchmyemail.com/salesforce-15-to-18/)
- [Medium: Salesforce Record ID 15 vs 18 Characters](https://medium.com/@shirley_peng/salesforce-record-id-15-vs-18-characters-why-you-see-both-fe9be3ce9ec2)

### Field Mapping
- [Salesforce Ben: Pardot Custom Field Mapping API Name Trap](https://www.salesforceben.com/mapping-pardot-and-salesforce-custom-fields-api-name/)
- [Medium: Salesforce Field Rename API Name Disaster](https://medium.com/@jcarmona86/salesforce-field-rename-api-name-disaster-broke-integrations-342f0e4473e4)
- [Salesforce Help: Difference between API Field Name and Field Label](https://help.salesforce.com/s/articleView?id=000387274&language=en_US&type=1)

### Long Text Area Limits
- [Salesforce FAQ: Long Text Area Salesforce](https://salesforcefaqs.com/create-text-area-long-field-type-in-salesforce/)

### Testing and Sandbox
- [Flosum: Salesforce Production vs Sandbox](https://www.flosum.com/blog/the-difference-between-salesforce-production-sandbox)
- [Salesforce Help: Sandbox vs Production performance](https://help.salesforce.com/s/articleView?id=000381735&language=en_US&type=1)
- [Salto: 6 types of Salesforce sandbox plus a clever sandbox strategy](https://www.salto.io/guides/salesforce-sandbox-strategies)

### Flow Error Handling
- [Salesforce Time: Retry Logic in Record-Triggered Flow Scheduled Paths](https://salesforcetime.com/2025/07/03/retry-logic-in-record-triggered-flow-scheduled-paths/)
- [Nick Frates: Salesforce Flow Error Handling 101](https://www.nickfrates.com/blog/salesforce-flow-error-handling-101-fault-paths-explained)
- [Salesforce Help: Handle Flow Errors with Fault Paths](https://help.salesforce.com/s/articleView?id=sf.flow_build_logic_fault.htm&language=en_US&type=5)

### Governor Limits
- [Salesforce Developer: Bulk API and Bulk API 2.0 Limits](https://developer.salesforce.com/docs/atlas.en-us.salesforce_app_limits_cheatsheet.meta/salesforce_app_limits_cheatsheet/salesforce_app_limits_platform_bulkapi.htm)
- [Flosum: Maximizing Salesforce Bulk API Limits](https://www.flosum.com/blog/maximizing-salesforce-bulk-api-limits-5-tips-for-enterprise-users)
- [Salesforce Bulk API 2.0 PDF (Spring '26)](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/api_asynch.pdf)

### Null Handling
- [Matheus.dev: How Salesforce handles Null and Empty Strings](https://matheus.dev/how-salesforce-handles-null-and-empty-strings/)
- [Salesforce Help: Data Loader behavior with Bulk API enabled](https://help.salesforce.com/s/articleView?id=000385696&language=en_US&type=1)
- [Trailblazer Community: Difference between null and empty string](https://trailhead.salesforce.com/trailblazer-community/feed/0D54S00000A83hySAB)

### Invocable Apex
- [Salesforce Diaries: LLM Mistakes in Apex & LWC](https://salesforcediaries.com/2026/01/16/llm-mistakes-in-apex-lwc-salesforce-code-generation-rules/)
- [Medium: JSON Parser in Salesforce Flow](https://munawirrahman.medium.com/json-parser-in-salesforce-flow-380b46c86030)
- [Salesforce Developer: JSONParser Class](https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_class_System_JsonParser.htm)

### Staging and Storage
- [Salesforce Ben: Custom Staging to Handle Email Volume Limits](https://www.salesforceben.com/how-to-use-custom-staging-to-handle-email-volume-limits-in-salesforce/)
- [Flosum: 8 Proven Tips for Managing Salesforce Storage](https://www.flosum.com/blog/salesforce-storage-limits)
- [Xappex: Salesforce Data Storage Optimization](https://www.xappex.com/blog/salesforce-data-storage/)

### Duplicate Management
- [Salesforce Ben: Complete Guide to Salesforce Duplicate Rules](https://www.salesforceben.com/salesforce-duplicate-rules/)
- [Xappex: Salesforce Duplicate Management – A Complete Guide](https://www.xappex.com/blog/salesforce-duplicate-management/)
- [Salesforce Help: Duplicate Detection and Handling Process](https://help.salesforce.com/s/articleView?id=sales.duplicate_detection_and_handling.htm&language=en_US&type=5)

### Concurrency and Locking
- [Salesforce Ben: Record Locking Tips for Developers](https://www.salesforceben.com/salesforce-record-locking-tips-for-developers-how-to-avoid-concurrency-issues/)
- [LinkedIn: Understanding Salesforce record locking](https://www.linkedin.com/pulse/understanding-salesforce-record-locking-preventing-them-david-masri)
- [Salesforce Developer: Record-Level Locking](https://developer.salesforce.com/docs/atlas.en-us.draes.meta/draes/draes_object_relationships_record_level_locking.htm)
- [Salesforce Help: Proactive Alert Monitoring - Concurrent API Errors](https://help.salesforce.com/s/articleView?id=000389067&language=en_US&type=1)

### SOQL Limits
- [Salesforce Developer: Understanding Relationship Query Limitations](https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql_relationships_query_limits.htm)
- [Salesforce Ben: Salesforce SOQL Queries and Limits](https://www.salesforceben.com/salesforce-soql-queries-and-limits/)
- [Salesforce SOQL and SOSL Reference (Spring '26)](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_soql_sosl.pdf)
