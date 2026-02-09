# Technology Stack: Salesforce Integration

**Project:** Quantico CRM - Salesforce Integration
**Researched:** 2026-02-09
**Confidence:** MEDIUM

## Recommended Stack

### OAuth 2.0 Authentication

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `golang.org/x/oauth2` | v0.35.0+ | OAuth 2.0 flows | Official Go OAuth library, supports all standard flows, automatic token refresh, widely adopted (13K+ imports) |
| `crypto/aes` (stdlib) | Go 1.22+ | Token encryption at rest | Standard library AES-GCM for authenticated encryption, no dependencies |

**Rationale for golang.org/x/oauth2:**
- Official Google-maintained package
- Supports all OAuth 2.0 flows (authorization code, client credentials, password, device)
- Automatic token refresh built-in
- PKCE support (RFC 7636) for enhanced security
- No Salesforce-specific package needed - configure endpoints manually
- Battle-tested with 13,352 packages importing it

**Configuration pattern:**
```go
conf := &oauth2.Config{
    ClientID:     "YOUR_CLIENT_ID",
    ClientSecret: "YOUR_CLIENT_SECRET",
    Endpoint: oauth2.Endpoint{
        AuthURL:  "https://login.salesforce.com/services/oauth2/authorize",
        TokenURL: "https://login.salesforce.com/services/oauth2/token",
    },
    RedirectURL: "https://yourapp.com/oauth/callback",
    Scopes:      []string{"api", "refresh_token"},
}
```

### Salesforce REST API Integration

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http` (stdlib) | Go 1.22+ | HTTP client | Direct REST API calls, full control, no abstraction overhead |
| Alternative: `github.com/simpleforce/simpleforce` | No releases | Optional wrapper | Provides convenience methods, but no stable releases, limited maintenance |

**Recommendation: Use net/http directly**

**Pros:**
- Full control over request/response handling
- No dependency on unmaintained third-party libraries
- Integrates seamlessly with `golang.org/x/oauth2.Transport`
- Easier to debug and customize
- No version lock-in

**Cons of wrapper libraries:**
- `go-force` (nimajalali): Last release August 2020, 18 open issues, appears unmaintained
- `simpleforce`: No releases published, maintenance status unclear
- All wrappers add abstraction that may not fit Quantico's multi-tenant model

**Integration pattern:**
```go
client := conf.Client(ctx, token)
req, _ := http.NewRequest("GET", "https://yourinstance.salesforce.com/services/data/v60.0/sobjects/Contact", nil)
resp, err := client.Do(req)
```

### Rate Limiting

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `golang.org/x/time/rate` | v0.14.0+ | Token bucket rate limiter | Official Go package, supports per-resource limiting, context-aware |

**Rationale:**
- Official `x/time/rate` package from Go team
- Token bucket algorithm with burst support
- Per-org rate limiting via `sync.Map[orgID]*rate.Limiter`
- Context-aware blocking (`Wait(ctx)`) for graceful timeout
- 13,352 packages already use it

**Alternative considered: uber-go/ratelimit**
- Simpler API but less flexible
- No burst control
- No context support
- Good for single global limiter, not per-org limiting

**Multi-tenant rate limiting pattern:**
```go
type RateLimiterManager struct {
    limiters sync.Map // map[orgID]*rate.Limiter
}

func (m *RateLimiterManager) GetLimiter(orgID string) *rate.Limiter {
    if limiter, ok := m.limiters.Load(orgID); ok {
        return limiter.(*rate.Limiter)
    }

    // Salesforce limits: 100K/24hr = ~1.15 req/sec
    // Use 1 req/sec with burst of 5 to be conservative
    limiter := rate.NewLimiter(rate.Limit(1.0), 5)
    m.limiters.Store(orgID, limiter)
    return limiter
}

// In handler
limiter := rateLimitMgr.GetLimiter(orgID)
if err := limiter.Wait(c.Context()); err != nil {
    return c.Status(429).JSON(fiber.Map{"error": "rate limit exceeded"})
}
```

### JWT Handling

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/golang-jwt/jwt/v5` | v5.x | JWT signing (if using JWT bearer flow) | Most popular JWT library, supports RS256 required by Salesforce |

**When needed:**
- Server-to-server integration (no user interaction)
- Salesforce OAuth 2.0 JWT Bearer Flow

**Standard OAuth flow (recommended for Quantico):**
- User-facing OAuth → Use `golang.org/x/oauth2` authorization code flow
- No custom JWT signing needed
- Access tokens are opaque strings from Salesforce

**If JWT bearer flow is needed later:**
```go
import "github.com/golang-jwt/jwt/v5"

// Sign JWT with RSA private key
token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
    "iss": clientID,
    "sub": username,
    "aud": "https://login.salesforce.com",
    "exp": time.Now().Add(5 * time.Minute).Unix(),
})
signedToken, _ := token.SignedString(privateKey)
```

### Token Storage

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Turso (SQLite) | Existing | Encrypted token persistence | Already in stack, per-org isolation, encryption at rest |
| `crypto/aes` (stdlib) | Go 1.22+ | Token encryption | AES-256-GCM for authenticated encryption |

**Schema:**
```sql
CREATE TABLE salesforce_connections (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    instance_url TEXT NOT NULL,
    access_token_encrypted BLOB NOT NULL,  -- AES-GCM encrypted
    refresh_token_encrypted BLOB NOT NULL, -- AES-GCM encrypted
    token_type TEXT DEFAULT 'Bearer',
    expires_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id)
);
CREATE INDEX idx_sf_conn_org ON salesforce_connections(org_id);
```

**Encryption pattern (AES-256-GCM):**
```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
)

func encryptToken(plaintext string, key []byte) ([]byte, error) {
    block, _ := aes.NewCipher(key) // 32 bytes for AES-256
    gcm, _ := cipher.NewGCM(block)

    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)

    // Nonce prepended to ciphertext
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return ciphertext, nil
}

func decryptToken(ciphertext []byte, key []byte) (string, error) {
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)

    nonceSize := gcm.NonceSize()
    nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]

    plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
    return string(plaintext), err
}
```

**Key management:**
- Store encryption key in environment variable `SALESFORCE_TOKEN_ENCRYPTION_KEY` (32 bytes base64)
- Use same key across all app instances for multi-instance deployments
- DO NOT store encryption key in database or git
- Consider HashiCorp Vault or AWS KMS for production key management

### Testing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http/httptest` (stdlib) | Go 1.22+ | Mock HTTP server | Standard library, no dependencies, perfect for API mocking |
| `github.com/stretchr/testify` | v1.9.x | Assertions | Already in use, provides `assert` and `mock` packages |

**Testing pattern:**
```go
func TestSalesforceAPI(t *testing.T) {
    // Mock Salesforce server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

        // Mock response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id": "003xx000000ABC",
            "Name": "Test Contact",
        })
    }))
    defer server.Close()

    // Test with mocked server
    client := &http.Client{}
    req, _ := http.NewRequest("GET", server.URL+"/services/data/v60.0/sobjects/Contact/003xx000000ABC", nil)
    req.Header.Set("Authorization", "Bearer test-token")
    resp, err := client.Do(req)

    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

**Alternative considered: github.com/jarcoal/httpmock**
- More features than httptest
- But adds dependency
- httptest is sufficient for REST API mocking

### Frontend (SvelteKit)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `@auth/sveltekit` | Latest (experimental) | OAuth flow UI | Built-in Salesforce provider, handles callback routing |
| Alternative: Custom implementation | - | Full control | Using SvelteKit form actions for OAuth initiation |

**Recommendation: Custom implementation using SvelteKit patterns**

**Why:**
- `@auth/sveltekit` is experimental, API may change
- Quantico already has auth patterns with Fiber backend
- OAuth flow is simple: redirect → callback → token exchange
- Full control over multi-tenant org mapping

**Frontend OAuth flow:**
```svelte
<!-- /routes/admin/integrations/salesforce/+page.svelte -->
<script>
async function connectSalesforce() {
    const response = await fetch('/api/v1/salesforce/oauth/authorize', {
        method: 'POST',
    });
    const { authUrl } = await response.json();
    window.location.href = authUrl; // Redirect to Salesforce
}
</script>

<button on:click={connectSalesforce}>
    Connect Salesforce
</button>
```

**Backend handler:**
```go
// Generate OAuth URL
func (h *SalesforceHandler) InitiateOAuth(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)

    state := generateState(orgID) // CSRF protection

    authURL := oauth2Config.AuthCodeURL(state,
        oauth2.AccessTypeOffline, // Get refresh token
        oauth2.ApprovalForce,     // Force approval prompt
    )

    return c.JSON(fiber.Map{"authUrl": authURL})
}

// OAuth callback
func (h *SalesforceHandler) OAuthCallback(c *fiber.Ctx) error {
    code := c.Query("code")
    state := c.Query("state")

    orgID := validateState(state)

    token, err := oauth2Config.Exchange(c.Context(), code)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "token exchange failed"})
    }

    // Encrypt and store tokens
    h.tokenService.StoreToken(orgID, token)

    // Redirect to success page
    return c.Redirect("/admin/integrations/salesforce?status=connected")
}
```

## Alternatives Considered

### Third-Party OAuth Providers (Nango, Paragon)

**Nango:**
- Pros: Pre-built integrations, handles token refresh, rate limiting, 68+ providers
- Cons: Adds external dependency, costs scale with API usage, less control
- Cost: Starts free, scales with millions of API requests

**Paragon:**
- Pros: Low-code integration builder, embedded workflow UI
- Cons: Expensive, bottlenecks at 10M requests/month, limited for technical teams

**Recommendation: Build directly**
- Salesforce OAuth is standard OAuth 2.0 (not complex)
- `golang.org/x/oauth2` handles 95% of complexity
- Quantico already has multi-tenant infrastructure
- Full control over rate limiting, encryption, error handling
- No per-API-request pricing
- Easier to debug and customize

## Installation

```bash
# Backend dependencies
cd backend
go get golang.org/x/oauth2@latest
go get golang.org/x/time/rate@latest
go get github.com/golang-jwt/jwt/v5@latest  # Only if JWT bearer flow needed

# Frontend (no additional dependencies for custom OAuth)
# If using @auth/sveltekit (not recommended):
# cd frontend
# npm install @auth/sveltekit
```

## Integration Points

### 1. Fiber Middleware (Rate Limiting)
```go
// internal/middleware/salesforce_ratelimit.go
func SalesforceRateLimiter(limitMgr *RateLimiterManager) fiber.Handler {
    return func(c *fiber.Ctx) error {
        orgID := c.Locals("orgID").(string)
        limiter := limitMgr.GetLimiter(orgID)

        if err := limiter.Wait(c.Context()); err != nil {
            return c.Status(429).JSON(fiber.Map{"error": "rate limit exceeded"})
        }

        return c.Next()
    }
}
```

### 2. Token Service (Encryption)
```go
// internal/service/salesforce_token.go
type SalesforceTokenService struct {
    repo          *repository.SalesforceRepo
    encryptionKey []byte
}

func (s *SalesforceTokenService) StoreToken(orgID string, token *oauth2.Token) error {
    accessEncrypted, _ := encryptToken(token.AccessToken, s.encryptionKey)
    refreshEncrypted, _ := encryptToken(token.RefreshToken, s.encryptionKey)

    return s.repo.UpsertConnection(orgID, accessEncrypted, refreshEncrypted, token.Expiry)
}

func (s *SalesforceTokenService) GetToken(orgID string) (*oauth2.Token, error) {
    conn, _ := s.repo.GetConnection(orgID)

    accessToken, _ := decryptToken(conn.AccessTokenEncrypted, s.encryptionKey)
    refreshToken, _ := decryptToken(conn.RefreshTokenEncrypted, s.encryptionKey)

    return &oauth2.Token{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        Expiry:       conn.ExpiresAt,
        TokenType:    "Bearer",
    }, nil
}
```

### 3. HTTP Client Factory
```go
// internal/salesforce/client.go
type ClientFactory struct {
    tokenService *SalesforceTokenService
    oauth2Config *oauth2.Config
}

func (f *ClientFactory) GetClient(ctx context.Context, orgID string) (*http.Client, error) {
    token, err := f.tokenService.GetToken(orgID)
    if err != nil {
        return nil, err
    }

    // oauth2.Config.Client auto-refreshes expired tokens
    return f.oauth2Config.Client(ctx, token), nil
}
```

### 4. Database Schema
```sql
-- migrations/XXX_salesforce_integration.sql

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

CREATE INDEX idx_sf_conn_org ON salesforce_connections(org_id);

-- Store Salesforce field mappings per org
CREATE TABLE salesforce_field_mappings (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL, -- Contact, Account, etc.
    quantico_field TEXT NOT NULL,
    salesforce_object TEXT NOT NULL,
    salesforce_field TEXT NOT NULL,
    sync_direction TEXT DEFAULT 'bidirectional', -- bidirectional, to_salesforce, from_salesforce
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, entity_type, quantico_field)
);

CREATE INDEX idx_sf_mapping_org_entity ON salesforce_field_mappings(org_id, entity_type);
```

## Environment Variables

```bash
# .env (backend)
SALESFORCE_CLIENT_ID=your_connected_app_client_id
SALESFORCE_CLIENT_SECRET=your_connected_app_client_secret
SALESFORCE_REDIRECT_URL=https://quanticocrm.com/api/v1/salesforce/oauth/callback
SALESFORCE_TOKEN_ENCRYPTION_KEY=base64_encoded_32_byte_key

# For sandbox testing
SALESFORCE_AUTH_URL=https://test.salesforce.com/services/oauth2/authorize
SALESFORCE_TOKEN_URL=https://test.salesforce.com/services/oauth2/token

# For production
# SALESFORCE_AUTH_URL=https://login.salesforce.com/services/oauth2/authorize
# SALESFORCE_TOKEN_URL=https://login.salesforce.com/services/oauth2/token
```

## Salesforce API Rate Limits

**Per org limits (Enterprise Edition):**
- Base: 100,000 API calls per 24 hours
- Additional calls based on user licenses
- Monitor via `Sforce-Limit-Info` header in responses
- Check programmatically: `GET /services/data/v60.0/limits`

**Rate limiting strategy:**
- Conservative: 1 request/second per org (86,400/day)
- Burst: Allow 5 concurrent requests
- Adjust based on org's actual Salesforce limit
- Store org-specific limits in database

**Response headers to monitor:**
```
Sforce-Limit-Info: api-usage=123/100000
```

## Version Constraints

```go
// go.mod
require (
    golang.org/x/oauth2 v0.35.0 // or latest
    golang.org/x/time v0.14.0   // or latest
    github.com/golang-jwt/jwt/v5 v5.2.0 // if JWT bearer flow needed
)
```

## Sources

**HIGH Confidence:**
- [golang.org/x/oauth2 package](https://pkg.go.dev/golang.org/x/oauth2) - Official OAuth2 library
- [golang.org/x/time/rate package](https://pkg.go.dev/golang.org/x/time/rate) - Official rate limiting
- [Salesforce OAuth 2.0 JWT Bearer Flow](https://help.salesforce.com/s/articleView?id=xcloud.remoteaccess_oauth_jwt_flow.htm&language=en_US&type=5) - Official Salesforce docs
- [Salesforce API Limits](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_limits.htm) - Official limits documentation

**MEDIUM Confidence:**
- [Authenticate Salesforce from Golang using JWT bearer token](https://bsathish-civ.medium.com/authenticate-salesforce-from-golang-using-connected-app-jwt-bearer-token-flow-bc469b016940)
- [Nango vs Paragon comparison](https://nango.dev/paragon-vs-nango)
- [AES-GCM encryption in Go](https://karbhawono.medium.com/encryption-using-aes-gcm-b981bf4890f3)
- [Testing external API calls in Go](https://liza.io/testing-external-api-calls-in-go/)

**LOW Confidence:**
- Go Salesforce SDK comparisons - most libraries are unmaintained or lack releases
- @auth/sveltekit Salesforce integration - marked experimental, API may change
