package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// ==========================================
// ACCOUNT CRUD TESTS
// ==========================================

func TestAccount_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	t.Run("creates account with required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Acme Corporation",
		}

		var response struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected account ID to be set")
		}
		if response.Name != "Acme Corporation" {
			t.Errorf("Expected name 'Acme Corporation', got %s", response.Name)
		}
	})

	t.Run("creates account with all fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name":                   "Full Account Corp",
			"website":                "https://example.com",
			"emailAddress":           "contact@example.com",
			"phoneNumber":            "555-9876",
			"type":                   "Customer",
			"industry":               "Technology",
			"sicCode":                "1234",
			"billingAddressStreet":   "100 Business Ave",
			"billingAddressCity":     "San Francisco",
			"billingAddressState":    "CA",
			"billingAddressCountry":  "USA",
			"billingAddressPostal":   "94102",
			"shippingAddressStreet":  "200 Shipping Blvd",
			"shippingAddressCity":    "Oakland",
			"shippingAddressState":   "CA",
			"shippingAddressCountry": "USA",
			"shippingAddressPostal":  "94601",
			"description":            "A full-featured test account",
		}

		var response struct {
			ID                  string `json:"id"`
			Name                string `json:"name"`
			Website             string `json:"website"`
			EmailAddress        string `json:"emailAddress"`
			Type                string `json:"type"`
			Industry            string `json:"industry"`
			BillingAddressCity  string `json:"billingAddressCity"`
			ShippingAddressCity string `json:"shippingAddressCity"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Name != "Full Account Corp" {
			t.Errorf("Expected name 'Full Account Corp', got %s", response.Name)
		}
		if response.Website != "https://example.com" {
			t.Errorf("Expected website 'https://example.com', got %s", response.Website)
		}
		if response.BillingAddressCity != "San Francisco" {
			t.Errorf("Expected billingAddressCity 'San Francisco', got %s", response.BillingAddressCity)
		}
	})

	t.Run("fails without name", func(t *testing.T) {
		body := map[string]interface{}{
			"website": "https://example.com",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/accounts/", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails without authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Unauthorized Account",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/accounts/", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestAccount_Get(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create an account
	createBody := map[string]interface{}{
		"name":    "Get Test Account",
		"website": "https://gettest.com",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("gets account by ID", func(t *testing.T) {
		var response struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Website string `json:"website"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/"+created.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
		}
		if response.Name != "Get Test Account" {
			t.Errorf("Expected name 'Get Test Account', got %s", response.Name)
		}
	})

	t.Run("returns 404 for non-existent account", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/accounts/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestAccount_List(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create multiple accounts
	accounts := []map[string]interface{}{
		{"name": "Alpha Corp", "type": "Customer"},
		{"name": "Beta Inc", "type": "Partner"},
		{"name": "Gamma LLC", "type": "Customer"},
	}

	for _, a := range accounts {
		resp := app.MakeRequest(t, "POST", "/api/v1/accounts/", a, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	}

	t.Run("lists all accounts", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			Total      int           `json:"total"`
			Page       int           `json:"page"`
			PageSize   int           `json:"pageSize"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 accounts, got %d", len(response.Data))
		}
	})

	t.Run("paginates accounts", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			Total      int           `json:"total"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/?page=1&pageSize=2", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 2 {
			t.Errorf("Expected 2 accounts, got %d", len(response.Data))
		}
		if response.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got %d", response.TotalPages)
		}
	})

	t.Run("searches accounts", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/?search=Alpha", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 1 {
			t.Errorf("Expected 1 account matching 'Alpha', got %d", len(response.Data))
		}
	})

	t.Run("sorts accounts", func(t *testing.T) {
		var response struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/?sortBy=name&sortDir=asc", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) < 2 {
			t.Skip("Not enough accounts to test sorting")
		}
		if response.Data[0].Name != "Alpha Corp" {
			t.Errorf("Expected first account to be 'Alpha Corp', got %s", response.Data[0].Name)
		}
	})
}

func TestAccount_Update(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create an account
	createBody := map[string]interface{}{
		"name":    "Original Account",
		"website": "https://original.com",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("updates account with PUT", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name":    "Updated Account",
			"website": "https://updated.com",
		}

		var response struct {
			Name    string `json:"name"`
			Website string `json:"website"`
		}

		resp := app.MakeRequestWithResponse(t, "PUT", "/api/v1/accounts/"+created.ID, updateBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Name != "Updated Account" {
			t.Errorf("Expected name 'Updated Account', got %s", response.Name)
		}
		if response.Website != "https://updated.com" {
			t.Errorf("Expected website 'https://updated.com', got %s", response.Website)
		}
	})

	t.Run("partial update with PATCH", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"industry": "Technology",
		}

		var response struct {
			Name     string `json:"name"`
			Industry string `json:"industry"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/accounts/"+created.ID, patchBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Name != "Updated Account" {
			t.Errorf("Expected name to remain 'Updated Account', got %s", response.Name)
		}
		if response.Industry != "Technology" {
			t.Errorf("Expected industry 'Technology', got %s", response.Industry)
		}
	})

	t.Run("returns 404 for non-existent account", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name": "Test",
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/accounts/nonexistent-id", updateBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestAccount_Delete(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create an account
	createBody := map[string]interface{}{
		"name": "ToDelete Account",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("deletes account", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/accounts/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNoContent)

		// Verify account is no longer accessible
		resp = app.MakeRequest(t, "GET", "/api/v1/accounts/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for non-existent account", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/accounts/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestAccount_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "password123", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "password123", "Org Two")

	// User 1 creates an account
	createBody := map[string]interface{}{
		"name": "Private Account",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", createBody, user1.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("user cannot access other org's accounts", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/accounts/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot update other org's accounts", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name": "Hacked Account",
		}
		resp := app.MakeRequest(t, "PUT", "/api/v1/accounts/"+created.ID, updateBody, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot delete other org's accounts", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/accounts/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user list does not include other org's accounts", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/", nil, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Total != 0 {
			t.Errorf("Expected 0 accounts for User 2, got %d", response.Total)
		}
	})
}

func TestAccount_ContactRelationship(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create an account
	accountBody := map[string]interface{}{
		"name": "Parent Account",
	}
	var account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", accountBody, user.AccessToken, &account)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("creates contact linked to account", func(t *testing.T) {
		contactBody := map[string]interface{}{
			"firstName":   "Linked",
			"lastName":    "Contact",
			"accountId":   account.ID,
			"accountName": account.Name,
		}

		var contact struct {
			ID          string  `json:"id"`
			AccountID   *string `json:"accountId"`
			AccountName string  `json:"accountName"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
		AssertStatus(t, resp, http.StatusCreated)

		if contact.AccountID == nil || *contact.AccountID != account.ID {
			t.Errorf("Expected accountId %s, got %v", account.ID, contact.AccountID)
		}
		if contact.AccountName != "Parent Account" {
			t.Errorf("Expected accountName 'Parent Account', got %s", contact.AccountName)
		}
	})
}

func TestAccount_ListTasks(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	// Create an account
	accountBody := map[string]interface{}{
		"name": "Account With Tasks",
	}
	var account struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", accountBody, user.AccessToken, &account)
	AssertStatus(t, resp, http.StatusCreated)

	// Create tasks linked to the account
	for i := 0; i < 3; i++ {
		taskBody := map[string]interface{}{
			"name":       fmt.Sprintf("Account Task %d", i+1),
			"type":       "Todo",
			"parentType": "Account",
			"parentId":   account.ID,
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/tasks/", taskBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	}

	t.Run("lists tasks for account", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/accounts/"+account.ID+"/tasks", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 tasks, got %d", len(response.Data))
		}
	})

	t.Run("returns 404 for non-existent account", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/accounts/nonexistent/tasks", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestAccount_CustomFields(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "account@example.com", "password123", "Account Test Org")

	t.Run("creates account with custom fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Custom Account",
			"customFields": map[string]interface{}{
				"annualRevenue": 1000000,
				"employees":     50,
				"isEnterprise":  true,
			},
		}

		var response struct {
			ID           string                 `json:"id"`
			CustomFields map[string]interface{} `json:"customFields"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.CustomFields == nil {
			t.Error("Expected customFields to be set")
		}
		if response.CustomFields["annualRevenue"] != float64(1000000) {
			t.Errorf("Expected annualRevenue 1000000, got %v", response.CustomFields["annualRevenue"])
		}
	})
}
