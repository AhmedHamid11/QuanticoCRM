package tests

import (
	"net/http"
	"testing"
)

// ==========================================
// BULK OPERATIONS TESTS
// ==========================================

func TestBulk_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "Qw!x7Km9pZr2", "Bulk Test Org")

	t.Run("bulk creates multiple contacts", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"lastName": "Smith", "firstName": "John"},
				{"lastName": "Doe", "firstName": "Jane"},
				{"lastName": "Wilson", "firstName": "Bob"},
			},
		}

		var response struct {
			Created int      `json:"created"`
			Failed  int      `json:"failed"`
			IDs     []string `json:"ids"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/entities/Contact/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated) // 201 for successful bulk create

		if response.Created != 3 {
			t.Errorf("Expected 3 created records, got %d", response.Created)
		}
		if len(response.IDs) != 3 {
			t.Errorf("Expected 3 IDs, got %d", len(response.IDs))
		}
	})

	t.Run("bulk creates multiple accounts", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"name": "Acme Corp"},
				{"name": "Tech Solutions"},
				{"name": "Global Industries"},
			},
		}

		var response struct {
			Created int      `json:"created"`
			Failed  int      `json:"failed"`
			IDs     []string `json:"ids"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/entities/Account/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated) // 201 for successful bulk create

		if response.Created != 3 {
			t.Errorf("Expected 3 created accounts, got %d", response.Created)
		}
	})

	t.Run("handles partial failures", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"lastName": "Valid"},     // Valid
				{"firstName": "NoLast"},   // Invalid - missing lastName
				{"lastName": "AlsoValid"}, // Valid
			},
		}

		var response struct {
			Created int `json:"created"`
			Failed  int `json:"failed"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/entities/Contact/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusUnprocessableEntity) // 422 when any records fail

		if response.Failed != 1 {
			t.Errorf("Expected 1 failed, got %d", response.Failed)
		}
	})

	t.Run("fails without authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"lastName": "Test"},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/entities/Contact/bulk", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("fails with empty records", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/entities/Contact/bulk", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestBulk_Update(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "Qw!x7Km9pZr2", "Bulk Test Org")

	// Create contacts to update
	var contactIDs []string
	for i := 0; i < 3; i++ {
		contactBody := map[string]interface{}{
			"lastName": "Original",
		}
		var contact struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
		AssertStatus(t, resp, http.StatusCreated)
		contactIDs = append(contactIDs, contact.ID)
	}

	t.Run("bulk updates multiple contacts", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"id": contactIDs[0], "firstName": "Updated1"},
				{"id": contactIDs[1], "firstName": "Updated2"},
				{"id": contactIDs[2], "firstName": "Updated3"},
			},
		}

		var response struct {
			Updated int      `json:"updated"`
			Failed  int      `json:"failed"`
			IDs     []string `json:"ids"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/entities/Contact/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Updated != 3 {
			t.Errorf("Expected 3 updated records, got %d", response.Updated)
		}
	})

	t.Run("handles non-existent records", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"id": contactIDs[0], "firstName": "Valid"},
				{"id": "nonexistent-id", "firstName": "Invalid"},
			},
		}

		var response struct {
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/entities/Contact/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusUnprocessableEntity) // 422 when any records fail

		if response.Failed != 1 {
			t.Errorf("Expected 1 failed, got %d", response.Failed)
		}
	})
}

func TestBulk_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "Qw!x7Km9pZr2", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "Qw!x7Km9pZr2", "Org Two")

	// User 1 creates a contact
	contactBody := map[string]interface{}{
		"lastName": "Org1Contact",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user1.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("user cannot bulk update other org's records", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"id": contact.ID, "firstName": "Hacked"},
			},
		}

		var response struct {
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/entities/Contact/bulk", body, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusUnprocessableEntity) // 422 when record not found

		// Should have 0 updates and 1 error (not found)
		if response.Failed != 1 {
			t.Errorf("Expected 1 failed, got %d", response.Failed)
		}
	})
}

func TestBulk_ValidationRules(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "Qw!x7Km9pZr2", "Bulk Test Org")

	// Create a validation rule that requires email for VIP contacts
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
				"errorMessage": "Email required for VIP",
			},
		},
	}
	resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Contact/validation-rules", ruleBody, user.AccessToken)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("validation rules apply to bulk create", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"lastName": "Regular"},                                                              // Should pass
				{"lastName": "VIPNoEmail", "description": "VIP"},                                     // Should fail - VIP without email
				{"lastName": "VIPWithEmail", "description": "VIP", "emailAddress": "vip@example.com"}, // Should pass
			},
		}

		var response struct {
			Created int `json:"created"`
			Failed  int `json:"failed"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/entities/Contact/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusUnprocessableEntity) // 422 when validation fails

		if response.Failed != 1 {
			t.Errorf("Expected 1 failed, got %d", response.Failed)
		}
	})
}

func TestBulk_Tasks(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "Qw!x7Km9pZr2", "Bulk Test Org")

	// Create a contact to link tasks to
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("bulk creates multiple tasks", func(t *testing.T) {
		body := map[string]interface{}{
			"records": []map[string]interface{}{
				{"subject": "Task 1", "type": "Todo", "parentType": "Contact", "parentId": contact.ID},
				{"subject": "Task 2", "type": "Todo", "parentType": "Contact", "parentId": contact.ID},
				{"subject": "Task 3", "type": "Todo", "parentType": "Contact", "parentId": contact.ID},
			},
		}

		var response struct {
			Created int      `json:"created"`
			Failed  int      `json:"failed"`
			IDs     []string `json:"ids"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/entities/Task/bulk", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated) // 201 for successful bulk create

		if response.Created != 3 {
			t.Errorf("Expected 3 created tasks, got %d", response.Created)
		}
	})
}
