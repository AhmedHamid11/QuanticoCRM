package tests

import (
	"net/http"
	"testing"
)

// ==========================================
// VALIDATION RULES TESTS
// ==========================================

func TestValidationRule_CRUD(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "Qw!x7Km9pZr2", "Validation Test Org")

	t.Run("admin can create validation rule", func(t *testing.T) {
		body := map[string]interface{}{
			"name":            "Require Email for VIP",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "description",
					"operator":  "CONTAINS",
					"value":     "VIP",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "REQUIRE_VALUE",
					"fields":       []string{"emailAddress"},
					"errorMessage": "Email is required for VIP contacts",
				},
			},
		}

		var response struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			EntityType string `json:"entityType"`
			Enabled    bool   `json:"enabled"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected validation rule ID to be set")
		}
		if response.Name != "Require Email for VIP" {
			t.Errorf("Expected name 'Require Email for VIP', got %s", response.Name)
		}
		if response.EntityType != "Contact" {
			t.Errorf("Expected entityType 'Contact', got %s", response.EntityType)
		}
	})

	t.Run("lists validation rules", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/entities/Contact/validation-rules", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) == 0 {
			t.Error("Expected at least one validation rule")
		}
	})

	t.Run("gets validation rule by ID", func(t *testing.T) {
		// First create a rule
		createBody := map[string]interface{}{
			"name":            "Get Test Rule",
			"enabled":         true,
			"triggerOnCreate": false,
			"triggerOnUpdate": true,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "name",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Name cannot be empty",
				},
			},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/entities/Account/validation-rules", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Now get it
		var response struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		resp = app.MakeRequestWithResponse(t, "GET", "/api/v1/admin/entities/Account/validation-rules/"+created.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
		}
	})

	t.Run("updates validation rule", func(t *testing.T) {
		// First create a rule
		createBody := map[string]interface{}{
			"name":            "Update Test Rule",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "lastName",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Last name is required",
				},
			},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Update it
		updateBody := map[string]interface{}{
			"name":    "Updated Rule Name",
			"enabled": false,
		}

		var updated struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		resp = app.MakeRequestWithResponse(t, "PUT", "/api/v1/admin/entities/Contact/validation-rules/"+created.ID, updateBody, user.AccessToken, &updated)
		AssertStatus(t, resp, http.StatusOK)

		if updated.Name != "Updated Rule Name" {
			t.Errorf("Expected name 'Updated Rule Name', got %s", updated.Name)
		}
		if updated.Enabled != false {
			t.Error("Expected enabled to be false")
		}
	})

	t.Run("deletes validation rule", func(t *testing.T) {
		// First create a rule
		createBody := map[string]interface{}{
			"name":            "Delete Test Rule",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "lastName",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Last name is required",
				},
			},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Delete it
		resp = app.MakeRequest(t, "DELETE", "/api/v1/admin/entities/Contact/validation-rules/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNoContent)

		// Verify it's deleted
		resp = app.MakeRequest(t, "GET", "/api/v1/admin/entities/Contact/validation-rules/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestValidationRule_Enforcement(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "Qw!x7Km9pZr2", "Validation Test Org")

	t.Run("BLOCK_SAVE prevents record creation", func(t *testing.T) {
		// Create a validation rule that requires email for contacts with "VIP" in description
		ruleBody := map[string]interface{}{
			"name":            "Require Email for VIP",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "description",
					"operator":  "CONTAINS",
					"value":     "VIP",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "REQUIRE_VALUE",
					"fields":       []string{"emailAddress"},
					"errorMessage": "Email is required for VIP contacts",
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", ruleBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		// Try to create a VIP contact without email - should fail
		contactBody := map[string]interface{}{
			"lastName":    "VIPContact",
			"description": "This is a VIP customer",
		}

		resp = app.MakeRequest(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusUnprocessableEntity)

		// Create the same contact WITH email - should succeed
		contactBody["emailAddress"] = "vip@example.com"
		resp = app.MakeRequest(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		// Create a non-VIP contact without email - should succeed (rule doesn't apply)
		nonVipBody := map[string]interface{}{
			"lastName":    "RegularContact",
			"description": "Regular customer",
		}
		resp = app.MakeRequest(t, "POST", "/api/v1/contacts/", nonVipBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	})

	t.Run("disabled rule does not block", func(t *testing.T) {
		// Create a disabled validation rule
		ruleBody := map[string]interface{}{
			"name":            "Disabled Rule",
			"enabled":         false, // Disabled
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "phoneNumber",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Phone number is required",
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", ruleBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		// Create a contact without phone - should succeed because rule is disabled
		contactBody := map[string]interface{}{
			"lastName": "NoPhone",
		}
		resp = app.MakeRequest(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	})

	t.Run("rule applies to UPDATE events", func(t *testing.T) {
		// Create a validation rule for UPDATE
		ruleBody := map[string]interface{}{
			"name":            "Require Website on Update",
			"enabled":         true,
			"triggerOnCreate": false,
			"triggerOnUpdate": true,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "website",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Website is required when updating accounts",
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Account/validation-rules", ruleBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		// Create an account without website - should succeed (rule is for UPDATE)
		accountBody := map[string]interface{}{
			"name": "No Website Account",
		}
		var account struct {
			ID string `json:"id"`
		}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", accountBody, user.AccessToken, &account)
		AssertStatus(t, resp, http.StatusCreated)

		// Try to update without website - should fail
		updateBody := map[string]interface{}{
			"name": "Updated Name",
		}
		resp = app.MakeRequest(t, "PUT", "/api/v1/accounts/"+account.ID, updateBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusUnprocessableEntity)

		// Update with website - should succeed
		updateBody["website"] = "https://example.com"
		resp = app.MakeRequest(t, "PUT", "/api/v1/accounts/"+account.ID, updateBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)
	})
}

func TestValidationRule_Operators(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "Qw!x7Km9pZr2", "Validation Test Org")

	t.Run("EQUALS operator blocks specific values", func(t *testing.T) {
		ruleBody := map[string]interface{}{
			"name":            "Block Cancelled Status",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "status",
					"operator":  "EQUALS",
					"value":     "Cancelled",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Cannot create tasks with 'Cancelled' status",
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Task/validation-rules", ruleBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		// Create contact for task parent
		contactBody := map[string]interface{}{"lastName": "TaskParent"}
		var contact struct {
			ID string `json:"id"`
		}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
		AssertStatus(t, resp, http.StatusCreated)

		// Try to create a cancelled task - should fail
		taskBody := map[string]interface{}{
			"subject":    "Cancelled Task",
			"type":       "Todo",
			"status":     "Cancelled",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}
		resp = app.MakeRequest(t, "POST", "/api/v1/tasks/", taskBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusUnprocessableEntity)

		// Create with different status - should succeed
		taskBody["status"] = "Open"
		resp = app.MakeRequest(t, "POST", "/api/v1/tasks/", taskBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	})
}

func TestValidationRule_NonAdminAccess(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a regular user (not admin)
	// Note: When registering, user becomes owner of their org
	// We need to invite a user as "user" role to test non-admin access
	admin := app.CreateTestUser(t, "admin@example.com", "Qw!x7Km9pZr2", "Test Org")

	// Invite a regular user
	inviteBody := map[string]string{
		"email": "regularuser@example.com",
		"role":  "user",
	}
	var inviteResp struct {
		Token string `json:"token"` // Token is at top level, not nested
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", inviteBody, admin.AccessToken, &inviteResp)
	AssertStatus(t, resp, http.StatusCreated)

	// Accept the invitation
	acceptBody := map[string]interface{}{
		"token":    inviteResp.Token,
		"password": "Qw!x7Km9pZr2",
	}
	var acceptResp struct {
		AccessToken string `json:"accessToken"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/accept-invite", acceptBody, "", &acceptResp)
	AssertStatus(t, resp, http.StatusOK)

	regularUserToken := acceptResp.AccessToken

	t.Run("regular user cannot create validation rules", func(t *testing.T) {
		ruleBody := map[string]interface{}{
			"name":            "Unauthorized Rule",
			"enabled":         true,
			"triggerOnCreate": true,
			"triggerOnUpdate": false,
			"triggerOnDelete": false,
			"conditionLogic":  "AND",
			"conditions": []map[string]interface{}{
				{
					"fieldName": "lastName",
					"operator":  "IS_EMPTY",
				},
			},
			"actions": []map[string]interface{}{
				{
					"type":         "BLOCK_SAVE",
					"errorMessage": "Test",
				},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", ruleBody, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot list validation rules", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/admin/entities/Contact/validation-rules", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})
}
