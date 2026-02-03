package tests

import (
	"net/http"
	"testing"
)

// ==========================================
// TASK CRUD TESTS
// ==========================================

func TestTask_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact to link tasks to
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("creates task with required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Test Task",
			"type":       "Todo",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}

		var response struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Status     string `json:"status"`
			ParentType string `json:"parentType"`
			ParentID   string `json:"parentId"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected task ID to be set")
		}
		if response.Name != "Test Task" {
			t.Errorf("Expected name 'Test Task', got %s", response.Name)
		}
		if response.Type != "Todo" {
			t.Errorf("Expected type 'Todo', got %s", response.Type)
		}
		if response.Status != "Not Started" {
			t.Errorf("Expected default status 'Not Started', got %s", response.Status)
		}
	})

	t.Run("creates task with all fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "Full Task",
			"type":        "Call",
			"status":      "In Progress",
			"priority":    "High",
			"description": "Task description",
			"dueDate":     "2024-12-31",
			"parentType":  "Contact",
			"parentId":    contact.ID,
		}

		var response struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Priority    string `json:"priority"`
			Description string `json:"description"`
			DueDate     string `json:"dueDate"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Type != "Call" {
			t.Errorf("Expected type 'Call', got %s", response.Type)
		}
		if response.Status != "In Progress" {
			t.Errorf("Expected status 'In Progress', got %s", response.Status)
		}
		if response.Priority != "High" {
			t.Errorf("Expected priority 'High', got %s", response.Priority)
		}
	})

	t.Run("creates different task types", func(t *testing.T) {
		taskTypes := []string{"Todo", "Call", "Email", "Meeting"}

		for _, taskType := range taskTypes {
			body := map[string]interface{}{
				"name":       taskType + " Task",
				"type":       taskType,
				"parentType": "Contact",
				"parentId":   contact.ID,
			}

			var response struct {
				Type string `json:"type"`
			}

			resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", body, user.AccessToken, &response)
			AssertStatus(t, resp, http.StatusCreated)

			if response.Type != taskType {
				t.Errorf("Expected type '%s', got %s", taskType, response.Type)
			}
		}
	})

	t.Run("fails without name", func(t *testing.T) {
		body := map[string]interface{}{
			"type":       "Todo",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/tasks/", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails without authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Unauthorized Task",
			"type":       "Todo",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/tasks/", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestTask_Get(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create a task
	createBody := map[string]interface{}{
		"name":       "Get Test Task",
		"type":       "Todo",
		"parentType": "Contact",
		"parentId":   contact.ID,
	}
	var created struct {
		ID string `json:"id"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("gets task by ID", func(t *testing.T) {
		var response struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/"+created.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
		}
		if response.Name != "Get Test Task" {
			t.Errorf("Expected name 'Get Test Task', got %s", response.Name)
		}
	})

	t.Run("returns 404 for non-existent task", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tasks/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTask_List(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create multiple tasks
	tasks := []map[string]interface{}{
		{"name": "Task 1", "type": "Todo", "status": "Not Started", "parentType": "Contact", "parentId": contact.ID},
		{"name": "Task 2", "type": "Call", "status": "In Progress", "parentType": "Contact", "parentId": contact.ID},
		{"name": "Task 3", "type": "Meeting", "status": "Completed", "parentType": "Contact", "parentId": contact.ID},
	}

	for _, task := range tasks {
		resp := app.MakeRequest(t, "POST", "/api/v1/tasks/", task, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)
	}

	t.Run("lists all tasks", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			Total      int           `json:"total"`
			Page       int           `json:"page"`
			PageSize   int           `json:"pageSize"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 tasks, got %d", len(response.Data))
		}
	})

	t.Run("filters tasks by status", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/?status=Completed", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 1 {
			t.Errorf("Expected 1 completed task, got %d", len(response.Data))
		}
	})

	t.Run("filters tasks by type", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/?type=Call", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 1 {
			t.Errorf("Expected 1 call task, got %d", len(response.Data))
		}
	})

	t.Run("filters tasks by parent", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/?parentType=Contact&parentId="+contact.ID, nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 3 {
			t.Errorf("Expected 3 tasks for contact, got %d", len(response.Data))
		}
	})

	t.Run("paginates tasks", func(t *testing.T) {
		var response struct {
			Data       []interface{} `json:"data"`
			TotalPages int           `json:"totalPages"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/?page=1&pageSize=2", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) != 2 {
			t.Errorf("Expected 2 tasks, got %d", len(response.Data))
		}
		if response.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got %d", response.TotalPages)
		}
	})

	t.Run("sorts tasks", func(t *testing.T) {
		var response struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/?sortBy=name&sortDir=asc", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Data) < 2 {
			t.Skip("Not enough tasks to test sorting")
		}
		if response.Data[0].Name != "Task 1" {
			t.Errorf("Expected first task to be 'Task 1', got %s", response.Data[0].Name)
		}
	})
}

func TestTask_Update(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create a task
	createBody := map[string]interface{}{
		"name":       "Original Task",
		"type":       "Todo",
		"status":     "Not Started",
		"parentType": "Contact",
		"parentId":   contact.ID,
	}
	var created struct {
		ID string `json:"id"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("updates task with PUT", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name":   "Updated Task",
			"status": "In Progress",
		}

		var response struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}

		resp := app.MakeRequestWithResponse(t, "PUT", "/api/v1/tasks/"+created.ID, updateBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Name != "Updated Task" {
			t.Errorf("Expected name 'Updated Task', got %s", response.Name)
		}
		if response.Status != "In Progress" {
			t.Errorf("Expected status 'In Progress', got %s", response.Status)
		}
	})

	t.Run("marks task as completed", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"status": "Completed",
		}

		var response struct {
			Status string `json:"status"`
		}

		resp := app.MakeRequestWithResponse(t, "PATCH", "/api/v1/tasks/"+created.ID, patchBody, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Status != "Completed" {
			t.Errorf("Expected status 'Completed', got %s", response.Status)
		}
	})

	t.Run("returns 404 for non-existent task", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name": "Test",
		}

		resp := app.MakeRequest(t, "PUT", "/api/v1/tasks/nonexistent-id", updateBody, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTask_Delete(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create a task
	createBody := map[string]interface{}{
		"name":       "ToDelete Task",
		"type":       "Todo",
		"parentType": "Contact",
		"parentId":   contact.ID,
	}
	var created struct {
		ID string `json:"id"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", createBody, user.AccessToken, &created)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("deletes task", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/tasks/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNoContent)

		// Verify task is no longer accessible
		resp = app.MakeRequest(t, "GET", "/api/v1/tasks/"+created.ID, nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for non-existent task", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/tasks/nonexistent-id", nil, user.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTask_OrgIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two users in different organizations
	user1 := app.CreateTestUser(t, "user1@example.com", "password123", "Org One")
	user2 := app.CreateTestUser(t, "user2@example.com", "password123", "Org Two")

	// User 1 creates a contact and task
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user1.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	taskBody := map[string]interface{}{
		"name":       "Private Task",
		"type":       "Todo",
		"parentType": "Contact",
		"parentId":   contact.ID,
	}
	var task struct {
		ID string `json:"id"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", taskBody, user1.AccessToken, &task)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("user cannot access other org's tasks", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/tasks/"+task.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot update other org's tasks", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name": "Hacked Task",
		}
		resp := app.MakeRequest(t, "PUT", "/api/v1/tasks/"+task.ID, updateBody, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user cannot delete other org's tasks", func(t *testing.T) {
		resp := app.MakeRequest(t, "DELETE", "/api/v1/tasks/"+task.ID, nil, user2.AccessToken)
		AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("user list does not include other org's tasks", func(t *testing.T) {
		var response struct {
			Data  []interface{} `json:"data"`
			Total int           `json:"total"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/tasks/", nil, user2.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.Total != 0 {
			t.Errorf("Expected 0 tasks for User 2, got %d", response.Total)
		}
	})
}

func TestTask_ParentTypeValidation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "task@example.com", "password123", "Task Test Org")

	// Create a contact
	contactBody := map[string]interface{}{
		"lastName": "TaskParent",
	}
	var contact struct {
		ID string `json:"id"`
	}
	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", contactBody, user.AccessToken, &contact)
	AssertStatus(t, resp, http.StatusCreated)

	// Create an account
	accountBody := map[string]interface{}{
		"name": "Account Parent",
	}
	var account struct {
		ID string `json:"id"`
	}
	resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/accounts/", accountBody, user.AccessToken, &account)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("creates task linked to Contact", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Contact Task",
			"type":       "Todo",
			"parentType": "Contact",
			"parentId":   contact.ID,
		}

		var response struct {
			ParentType string `json:"parentType"`
			ParentID   string `json:"parentId"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ParentType != "Contact" {
			t.Errorf("Expected parentType 'Contact', got %s", response.ParentType)
		}
		if response.ParentID != contact.ID {
			t.Errorf("Expected parentId %s, got %s", contact.ID, response.ParentID)
		}
	})

	t.Run("creates task linked to Account", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Account Task",
			"type":       "Todo",
			"parentType": "Account",
			"parentId":   account.ID,
		}

		var response struct {
			ParentType string `json:"parentType"`
			ParentID   string `json:"parentId"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/tasks/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ParentType != "Account" {
			t.Errorf("Expected parentType 'Account', got %s", response.ParentType)
		}
	})
}
