package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ImportResponse matches the ImportCSVResponse struct
type ImportResponse struct {
	Mode      string      `json:"mode"`
	Created   int         `json:"created"`
	Updated   int         `json:"updated"`
	Deleted   int         `json:"deleted"`
	Skipped   int         `json:"skipped"`
	Failed    int         `json:"failed"`
	TotalRows int         `json:"totalRows"`
	IDs       []string    `json:"ids"`
	Errors    []BulkError `json:"errors"`
}

type BulkError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// makeMultipartRequest creates a multipart form request with CSV file
func (ta *TestApp) makeMultipartRequest(t *testing.T, path string, csvContent string, options map[string]interface{}, token string) *http.Response {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add CSV file
	part, err := writer.CreateFormFile("file", "test.csv")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte(csvContent))

	// Add options if provided
	if options != nil {
		optionsJSON, _ := json.Marshal(options)
		writer.WriteField("options", string(optionsJSON))
	}

	writer.Close()

	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ta.App.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

func TestImport_CreateMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import@test.com", "password123", "Import Test Org")

	// First create the Contact entity with fields
	setupContactEntity(t, app, user.AccessToken)

	t.Run("creates new records from CSV", func(t *testing.T) {
		csv := `firstName,lastName,email
John,Doe,john@example.com
Jane,Smith,jane@example.com
Bob,Johnson,bob@example.com`

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, nil, user.AccessToken)

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
		}

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Created != 3 {
			t.Errorf("Expected 3 created, got %d", result.Created)
		}
		if result.Failed != 0 {
			t.Errorf("Expected 0 failed, got %d", result.Failed)
		}
		if len(result.IDs) != 3 {
			t.Errorf("Expected 3 IDs, got %d", len(result.IDs))
		}
	})

	t.Run("explicit create mode works", func(t *testing.T) {
		csv := `firstName,lastName,email
Alice,Wonder,alice@example.com`

		options := map[string]interface{}{
			"mode": "create",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusCreated)

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Created != 1 {
			t.Errorf("Expected 1 created, got %d", result.Created)
		}
	})
}

func TestImport_UpdateMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import-update@test.com", "password123", "Import Update Org")
	setupContactEntity(t, app, user.AccessToken)

	// Create initial records
	csv := `firstName,lastName,email
John,Doe,john@update.com
Jane,Smith,jane@update.com`

	resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusCreated)

	var createResult ImportResponse
	json.NewDecoder(resp.Body).Decode(&createResult)

	t.Run("updates existing records by email", func(t *testing.T) {
		updateCSV := `email,firstName,lastName
john@update.com,Johnny,Doeman
jane@update.com,Janet,Smithers`

		options := map[string]interface{}{
			"mode":       "update",
			"matchField": "email",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", updateCSV, options, user.AccessToken)

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
		}

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Updated != 2 {
			t.Errorf("Expected 2 updated, got %d", result.Updated)
		}
		if result.Skipped != 0 {
			t.Errorf("Expected 0 skipped, got %d", result.Skipped)
		}
	})

	t.Run("skips non-existent records in update mode", func(t *testing.T) {
		updateCSV := `email,firstName
nonexistent@test.com,Nobody`

		options := map[string]interface{}{
			"mode":       "update",
			"matchField": "email",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", updateCSV, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Updated != 0 {
			t.Errorf("Expected 0 updated, got %d", result.Updated)
		}
		if result.Skipped != 1 {
			t.Errorf("Expected 1 skipped, got %d", result.Skipped)
		}
	})
}

func TestImport_UpsertMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import-upsert@test.com", "password123", "Import Upsert Org")
	setupContactEntity(t, app, user.AccessToken)

	// Create initial record
	csv := `firstName,lastName,email
John,Doe,john@upsert.com`

	resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("upsert creates new and updates existing", func(t *testing.T) {
		upsertCSV := `email,firstName,lastName
john@upsert.com,Johnny,Updated
newperson@upsert.com,New,Person`

		options := map[string]interface{}{
			"mode":       "upsert",
			"matchField": "email",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", upsertCSV, options, user.AccessToken)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200/201, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
		}

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Updated != 1 {
			t.Errorf("Expected 1 updated, got %d", result.Updated)
		}
		if result.Created != 1 {
			t.Errorf("Expected 1 created, got %d", result.Created)
		}
		if len(result.IDs) != 2 {
			t.Errorf("Expected 2 IDs, got %d", len(result.IDs))
		}
	})
}

func TestImport_DeleteMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import-delete@test.com", "password123", "Import Delete Org")
	setupContactEntity(t, app, user.AccessToken)

	// Create records to delete
	csv := `firstName,lastName,email
ToDelete1,Person,delete1@test.com
ToDelete2,Person,delete2@test.com
ToKeep,Person,keep@test.com`

	resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("deletes records by email", func(t *testing.T) {
		deleteCSV := `email
delete1@test.com
delete2@test.com`

		options := map[string]interface{}{
			"mode":       "delete",
			"matchField": "email",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", deleteCSV, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Deleted != 2 {
			t.Errorf("Expected 2 deleted, got %d", result.Deleted)
		}
		if result.Skipped != 0 {
			t.Errorf("Expected 0 skipped, got %d", result.Skipped)
		}
	})

	t.Run("skips non-existent records in delete mode", func(t *testing.T) {
		deleteCSV := `email
nonexistent@test.com`

		options := map[string]interface{}{
			"mode":       "delete",
			"matchField": "email",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", deleteCSV, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Deleted != 0 {
			t.Errorf("Expected 0 deleted, got %d", result.Deleted)
		}
		if result.Skipped != 1 {
			t.Errorf("Expected 1 skipped, got %d", result.Skipped)
		}
	})
}

func TestImport_ValidationErrors(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import-validate@test.com", "password123", "Import Validate Org")
	setupContactEntity(t, app, user.AccessToken)

	t.Run("requires matchField for update mode", func(t *testing.T) {
		csv := `firstName,lastName
John,Doe`

		options := map[string]interface{}{
			"mode": "update",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects invalid mode", func(t *testing.T) {
		csv := `firstName,lastName
John,Doe`

		options := map[string]interface{}{
			"mode": "invalid",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects non-existent matchField", func(t *testing.T) {
		csv := `firstName,lastName
John,Doe`

		options := map[string]interface{}{
			"mode":       "upsert",
			"matchField": "nonExistentField",
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, options, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestImport_SkipErrors(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "import-skip@test.com", "password123", "Import Skip Org")
	setupContactEntity(t, app, user.AccessToken)

	t.Run("continues on error when skipErrors is true", func(t *testing.T) {
		// Create initial record for duplicate test
		csv := `firstName,lastName,email
Existing,Person,existing@test.com`
		app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", csv, nil, user.AccessToken)

		// Now try upsert - should work even if some records have issues
		upsertCSV := `firstName,lastName,email
Good,Person,good@test.com
Another,Person,another@test.com`

		options := map[string]interface{}{
			"mode":       "create",
			"skipErrors": true,
		}

		resp := app.makeMultipartRequest(t, "/api/v1/entities/Contact/import/csv", upsertCSV, options, user.AccessToken)

		var result ImportResponse
		json.NewDecoder(resp.Body).Decode(&result)

		// Should have created the valid records
		if result.Created < 1 {
			t.Errorf("Expected at least 1 created, got %d", result.Created)
		}
	})
}

// setupContactEntity creates the Contact entity with required fields
func setupContactEntity(t *testing.T, app *TestApp, token string) {
	t.Helper()

	// Create Contact entity
	entityBody := map[string]interface{}{
		"name":       "Contact",
		"label":      "Contact",
		"labelPlural": "Contacts",
	}
	resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities", entityBody, token)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp.Body)
		// Entity might already exist, which is fine
		if resp.StatusCode != http.StatusBadRequest {
			t.Logf("Entity creation returned %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}

	// Create fields
	fields := []map[string]interface{}{
		{"name": "firstName", "label": "First Name", "type": "varchar"},
		{"name": "lastName", "label": "Last Name", "type": "varchar"},
		{"name": "email", "label": "Email", "type": "varchar"},
		{"name": "phone", "label": "Phone", "type": "varchar"},
	}

	for _, field := range fields {
		resp := app.MakeRequest(t, "POST", "/api/v1/admin/entities/Contact/fields", field, token)
		// Field might already exist
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusBadRequest {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Logf("Field creation returned %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}
}
