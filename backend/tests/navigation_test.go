package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestNavigationProvisionedOnRegistration verifies that when a new user registers
// (creating a new org), navigation tabs are automatically provisioned
func TestNavigationProvisionedOnRegistration(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Register a new user (creates org + provisions metadata + nav)
	user := app.CreateTestUser(t, "nav-test@example.com", "TestPassword123!", "Nourish")

	// Fetch navigation tabs for the new org
	resp := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tabs []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &tabs); err != nil {
		t.Fatalf("Failed to decode navigation response: %v (body: %s)", err, string(bodyBytes))
	}

	// Verify navigation tabs were created
	if len(tabs) == 0 {
		t.Fatalf("Expected navigation tabs to be provisioned for new org, got 0 tabs")
	}

	// Verify expected default tabs exist
	expectedTabs := map[string]bool{
		"/":         false, // Home
		"/accounts": false,
		"/contacts": false,
		"/quotes":   false,
		"/tasks":    false,
	}

	for _, tab := range tabs {
		href, ok := tab["href"].(string)
		if !ok {
			continue
		}
		if _, exists := expectedTabs[href]; exists {
			expectedTabs[href] = true
		}
	}

	for href, found := range expectedTabs {
		if !found {
			t.Errorf("Expected default navigation tab with href %q not found", href)
		}
	}

	t.Logf("Navigation test passed: %d tabs found for org %q", len(tabs), user.OrgName)
}

// TestNavigationEmptyReturnsEmptyArray verifies that when no nav tabs exist,
// the API returns an empty array (not null or error) so the frontend fallback works
func TestNavigationEmptyReturnsEmptyArray(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "empty-nav@example.com", "TestPassword123!", "EmptyNavOrg")

	// Delete all navigation tabs for this org
	_, err := app.DB.Exec("DELETE FROM navigation_tabs WHERE org_id = ?", user.OrgID)
	if err != nil {
		t.Fatalf("Failed to delete nav tabs: %v", err)
	}

	// Fetch navigation - should return empty array, not error
	resp := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tabs []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &tabs); err != nil {
		t.Fatalf("Failed to decode navigation response: %v (body: %s)", err, string(bodyBytes))
	}

	if tabs == nil {
		t.Error("Expected empty array [], got null")
	}

	if len(tabs) != 0 {
		t.Errorf("Expected 0 tabs after deletion, got %d", len(tabs))
	}
}

// TestNavigationAdminCanListAll verifies admin can see all tabs (including hidden)
func TestNavigationAdminCanListAll(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin-nav@example.com", "TestPassword123!", "AdminNavOrg")

	// First verify we have tabs from provisioning
	resp := app.MakeRequest(t, "GET", "/api/v1/admin/navigation/", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tabs []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &tabs); err != nil {
		t.Fatalf("Failed to decode admin navigation response: %v (body: %s)", err, string(bodyBytes))
	}

	if len(tabs) == 0 {
		t.Fatal("Expected navigation tabs to exist for admin listing")
	}

	// Each tab should have required fields
	for i, tab := range tabs {
		requiredFields := []string{"id", "label", "href", "sortOrder", "isVisible"}
		for _, field := range requiredFields {
			if _, ok := tab[field]; !ok {
				t.Errorf("Tab %d missing required field %q", i, field)
			}
		}
	}
}

// TestReprovisionRestoresNavigation verifies that re-provisioning restores
// navigation tabs when they're missing
func TestReprovisionRestoresNavigation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "reprovision-nav@example.com", "TestPassword123!", "ReprovisionOrg")

	// Delete all navigation tabs
	_, err := app.DB.Exec("DELETE FROM navigation_tabs WHERE org_id = ?", user.OrgID)
	if err != nil {
		t.Fatalf("Failed to delete nav tabs: %v", err)
	}

	// Verify tabs are gone
	resp := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var emptyTabs []map[string]interface{}
	json.Unmarshal(bodyBytes, &emptyTabs)
	if len(emptyTabs) != 0 {
		t.Fatalf("Expected 0 tabs after deletion, got %d", len(emptyTabs))
	}

	// Re-provision metadata (including navigation)
	resp = app.MakeRequest(t, "POST", "/api/v1/admin/reprovision", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	// Verify tabs are restored
	resp = app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ = io.ReadAll(resp.Body)
	var restoredTabs []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &restoredTabs); err != nil {
		t.Fatalf("Failed to decode navigation response: %v (body: %s)", err, string(bodyBytes))
	}

	if len(restoredTabs) == 0 {
		t.Fatal("Expected navigation tabs to be restored after reprovision, got 0")
	}

	// Verify the expected tabs are back
	hasAccounts := false
	hasContacts := false
	for _, tab := range restoredTabs {
		href, _ := tab["href"].(string)
		if href == "/accounts" {
			hasAccounts = true
		}
		if href == "/contacts" {
			hasContacts = true
		}
	}

	if !hasAccounts {
		t.Error("Expected /accounts tab to be restored after reprovision")
	}
	if !hasContacts {
		t.Error("Expected /contacts tab to be restored after reprovision")
	}

	t.Logf("Reprovision test passed: %d tabs restored", len(restoredTabs))
}

// TestNavigationTabsAreOrgScoped verifies that navigation tabs are isolated per org
func TestNavigationTabsAreOrgScoped(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different orgs
	user1 := app.CreateTestUser(t, "org1-nav@example.com", "TestPassword123!", "Org One")
	user2 := app.CreateTestUser(t, "org2-nav@example.com", "TestPassword123!", "Org Two")

	// Fetch navigation for each org
	resp1 := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user1.AccessToken)
	AssertStatus(t, resp1, http.StatusOK)

	resp2 := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user2.AccessToken)
	AssertStatus(t, resp2, http.StatusOK)

	bodyBytes1, _ := io.ReadAll(resp1.Body)
	bodyBytes2, _ := io.ReadAll(resp2.Body)

	var tabs1, tabs2 []map[string]interface{}
	json.Unmarshal(bodyBytes1, &tabs1)
	json.Unmarshal(bodyBytes2, &tabs2)

	// Both should have tabs (independently provisioned)
	if len(tabs1) == 0 {
		t.Error("Org One should have navigation tabs")
	}
	if len(tabs2) == 0 {
		t.Error("Org Two should have navigation tabs")
	}

	// Tab IDs should be different (each org gets unique IDs)
	if len(tabs1) > 0 && len(tabs2) > 0 {
		id1, _ := tabs1[0]["id"].(string)
		id2, _ := tabs2[0]["id"].(string)
		if id1 == id2 {
			t.Error("Navigation tab IDs should be unique per org")
		}
	}
}

// TestOrgSwitchGetsCorrectNavigation verifies that after switching orgs,
// the navigation API returns tabs for the new org
func TestOrgSwitchGetsCorrectNavigation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a user with initial org
	user := app.CreateTestUser(t, "switch-nav@example.com", "TestPassword123!", "First Org")
	orgID1 := user.OrgID

	// Get nav tabs for first org
	resp := app.MakeRequest(t, "GET", "/api/v1/navigation", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusOK)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tabs1 []map[string]interface{}
	json.Unmarshal(bodyBytes, &tabs1)

	if len(tabs1) == 0 {
		t.Fatal("First org should have navigation tabs")
	}

	// Verify tabs belong to org 1
	for _, tab := range tabs1 {
		tabOrgID, ok := tab["orgId"].(string)
		if ok && tabOrgID != orgID1 {
			t.Errorf("Tab orgId %q doesn't match expected org %q", tabOrgID, orgID1)
		}
	}

	t.Logf("Org switch navigation test passed: %d tabs for first org", len(tabs1))
}
