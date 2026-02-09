# Stack Pitfalls: Salesforce Integration

**Domain:** Go-based Salesforce OAuth & API Integration
**Researched:** 2026-02-09
**Confidence:** MEDIUM

## Critical Pitfalls

### Pitfall 1: Using Unmaintained Salesforce Go SDKs
**What goes wrong:** Integration breaks when Salesforce API changes, no community support for issues
**Why it happens:** Developer searches "golang salesforce sdk" and uses first result without checking maintenance
**Consequences:**
- `go-force`: Last release Aug 2020, 18 open issues, appears abandoned
- `simpleforce`: No releases published, unclear if maintained
- Breaking changes in Salesforce API v61+ may not be handled
**Prevention:**
- Use stdlib `net/http` with `golang.org/x/oauth2` instead
- Full control over API calls, no abstraction overhead
- Official oauth2 library is well-maintained
**Detection:** SDK import in go.mod, check GitHub last commit date

### Pitfall 2: Storing OAuth Tokens in Plain Text
**What goes wrong:** Security breach, compliance violations (SOX, GDPR, PCI-DSS)
**Why it happens:** Developer skips encryption to ship faster
**Consequences:**
- Tokens leaked in database dumps, logs, backups
- Attackers gain full Salesforce access
- Compliance audits fail
**Prevention:**
```go
// Encrypt tokens before storing
encrypted, _ := encryptToken(token.AccessToken, encryptionKey)
repo.UpsertConnection(orgID, encrypted, ...)

// Use AES-256-GCM (crypto/aes)
func encryptToken(plaintext string, key []byte) ([]byte, error) {
    block, _ := aes.NewCipher(key) // 32 bytes for AES-256
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}
```
**Detection:** Check database schema for BLOB vs TEXT type, grep codebase for "AccessToken"

### Pitfall 3: Global Rate Limiter (Multi-Tenant Failure)
**What goes wrong:** One org hits rate limit, blocks all other orgs
**Why it happens:** Simpler to implement single global limiter
**Consequences:**
- Customer A makes 1000 API calls, customer B can't sync
- Multi-tenant isolation broken
- Cascading failures across all orgs
**Prevention:**
```go
// Per-org rate limiting
type RateLimiterManager struct {
    limiters sync.Map // map[orgID]*rate.Limiter
}

func (m *RateLimiterManager) GetLimiter(orgID string) *rate.Limiter {
    if limiter, ok := m.limiters.Load(orgID); ok {
        return limiter.(*rate.Limiter)
    }
    limiter := rate.NewLimiter(rate.Limit(1.0), 5)
    m.limiters.Store(orgID, limiter)
    return limiter
}
```
**Detection:** Multiple orgs report 429 errors simultaneously despite low individual usage

### Pitfall 4: Not Handling Token Refresh
**What goes wrong:** Access tokens expire after 24 hours, API calls fail with 401
**Why it happens:** Developer assumes tokens never expire
**Consequences:**
- Integration breaks daily
- Admin must manually reconnect daily
- Poor user experience
**Prevention:**
```go
// Use oauth2.Config.Client() which auto-refreshes
client := oauth2Config.Client(ctx, token)
resp, err := client.Do(req) // Automatically refreshes if expired
```
**Detection:** 401 errors after 24 hours, repeated OAuth reconnections

## Moderate Pitfalls

### Pitfall 5: No Retry Logic for Transient Errors
**What goes wrong:** Network blips cause permanent failures
**Why it happens:** Developer doesn't distinguish transient vs permanent errors
**Consequences:**
- Data loss from network hiccups
- Manual intervention required for every failure
**Prevention:**
```go
func isRetryableError(err error) bool {
    if httpErr, ok := err.(*HTTPError); ok {
        switch httpErr.StatusCode {
        case 429, 500, 502, 503, 504:
            return true // Retryable
        case 400, 401, 403, 404:
            return false // Permanent
        }
    }
    return false
}

// Retry with exponential backoff
for attempt := 0; attempt < maxRetries; attempt++ {
    err := sendBatch(payload)
    if err == nil || !isRetryableError(err) {
        return err
    }
    time.Sleep(backoff * time.Duration(1<<attempt))
}
```
**Detection:** High failure rate for batches that succeed on manual retry

### Pitfall 6: Hardcoding Salesforce Endpoints
**What goes wrong:** Can't switch between sandbox and production
**Why it happens:** Quick implementation without configuration
**Consequences:**
- Must redeploy code to test in sandbox
- Can't support multiple Salesforce environments
**Prevention:**
```go
authURL := os.Getenv("SALESFORCE_AUTH_URL")
if authURL == "" {
    authURL = "https://login.salesforce.com/services/oauth2/authorize"
}

// .env.sandbox
SALESFORCE_AUTH_URL=https://test.salesforce.com/services/oauth2/authorize

// .env.production
SALESFORCE_AUTH_URL=https://login.salesforce.com/services/oauth2/authorize
```
**Detection:** String literals "login.salesforce.com" in code

### Pitfall 7: Logging Sensitive Data
**What goes wrong:** Tokens leaked in application logs
**Why it happens:** Overly verbose debug logging
**Consequences:**
- Security breach
- Compliance violation
- Tokens accessible to support staff
**Prevention:**
```go
// Bad
log.Printf("Token: %s", token.AccessToken)

// Good
log.Printf("Token retrieved for orgID: %s", orgID)

// Best (structured logging with redaction)
logger.Info("OAuth token retrieved",
    zap.String("orgID", orgID),
    zap.Bool("hasRefreshToken", token.RefreshToken != ""),
)
```
**Detection:** Grep logs for "Bearer " or long alphanumeric strings

## Minor Pitfalls

### Pitfall 8: Not Validating State Token (CSRF)
**What goes wrong:** CSRF attacks during OAuth callback
**Why it happens:** Developer skips state validation
**Consequences:**
- Attacker tricks admin into connecting attacker's Salesforce org
- Data exfiltration
**Prevention:**
```go
// Generate state token
state := base64.URLEncoding.EncodeToString(randomBytes(32))
cache.Set("oauth_state:"+state, orgID, 10*time.Minute)

// Validate on callback
orgID, ok := cache.Get("oauth_state:" + state)
if !ok {
    return errors.New("invalid state token")
}
cache.Delete("oauth_state:" + state) // One-time use
```
**Detection:** OAuth callback doesn't check state parameter

### Pitfall 9: Ignoring Salesforce API Version Changes
**What goes wrong:** Breaking changes in new Salesforce API versions
**Why it happens:** Hardcoded `/services/data/v60.0/` in URLs
**Consequences:**
- Deprecated endpoints removed in v61+
- Integration breaks on Salesforce upgrade
**Prevention:**
```go
apiVersion := os.Getenv("SALESFORCE_API_VERSION")
if apiVersion == "" {
    apiVersion = "v60.0" // Default
}
url := fmt.Sprintf("https://%s/services/data/%s/sobjects/Contact", instance, apiVersion)
```
**Detection:** Hardcoded version numbers in API URLs

### Pitfall 10: Not Monitoring API Usage
**What goes wrong:** Hit rate limits without warning
**Why it happens:** No tracking of API call counts
**Consequences:**
- 429 errors during peak usage
- Batch deliveries fail unpredictably
**Prevention:**
```go
// Parse Sforce-Limit-Info header
limitInfo := resp.Header.Get("Sforce-Limit-Info")
// Example: "api-usage=123/100000"
parts := strings.Split(limitInfo, "=")
usage := strings.Split(parts[1], "/")
used, _ := strconv.Atoi(usage[0])
limit, _ := strconv.Atoi(usage[1])

// Warn at 80% usage
if float64(used)/float64(limit) > 0.8 {
    log.Warn("Approaching API limit", zap.Int("used", used), zap.Int("limit", limit))
}
```
**Detection:** 429 errors without prior warning

## Phase-Specific Warnings

| Phase | Likely Pitfall | Mitigation |
|-------|----------------|------------|
| Phase 1: OAuth Setup | Not validating state token | Use secure random state, cache with TTL |
| Phase 1: OAuth Setup | Hardcoding sandbox vs production URLs | Use environment variables |
| Phase 2: Rate Limiting | Global rate limiter | Per-org sync.Map of limiters |
| Phase 2: Error Handling | No retry for transient errors | Exponential backoff with isRetryableError |
| Phase 3: Token Storage | Plain text storage | AES-256-GCM encryption |
| Phase 3: Token Refresh | Manual refresh only | Use oauth2.Config.Client() auto-refresh |
| Phase 4: Admin UI | Logging tokens in UI | Redact sensitive fields |

## Testing Checklist

**Before considering Phase complete:**
- [ ] Tokens encrypted at rest (check database)
- [ ] Rate limiter is per-org (test with multiple orgs)
- [ ] Token auto-refresh works (wait 24 hours or mock expiry)
- [ ] Retry logic handles 429 (mock 429 response)
- [ ] State token validated on OAuth callback (test CSRF)
- [ ] No tokens in logs (grep logs for "Bearer")
- [ ] Sandbox and production URLs configurable (test with env vars)
- [ ] API usage monitored (check Sforce-Limit-Info parsing)

## Sources

**HIGH Confidence:**
- [Salesforce OAuth 2.0 Security Best Practices](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_web_server_flow.htm&type=5)
- [golang.org/x/oauth2 Token Refresh](https://pkg.go.dev/golang.org/x/oauth2)
- [AES-GCM Encryption in Go](https://karbhawono.medium.com/encryption-using-aes-gcm-b981bf4890f3)

**MEDIUM Confidence:**
- [Rate Limiting in Go](https://oneuptime.com/blog/post/2026-01-23-go-rate-limiting/view)
- [Go Salesforce OAuth JWT Implementation](https://bsathish-civ.medium.com/authenticate-salesforce-from-golang-using-connected-app-jwt-bearer-token-flow-bc469b016940)
