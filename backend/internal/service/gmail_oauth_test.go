package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// ============================================================
// Minimal in-memory DB for connection status test
// ============================================================

// fakeEngagementDB is an in-memory SQLite for testing GetConnectionStatus
func newFakeEngagementDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE gmail_oauth_tokens (
			id                      TEXT PRIMARY KEY,
			org_id                  TEXT NOT NULL,
			user_id                 TEXT NOT NULL,
			access_token_encrypted  BLOB,
			refresh_token_encrypted BLOB,
			token_expiry            TEXT,
			gmail_address           TEXT NOT NULL DEFAULT '',
			dns_spf_valid           INTEGER NOT NULL DEFAULT 0,
			dns_dkim_valid          INTEGER NOT NULL DEFAULT 0,
			dns_dmarc_valid         INTEGER NOT NULL DEFAULT 0,
			connected_at            TEXT,
			created_at              TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at              TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, user_id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	return db
}

// ============================================================
// TestGmailOAuthStateEncodeDecode
// ============================================================

func TestGmailOAuthStateEncodeDecode(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)
	svc := NewGmailOAuthService(r, make([]byte, 32))

	orgID := "org-abc-123"
	userID := "user-xyz-456"

	// Use the exported helper to encode state
	state, err := svc.encodeState(orgID, userID)
	if err != nil {
		t.Fatalf("encodeState returned error: %v", err)
	}
	if state == "" {
		t.Fatal("encodeState returned empty state")
	}

	// Decode it back
	gotOrg, gotUser, err := svc.decodeState(state)
	if err != nil {
		t.Fatalf("decodeState returned error: %v", err)
	}
	if gotOrg != orgID {
		t.Errorf("decodeState orgID: want %q, got %q", orgID, gotOrg)
	}
	if gotUser != userID {
		t.Errorf("decodeState userID: want %q, got %q", userID, gotUser)
	}
}

// ============================================================
// TestGmailOAuthDNSValidation — gmail.com is a well-known domain
// SPF and DMARC records exist; DKIM is checked via google._domainkey
// ============================================================

func TestGmailOAuthDNSValidation(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)
	svc := NewGmailOAuthService(r, make([]byte, 32))

	ctx := context.Background()
	spf, dkim, dmarc := svc.validateDNS(ctx, "test@gmail.com")

	// gmail.com is guaranteed to have SPF and DMARC (global infrastructure)
	if spf != 1 {
		t.Errorf("expected SPF=1 for gmail.com, got %d", spf)
	}
	if dmarc != 1 {
		t.Errorf("expected DMARC=1 for gmail.com, got %d", dmarc)
	}
	// DKIM result may be 0 or 1 (selector resolution varies) — just check no panic
	_ = dkim
}

// ============================================================
// TestGmailOAuthDNSValidationUnknownDomain — nonexistent domain returns all zeros
// ============================================================

func TestGmailOAuthDNSValidationUnknownDomain(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)
	svc := NewGmailOAuthService(r, make([]byte, 32))

	ctx := context.Background()
	// Use a clearly invalid domain that will never resolve
	spf, dkim, dmarc := svc.validateDNS(ctx, "test@this-domain-does-not-exist-xyz-q7z.invalid")

	if spf != 0 {
		t.Errorf("expected SPF=0 for invalid domain, got %d", spf)
	}
	if dkim != 0 {
		t.Errorf("expected DKIM=0 for invalid domain, got %d", dkim)
	}
	if dmarc != 0 {
		t.Errorf("expected DMARC=0 for invalid domain, got %d", dmarc)
	}
}

// ============================================================
// TestGmailOAuthConnectionStatusNotConnected
// ============================================================

func TestGmailOAuthConnectionStatusNotConnected(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)
	svc := NewGmailOAuthService(r, make([]byte, 32))

	ctx := context.Background()
	status, err := svc.GetConnectionStatus(ctx, "org-123", "user-456")
	if err != nil {
		t.Fatalf("GetConnectionStatus error: %v", err)
	}

	// No token in DB → should return a status with connected=false
	if status.Connected {
		t.Errorf("expected Connected=false when no token in DB, got true")
	}
}

// ============================================================
// Helpers: verify base64 state encoding format
// ============================================================

func TestGmailOAuthStateEncodingFormat(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)
	svc := NewGmailOAuthService(r, make([]byte, 32))

	state, err := svc.encodeState("org1", "user1")
	if err != nil {
		t.Fatal(err)
	}

	// State must be valid base64
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		t.Fatalf("state is not valid base64: %v", err)
	}

	// Inner format: "orgID:userID:randomBytes"
	inner := string(decoded)
	parts := strings.SplitN(inner, ":", 3)
	if len(parts) != 3 {
		t.Fatalf("inner state has %d parts (want 3): %q", len(parts), inner)
	}
	if parts[0] != "org1" {
		t.Errorf("part[0] (orgID): want %q, got %q", "org1", parts[0])
	}
	if parts[1] != "user1" {
		t.Errorf("part[1] (userID): want %q, got %q", "user1", parts[1])
	}
}

// ============================================================
// Upsert + Get round-trip test
// ============================================================

func TestGmailOAuthTokenUpsertAndGet(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)

	now := time.Now().UTC().Truncate(time.Second)
	tok := &entity.GmailOAuthToken{
		ID:           "tok-1",
		OrgID:        "org-1",
		UserID:       "user-1",
		GmailAddress: "alice@example.com",
		DNSSPFValid:  1,
		DNSDKIMValid: 0,
		DNSDMARCValid: 1,
		ConnectedAt:  &now,
	}
	if err := r.UpsertGmailOAuthToken(context.Background(), tok); err != nil {
		t.Fatalf("UpsertGmailOAuthToken: %v", err)
	}

	got, err := r.GetGmailOAuthToken(context.Background(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("GetGmailOAuthToken: %v", err)
	}
	if got.GmailAddress != "alice@example.com" {
		t.Errorf("GmailAddress: want %q, got %q", "alice@example.com", got.GmailAddress)
	}
	if got.DNSSPFValid != 1 {
		t.Errorf("DNSSPFValid: want 1, got %d", got.DNSSPFValid)
	}
}

// ============================================================
// Delete test
// ============================================================

func TestGmailOAuthTokenDelete(t *testing.T) {
	db := newFakeEngagementDB(t)
	defer db.Close()

	r := repo.NewEngagementRepo(db)

	tok := &entity.GmailOAuthToken{
		ID:           "tok-2",
		OrgID:        "org-2",
		UserID:       "user-2",
		GmailAddress: "bob@example.com",
	}
	if err := r.UpsertGmailOAuthToken(context.Background(), tok); err != nil {
		t.Fatalf("UpsertGmailOAuthToken: %v", err)
	}

	if err := r.DeleteGmailOAuthToken(context.Background(), "org-2", "user-2"); err != nil {
		t.Fatalf("DeleteGmailOAuthToken: %v", err)
	}

	got, err := r.GetGmailOAuthToken(context.Background(), "org-2", "user-2")
	if err != nil {
		t.Fatalf("GetGmailOAuthToken after delete: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}
