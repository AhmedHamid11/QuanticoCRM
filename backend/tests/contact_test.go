package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// ==========================================
// CONTACT CRUD TESTS
// ==========================================

func TestContact_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	t.Run("creates contact with required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"lastName": "Smith",
		}

		var response struct {
			ID       string `json:"id"`
			LastName string `json:"lastName"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected contact ID to be set")
		}
		if response.LastName != "Smith" {
			t.Errorf("Expected lastName 'Smith', got %s", response.LastName)
		}
	})

	t.Run("creates contact with all fields", func(t *testing.T) {
		body := map[string]interface{}{
			"salutationName":    "Mr.",
			"firstName":         "John",
			"lastName":          "Doe",
			"emailAddress":      "john.doe@example.com",
			"phoneNumber":       "555-1234",
			"phoneNumberType":   "Mobile",
			"doNotCall":         false,
			"description":       "Test contact description",
			"addressStreet":     "123 Main St",
			"addressCity":       "New York",
			"addressState":      "NY",
			"addressCountry":    "USA",
			"addressPostalCode": "10001",
		}

		var response struct {
			ID              string `json:"id"`
			SalutationName  string `json:"salutationName"`
			FirstName       string `json:"firstName"`
			LastName        string `json:"lastName"`
			EmailAddress    string `json:"emailAddress"`
			PhoneNumber     string `json:"phoneNumber"`
			PhoneNumberType string `json:"phoneNumberType"`
			DoNotCall       bool   `json:"doNotCall"`
			Description     string `json:"description"`
			AddressStreet   string `json:"addressStreet"`
			AddressCity     string `json:"addressCity"`
			AddressState    string `json:"addressState"`
			AddressCountry  string `json:"addressCountry"`
			AddressPostal   string `json:"addressPostalCode"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.FirstName != "John" {
			t.Errorf("Expected firstName 'John', got %s", response.FirstName)
		}
		if response.LastName != "Doe" {
			t.Errorf("Expected lastName 'Doe', got %s", response.LastName)
		}
		if response.EmailAddress != "john.doe@example.com" {
			t.Errorf("Expected emailAddress 'john.doe@example.com', got %s", response.EmailAddress)
		}
		if response.AddressCity != "New York" {
			t.Errorf("Expected addressCity 'New York', got %s", response.AddressCity)
		}
	})

	t.Run("fails without lastName", func(t *testing.T) {
		body := map[string]interface{}{
			"firstName": "John",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails without authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"lastName": "Smith",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestContact_Get(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	// Create a contact
	createBody := map[string]interface{}{
		"firstName": "Jane",
		"lastName":  "Doe",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("gets contact by ID", func(t *testing.T) {
		var response struct {
			ID        string `json:"id"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/"+created.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
		}
		if response.FirstName != "Jane" {
			t.Errorf("Expected firstName 'Jane', got %s", response.FirstName)
		}
	})

	t.Run("returns 404 for non-existent contact", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("fails without authentication", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+created.ID, nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestContact_List(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	// Create multiple contacts
	contacts := []map[string]interface{}{
		{"firstName": "Alice", "lastName": "Anderson"},
		{"firstName": "Bob", "lastName": "Brown"},
		{"firstName": "Charlie", "lastName": "Clark"},
	}

	for _, c := range contacts {
		resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", c, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	}

	t.Run("lists all contacts", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			Total      int           `json:"total"`
			Page       int           `json:"page"`
			PageSize   int           `json:"pageSize"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 contacts, got %d", len(response.Data))
		}
		if response.Total != 3 {
			t.Errorf("Expected total 3, got %d", response.Total)
		}
	})

	t.Run("paginates contacts", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			Total      int           `json:"total"`
			Page       int           `json:"page"`
			PageSize   int           `json:"pageSize"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/?page=1&pageSize=2", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 2 {
			t.Errorf("Expected 2 contacts, got %d", len(response.Data))
		}
		if response.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got %d", response.TotalPages)
		}
	})

	t.Run("searches contacts", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/?search=Alice", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 1 {
			t.Errorf("Expected 1 contact matching 'Alice', got %d", len(response.Data))
		}
	})

	t.Run("sorts contacts", func(t *testing.T) {
		var response struct {
			Data []struct {
				LastName string `json:"lastName"`
			} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/?sortBy=lastName&sortDir=asc", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) < 2 {
			t.Skip("Not enough contacts to test sorting")
		}
		if response.Data[0].LastName != "Anderson" {
			t.Errorf("Expected first contact to be 'Anderson', got %s", response.Data[0].LastName)
		}
	})
}

func TestContact_Update(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	// Create a contact
	createBody := map[string]interface{}{
		"firstName": "Original",
		"lastName":  "Name",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("updates contact with PUT", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"firstName": "Updated",
			"lastName":  "Name",
		}

		var response struct {
			FirstName string `json:"firstName"`
		}

		resp := app.MakeRequestWithResponse(t, "PUT", "/api/v1/contacts/"+created.ID, updateBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.FirstName != "Updated" {
			t.Errorf("Expected firstName 'Updated', got %s", response.FirstName)
		}
	})

	t.Run("partial update with PATCH", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"emailAddress": "updated@example.com",
		}

		var response struct {
			FirstName    string `json:"firstName"`
			EmailAddress string `json:"emailAddress"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/contacts/"+created.ID, patchBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		// FirstName should remain from previous update
		if response.FirstName != "Updated" {
			t.Errorf("Expected firstName 'Updated', got %s", response.FirstName)
		}
		if response.EmailAddress != "updated@example.com" {
			t.Errorf("Expected emailAddress 'updated@example.com', got %s", response.EmailAddress)
		}
	})

	t.Run("returns 404 for non-existent contact", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"firstName": "Test",
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/contacts/nonexistent-id", updateBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestContact_Delete(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	// Create a contact
	createBody := map[string]interface{}{
		"lastName": "ToDelete",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("deletes contact", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/contacts/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNoContent)

		// Verify contact is no longer accessible
		resp = app.MakeRequest(t, "GET", "/api/v1/contacts/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for non-existent contact", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/contacts/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestContact_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "Qw!x7Km9pZr2", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "Qw!x7Km9pZr2", "Org Two")

	// User 1 creates a contact
	createBody := map[string]interface{}{
		"firstName": "Private",
		"lastName":  "Contact",
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", createBody, user1.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("user cannot access other org's contacts", func(t *testing.T) {
		// User 2 tries to get User 1's contact
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot update other org's contacts", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"firstName": "Hacked",
		}
		resp := app.MakeRequest(t, "PUT", "/api/v1/contacts/"+created.ID, updateBody, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot delete other org's contacts", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/contacts/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user list does not include other org's contacts", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/", nil, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Total != 0 {
			t.Errorf("Expected 0 contacts for User 2, got %d", response.Total)
		}
	})
}

func TestContact_CustomFields(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	t.Run("creates contact with custom fields", func(t *testing.T) {
		body := map[string]interface{}{
			"lastName": "Custom",
			"customFields": map[string]interface{}{
				"customField1": "value1",
				"customField2": 123,
				"customBool":   true,
			},
		}

		var response struct {
			ID           string                 `json:"id"`
			CustomFields map[string]interface{} `json:"customFields"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.CustomFields == nil {
			t.Error("Expected customFields to be set")
		}
		if response.CustomFields["customField1"] != "value1" {
			t.Errorf("Expected customField1 'value1', got %v", response.CustomFields["customField1"])
		}
	})

	t.Run("updates custom fields", func(t *testing.T) {
		// Create contact
		createBody := map[string]interface{}{
			"lastName": "CustomUpdate",
			"customFields": map[string]interface{}{
				"field1": "original",
			},
		}
		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Update custom fields
		updateBody := map[string]interface{}{
			"customFields": map[string]interface{}{
				"field1": "updated",
				"field2": "new",
			},
		}
		var updated struct {
			CustomFields map[string]interface{} `json:"customFields"`
		}
		resp = app.MakeRequestWithResponse(t, "PATCH", "/api/v1/contacts/"+created.ID, updateBody, user.AccessToken, &updated)
		AssertStatus(t, resp, http.StatusOK)

		if updated.CustomFields["field1"] != "updated" {
			t.Errorf("Expected field1 'updated', got %v", updated.CustomFields["field1"])
		}
	})
}

func TestContact_ListTasks(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "Qw!x7Km9pZr2", "Contact Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "WithTasks",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create tasks linked to the contact
	for i := 0; i < 3; i++ {
		taskBody := map[string]interface{}{
			"name":       fmt.Sprintf("Task %d", i+1),
			"type":       "Todo",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/tasks/", taskBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	}

	t.Run("lists tasks for contact", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/contacts/"+contact.ID+"/tasks", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 tasks, got %d", len(response.Data))
		}
	})

	t.Run("returns 404 for non-existent contact", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/contacts/nonexistent/tasks", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}
