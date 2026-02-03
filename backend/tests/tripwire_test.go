package tests

import (
	"net/http"
	"testing"
)

// ==========================================
// TRIPWIRE (WEBHOOK) TESTS
// ==========================================

func TestTripwire_CRUD(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Tripwire Test Org")

	t.Run("admin can create tripwire", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Contact Created Webhook",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/contact-created",
			"httpMethod": "POST",
			"headers": map[string]string{
				"X-Custom-Header": "value",
			},
			"conditions": []map[string]interface{}{},
		}

		var response struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			EntityType string `json:"entityType"`
			Event      string `json:"event"`
			IsActive   bool   `json:"isActive"`
			WebhookUrl string `json:"webhookUrl"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected tripwire ID to be set")
		}
		if response.Name != "Contact Created Webhook" {
			t.Errorf("Expected name 'Contact Created Webhook', got %s", response.Name)
		}
		if response.EntityType != "Contact" {
			t.Errorf("Expected entityType 'Contact', got %s", response.EntityType)
		}
		if response.Event != "CREATE" {
			t.Errorf("Expected event 'CREATE', got %s", response.Event)
		}
	})

	t.Run("creates tripwire with conditions", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "VIP Contact Webhook",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/vip",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{
				{
					"field":    "description",
					"operator": "CONTAINS",
					"value":    "VIP",
				},
			},
			"conditionLogic": "AND",
		}

		var response struct {
			ID         string `json:"id"`
			Conditions []struct {
				Field    string `json:"field"`
				Operator string `json:"operator"`
				Value    string `json:"value"`
			} `json:"conditions"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if len(response.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(response.Conditions))
		}
	})

	t.Run("lists tripwires", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tripwires/", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) < 2 {
			t.Errorf("Expected at least 2 tripwires, got %d", len(response.Data))
		}
	})

	t.Run("gets tripwire by ID", func(t *testing.T) {
		// First create a tripwire
		createBody := map[string]interface{}{
			"name":       "Get Test Tripwire",
			"entityType": "Account",
			"event":      "UPDATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/account",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Now get it
		var response struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		resp = app.MakeRequestWithResponse(t, "GET", "/api/v1/tripwires/"+created.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
		}
	})

	t.Run("updates tripwire", func(t *testing.T) {
		// First create a tripwire
		createBody := map[string]interface{}{
			"name":       "Update Test Tripwire",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/original",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Update it
		updateBody := map[string]interface{}{
			"name":       "Updated Tripwire Name",
			"webhookUrl": "https://webhook.example.com/updated",
			"isActive":   false,
		}

		var updated struct {
			Name       string `json:"name"`
			WebhookUrl string `json:"webhookUrl"`
			IsActive   bool   `json:"isActive"`
		}
		resp = app.MakeRequestWithResponse(t, "PUT", "/api/v1/tripwires/"+created.ID, updateBody, user.AccessToken, &updated)
		AssertStatus(t, resp, http.StatusOK)

		if updated.Name != "Updated Tripwire Name" {
			t.Errorf("Expected name 'Updated Tripwire Name', got %s", updated.Name)
		}
		if updated.IsActive != false {
			t.Error("Expected isActive to be false")
		}
	})

	t.Run("deletes tripwire", func(t *testing.T) {
		// First create a tripwire
		createBody := map[string]interface{}{
			"name":       "Delete Test Tripwire",
			"entityType": "Contact",
			"event":      "DELETE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/delete",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Delete it
		resp = app.MakeRequest(t, "DELETE", "/api/v1/tripwires/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNoContent)

		// Verify it's deleted
		resp = app.MakeRequest(t, "GET", "/api/v1/tripwires/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTripwire_Events(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Tripwire Test Org")

	t.Run("creates tripwire for CREATE event", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Create Event Tripwire",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/create",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var response struct {
			Event string `json:"event"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Event != "CREATE" {
			t.Errorf("Expected event 'CREATE', got %s", response.Event)
		}
	})

	t.Run("creates tripwire for UPDATE event", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Update Event Tripwire",
			"entityType": "Contact",
			"event":      "UPDATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/update",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var response struct {
			Event string `json:"event"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Event != "UPDATE" {
			t.Errorf("Expected event 'UPDATE', got %s", response.Event)
		}
	})

	t.Run("creates tripwire for DELETE event", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Delete Event Tripwire",
			"entityType": "Contact",
			"event":      "DELETE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/delete",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		var response struct {
			Event string `json:"event"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Event != "DELETE" {
			t.Errorf("Expected event 'DELETE', got %s", response.Event)
		}
	})
}

func TestTripwire_EntityTypes(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Tripwire Test Org")

	entityTypes := []string{"Contact", "Account", "Task"}

	for _, entityType := range entityTypes {
		t.Run("creates tripwire for "+entityType, func(t *testing.T) {
			body := map[string]interface{}{
				"name":       entityType + " Tripwire",
				"entityType": entityType,
				"event":      "CREATE",
				"isActive":   true,
				"webhookUrl": "https://webhook.example.com/" + entityType,
				"httpMethod": "POST",
				"conditions": []map[string]interface{}{},
			}

			var response struct {
				EntityType string `json:"entityType"`
			}

			resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
			AssertStatus(t, resp, http.StatusCreated)

			if response.EntityType != entityType {
				t.Errorf("Expected entityType '%s', got %s", entityType, response.EntityType)
			}
		})
	}
}

func TestTripwire_HttpMethods(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Tripwire Test Org")

	httpMethods := []string{"POST", "PUT", "PATCH"}

	for _, method := range httpMethods {
		t.Run("creates tripwire with "+method+" method", func(t *testing.T) {
			body := map[string]interface{}{
				"name":       method + " Method Tripwire",
				"entityType": "Contact",
				"event":      "CREATE",
				"isActive":   true,
				"webhookUrl": "https://webhook.example.com/test",
				"httpMethod": method,
				"conditions": []map[string]interface{}{},
			}

			var response struct {
				HttpMethod string `json:"httpMethod"`
			}

			resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
			AssertStatus(t, resp, http.StatusCreated)

			if response.HttpMethod != method {
				t.Errorf("Expected httpMethod '%s', got %s", method, response.HttpMethod)
			}
		})
	}
}

func TestTripwire_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "password123", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "password123", "Org Two")

	// User 1 creates a tripwire
	createBody := map[string]interface{}{
		"name":       "Private Tripwire",
		"entityType": "Contact",
		"event":      "CREATE",
		"isActive":   true,
		"webhookUrl": "https://webhook.example.com/private",
		"httpMethod": "POST",
		"conditions": []map[string]interface{}{},
	}
	var created struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", createBody, user1.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("user cannot access other org's tripwires", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tripwires/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot update other org's tripwires", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name": "Hacked Tripwire",
		}
		resp := app.MakeRequest(t, "PUT", "/api/v1/tripwires/"+created.ID, updateBody, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot delete other org's tripwires", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/tripwires/"+created.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user list does not include other org's tripwires", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tripwires/", nil, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Total != 0 {
			t.Errorf("Expected 0 tripwires for User 2, got %d", response.Total)
		}
	})
}

func TestTripwire_NonAdminAccess(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create admin and invite a regular user
	admin := app.CreateTestUser(t, "admin@example.com", "password123", "Test Org")

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
		AccessToken string `json:"accessToken"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/accept-invite", acceptBody, "", &acceptResp)
	AssertStatus(t, resp, http.StatusOK)

	regularUserToken := acceptResp.AccessToken

	t.Run("regular user cannot create tripwires", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Unauthorized Tripwire",
			"entityType": "Contact",
			"event":      "CREATE",
			"isActive":   true,
			"webhookUrl": "https://webhook.example.com/unauthorized",
			"httpMethod": "POST",
			"conditions": []map[string]interface{}{},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/tripwires/", body, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot list tripwires", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tripwires/", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})
}
