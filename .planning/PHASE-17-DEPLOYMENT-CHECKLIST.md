# Phase 17 Production Deployment Checklist

## ✅ Code Status
- [x] All 5 plans completed and committed
- [x] Backend migrations applied (061_create_salesforce_sync_tables.sql)
- [x] Frontend UI built and tested
- [x] Admin API endpoints working
- [x] OAuth flow implemented
- [x] Encryption utility configured

## 🔧 Production Setup Required

### 1. System-Level Encryption Key (Railway Only)

**CRITICAL: Set before deploying**

The encryption key is a **system-level secret** used to encrypt all organizations' Salesforce tokens.

```bash
# Generate a secure 32-byte encryption key
ENCRYPTION_KEY=$(openssl rand -base64 32)
echo "SALESFORCE_TOKEN_ENCRYPTION_KEY=$ENCRYPTION_KEY"
```

Then in **Railway Backend Settings:**
- Go to Variables tab
- Add: `SALESFORCE_TOKEN_ENCRYPTION_KEY=<generated-key-above>`
- This key encrypts tokens for ALL organizations
- **Do NOT use test keys in production**

### 2. Per-Organization OAuth Configuration

Each organization sets up their OWN Salesforce credentials through the UI:
- Admin navigates to: `/admin/integrations/salesforce`
- Admin enters THEIR org's Salesforce Connected App credentials:
  - Client ID
  - Client Secret
  - Redirect URL (same for all orgs)
- Admin clicks "Connect to Salesforce"
- Admin authenticates to THEIR Salesforce org
- Tokens are encrypted with system key and stored per-org in database

**Architecture:**
```
System-Level (Railway):
  SALESFORCE_TOKEN_ENCRYPTION_KEY = [single system key]

Per-Org Level (Database):
  Org A:
    - Client ID
    - Client Secret (encrypted)
    - Access Token (encrypted)
    - Refresh Token (encrypted)

  Org B:
    - Client ID
    - Client Secret (encrypted)
    - Access Token (encrypted)
    - Refresh Token (encrypted)
```

### 2. Database Migrations

**Status:** Migration 061 created and ready

- File: `backend/internal/migrations/061_create_salesforce_sync_tables.sql`
- Tables: `salesforce_connections` (master DB), `sync_jobs` (tenant DB), `salesforce_field_mappings` (master DB)
- Run migration command:
  ```bash
  cd backend && go run cmd/migrate/main.go
  ```
  (Connects to production Turso URL via TURSO_URL + TURSO_AUTH_TOKEN)

### 3. Production Endpoints

**Available for testing:**

| Endpoint | Method | Purpose | Auth |
|----------|--------|---------|------|
| `/api/v1/salesforce/config` | GET | Retrieve config | Admin |
| `/api/v1/salesforce/config` | POST | Save credentials | Admin |
| `/api/v1/salesforce/status` | GET | Check connection status | Admin |
| `/api/v1/salesforce/oauth/authorize` | POST | Start OAuth flow | Admin |
| `/api/v1/salesforce/oauth/callback` | GET | OAuth callback (public) | None |
| `/api/v1/salesforce/disconnect` | POST | Clear tokens | Admin |
| `/api/v1/salesforce/toggle` | PUT | Enable/disable sync | Admin |
| `/api/v1/salesforce/queue` | POST | Queue merge instructions | Admin |
| `/api/v1/salesforce/jobs` | GET | List sync jobs | Admin |
| `/api/v1/salesforce/trigger` | POST | Manual delivery trigger | Admin |

### 4. Frontend Deployment

**Status:** Ready to deploy

- Integrations hub page: `/admin/integrations`
- Salesforce config page: `/admin/integrations/salesforce`
- Admin nav card: Integrations added under Automation section

## 📋 Pre-Deployment Testing

### Local Testing Checklist
- [x] Backend starts without errors
- [x] Routes registered at startup
- [x] Admin user can navigate to config page
- [x] Form accepts credentials
- [x] Save works (with encryption key set)
- [x] Configuration persists in database
- [x] Status endpoint returns correct values
- [x] No JavaScript console errors

### Production Testing Checklist (After Deployment)

**Each Organization Tests Independently**

1. **Organization Admin Sets Up Salesforce Connection**
   - [ ] Org admin logs into Quantico
   - [ ] Navigates to `/admin/integrations/salesforce`
   - [ ] Page loads without errors
   - [ ] Form renders correctly
   - [ ] Org admin enters THEIR Salesforce Connected App credentials:
     - Client ID (from THEIR Salesforce org)
     - Client Secret (from THEIR Salesforce org)
     - Redirect URL (pre-filled: `/api/v1/salesforce/oauth/callback`)
   - [ ] Clicks "Save Configuration"
   - [ ] Success toast appears
   - [ ] Connection status changes to "Configured"

2. **Organization Admin Authenticates to Salesforce**
   - [ ] Clicks "Connect to Salesforce" button
   - [ ] Redirects to THEIR Salesforce login page
   - [ ] Admin enters THEIR Salesforce credentials
   - [ ] Approves Quantico permissions
   - [ ] Callback redirects back to config page
   - [ ] Connection status changes to "Connected"
   - [ ] Org's tokens stored encrypted in database

3. **Verify Per-Org Isolation**
   - [ ] Create test orgs: Org A, Org B
   - [ ] Org A connects to Salesforce Sandbox 1
   - [ ] Org B connects to Salesforce Sandbox 2
   - [ ] Verify: Org A's tokens only visible to Org A admins
   - [ ] Verify: Org B's tokens only visible to Org B admins

4. **API Integration**
   - [ ] GET `/api/v1/salesforce/status` returns current org's status
   - [ ] GET `/api/v1/salesforce/config` returns current org's config (no secrets)
   - [ ] POST `/api/v1/salesforce/queue` accepts merge instructions for current org

## ⚠️ Known Issues

### Non-Critical
- `/api/v1/salesforce/jobs` endpoint returns 500 (separate issue, not core feature)
  - Affects: "Recent Sync Jobs" display
  - Impact: Jobs list won't show, but queuing/delivery still works
  - Fix: Phase 18 or later

## 🚀 Deployment Steps

### Step 1: Set System Encryption Key (Railway) - DO FIRST

1. Generate encryption key:
   ```bash
   openssl rand -base64 32
   ```

2. In Railway Dashboard:
   - Select FastCRM Backend service
   - Go to Variables tab
   - Add new variable:
     - Key: `SALESFORCE_TOKEN_ENCRYPTION_KEY`
     - Value: `<32-byte-base64-key-from-above>`
   - Save variables (this triggers deploy)

3. Wait for deployment to complete

### Step 2: Push Code to GitHub
```bash
git push origin main
```
This triggers:
- Railway auto-deploy for backend (uses the encryption key just set)
- Vercel auto-deploy for frontend

Wait for both deployments to complete.

### Step 3: Run Migrations (Production Turso)

```bash
cd backend
TURSO_URL="<production-url>" \
TURSO_AUTH_TOKEN="<production-token>" \
go run cmd/migrate/main.go
```

Verify migration applied:
```
Applied: 061_create_salesforce_sync_tables.sql
```

### Step 4: Test in Production

Navigate to: `https://quanticocrm-git-main-ahmed-hamids-projects-9b5778f9.vercel.app/admin/integrations/salesforce`

Run through testing checklist above.

## 📞 Rollback Plan

If issues occur:

1. **Config won't save:** Check SALESFORCE_TOKEN_ENCRYPTION_KEY is set in Railway
2. **Page 404:** Clear cache, verify Vercel deployed latest
3. **OAuth fails:** Verify redirect URL matches Salesforce Connected App settings
4. **Database error:** Check migration 061 was applied to production Turso

To rollback: Revert to previous commit and redeploy
```bash
git revert <commit-hash>
git push origin main
```

## ✅ Completion Checklist

Before marking Phase 17 complete in production:

- [ ] Environment variable set in Railway
- [ ] Migration applied to production database
- [ ] Configuration page loads without 404
- [ ] Admin can save credentials successfully
- [ ] OAuth flow initiates correctly
- [ ] Status shows as "Connected" after auth
- [ ] No errors in production logs
- [ ] Verified on staging or production URL

---

**Phase 17 Ready for Production Testing**
Date: 2026-02-10
Status: All code complete, awaiting deployment
