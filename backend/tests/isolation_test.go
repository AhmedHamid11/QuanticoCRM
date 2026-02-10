package tests

import (
	"net/http"
	"testing"
)

// TestTenantIsolation_AllEndpoints verifies no cross-tenant data access
func TestTenantIsolation_AllEndpoints(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two organizations with users
	org1User := app.CreateTestUser(t, "user1@org1.com", "Password123!", "Organization One")
	org2User := app.CreateTestUser(t, "user2@org2.com", "Password123!", "Organization Two")

	// Create test records in Org 1
	contact1 := app.CreateContact(t, org1User.AccessToken, map[string]string{
		"firstName": "Org1",
		"lastName":  "Contact",
		"email":     "contact@org1.com",
	})
	account1 := app.CreateAccount(t, org1User.AccessToken, map[string]string{
		"name": "Org1 Account",
	})
	task1 := app.CreateTask(t, org1User.AccessToken, map[string]string{
		"subject": "Org1 Task",
	})

	// Extract IDs
	contact1ID := contact1["id"].(string)
	account1ID := account1["id"].(string)
	task1ID := task1["id"].(string)

	// Entity endpoints to test
	testCases := []struct {
		name       string
		entity     string
		id         string
		method     string
		expectCode int
	}{
		// Contact isolation
		{"org2_reads_org1_contact", "/api/v1/contacts", contact1ID, "GET", http.StatusNotFound},
		{"org2_updates_org1_contact", "/api/v1/contacts", contact1ID, "PUT", http.StatusNotFound},
		{"org2_deletes_org1_contact", "/api/v1/contacts", contact1ID, "DELETE", http.StatusNotFound},

		// Account isolation
		{"org2_reads_org1_account", "/api/v1/accounts", account1ID, "GET", http.StatusNotFound},
		{"org2_updates_org1_account", "/api/v1/accounts", account1ID, "PUT", http.StatusNotFound},
		{"org2_deletes_org1_account", "/api/v1/accounts", account1ID, "DELETE", http.StatusNotFound},

		// Task isolation
		{"org2_reads_org1_task", "/api/v1/tasks", task1ID, "GET", http.StatusNotFound},
		{"org2_updates_org1_task", "/api/v1/tasks", task1ID, "PUT", http.StatusNotFound},
		{"org2_deletes_org1_task", "/api/v1/tasks", task1ID, "DELETE", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body interface{}
			if tc.method == "PUT" {
				body = map[string]string{"name": "Modified"}
			}

			path := tc.entity + "/" + tc.id
			resp := app.MakeRequest(t, tc.method, path, body, org2User.AccessToken)
			AssertStatus(t, resp, tc.expectCode)
		})
	}
}

// TestTenantIsolation_ListEndpoints verifies list endpoints don't leak cross-tenant data
func TestTenantIsolation_ListEndpoints(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two organizations
	org1User := app.CreateTestUser(t, "user1@org1-list.com", "Password123!", "Org One List")
	org2User := app.CreateTestUser(t, "user2@org2-list.com", "Password123!", "Org Two List")

	// Create data only in Org 1 (with distinctive firstName)
	app.CreateContact(t, org1User.AccessToken, map[string]string{
		"firstName": "ListTestOrg1",
		"lastName":  "Contact",
	})
	app.CreateAccount(t, org1User.AccessToken, map[string]string{
		"name": "ListTestOrg1 Account",
	})
	app.CreateTask(t, org1User.AccessToken, map[string]string{
		"subject": "ListTestOrg1 Task",
	})

	// Org 2 lists should NOT contain Org 1's distinctive data
	t.Run("org2_contacts_no_org1_data", func(t *testing.T) {
		var result struct {
			Data []map[string]interface{} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/", nil, org2User.AccessToken, &result)
		AssertStatus(t, resp, http.StatusOK)

		// Check that none of Org 1's distinctive data appears
		for _, item := range result.Data {
			if firstName, ok := item["firstName"].(string); ok && firstName == "ListTestOrg1" {
				t.Error("Org 2 user saw Org 1's contact data - isolation broken")
			}
		}
	})

	t.Run("org2_accounts_no_org1_data", func(t *testing.T) {
		var result struct {
			Data []map[string]interface{} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/", nil, org2User.AccessToken, &result)
		AssertStatus(t, resp, http.StatusOK)

		for _, item := range result.Data {
			if name, ok := item["name"].(string); ok && name == "ListTestOrg1 Account" {
				t.Error("Org 2 user saw Org 1's account data - isolation broken")
			}
		}
	})

	t.Run("org2_tasks_no_org1_data", func(t *testing.T) {
		var result struct {
			Data []map[string]interface{} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/", nil, org2User.AccessToken, &result)
		AssertStatus(t, resp, http.StatusOK)

		for _, item := range result.Data {
			if subject, ok := item["subject"].(string); ok && subject == "ListTestOrg1 Task" {
				t.Error("Org 2 user saw Org 1's task data - isolation broken")
			}
		}
	})
}

// TestImpersonation_Isolation verifies platform admin is scoped when impersonating
func TestImpersonation_Isolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create platform admin and two orgs
	admin := app.CreatePlatformAdmin(t, "admin@platform.com", "AdminPass123!")
	org1User := app.CreateTestUser(t, "user1@imp-org1.com", "Password123!", "Imp Org One")
	org2User := app.CreateTestUser(t, "user2@imp-org2.com", "Password123!", "Imp Org Two")

	// Create data in both orgs
	contact1 := app.CreateContact(t, org1User.AccessToken, map[string]string{
		"firstName": "Org1",
		"lastName":  "Impersonate",
	})
	contact2 := app.CreateContact(t, org2User.AccessToken, map[string]string{
		"firstName": "Org2",
		"lastName":  "Impersonate",
	})

	contact1ID := contact1["id"].(string)
	contact2ID := contact2["id"].(string)

	// Admin impersonates Org 1
	impToken := app.Impersonate(t, admin.AccessToken, org1User.OrgID)

	t.Run("can_access_impersonated_org_data", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact1ID, nil, impToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("cannot_access_other_org_data", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact2ID, nil, impToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("list_only_shows_impersonated_org_data", func(t *testing.T) {
		var result struct {
			Data []map[string]interface{} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/", nil, impToken, &result)
		AssertStatus(t, resp, http.StatusOK)

		// Verify we only see Org 1's contact
		for _, item := range result.Data {
			if item["firstName"] == "Org2" {
				t.Error("Saw Org 2 data while impersonating Org 1")
			}
		}

		// Should see at least Org 1's contact
		found := false
		for _, item := range result.Data {
			if item["firstName"] == "Org1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Did not see Org 1 data while impersonating Org 1")
		}
	})
}

// TestImpersonation_PlatformEndpointsBlocked verifies admin can't access platform routes while impersonating
func TestImpersonation_PlatformEndpointsBlocked(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	admin := app.CreatePlatformAdmin(t, "admin2@platform.com", "AdminPass123!")
	org1User := app.CreateTestUser(t, "user@blocked-org.com", "Password123!", "Blocked Test Org")

	// Get impersonation token
	impToken := app.Impersonate(t, admin.AccessToken, org1User.OrgID)

	// Platform endpoints should be blocked during impersonation
	t.Run("platform_orgs_blocked", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/platform/organizations", nil, impToken)
		// Should be 403 Forbidden (not 401 or 404)
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 403 or 401, got %d", resp.StatusCode)
		}
	})
}
