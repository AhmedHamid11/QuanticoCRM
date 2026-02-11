package tests

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
)

// TestIngest_E2E tests the complete Salesforce-style ingest flow
// TEST-01: Salesforce-style payload flows through mirror config to Quantico entity records
func TestIngest_E2E(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()
	user := app.CreateTestUser(t, "ingest-e2e@test.com", "SecureP@ssw0rd!E2E", "E2E Ingest Org")

	var apiKey string
	var mirrorID string

	// Step 1: Create ingest API key via admin API
	t.Run("Create API Key", func(t *testing.T) {
		body := map[string]interface{}{
			"name":      "Test Ingest Key",
			"rateLimit": 500,
		}

		var result map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", body, user.AccessToken, &result)
		AssertStatus(t, resp, http.StatusCreated)

		// Store the full key (shown only once)
		fullKey, ok := result["key"].(string)
		if !ok || fullKey == "" {
			t.Fatalf("Expected full key in response, got: %v", result)
		}

		apiKey = fullKey
	})

	// Step 2: Create mirror via admin API
	t.Run("Create Mirror", func(t *testing.T) {
		body := map[string]interface{}{
			"name":              "Salesforce Contacts",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id", // This is the SOURCE field name used for deduplication
			"unmappedFieldMode": "flexible",
			"sourceFields": []map[string]interface{}{
				{
					"fieldName":  "sf_id",
					"fieldType":  "text",
					"isRequired": true,
					"mapField":   "description", // Using description to store SF ID for testing
				},
				{
					"fieldName":  "FirstName",
					"fieldType":  "text",
					"isRequired": true,
					"mapField":   "firstName",
				},
				{
					"fieldName":  "LastName",
					"fieldType":  "text",
					"isRequired": true,
					"mapField":   "lastName",
				},
				{
					"fieldName":  "Email",
					"fieldType":  "email",
					"isRequired": false,
					"mapField":   "emailAddress",
				},
				{
					"fieldName":  "Phone",
					"fieldType":  "phone",
					"isRequired": false,
					"mapField":   "phoneNumber",
				},
			},
		}

		var result map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", body, user.AccessToken, &result)
		AssertStatus(t, resp, http.StatusCreated)

		// Extract mirror ID
		id, ok := result["id"].(string)
		if !ok || id == "" {
			t.Fatalf("Expected mirror ID in response, got: %v", result)
		}
		mirrorID = id
	})

	var jobID string
	// Step 3: Load testdata fixture and POST to /api/v1/ingest
	t.Run("POST Ingest Request", func(t *testing.T) {
		// Load fixture
		fixtureData, err := os.ReadFile("testdata/salesforce_contacts.json")
		if err != nil {
			t.Fatalf("Failed to read fixture: %v", err)
		}

		var fixture struct {
			Records []map[string]interface{} `json:"records"`
		}
		if err := json.Unmarshal(fixtureData, &fixture); err != nil {
			t.Fatalf("Failed to parse fixture: %v", err)
		}

		// Build ingest request
		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   fixture.Records,
		}

		var result map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			t.Fatalf("Failed to parse ingest response: %v", err)
		}

		// Assert response structure
		if result["status"] != "accepted" {
			t.Errorf("Expected status 'accepted', got: %v", result["status"])
		}

		recordsReceived, ok := result["records_received"].(float64)
		if !ok || int(recordsReceived) != 5 {
			t.Errorf("Expected 5 records_received, got: %v", result["records_received"])
		}

		jobID, ok = result["job_id"].(string)
		if !ok || jobID == "" {
			t.Fatalf("Expected job_id in response, got: %v", result)
		}
	})

	// Step 4: Wait for async processing to complete
	var job map[string]interface{}
	t.Run("Wait for Job Completion", func(t *testing.T) {
		job = app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert job completed successfully
		if job["status"] != "complete" {
			t.Errorf("Expected status 'complete', got: %v", job["status"])
		}
	})

	// Step 5: Verify job result (JSON uses camelCase per entity struct tags)
	t.Run("Verify Job Result", func(t *testing.T) {
		recordsPromoted, ok := job["recordsPromoted"].(float64)
		if !ok || int(recordsPromoted) != 5 {
			t.Errorf("Expected 5 recordsPromoted, got: %v", job["recordsPromoted"])
		}

		recordsSkipped, ok := job["recordsSkipped"].(float64)
		if !ok || int(recordsSkipped) != 0 {
			t.Errorf("Expected 0 recordsSkipped, got: %v", job["recordsSkipped"])
		}

		recordsFailed, ok := job["recordsFailed"].(float64)
		if !ok || int(recordsFailed) != 0 {
			t.Errorf("Expected 0 recordsFailed, got: %v", job["recordsFailed"])
		}
	})

	// Step 6: Verify records exist in entity table
	t.Run("Verify Records in Database", func(t *testing.T) {
		rows, err := app.DB.Query(`
			SELECT COUNT(*) as count,
			       GROUP_CONCAT(first_name) as names,
			       GROUP_CONCAT(email_address) as emails
			FROM contacts
			WHERE org_id = ?
		`, user.OrgID)
		if err != nil {
			t.Fatalf("Failed to query contacts: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Expected at least one row from COUNT query")
		}

		var count int
		var names, emails sql.NullString
		if err := rows.Scan(&count, &names, &emails); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected 5 contacts in DB, got: %d", count)
		}

		// Verify field mapping worked (spot check for expected names)
		if names.Valid {
			namesStr := names.String
			if !contains(namesStr, "John") || !contains(namesStr, "Jane") {
				t.Errorf("Expected names to include John and Jane, got: %s", namesStr)
			}
		}

		if emails.Valid {
			emailsStr := emails.String
			if !contains(emailsStr, "john.smith@example.com") {
				t.Errorf("Expected emails to include john.smith@example.com, got: %s", emailsStr)
			}
		}
	})
}

// TestIngest_DeltaSync tests that re-pushing the same payload produces no duplicates
// TEST-02: Delta sync - re-pushing same payload creates 0 new records
func TestIngest_DeltaSync(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()
	user := app.CreateTestUser(t, "delta@test.com", "SecureP@ssw0rd!Delta", "Delta Sync Org")

	// Setup: Create API key and mirror
	var apiKey, mirrorID string

	t.Run("Setup API Key and Mirror", func(t *testing.T) {
		// Create API key
		keyBody := map[string]interface{}{
			"name":      "Delta Test Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, user.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKey = keyResult["key"].(string)

		// Create mirror
		mirrorBody := map[string]interface{}{
			"name":              "Delta Test Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "flexible",
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
				{"fieldName": "Email", "fieldType": "email", "isRequired": false, "mapField": "emailAddress"},
				{"fieldName": "Phone", "fieldType": "phone", "isRequired": false, "mapField": "phoneNumber"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, user.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorID = mirrorResult["id"].(string)
	})

	// Load fixture once
	fixtureData, err := os.ReadFile("testdata/salesforce_contacts.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	var fixture struct {
		Records []map[string]interface{} `json:"records"`
	}
	if err := json.Unmarshal(fixtureData, &fixture); err != nil {
		t.Fatalf("Failed to parse fixture: %v", err)
	}

	// Step 2: First ingest - 5 records
	t.Run("First Ingest - 5 New Records", func(t *testing.T) {
		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   fixture.Records,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert all 5 promoted
		recordsPromoted := int(job["recordsPromoted"].(float64))
		recordsSkipped := int(job["recordsSkipped"].(float64))

		if recordsPromoted != 5 {
			t.Errorf("Expected 5 promoted on first ingest, got: %d", recordsPromoted)
		}
		if recordsSkipped != 0 {
			t.Errorf("Expected 0 skipped on first ingest, got: %d", recordsSkipped)
		}
	})

	// Step 3: Second ingest - same 5 records (should all be skipped)
	t.Run("Second Ingest - 5 Duplicates (All Skipped)", func(t *testing.T) {
		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   fixture.Records,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert all 5 skipped, 0 promoted
		recordsPromoted := int(job["recordsPromoted"].(float64))
		recordsSkipped := int(job["recordsSkipped"].(float64))

		if recordsPromoted != 0 {
			t.Errorf("Expected 0 promoted on duplicate ingest, got: %d", recordsPromoted)
		}
		if recordsSkipped != 5 {
			t.Errorf("Expected 5 skipped on duplicate ingest, got: %d", recordsSkipped)
		}

		if job["status"] != "complete" {
			t.Errorf("Expected status 'complete' even with all skipped, got: %v", job["status"])
		}
	})

	// Step 4: Verify no duplicates in entity table
	t.Run("Verify Still Only 5 Records", func(t *testing.T) {
		var count int
		err := app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, user.OrgID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count contacts: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected still 5 contacts (no duplicates), got: %d", count)
		}
	})

	// Step 5: Third ingest - mix of old and new (2 existing + 3 new)
	t.Run("Third Ingest - Mixed Old and New", func(t *testing.T) {
		mixedRecords := []map[string]interface{}{
			// 2 existing records (should be skipped)
			fixture.Records[0], // sf_id: 003MOCK000001
			fixture.Records[1], // sf_id: 003MOCK000002
			// 3 new records (should be promoted)
			{
				"sf_id":     "003MOCK000006",
				"FirstName": "David",
				"LastName":  "Miller",
				"Email":     "david.m@example.com",
				"Phone":     "555-0106",
			},
			{
				"sf_id":     "003MOCK000007",
				"FirstName": "Emma",
				"LastName":  "Davis",
				"Email":     "emma.d@example.com",
				"Phone":     "555-0107",
			},
			{
				"sf_id":     "003MOCK000008",
				"FirstName": "Frank",
				"LastName":  "Wilson",
				"Email":     "frank.w@example.com",
				"Phone":     "555-0108",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   mixedRecords,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert 3 promoted, 2 skipped
		recordsPromoted := int(job["recordsPromoted"].(float64))
		recordsSkipped := int(job["recordsSkipped"].(float64))

		if recordsPromoted != 3 {
			t.Errorf("Expected 3 promoted on mixed ingest, got: %d", recordsPromoted)
		}
		if recordsSkipped != 2 {
			t.Errorf("Expected 2 skipped on mixed ingest, got: %d", recordsSkipped)
		}
	})

	// Step 6: Verify final count is 8 total
	t.Run("Verify Total 8 Records", func(t *testing.T) {
		var count int
		err := app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, user.OrgID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count contacts: %v", err)
		}

		if count != 8 {
			t.Errorf("Expected 8 total contacts (5 original + 3 new), got: %d", count)
		}
	})
}

// TestIngest_StrictMode tests that strict mode rejects payloads with unmapped fields
// TEST-03: Strict mode rejects unmapped fields with UNMAPPED_FIELD error code
func TestIngest_StrictMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()
	user := app.CreateTestUser(t, "strict@test.com", "SecureP@ssw0rd!Strict", "Strict Mode Org")

	var apiKey, mirrorID string

	// Setup: Create API key and mirror with strict mode
	t.Run("Setup API Key and Mirror (Strict Mode)", func(t *testing.T) {
		// Create API key
		keyBody := map[string]interface{}{
			"name":      "Strict Test Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, user.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKey = keyResult["key"].(string)

		// Create mirror with strict mode, only 3 source fields (sf_id, FirstName, LastName)
		// This will cause Email, Phone, Company from fixture to be unmapped
		mirrorBody := map[string]interface{}{
			"name":              "Strict Mode Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "strict",
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, user.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorID = mirrorResult["id"].(string)
	})

	// Test 1: Ingest records with unmapped fields - should all fail
	t.Run("Rejects Records with Unmapped Fields", func(t *testing.T) {
		// Load fixture (has Email, Phone as extra fields)
		fixtureData, err := os.ReadFile("testdata/salesforce_contacts.json")
		if err != nil {
			t.Fatalf("Failed to read fixture: %v", err)
		}

		var fixture struct {
			Records []map[string]interface{} `json:"records"`
		}
		if err := json.Unmarshal(fixtureData, &fixture); err != nil {
			t.Fatalf("Failed to parse fixture: %v", err)
		}

		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   fixture.Records,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert all records failed due to unmapped fields
		recordsFailed := int(job["recordsFailed"].(float64))
		recordsPromoted := int(job["recordsPromoted"].(float64))

		if recordsFailed != 5 {
			t.Errorf("Expected 5 records failed (all have unmapped fields), got: %d", recordsFailed)
		}
		if recordsPromoted != 0 {
			t.Errorf("Expected 0 records promoted in strict mode with unmapped fields, got: %d", recordsPromoted)
		}

		// Verify errors contain UNMAPPED_FIELD error code
		errors, ok := job["errors"].([]interface{})
		if !ok || len(errors) == 0 {
			t.Fatalf("Expected errors array with UNMAPPED_FIELD codes, got: %v", job["errors"])
		}

		// Check first error for UNMAPPED_FIELD code
		if len(errors) > 0 {
			firstError, ok := errors[0].(map[string]interface{})
			if ok {
				errorCode, _ := firstError["code"].(string)
				if errorCode != "UNMAPPED_FIELD" {
					t.Errorf("Expected error code UNMAPPED_FIELD, got: %s", errorCode)
				}
			}
		}

		// Verify no records were promoted to entity table
		var count int
		err = app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, user.OrgID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count contacts: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 contacts in DB (all rejected), got: %d", count)
		}
	})

	// Test 2: Ingest records WITHOUT unmapped fields - should succeed
	t.Run("Accepts Records Without Unmapped Fields", func(t *testing.T) {
		// Create records with only the 3 defined fields
		cleanRecords := []map[string]interface{}{
			{
				"sf_id":     "003STRICT0001",
				"FirstName": "Alice",
				"LastName":  "Johnson",
			},
			{
				"sf_id":     "003STRICT0002",
				"FirstName": "Bob",
				"LastName":  "Williams",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   cleanRecords,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert all records promoted successfully
		recordsPromoted := int(job["recordsPromoted"].(float64))
		recordsFailed := int(job["recordsFailed"].(float64))

		if recordsPromoted != 2 {
			t.Errorf("Expected 2 records promoted (no unmapped fields), got: %d", recordsPromoted)
		}
		if recordsFailed != 0 {
			t.Errorf("Expected 0 records failed (valid payload), got: %d", recordsFailed)
		}

		if job["status"] != "complete" {
			t.Errorf("Expected status 'complete', got: %v", job["status"])
		}

		// Verify records exist in DB
		var count int
		err := app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, user.OrgID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count contacts: %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 contacts in DB, got: %d", count)
		}
	})
}

// TestIngest_FlexibleMode tests that flexible mode accepts unmapped fields with warnings
// TEST-04: Flexible mode accepts unmapped fields and produces warnings
func TestIngest_FlexibleMode(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()
	user := app.CreateTestUser(t, "flexible@test.com", "SecureP@ssw0rd!Flexible", "Flexible Mode Org")

	var apiKey, mirrorID string

	// Setup: Create API key and mirror with flexible mode
	t.Run("Setup API Key and Mirror (Flexible Mode)", func(t *testing.T) {
		// Create API key
		keyBody := map[string]interface{}{
			"name":      "Flexible Test Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, user.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKey = keyResult["key"].(string)

		// Create mirror with flexible mode, only 3 source fields (sf_id, FirstName, LastName)
		// This will cause Email, Phone from fixture to be unmapped but NOT rejected
		mirrorBody := map[string]interface{}{
			"name":              "Flexible Mode Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "flexible",
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, user.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorID = mirrorResult["id"].(string)
	})

	// Test: Ingest records with unmapped fields - should all succeed with warnings
	t.Run("Accepts Records with Unmapped Fields", func(t *testing.T) {
		// Load fixture (has Email, Phone as extra fields)
		fixtureData, err := os.ReadFile("testdata/salesforce_contacts.json")
		if err != nil {
			t.Fatalf("Failed to read fixture: %v", err)
		}

		var fixture struct {
			Records []map[string]interface{} `json:"records"`
		}
		if err := json.Unmarshal(fixtureData, &fixture); err != nil {
			t.Fatalf("Failed to parse fixture: %v", err)
		}

		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   fixture.Records,
		}

		var ingestResult map[string]interface{}
		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &ingestResult)

		jobID := ingestResult["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

		// Assert all records promoted despite unmapped fields
		recordsPromoted := int(job["recordsPromoted"].(float64))
		recordsFailed := int(job["recordsFailed"].(float64))

		if recordsPromoted != 5 {
			t.Errorf("Expected 5 records promoted (flexible mode accepts unmapped fields), got: %d", recordsPromoted)
		}
		if recordsFailed != 0 {
			t.Errorf("Expected 0 records failed (flexible mode), got: %d", recordsFailed)
		}

		if job["status"] != "complete" {
			t.Errorf("Expected status 'complete', got: %v", job["status"])
		}

		// Verify warnings array exists and contains unmapped field warnings
		warnings, ok := job["warnings"].([]interface{})
		if !ok || len(warnings) == 0 {
			t.Errorf("Expected warnings array with unmapped field warnings, got: %v", job["warnings"])
		} else {
			// Check that at least one warning mentions unmapped fields
			foundUnmappedWarning := false
			for _, w := range warnings {
				if wStr, ok := w.(string); ok {
					if contains(wStr, "unmapped") || contains(wStr, "Unmapped") {
						foundUnmappedWarning = true
						break
					}
				}
			}
			if !foundUnmappedWarning {
				t.Errorf("Expected at least one warning about unmapped fields, got warnings: %v", warnings)
			}
		}

		// Verify records were promoted to entity table
		var count int
		err = app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, user.OrgID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count contacts: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected 5 contacts in DB (all promoted), got: %d", count)
		}
	})
}

// TestIngest_RateLimiting tests that rate limiting returns 429 with Retry-After
// TEST-05: Rate limit enforcement with 429 response and Retry-After header
func TestIngest_RateLimiting(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()
	user := app.CreateTestUser(t, "ratelimit@test.com", "SecureP@ssw0rd!RateLimit", "Rate Limit Org")

	var apiKey, mirrorID string

	// Setup: Create API key and mirror with very low rate limit
	t.Run("Setup API Key and Mirror with Low Rate Limit", func(t *testing.T) {
		// Create API key
		keyBody := map[string]interface{}{
			"name":      "Rate Limit Test Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, user.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKey = keyResult["key"].(string)

		// Create mirror with low rate limit (3 requests per minute)
		mirrorBody := map[string]interface{}{
			"name":              "Rate Limited Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "flexible",
			"rateLimit":         3, // Very low for testing
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, user.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorID = mirrorResult["id"].(string)
	})

	// Store job IDs for verification
	var jobIDs []string

	// Test: Send 3 requests (up to limit)
	t.Run("Accept Requests Up to Limit", func(t *testing.T) {
		for i := 1; i <= 3; i++ {
			records := []map[string]interface{}{
				{
					"sf_id":     "003RATE00000" + string(rune('0'+i)),
					"FirstName": "Rate",
					"LastName":  "Test" + string(rune('0'+i)),
				},
			}

			ingestBody := map[string]interface{}{
				"org_id":    user.OrgID,
				"mirror_id": mirrorID,
				"records":   records,
			}

			resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)
			AssertStatus(t, resp, http.StatusAccepted)

			bodyBytes, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(bodyBytes, &result)

			jobID, ok := result["job_id"].(string)
			if !ok || jobID == "" {
				t.Fatalf("Request %d: expected job_id in response", i)
			}
			jobIDs = append(jobIDs, jobID)
		}
	})

	// Test: 4th request should be rate limited
	t.Run("Reject 4th Request with 429", func(t *testing.T) {
		records := []map[string]interface{}{
			{
				"sf_id":     "003RATE000004",
				"FirstName": "Rate",
				"LastName":  "Test4",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    user.OrgID,
			"mirror_id": mirrorID,
			"records":   records,
		}

		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKey)

		// Assert 429 status
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected status 429 Too Many Requests, got: %d", resp.StatusCode)
		}

		// Assert Retry-After header exists
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			t.Error("Expected Retry-After header in 429 response")
		}

		// Assert response body structure
		bodyBytes, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			t.Fatalf("Failed to parse 429 response: %v", err)
		}

		errorMsg, ok := result["error"].(string)
		if !ok || !contains(errorMsg, "Rate limit exceeded") {
			t.Errorf("Expected 'Rate limit exceeded' error, got: %v", result["error"])
		}

		// Check for retry_after in response body
		if _, ok := result["retry_after"]; !ok {
			t.Errorf("Expected retry_after field in response body, got: %v", result)
		}

		// Check for limit in response body
		limit, ok := result["limit"].(float64)
		if !ok || int(limit) != 3 {
			t.Errorf("Expected limit=3 in response body, got: %v", result["limit"])
		}
	})

	// Test: Verify accepted jobs completed
	t.Run("Verify Accepted Jobs Completed", func(t *testing.T) {
		for i, jobID := range jobIDs {
			job := app.WaitForJobCompletion(t, apiKey, jobID, 10)

			if job["status"] != "complete" {
				t.Errorf("Job %d (%s) expected status 'complete', got: %v", i+1, jobID, job["status"])
			}

			recordsPromoted := int(job["recordsPromoted"].(float64))
			if recordsPromoted != 1 {
				t.Errorf("Job %d expected 1 record promoted, got: %d", i+1, recordsPromoted)
			}
		}
	})
}

// TestIngest_MultiTenantIsolation tests that API keys cannot access other tenants' data
// TEST-06: Multi-tenant isolation prevents cross-tenant access
func TestIngest_MultiTenantIsolation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create two separate tenants
	userA := app.CreateTestUser(t, "tenantA@test.com", "SecureP@ssw0rd!TenantA", "Tenant A Org")
	userB := app.CreateTestUser(t, "tenantB@test.com", "SecureP@ssw0rd!TenantB", "Tenant B Org")

	var apiKeyA, apiKeyB, mirrorAID, mirrorBID string

	// Setup: Create API keys and mirrors for both tenants
	t.Run("Setup Tenant A Resources", func(t *testing.T) {
		// Create API key for Tenant A
		keyBody := map[string]interface{}{
			"name":      "Tenant A Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, userA.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKeyA = keyResult["key"].(string)

		// Create mirror for Tenant A
		mirrorBody := map[string]interface{}{
			"name":              "Tenant A Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "flexible",
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, userA.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorAID = mirrorResult["id"].(string)
	})

	t.Run("Setup Tenant B Resources", func(t *testing.T) {
		// Create API key for Tenant B
		keyBody := map[string]interface{}{
			"name":      "Tenant B Key",
			"rateLimit": 500,
		}

		var keyResult map[string]interface{}
		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/ingest-keys", keyBody, userB.AccessToken, &keyResult)
		AssertStatus(t, resp, http.StatusCreated)

		apiKeyB = keyResult["key"].(string)

		// Create mirror for Tenant B
		mirrorBody := map[string]interface{}{
			"name":              "Tenant B Mirror",
			"targetEntity":      "Contact",
			"uniqueKeyField":    "sf_id",
			"unmappedFieldMode": "flexible",
			"sourceFields": []map[string]interface{}{
				{"fieldName": "sf_id", "fieldType": "text", "isRequired": true, "mapField": "description"},
				{"fieldName": "FirstName", "fieldType": "text", "isRequired": true, "mapField": "firstName"},
				{"fieldName": "LastName", "fieldType": "text", "isRequired": true, "mapField": "lastName"},
			},
		}

		var mirrorResult map[string]interface{}
		resp = app.MakeRequestWithResponse(t, "POST", "/api/v1/admin/mirrors", mirrorBody, userB.AccessToken, &mirrorResult)
		AssertStatus(t, resp, http.StatusCreated)

		mirrorBID = mirrorResult["id"].(string)
	})

	// Test 1: API key A cannot ingest to Tenant B's mirror
	t.Run("API Key A Cannot Access Tenant B Mirror", func(t *testing.T) {
		records := []map[string]interface{}{
			{
				"sf_id":     "003CROSS00001",
				"FirstName": "Cross",
				"LastName":  "Tenant",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    userA.OrgID, // Tenant A org
			"mirror_id": mirrorBID,   // Tenant B mirror
			"records":   records,
		}

		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKeyA)

		// Should return 404 (mirror not found in Tenant A's context)
		// Mirror lookup is scoped to tenant DB resolved from API key
		if resp.StatusCode != http.StatusNotFound {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 404 Not Found (mirror belongs to different tenant), got: %d - %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Test 2: API key A cannot use Tenant B's org_id
	t.Run("API Key A Cannot Use Tenant B Org ID", func(t *testing.T) {
		records := []map[string]interface{}{
			{
				"sf_id":     "003CROSS00002",
				"FirstName": "Wrong",
				"LastName":  "Org",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    userB.OrgID, // Tenant B org
			"mirror_id": mirrorAID,   // Tenant A mirror
			"records":   records,
		}

		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKeyA)

		// Should return 403 (belt-and-suspenders org_id mismatch)
		if resp.StatusCode != http.StatusForbidden {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 403 Forbidden (org_id mismatch), got: %d - %s", resp.StatusCode, string(bodyBytes))
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			errorMsg, _ := result["error"].(string)
			if !contains(errorMsg, "org_id") {
				t.Errorf("Expected error about org_id mismatch, got: %s", errorMsg)
			}
		}
	})

	// Test 3: Successful ingests are isolated to own tenant
	var jobAID, jobBID string

	t.Run("Tenant A Successful Ingest", func(t *testing.T) {
		records := []map[string]interface{}{
			{
				"sf_id":     "003TENA000001",
				"FirstName": "Alice",
				"LastName":  "TenantA",
			},
			{
				"sf_id":     "003TENA000002",
				"FirstName": "Bob",
				"LastName":  "TenantA",
			},
			{
				"sf_id":     "003TENA000003",
				"FirstName": "Charlie",
				"LastName":  "TenantA",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    userA.OrgID,
			"mirror_id": mirrorAID,
			"records":   records,
		}

		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKeyA)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(bodyBytes, &result)

		jobAID = result["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKeyA, jobAID, 10)

		recordsPromoted := int(job["recordsPromoted"].(float64))
		if recordsPromoted != 3 {
			t.Errorf("Tenant A: expected 3 records promoted, got: %d", recordsPromoted)
		}
	})

	t.Run("Tenant B Successful Ingest", func(t *testing.T) {
		records := []map[string]interface{}{
			{
				"sf_id":     "003TENB000001",
				"FirstName": "David",
				"LastName":  "TenantB",
			},
			{
				"sf_id":     "003TENB000002",
				"FirstName": "Emma",
				"LastName":  "TenantB",
			},
		}

		ingestBody := map[string]interface{}{
			"org_id":    userB.OrgID,
			"mirror_id": mirrorBID,
			"records":   records,
		}

		resp := app.MakeIngestRequest(t, "POST", "/api/v1/ingest", ingestBody, apiKeyB)
		AssertStatus(t, resp, http.StatusAccepted)

		bodyBytes, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(bodyBytes, &result)

		jobBID = result["job_id"].(string)
		job := app.WaitForJobCompletion(t, apiKeyB, jobBID, 10)

		recordsPromoted := int(job["recordsPromoted"].(float64))
		if recordsPromoted != 2 {
			t.Errorf("Tenant B: expected 2 records promoted, got: %d", recordsPromoted)
		}
	})

	// Test 4: API key A cannot poll Tenant B's job
	t.Run("API Key A Cannot Access Tenant B Job", func(t *testing.T) {
		// Try to poll Tenant B's job using Tenant A's API key
		resp := app.MakeIngestRequest(t, "GET", "/api/v1/ingest/jobs/"+jobBID, nil, apiKeyA)

		// Should return 404 (job not in Tenant A's DB context)
		if resp.StatusCode != http.StatusNotFound {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 404 Not Found (job belongs to different tenant), got: %d - %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Test 5: Verify data isolation at DB level
	t.Run("Verify Data Isolation in Database", func(t *testing.T) {
		// Count Tenant A contacts
		var countA int
		err := app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, userA.OrgID).Scan(&countA)
		if err != nil {
			t.Fatalf("Failed to count Tenant A contacts: %v", err)
		}

		if countA != 3 {
			t.Errorf("Tenant A: expected 3 contacts, got: %d", countA)
		}

		// Count Tenant B contacts
		var countB int
		err = app.DB.QueryRow(`SELECT COUNT(*) FROM contacts WHERE org_id = ?`, userB.OrgID).Scan(&countB)
		if err != nil {
			t.Fatalf("Failed to count Tenant B contacts: %v", err)
		}

		if countB != 2 {
			t.Errorf("Tenant B: expected 2 contacts, got: %d", countB)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > len(substr) && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || containsInMiddle(str, substr)))
}

func containsInMiddle(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
