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

	user := app.CreateTestUser(t, "bulk@example.com", "password123", "Bulk Test Org")

	t.Run("bulk creates multiple contacts", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"lastName": "Smith", "firstName": "John"},
				{"lastName": "Doe", "firstName": "Jane"},
				{"lastName": "Wilson", "firstName": "Bob"},
			},
		}

		var response struct {
			Created []struct {
				ID       string `json:"id"`
				LastName string `json:"lastName"`
			} `json:"created"`
			Errors []interface{} `json:"errors"`
			Total  int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/create", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Created) != 3 {
			t.Errorf("Expected 3 created records, got %d", len(response.Created))
		}
		if response.Total != 3 {
			t.Errorf("Expected total 3, got %d", response.Total)
		}
	})

	t.Run("bulk creates multiple accounts", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Account",
			"records": []map[string]interface{}{
				{"name": "Acme Corp"},
				{"name": "Tech Solutions"},
				{"name": "Global Industries"},
			},
		}

		var response struct {
			Created []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"created"`
			Total int `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/create", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Created) != 3 {
			t.Errorf("Expected 3 created accounts, got %d", len(response.Created))
		}
	})

	t.Run("handles partial failures", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"lastName": "Valid"},     // Valid
				{"firstName": "NoLast"},   // Invalid - missing lastName
				{"lastName": "AlsoValid"}, // Valid
			},
		}

		var response struct {
			Created []interface{} `json:"created"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
			Total int `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/create", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Created) != 2 {
			t.Errorf("Expected 2 created records, got %d", len(response.Created))
		}
		if len(response.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(response.Errors))
		}
		if len(response.Errors) > 0 && response.Errors[0].Index != 1 {
			t.Errorf("Expected error at index 1, got %d", response.Errors[0].Index)
		}
	})

	t.Run("fails without authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"lastName": "Test"},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/bulk/create", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("fails with empty records", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records":    []map[string]interface{}{},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/bulk/create", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestBulk_Update(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "password123", "Bulk Test Org")

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
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"id": contactIDs[0], "firstName": "Updated1"},
				{"id": contactIDs[1], "firstName": "Updated2"},
				{"id": contactIDs[2], "firstName": "Updated3"},
			},
		}

		var response struct {
			Updated []struct {
				ID        string `json:"id"`
				FirstName string `json:"firstName"`
			} `json:"updated"`
			Errors []interface{} `json:"errors"`
			Total  int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/update", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Updated) != 3 {
			t.Errorf("Expected 3 updated records, got %d", len(response.Updated))
		}
	})

	t.Run("handles non-existent records", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"id": contactIDs[0], "firstName": "Valid"},
				{"id": "nonexistent-id", "firstName": "Invalid"},
			},
		}

		var response struct {
			Updated []interface{} `json:"updated"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/update", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Updated) != 1 {
			t.Errorf("Expected 1 updated record, got %d", len(response.Updated))
		}
		if len(response.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(response.Errors))
		}
	})
}

func TestBulk_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "password123", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "password123", "Org Two")

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
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"id": contact.ID, "firstName": "Hacked"},
			},
		}

		var response struct {
			Updated []interface{} `json:"updated"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/update", body, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		// Should have 0 updates and 1 error (not found)
		if len(response.Updated) != 0 {
			t.Errorf("Expected 0 updated records, got %d", len(response.Updated))
		}
		if len(response.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(response.Errors))
		}
	})
}

func TestBulk_ValidationRules(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "password123", "Bulk Test Org")

	// Create a validation rule that requires email for VIP contacts
	ruleBody := map[string]interface{}{
		"name":       "Require Email for VIP",
		"entityType": "Contact",
		"event":      "CREATE",
		"isActive":   true,
		"action":     "BLOCK_SAVE",
		"message":    "Email required for VIP",
		"conditions": []map[string]interface{}{
			{
				"field":    "description",
				"operator": "CONTAINS",
				"value":    "VIP",
			},
		},
		"validations": []map[string]interface{}{
			{
				"field":    "emailAddress",
				"operator": "NOT_EMPTY",
			},
		},
	}
	resp := app.MakeRequest(t, "POST", "/api/v1/validation-rules/", ruleBody, user.AccessToken)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("validation rules apply to bulk create", func(t *testing.T) {
		body := map[string]interface{}{
			"entityType": "Contact",
			"records": []map[string]interface{}{
				{"lastName": "Regular"},                             // Should pass
				{"lastName": "VIPNoEmail", "description": "VIP"},    // Should fail - VIP without email
				{"lastName": "VIPWithEmail", "description": "VIP", "emailAddress": "vip@example.com"}, // Should pass
			},
		}

		var response struct {
			Created []interface{} `json:"created"`
			Errors  []struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			} `json:"errors"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/create", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Created) != 2 {
			t.Errorf("Expected 2 created records, got %d", len(response.Created))
		}
		if len(response.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(response.Errors))
		}
		if len(response.Errors) > 0 && response.Errors[0].Index != 1 {
			t.Errorf("Expected error at index 1, got %d", response.Errors[0].Index)
		}
	})
}

func TestBulk_Tasks(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "bulk@example.com", "password123", "Bulk Test Org")

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
			"entityType": "Task",
			"records": []map[string]interface{}{
				{"name": "Task 1", "type": "Todo", "parentType": "Contact", "parentId": contact.ID},
				{"name": "Task 2", "type": "Call", "parentType": "Contact", "parentId": contact.ID},
				{"name": "Task 3", "type": "Email", "parentType": "Contact", "parentId": contact.ID},
			},
		}

		var response struct {
			Created []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"created"`
			Total int `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/bulk/create", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Created) != 3 {
			t.Errorf("Expected 3 created tasks, got %d", len(response.Created))
		}
	})
}
