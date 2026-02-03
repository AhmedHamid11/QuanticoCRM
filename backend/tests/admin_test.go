package tests

import (
	"net/http"
	"testing"
)

// ==========================================
// ADMIN & AUTHORIZATION TESTS
// ==========================================

func TestAdmin_EntityManagement(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("admin can list entities", func(t *testing.T) {
		var response struct {
			Entities []struct {
				Name  string `json:"name"`
				Label string `json:"label"`
			} `json:"entities"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/entities", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		// Should have built-in entities
		if len(response.Entities) == 0 {
			t.Error("Expected at least some entities")
		}
	})

	t.Run("admin can get entity definition", func(t *testing.T) {
		var response struct {
			Name   string `json:"name"`
			Label  string `json:"label"`
			Fields []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"fields"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/entities/Contact", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Name != "Contact" {
			t.Errorf("Expected entity name 'Contact', got %s", response.Name)
		}
	})

	t.Run("returns 404 for non-existent entity", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/entities/NonExistent", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestAdmin_FieldTypes(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("admin can list field types", func(t *testing.T) {
		var response struct {
			FieldTypes []struct {
				Name  string `json:"name"`
				Label string `json:"label"`
			} `json:"fieldTypes"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/field-types", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.FieldTypes) == 0 {
			t.Error("Expected at least some field types")
		}

		// Check for common field types
		foundText := false
		foundNumber := false
		for _, ft := range response.FieldTypes {
			if ft.Name == "text" || ft.Name == "varchar" {
				foundText = true
			}
			if ft.Name == "int" || ft.Name == "number" || ft.Name == "float" {
				foundNumber = true
			}
		}

		if !foundText {
			t.Error("Expected to find a text field type")
		}
		if !foundNumber {
			t.Error("Expected to find a number field type")
		}
	})
}

func TestAdmin_Fields(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("admin can list entity fields", func(t *testing.T) {
		var response struct {
			Fields []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"fields"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/entities/Contact/fields", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Fields) == 0 {
			t.Error("Expected Contact to have fields")
		}

		// Check for required fields
		foundLastName := false
		for _, f := range response.Fields {
			if f.Name == "lastName" {
				foundLastName = true
			}
		}
		if !foundLastName {
			t.Error("Expected Contact to have lastName field")
		}
	})
}

func TestAdmin_Navigation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("user can get navigation", func(t *testing.T) {
		var response struct {
			Tabs []struct {
				Entity string `json:"entity"`
				Label  string `json:"label"`
			} `json:"tabs"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/navigation/", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		// Should have some navigation tabs
		if len(response.Tabs) == 0 {
			t.Log("Warning: No navigation tabs returned (might need to be configured)")
		}
	})

	t.Run("admin can get admin navigation", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/navigation/", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("admin can update navigation", func(t *testing.T) {
		body := map[string]interface{}{
			"tabs": []map[string]interface{}{
				{"entity": "Contact", "label": "Contacts", "order": 1, "enabled": true},
				{"entity": "Account", "label": "Accounts", "order": 2, "enabled": true},
				{"entity": "Task", "label": "Tasks", "order": 3, "enabled": true},
			},
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/admin/navigation/", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})
}

func TestAdmin_Users(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create admin
	admin := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	// Invite a regular user
	inviteBody := map[string]string{
		"email": "regularuser@example.com",
		"role":  "user",
	}
	var inviteResp struct {
		Invitation struct {
			Token string `json:"token"`
		} `json:"invitation"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", inviteBody, admin.AccessToken, &inviteResp)
	AssertStatus(t, resp, http.StatusCreated)

	// Accept the invitation
	acceptBody := map[string]interface{}{
		"token":    inviteResp.Invitation.Token,
		"password": "password123",
	}
	var acceptResp struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/accept-invite", acceptBody, "", &acceptResp)
	AssertStatus(t, resp, http.StatusOK)

	t.Run("admin can list users", func(t *testing.T) {
		var response struct {
			Data []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"data"`
			Total int `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/users/", nil, admin.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Total < 2 {
			t.Errorf("Expected at least 2 users, got %d", response.Total)
		}
	})

	t.Run("admin can update user role", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"role": "admin",
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/admin/users/"+acceptResp.User.ID, updateBody, admin.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("admin cannot promote user to owner", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"role": "owner",
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/admin/users/"+acceptResp.User.ID, updateBody, admin.AccessToken)
		// Should fail - only owners can promote to owner, and even then it's restricted
		AssertStatus(t, resp, http.StatusForbidden)
	})
}

func TestAuthorization_RoleBasedAccess(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create an owner/admin
	owner := app.CreateTestUser(t, "owner@example.com", "password123", "Role Test Org")

	// Invite a regular user
	inviteBody := map[string]string{
		"email": "regularuser@example.com",
		"role":  "user",
	}
	var inviteResp struct {
		Invitation struct {
			Token string `json:"token"`
		} `json:"invitation"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", inviteBody, owner.AccessToken, &inviteResp)
	AssertStatus(t, resp, http.StatusCreated)

	// Accept the invitation
	acceptBody := map[string]interface{}{
		"token":    inviteResp.Invitation.Token,
		"password": "password123",
	}
	var acceptResp struct {
		AccessToken string `json:"accessToken"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/accept-invite", acceptBody, "", &acceptResp)
	AssertStatus(t, resp, http.StatusOK)

	regularUserToken := acceptResp.AccessToken

	// Test regular user access to CRM data
	t.Run("regular user can access CRM data", func(t *testing.T) {
		// Create a contact
		contactBody := map[string]interface{}{
			"lastName": "TestContact",
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", contactBody, regularUserToken)
		AssertStatus(t, resp, http.StatusCreated)

		// List contacts
		resp = app.MakeRequest(t, "GET", "/api/v1/contacts/", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("regular user can get navigation", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/navigation/", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	// Test regular user blocked from admin routes
	t.Run("regular user cannot access admin entities", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/entities", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot access admin field types", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/field-types", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot create tripwires", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Test",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://example.com/webhook",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/tripwires/", body, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot create validation rules", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Test",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"action":     "BLOCK_SAVE",
			"message":    "Test",
			"conditions": []map[string]interface{}{},
			"validations": []map[string]interface{}{
				{"field": "lastName", "operator": "NOT_EMPTY"},
			},
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/validation-rules/", body, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot invite users", func(t *testing.T) {
		inviteBody := map[string]string{
			"email": "another@example.com",
			"role":  "user",
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/invite", inviteBody, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot update navigation", func(t *testing.T) {
		body := map[string]interface{}{
			"tabs": []map[string]interface{}{},
		}
		resp := app.MakeRequest(t, "PUT", "/api/v1/admin/navigation/", body, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})
}

func TestAuthorization_UnauthenticatedAccess(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	t.Run("unauthenticated cannot access contacts", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("unauthenticated cannot access accounts", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/accounts/", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("unauthenticated cannot access tasks", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tasks/", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("unauthenticated cannot access admin routes", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/entities", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("unauthenticated can access health check", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/health", nil, "")
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("unauthenticated can register", func(t *testing.T) {
		body := map[string]string{
			"email":    "newuser@example.com",
			"password": "password123",
			"orgName":  "New Org",
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusCreated)
	})

	t.Run("unauthenticated can login", func(t *testing.T) {
		// First register
		registerBody := map[string]string{
			"email":    "logintest@example.com",
			"password": "password123",
			"orgName":  "Login Test Org",
		}
		app.MakeRequest(t, "POST", "/api/v1/auth/register", registerBody, "")

		// Then login
		loginBody := map[string]string{
			"email":    "logintest@example.com",
			"password": "password123",
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/login", loginBody, "")
		AssertStatus(t, resp, http.StatusOK)
	})
}

func TestAuthorization_InvalidToken(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	t.Run("invalid token returns 401", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/", nil, "invalid-token")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		// This would require a way to generate an expired token
		// For now, just test with a malformed token
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/", nil, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZXhwIjoxfQ.invalid")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestAdmin_Bearings(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("admin can get bearings", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/bearings/Contact", nil, user.AccessToken)
		// May return 200 with empty config or 404 if no config exists
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 200 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("admin can create/update bearings", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"fields": []map[string]interface{}{
				{
					"name":     "createdAt",
					"readOnly": true,
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/bearings/Contact", body, user.AccessToken)
		// Should be 200 or 201
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 200 or 201, got %d", resp.StatusCode)
		}
	})
}

func TestAdmin_RelatedLists(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Admin Test Org")

	t.Run("admin can get related list options", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/related-list/Account/options", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("admin can get related list configs", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/related-list/Account/configs", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("admin can save related list configs", func(t *testing.T) {
		body := map[string]interface{}{
			"configs": []map[string]interface{}{
				{
					"entityType":    "Account",
					"relatedEntity": "Contact",
					"lookupField":   "accountId",
					"label":         "Contacts",
					"enabled":       true,
					"displayFields": []map[string]interface{}{
						{"name": "firstName", "label": "First Name"},
						{"name": "lastName", "label": "Last Name"},
					},
					"sortOrder": 0,
					"pageSize":  5,
				},
			},
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/related-list/Account/configs", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})
}
