package tests

import (
	"encoding/json"
	"io"
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
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/contact-created",
			"conditions": []map[string]interface{}{
				{"type": "ISNEW"}, // Condition for CREATE events
			},
		}

		var response struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			EntityType  string `json:"entityType"`
			Enabled     bool   `json:"enabled"`
			EndpointUrl string `json:"endpointUrl"`
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
	})

	t.Run("creates tripwire with field conditions", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "VIP Contact Webhook",
			"entityType": "Contact",
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/vip",
			"conditions": []map[string]interface{}{
				{
					"type":      "FIELD_EQUALS",
					"fieldName": "description",
					"value":     "VIP",
				},
			},
			"conditionLogic": "AND",
		}

		var response struct {
			ID         string `json:"id"`
			Conditions []struct {
				Type      string  `json:"type"`
				FieldName *string `json:"fieldName"`
				Value     *string `json:"value"`
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
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/account",
			"conditions": []map[string]interface{}{
				{"type": "ISCHANGED"},
			},
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
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/original",
			"conditions": []map[string]interface{}{
				{"type": "ISNEW"},
			},
		}

		var created struct {
			ID string `json:"id"`
		}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", createBody, user.AccessToken, &created)
		AssertStatus(t, resp, http.StatusCreated)

		// Update it
		updateBody := map[string]interface{}{
			"name":        "Updated Tripwire Name",
			"endpointUrl": "https://webhook.example.com/updated",
			"enabled":     false,
		}

		var updated struct {
			Name        string `json:"name"`
			EndpointUrl string `json:"endpointUrl"`
			Enabled     bool   `json:"enabled"`
		}
		resp = app.MakeRequestWithResponse(t, "PUT", "/api/v1/tripwires/"+created.ID, updateBody, user.AccessToken, &updated)
		AssertStatus(t, resp, http.StatusOK)

		if updated.Name != "Updated Tripwire Name" {
			t.Errorf("Expected name 'Updated Tripwire Name', got %s", updated.Name)
		}
		if updated.Enabled != false {
			t.Error("Expected enabled to be false")
		}
	})

	t.Run("deletes tripwire", func(t *testing.T) {
		// First create a tripwire
		createBody := map[string]interface{}{
			"name":       "Delete Test Tripwire",
			"entityType": "Contact",
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/delete",
			"conditions": []map[string]interface{}{
				{"type": "ISDELETED"},
			},
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

func TestTripwire_ConditionTypes(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "admin@example.com", "password123", "Tripwire Test Org")

	conditionTypes := []struct {
		name     string
		condType string
	}{
		{"ISNEW (Create)", "ISNEW"},
		{"ISCHANGED (Update)", "ISCHANGED"},
		{"ISDELETED (Delete)", "ISDELETED"},
	}

	for _, tc := range conditionTypes {
		t.Run("creates tripwire for "+tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"name":       tc.name + " Tripwire",
				"entityType": "Contact",
				"enabled":    true,
				"endpointUrl": "https://webhook.example.com/test",
				"conditions": []map[string]interface{}{
					{"type": tc.condType},
				},
			}

			var response struct {
				Conditions []struct {
					Type string `json:"type"`
				} `json:"conditions"`
			}

			resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tripwires/", body, user.AccessToken, &response)
			AssertStatus(t, resp, http.StatusCreated)

			if len(response.Conditions) != 1 || response.Conditions[0].Type != tc.condType {
				t.Errorf("Expected condition type '%s', got %+v", tc.condType, response.Conditions)
			}
		})
	}
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
				"enabled":    true,
				"endpointUrl": "https://webhook.example.com/" + entityType,
				"conditions": []map[string]interface{}{
					{"type": "ISNEW"},
				},
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
		"enabled":    true,
		"endpointUrl": "https://webhook.example.com/private",
		"conditions": []map[string]interface{}{
			{"type": "ISNEW"},
		},
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
		Token string `json:"token"` // Token is at top level, not nested
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", inviteBody, admin.AccessToken, &inviteResp)
	AssertStatus(t, resp, http.StatusCreated)

	// Accept the invitation
	acceptBody := map[string]interface{}{
		"token":    inviteResp.Token,
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
			"enabled":    true,
			"endpointUrl": "https://webhook.example.com/unauthorized",
			"conditions": []map[string]interface{}{
				{"type": "ISNEW"},
			},
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/tripwires/", body, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("regular user cannot list tripwires", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tripwires/", nil, regularUserToken)
		AssertStatus(t, resp, http.StatusForbidden)
	})
}

// Helper function to read token from response
func readTokenFromResponse(t *testing.T, resp *http.Response) string {
	t.Helper()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(bodyBytes, &result)
	return result.Token
}
