# Stack Research Summary: Salesforce Integration

**Project:** Quantico CRM - Salesforce OAuth & API Integration
**Researched:** 2026-02-09
**Overall Confidence:** MEDIUM

## Executive Summary

The Salesforce integration stack for Quantico requires OAuth 2.0 authentication, rate-limited REST API calls, encrypted token storage, and robust error handling. After researching available Go libraries, third-party OAuth providers, and Salesforce-specific SDKs, the recommended approach is to build directly on standard libraries rather than using unmaintained wrappers or expensive third-party services.

**Core Stack Decision:** Use `golang.org/x/oauth2` for OAuth flows, `net/http` stdlib for API calls, `golang.org/x/time/rate` for rate limiting, and `crypto/aes` for token encryption. Avoid Salesforce-specific Go SDKs (all unmaintained) and third-party OAuth providers like Nango/Paragon (adds cost and complexity for standard OAuth).

## Key Findings

### 1. OAuth Library: golang.org/x/oauth2 (HIGH Confidence)

**Recommendation:** Use official `golang.org/x/oauth2` package

**Why:**
- Official Google-maintained library (v0.35.0+)
- Supports all OAuth 2.0 flows (authorization code, refresh token, PKCE)
- Automatic token refresh built-in via `oauth2.Config.Client()`
- 13,352 packages already use it (battle-tested)
- No Salesforce-specific package needed - just configure endpoints

**Integration:**
```go
conf := &oauth2.Config{
    ClientID:     os.Getenv("SALESFORCE_CLIENT_ID"),
    ClientSecret: os.Getenv("SALESFORCE_CLIENT_SECRET"),
    Endpoint: oauth2.Endpoint{
        AuthURL:  "https://login.salesforce.com/services/oauth2/authorize",
        TokenURL: "https://login.salesforce.com/services/oauth2/token",
    },
    RedirectURL: "https://quanticocrm.com/api/v1/salesforce/oauth/callback",
    Scopes:      []string{"api", "refresh_token"},
}
```

**Alternatives Considered:**
- Third-party OAuth providers (Nango, Paragon): Rejected due to cost scaling with API usage, external dependency, less control
- Custom OAuth implementation: Rejected due to security complexity, reinventing wheel

### 2. Salesforce API Integration: net/http stdlib (MEDIUM Confidence)

**Recommendation:** Use Go stdlib `net/http` directly, no wrapper library

**Why:**
- Full control over request/response handling
- Integrates seamlessly with `golang.org/x/oauth2.Transport`
- All Go Salesforce SDKs are unmaintained or lack releases:
  - `go-force` (nimajalali): Last release Aug 2020, 18 open issues
  - `simpleforce`: No releases published, maintenance unclear
- Wrappers add abstraction that doesn't fit Quantico's multi-tenant model

**Integration:**
```go
client := oauth2Config.Client(ctx, token) // Auto-refreshes tokens
req, _ := http.NewRequest("POST", "https://instance.salesforce.com/services/data/v60.0/sobjects/QuanticoMergeInstruction__c", payload)
resp, err := client.Do(req)
```

**Alternatives Considered:**
- `github.com/nimajalali/go-force`: 121 stars, v1.0.0 from Aug 2020, unmaintained
- `github.com/simpleforce/simpleforce`: No releases, unclear maintenance
- Both rejected due to lack of active maintenance and abstraction overhead

### 3. Rate Limiting: golang.org/x/time/rate (HIGH Confidence)

**Recommendation:** Use official `golang.org/x/time/rate` package with per-org limiters

**Why:**
- Official Go team package (v0.14.0+)
- Token bucket algorithm with burst support
- Context-aware blocking (`Wait(ctx)`) for graceful timeouts
- Per-org rate limiting via `sync.Map[orgID]*rate.Limiter`
- 13,352 packages use it

**Integration:**
```go
type RateLimiterManager struct {
    limiters sync.Map
}

func (m *RateLimiterManager) GetLimiter(orgID string) *rate.Limiter {
    if limiter, ok := m.limiters.Load(orgID); ok {
        return limiter.(*rate.Limiter)
    }
    // Salesforce: 100K/24hr = ~1.15 req/sec, use 1 req/sec with burst 5
    limiter := rate.NewLimiter(rate.Limit(1.0), 5)
    m.limiters.Store(orgID, limiter)
    return limiter
}
```

**Salesforce Rate Limits:**
- Enterprise Edition: 100,000 API calls per 24 hours base
- Additional calls based on user licenses
- Monitor via `Sforce-Limit-Info` header
- Check via `/services/data/v60.0/limits` endpoint

**Alternatives Considered:**
- `uber-go/ratelimit`: Simpler but lacks burst control, context support, per-resource limiting

### 4. Token Storage: Turso + crypto/aes (HIGH Confidence)

**Recommendation:** Store encrypted tokens in existing Turso database using AES-256-GCM

**Why:**
- Turso already in stack (per-org isolation)
- AES-256-GCM provides authenticated encryption (stdlib `crypto/aes`)
- Compliance requirement (SOX, GDPR mandate encryption at rest)
- No additional dependencies

**Schema:**
```sql
CREATE TABLE salesforce_connections (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    instance_url TEXT NOT NULL,
    access_token_encrypted BLOB NOT NULL,
    refresh_token_encrypted BLOB NOT NULL,
    token_type TEXT DEFAULT 'Bearer',
    expires_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id)
);
```

**Encryption Pattern:**
```go
// AES-256-GCM with nonce prepended to ciphertext
func encryptToken(plaintext string, key []byte) ([]byte, error) {
    block, _ := aes.NewCipher(key) // 32 bytes for AES-256
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}
```

**Key Management:**
- Store key in env var `SALESFORCE_TOKEN_ENCRYPTION_KEY` (32 bytes base64)
- Use same key across all instances (multi-instance deployments)
- Consider HashiCorp Vault or AWS KMS for production

**Alternatives Considered:**
- Plain text storage: Rejected (security nightmare, compliance violation)
- Redis for tokens: Rejected (adds dependency, Turso sufficient for current scale)

### 5. JWT Handling: Optional for v4.0 (MEDIUM Confidence)

**Recommendation:** Use standard OAuth authorization code flow, no JWT bearer flow needed

**Why:**
- User-facing OAuth flow (authorization code) doesn't require JWT signing
- JWT bearer flow is for server-to-server integration (no user interaction)
- v4.0 requires user authorization in Salesforce (per spec)

**If needed later:**
```go
// github.com/golang-jwt/jwt/v5 for RS256 signing
token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
    "iss": clientID,
    "sub": username,
    "aud": "https://login.salesforce.com",
    "exp": time.Now().Add(5 * time.Minute).Unix(),
})
signedToken, _ := token.SignedString(privateKey)
```

### 6. Testing: net/http/httptest + testify (HIGH Confidence)

**Recommendation:** Use stdlib `httptest` for mocking Salesforce API, `testify` for assertions

**Why:**
- `httptest` is stdlib, no dependencies, perfect for REST API mocking
- `testify` already in Quantico stack (provides `assert` and `mock`)
- Simple, effective, widely used pattern

**Pattern:**
```go
func TestSalesforceAPI(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id": "003xx000000ABC",
        })
    }))
    defer server.Close()
    // Test with mocked server
}
```

**Alternatives Considered:**
- `github.com/jarcoal/httpmock`: More features but adds dependency, httptest sufficient

### 7. Frontend: Custom SvelteKit OAuth (MEDIUM Confidence)

**Recommendation:** Custom OAuth implementation using SvelteKit patterns, not @auth/sveltekit

**Why:**
- `@auth/sveltekit` is experimental, API may change
- Quantico already has auth patterns with Fiber backend
- OAuth flow is simple: redirect → callback → token exchange
- Full control over multi-tenant org mapping

**Frontend Pattern:**
```svelte
<!-- Admin initiates OAuth -->
<script>
async function connectSalesforce() {
    const { authUrl } = await fetch('/api/v1/salesforce/oauth/authorize', {
        method: 'POST',
    }).then(r => r.json());
    window.location.href = authUrl;
}
</script>
```

**Backend Handlers:**
- `POST /api/v1/salesforce/oauth/authorize` → Generate OAuth URL with state token
- `GET /api/v1/salesforce/oauth/callback?code=XXX&state=YYY` → Exchange code for tokens, redirect to success page

**Alternatives Considered:**
- `@auth/sveltekit`: Experimental, built-in Salesforce provider exists but API unstable

## Implications for Roadmap

### Dependencies to Install

```bash
# Backend
go get golang.org/x/oauth2@latest
go get golang.org/x/time/rate@latest
# (Optional) go get github.com/golang-jwt/jwt/v5@latest

# Frontend - no additional dependencies for custom OAuth
```

### Environment Variables Required

```bash
SALESFORCE_CLIENT_ID=your_connected_app_client_id
SALESFORCE_CLIENT_SECRET=your_connected_app_client_secret
SALESFORCE_REDIRECT_URL=https://quanticocrm.com/api/v1/salesforce/oauth/callback
SALESFORCE_TOKEN_ENCRYPTION_KEY=base64_encoded_32_byte_key

# Sandbox vs Production endpoints
SALESFORCE_AUTH_URL=https://login.salesforce.com/services/oauth2/authorize
SALESFORCE_TOKEN_URL=https://login.salesforce.com/services/oauth2/token
```

### Phase-Specific Implications

**Phase 1 (OAuth Setup):**
- Implement `OAuthService` using `golang.org/x/oauth2`
- Create Salesforce Connected App (manual setup in Salesforce org)
- Implement token encryption with `crypto/aes`
- Create `salesforce_connections` table migration

**Phase 2 (Rate Limiting & Error Handling):**
- Implement `RateLimiterManager` using `golang.org/x/time/rate`
- Add middleware for per-org rate limiting
- Implement exponential backoff retry logic
- Track API usage via `Sforce-Limit-Info` header

**Phase 3 (API Integration):**
- Implement `ClientFactory` for auto-refreshing HTTP clients
- Use `net/http` for Salesforce REST API calls
- Create `salesforce_batches` table for audit logging

**Phase 4 (Admin UI):**
- Custom SvelteKit OAuth flow (no additional deps)
- OAuth connection wizard
- Delivery queue UI
- API usage dashboard

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| OAuth Library | HIGH | Official golang.org/x/oauth2, well-documented, widely adopted |
| API Integration | MEDIUM | Net/http is correct choice, but lack of maintained Salesforce SDKs means more manual work |
| Rate Limiting | HIGH | golang.org/x/time/rate is official, proven solution |
| Token Storage | HIGH | AES-256-GCM is industry standard, crypto/aes is stdlib |
| JWT Handling | MEDIUM | Not needed for v4.0, but if needed later, golang-jwt/jwt is standard |
| Testing | HIGH | httptest is stdlib, testify already in use |
| Frontend | MEDIUM | Custom implementation is correct choice, but @auth/sveltekit experimental status introduces uncertainty |

## Gaps to Address

### Resolved During Research

- OAuth library selection (golang.org/x/oauth2 confirmed)
- Rate limiting approach (per-org limiters confirmed)
- Token encryption method (AES-256-GCM confirmed)
- Testing strategy (httptest + testify confirmed)

### Requiring Phase-Specific Research

**Phase 1:**
- Salesforce Connected App setup specifics (OAuth scopes, callback URL configuration)
- Token refresh timing (proactive vs reactive)

**Phase 2:**
- Optimal rate limit values per Salesforce edition (Enterprise, Unlimited, Developer)
- Retry backoff timing (5s, 10s, 20s validated)

**Phase 4:**
- SvelteKit OAuth flow UI/UX patterns (modal vs full page, error states)
- Real-time delivery queue updates (polling vs SSE)

### Open Questions

1. **Third-party OAuth provider adoption later?** If Salesforce integration proves complex, consider Nango for v4.1+
2. **Redis for distributed rate limiting?** At 1,000+ orgs, consider Redis for centralized API usage tracking
3. **HashiCorp Vault for key management?** Production deployment should use Vault/KMS instead of env vars

## Sources

**HIGH Confidence (Official Documentation):**
- [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
- [Salesforce OAuth 2.0 JWT Bearer Flow](https://help.salesforce.com/s/articleView?id=xcloud.remoteaccess_oauth_jwt_flow.htm&language=en_US&type=5)
- [Salesforce API Limits](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_limits.htm)

**MEDIUM Confidence (Verified Community Resources):**
- [Nango vs Paragon Comparison](https://nango.dev/paragon-vs-nango)
- [Go Salesforce OAuth JWT Implementation](https://bsathish-civ.medium.com/authenticate-salesforce-from-golang-using-connected-app-jwt-bearer-token-flow-bc469b016940)
- [AES-GCM Encryption in Go](https://karbhawono.medium.com/encryption-using-aes-gcm-b981bf4890f3)
- [Testing External API Calls in Go](https://liza.io/testing-external-api-calls-in-go/)
- [Rate Limiting in Go 2026](https://oneuptime.com/blog/post/2026-01-23-go-rate-limiting/view)

**LOW Confidence (Unmaintained Libraries):**
- [go-force](https://github.com/nimajalali/go-force) - Last release Aug 2020, not recommended
- [simpleforce](https://github.com/simpleforce/simpleforce) - No releases, not recommended

---

**Recommendation:** Proceed with recommended stack. All core libraries are official Go packages or stdlib, minimizing third-party dependency risk. Direct HTTP client approach provides full control needed for multi-tenant architecture.
