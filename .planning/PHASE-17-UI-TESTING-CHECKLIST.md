# Phase 17 Salesforce Integration - UI Testing Checklist

## Test Environment Setup

**Prerequisites:**
- Backend running with `SALESFORCE_TOKEN_ENCRYPTION_KEY` environment variable set
- Production migration applied (061_create_salesforce_sync_tables.sql)
- Frontend dev server or production Vercel deployment
- At least 2 test organizations created (for multi-tenant verification)
- Access to production/test Salesforce org credentials

---

# SECTION 1: Navigation & Page Load

## 1.1 Admin Page Integration
- [ ] Navigate to `http://localhost:5173/admin`
- [ ] Verify "Integrations" card appears in Automation section
- [ ] Card displays correct title and icon
- [ ] Click Integrations card → navigates to `/admin/integrations`
- [ ] No 404 errors, no console errors

## 1.2 Integrations Hub Page
- [ ] Page loads at `/admin/integrations`
- [ ] "Integrations Hub" heading displays
- [ ] Salesforce integration card displays
- [ ] Card shows status badge (initially gray "Not Configured")
- [ ] "Configure" link is clickable and styled correctly
- [ ] No console errors

## 1.3 Salesforce Configuration Page
- [ ] Click "Configure" → navigates to `/admin/integrations/salesforce`
- [ ] Page loads without 404
- [ ] Correct page title/heading displays
- [ ] Three distinct sections visible:
  - Configuration Form
  - Connection Status
  - Recent Sync Jobs
- [ ] No JavaScript console errors
- [ ] Page responsive (test at different viewport sizes)

---

# SECTION 2: Configuration Form (Section 1)

## 2.1 Form Rendering
- [ ] Three input fields visible:
  - [ ] Client ID (text input)
  - [ ] Client Secret (password input, dots/asterisks)
  - [ ] Redirect URL (text input, pre-filled)
- [ ] All fields have correct labels
- [ ] Redirect URL pre-filled with: `http://localhost:5173/api/v1/salesforce/oauth/callback` (or production equivalent)
- [ ] "Save Configuration" button visible and enabled
- [ ] Form has no validation errors on initial load

## 2.2 Form Input
- [ ] Type in Client ID field → value appears
- [ ] Type in Client Secret field → dots/asterisks appear (not visible text)
- [ ] Redirect URL field is not editable (disabled or read-only)
- [ ] All input fields accept text without errors

## 2.3 Form Validation
- [ ] Try submitting empty form → validation error appears
- [ ] Try submitting with only Client ID → validation error
- [ ] Try submitting with only Client Secret → validation error
- [ ] Try submitting with only Redirect URL → validation error
- [ ] Error messages are clear and helpful

## 2.4 Form Submission Success
- [ ] Fill all three fields with test Salesforce credentials
- [ ] Click "Save Configuration" button
- [ ] Button changes to "Saving..." state
- [ ] Wait for response (should complete within 3 seconds)
- [ ] Success toast appears: "Configuration saved successfully"
- [ ] Form values persist (Client ID still shows, Secret is cleared)
- [ ] No console errors
- [ ] Network: POST `/api/v1/salesforce/config` returns 200 status

## 2.5 Form Submission Error Handling
- [ ] Fill form with invalid credentials (e.g., empty strings with spaces)
- [ ] Click "Save Configuration"
- [ ] If error occurs, error message toast appears
- [ ] Button returns to normal state
- [ ] Form can be resubmitted after error
- [ ] No unhandled JavaScript errors

---

# SECTION 3: Connection Status Section (Section 2)

## 3.1 Status Display - Not Configured State
- [ ] Connection Status section displays
- [ ] Status shows: "Not Configured" with gray dot
- [ ] "Connect to Salesforce" button is DISABLED (grayed out)
- [ ] "Disconnect" button is DISABLED
- [ ] "Enable Sync" toggle is DISABLED
- [ ] Text explains: "Please save your Connected App credentials first"

## 3.2 Status Display - Configured State
- [ ] Save configuration (from Section 2.4)
- [ ] Status changes to: "Configured" with yellow dot
- [ ] "Connect to Salesforce" button becomes ENABLED
- [ ] Button text and styling are correct
- [ ] "Disconnect" button is still DISABLED (no active connection yet)
- [ ] "Enable Sync" toggle remains DISABLED until connected

## 3.3 Status Display - Connected State
- [ ] Click "Connect to Salesforce" button
- [ ] Browser redirects to Salesforce login page
- [ ] (Test environment: simulate Salesforce auth)
- [ ] Callback redirects back to `/admin/integrations/salesforce`
- [ ] Success toast appears: "Successfully connected to Salesforce"
- [ ] Status changes to: "Connected" with green dot
- [ ] Connection timestamp displays (if implemented)
- [ ] "Connect to Salesforce" button becomes DISABLED (already connected)
- [ ] "Disconnect" button becomes ENABLED
- [ ] "Enable Sync" toggle becomes ENABLED

## 3.4 Status Display - Expired Token State
- [ ] (Admin only) Manually expire token in database or API
- [ ] Refresh page or wait for auto-refresh
- [ ] Status changes to: "Expired" with red dot
- [ ] "Connect to Salesforce" button becomes ENABLED again
- [ ] Message indicates: "Token expired, please reconnect"

## 3.5 Sync Toggle
- [ ] When connected: Click "Enable Sync" toggle
- [ ] Toggle switches ON
- [ ] PUT `/api/v1/salesforce/toggle` called with `{enabled: true}`
- [ ] Toggle state persists on page refresh
- [ ] Click again to disable
- [ ] Toggle switches OFF
- [ ] PUT endpoint called with `{enabled: false}`

## 3.6 Disconnect Functionality
- [ ] When connected: Click "Disconnect" button
- [ ] Confirmation dialog appears (if implemented)
- [ ] Confirm disconnection
- [ ] POST `/api/v1/salesforce/disconnect` called
- [ ] Success toast appears
- [ ] Status reverts to: "Configured" with yellow dot
- [ ] "Connect to Salesforce" button becomes ENABLED
- [ ] "Disconnect" button becomes DISABLED
- [ ] Sync toggle becomes DISABLED

---

# SECTION 4: Recent Sync Jobs Section (Section 3)

## 4.1 Jobs Table - Empty State
- [ ] "Recent Sync Jobs" header displays
- [ ] Table shows: "No sync jobs yet"
- [ ] Message appears when org hasn't queued any deliveries
- [ ] No console errors

## 4.2 Jobs Table - With Data (if jobs exist)
- [ ] Jobs table displays with columns:
  - [ ] Batch ID
  - [ ] Entity (e.g., "Contact")
  - [ ] Status (pending/running/completed/failed)
  - [ ] Instructions (e.g., "5/10")
  - [ ] Created (timestamp)
  - [ ] Actions (retry button)
- [ ] Each row properly formatted
- [ ] Status badges have correct colors:
  - [ ] Pending: blue
  - [ ] Running: yellow
  - [ ] Completed: green
  - [ ] Failed: red

## 4.3 Jobs Table - Retry Action
- [ ] Failed job shows "Retry" button
- [ ] Click retry button
- [ ] POST `/api/v1/salesforce/jobs/{jobId}/retry` called
- [ ] Success toast appears
- [ ] Job status updates (if auto-refresh enabled)

---

# SECTION 5: Multi-Tenant Isolation Testing

## 5.1 Create Second Organization
- [ ] Create a second test organization (if not already created)
- [ ] Ensure it has different Salesforce credentials

## 5.2 Organization A Verification
- [ ] Log in as Organization A admin
- [ ] Navigate to `/admin/integrations/salesforce`
- [ ] Save Organization A's Salesforce credentials
- [ ] Status shows Organization A's configuration
- [ ] GET `/api/v1/salesforce/config` returns Org A's config (without secrets)
- [ ] GET `/api/v1/salesforce/status` returns Org A's status

## 5.3 Organization B Verification
- [ ] Log out from Organization A
- [ ] Log in as Organization B admin (different org)
- [ ] Navigate to `/admin/integrations/salesforce`
- [ ] Status should show "Not Configured" (Org B hasn't saved credentials yet)
- [ ] Save Organization B's DIFFERENT Salesforce credentials
- [ ] Status shows Organization B's configuration (different from Org A)
- [ ] Verify isolation:
  - [ ] GET `/api/v1/salesforce/config` returns Org B's config
  - [ ] Does NOT show Org A's config
  - [ ] GET `/api/v1/salesforce/status` returns Org B's status
  - [ ] Does NOT show Org A's status

## 5.4 Token Encryption Verification
- [ ] (Database verification) Check `salesforce_connections` table
- [ ] Org A's `client_secret_encrypted` is encrypted (not readable)
- [ ] Org B's `client_secret_encrypted` is encrypted (different ciphertext)
- [ ] Verify tokens differ between orgs (even for same password, encryption should be unique)

---

# SECTION 6: API Endpoint Testing

## 6.1 Configuration Endpoints
- [ ] POST `/api/v1/salesforce/config`
  - [ ] Accepts valid credentials
  - [ ] Returns 200 with `{"status": "configured"}`
  - [ ] Rejects missing fields (400)
  - [ ] Requires admin auth (401 without token)

- [ ] GET `/api/v1/salesforce/config`
  - [ ] Returns config without sensitive data
  - [ ] Returns 200 with fields: clientId, redirectUrl, instanceUrl, isEnabled, status
  - [ ] Does NOT return clientSecret or tokens
  - [ ] Requires admin auth

## 6.2 Status Endpoints
- [ ] GET `/api/v1/salesforce/status`
  - [ ] Returns current status (not_configured/configured/connected/expired)
  - [ ] Returns 200
  - [ ] Requires admin auth

## 6.3 OAuth Endpoints
- [ ] POST `/api/v1/salesforce/oauth/authorize`
  - [ ] Returns 200 with `{authUrl: "https://login.salesforce.com/..."}`
  - [ ] authUrl is valid and clickable
  - [ ] Requires admin auth

- [ ] GET `/api/v1/salesforce/oauth/callback` (PUBLIC)
  - [ ] Accepts ?code= and ?state= parameters
  - [ ] Exchanges code for tokens
  - [ ] Redirects to admin page on success
  - [ ] Redirects with ?status=error on failure

## 6.4 Sync Control Endpoints
- [ ] POST `/api/v1/salesforce/disconnect`
  - [ ] Clears tokens
  - [ ] Returns 200 with `{"status": "disconnected"}`
  - [ ] Requires admin auth

- [ ] PUT `/api/v1/salesforce/toggle`
  - [ ] Accepts `{enabled: true/false}`
  - [ ] Returns 200 with `{"isEnabled": true/false}`
  - [ ] Requires admin auth

## 6.5 Delivery Endpoints
- [ ] POST `/api/v1/salesforce/queue`
  - [ ] Accepts merge instructions
  - [ ] Returns 202 Accepted with `{jobId: "...", status: "pending"}`
  - [ ] Requires admin auth

- [ ] GET `/api/v1/salesforce/jobs`
  - [ ] Returns 200 with `{jobs: [...], total: N}`
  - [ ] Requires admin auth

---

# SECTION 7: Error Handling & Edge Cases

## 7.1 Network Error Handling
- [ ] Disconnect network while form is submitting
- [ ] Error toast appears
- [ ] No unhandled promise rejections in console
- [ ] Page remains functional (can retry)

## 7.2 Invalid Input Handling
- [ ] Paste very long strings in Client ID
- [ ] Form handles gracefully (truncates or rejects)
- [ ] Special characters in credentials
- [ ] Form handles without breaking
- [ ] SQL injection-like strings (e.g., `"; DROP TABLE`)
- [ ] Form safely encodes/escapes

## 7.3 Session Expiration
- [ ] Session expires while on configuration page
- [ ] Try to save credentials
- [ ] Redirected to login page (or 401 error)
- [ ] No data corruption

## 7.4 Concurrent Operations
- [ ] Open page in two tabs for same org
- [ ] Save credentials in Tab 1
- [ ] Tab 2 automatically updates (or manual refresh shows correct state)
- [ ] No conflicts or race conditions

## 7.5 Browser Compatibility
- [ ] Test in Chrome/Edge (Chromium-based)
- [ ] Test in Firefox (if available)
- [ ] Test in Safari (if available)
- [ ] All features work consistently
- [ ] No console errors in any browser

---

# SECTION 8: Performance & UX

## 8.1 Page Load Time
- [ ] Initial page load < 2 seconds
- [ ] Form submission completes < 3 seconds
- [ ] Status updates < 1 second
- [ ] No visible lag or freezing

## 8.2 Loading States
- [ ] "Saving..." button state shows during submission
- [ ] Loading spinner (if implemented) displays correctly
- [ ] Text clarity during async operations

## 8.3 Toast Notifications
- [ ] Success toasts display and auto-dismiss
- [ ] Error toasts display and auto-dismiss
- [ ] Toast position consistent (top-right, etc.)
- [ ] Multiple toasts stack properly
- [ ] Can manually dismiss toasts

## 8.4 Form Responsiveness
- [ ] Fields are accessible (tab order is logical)
- [ ] Labels associated with inputs (for accessibility)
- [ ] Client Secret masked on all browsers
- [ ] Copy/paste works in all fields except Redirect URL

---

# SECTION 9: Console & Network Verification

## 9.1 Console Check
- [ ] Open DevTools (F12)
- [ ] Go to Console tab
- [ ] No red error messages (warnings OK)
- [ ] No `undefined` or `null` reference errors
- [ ] No CORS errors
- [ ] No 400/500 errors for valid operations

## 9.2 Network Activity
- [ ] Save configuration
- [ ] Network tab shows:
  - [ ] POST `/api/v1/salesforce/config` → 200
  - [ ] Response payload is JSON and valid
  - [ ] No sensitive data in network logs (check token encryption)

- [ ] Fetch status
- [ ] Network tab shows:
  - [ ] GET `/api/v1/salesforce/status` → 200
  - [ ] GET `/api/v1/salesforce/config` → 200
  - [ ] Response format consistent

## 9.3 Security Headers
- [ ] Check response headers for security:
  - [ ] Content-Type: application/json
  - [ ] No sensitive headers exposed
  - [ ] CORS headers correct

---

# SECTION 10: Final Integration Verification

## 10.1 Full Happy Path (Complete Workflow)
- [ ] Admin navigates to integrations page
- [ ] Configures Salesforce credentials
- [ ] Saves successfully
- [ ] Connects to Salesforce
- [ ] Authentication completes
- [ ] Status shows "Connected"
- [ ] Can toggle sync on/off
- [ ] No errors throughout
- [ ] All data persists on page refresh

## 10.2 Data Persistence
- [ ] Save configuration
- [ ] Refresh page
- [ ] Saved Client ID still appears in form
- [ ] Status unchanged
- [ ] Connection still shows as Connected (if previously connected)

## 10.3 Admin-Only Access
- [ ] Log in as non-admin user
- [ ] Navigate directly to `/admin/integrations/salesforce`
- [ ] Access denied (403 or redirect to login/dashboard)
- [ ] Cannot call API endpoints without admin auth
- [ ] No information leakage

---

# COMPLETION CRITERIA

**Phase 17 is VERIFIED when:**

✅ All navigation tests pass (Section 1)
✅ All form functionality tests pass (Section 2)
✅ Connection status displays correctly in all states (Section 3)
✅ Jobs section renders correctly (Section 4)
✅ Multi-tenant isolation confirmed (Section 5)
✅ All API endpoints respond correctly (Section 6)
✅ Error cases handled gracefully (Section 7)
✅ Performance is acceptable (Section 8)
✅ No console errors or warnings (Section 9)
✅ Complete happy path works end-to-end (Section 10)

---

**Test Duration:** ~45 minutes for full verification
**Test Difficulty:** Moderate (requires Salesforce test org access)
**Browser Requirements:** Chrome/Edge, Firefox (if testing compatibility)

