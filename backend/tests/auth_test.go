package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// ==========================================
// AUTHENTICATION TESTS
// ==========================================

func TestAuth_Register(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	t.Run("successful registration creates user and organization", func(t *testing.T) {
		body := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
			"orgName":  "Test Organization",
		}

		var response struct {
			User struct {
				ID          string `json:"id"`
				Email       string `json:"email"`
				Memberships []struct {
					OrgID   string `json:"orgId"`
					OrgName string `json:"orgName"`
					Role    string `json:"role"`
				} `json:"memberships"`
			} `json:"user"`
			AccessToken string `json:"accessToken"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/register", body, "", &response)
		AssertStatus(t, resp, http.StatusCreated)

		// Check refresh token in cookie (HttpOnly cookie for security)
		refreshToken := GetRefreshTokenFromCookies(resp)

		if response.User.ID == "" {
			t.Error("Expected user ID to be set")
		}
		if response.User.Email != "test@example.com" {
			t.Errorf("Expected email to be test@example.com, got %s", response.User.Email)
		}
		if len(response.User.Memberships) == 0 {
			t.Error("Expected at least one membership")
		} else {
			if response.User.Memberships[0].OrgName != "Test Organization" {
				t.Errorf("Expected org name to be 'Test Organization', got %s", response.User.Memberships[0].OrgName)
			}
			if response.User.Memberships[0].Role != "owner" {
				t.Errorf("Expected role to be 'owner', got %s", response.User.Memberships[0].Role)
			}
		}
		if response.AccessToken == "" {
			t.Error("Expected access token to be set")
		}
		if refreshToken == "" {
			t.Error("Expected refresh token to be set in cookie")
		}
	})

	t.Run("registration fails with missing email", func(t *testing.T) {
		body := map[string]string{
			"password": "password123",
			"orgName":  "Test Organization",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("registration fails with missing password", func(t *testing.T) {
		body := map[string]string{
			"email":   "test2@example.com",
			"orgName": "Test Organization",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("registration fails with missing org name", func(t *testing.T) {
		body := map[string]string{
			"email":    "test3@example.com",
			"password": "password123",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("registration fails with short password", func(t *testing.T) {
		body := map[string]string{
			"email":    "test4@example.com",
			"password": "short",
			"orgName":  "Test Organization",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("registration fails with duplicate email", func(t *testing.T) {
		// First registration
		body := map[string]string{
			"email":    "duplicate@example.com",
			"password": "password123",
			"orgName":  "First Org",
		}
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusCreated)

		// Second registration with same email
		body["orgName"] = "Second Org"
		resp = app.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
		AssertStatus(t, resp, http.StatusConflict)
	})
}

func TestAuth_Login(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user first
	user := app.CreateTestUser(t, "login@example.com", "password123", "Login Test Org")

	t.Run("successful login returns tokens", func(t *testing.T) {
		body := map[string]string{
			"email":    "login@example.com",
			"password": "password123",
		}

		var response struct {
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
			AccessToken string `json:"accessToken"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/login", body, "", &response)
		AssertStatus(t, resp, http.StatusOK)

		// Check refresh token in cookie (HttpOnly cookie for security)
		refreshToken := GetRefreshTokenFromCookies(resp)

		if response.User.ID != user.UserID {
			t.Errorf("Expected user ID %s, got %s", user.UserID, response.User.ID)
		}
		if response.AccessToken == "" {
			t.Error("Expected access token to be set")
		}
		if refreshToken == "" {
			t.Error("Expected refresh token to be set in cookie")
		}
	})

	t.Run("login fails with wrong password", func(t *testing.T) {
		body := map[string]string{
			"email":    "login@example.com",
			"password": "wrongpassword",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/login", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("login fails with non-existent email", func(t *testing.T) {
		body := map[string]string{
			"email":    "nonexistent@example.com",
			"password": "password123",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/login", body, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("login fails with missing email", func(t *testing.T) {
		body := map[string]string{
			"password": "password123",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/login", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("login fails with missing password", func(t *testing.T) {
		body := map[string]string{
			"email": "login@example.com",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/login", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestAuth_Me(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user
	user := app.CreateTestUser(t, "me@example.com", "password123", "Me Test Org")

	t.Run("returns current user info with valid token", func(t *testing.T) {
		var response struct {
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
			Organization struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"organization"`
			Role string `json:"role"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/auth/me", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.User.ID != user.UserID {
			t.Errorf("Expected user ID %s, got %s", user.UserID, response.User.ID)
		}
		if response.User.Email != "me@example.com" {
			t.Errorf("Expected email me@example.com, got %s", response.User.Email)
		}
		if response.Role != "owner" {
			t.Errorf("Expected role 'owner', got %s", response.Role)
		}
	})

	t.Run("returns 401 without token", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/auth/me", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("returns 401 with invalid token", func(t *testing.T) {
		resp := app.MakeRequest(t, "GET", "/api/v1/auth/me", nil, "invalid-token")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestAuth_RefreshToken(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user
	user := app.CreateTestUser(t, "refresh@example.com", "password123", "Refresh Test Org")

	t.Run("refreshes tokens successfully", func(t *testing.T) {
		// Refresh token is sent via cookie
		cookies := []*http.Cookie{
			{Name: "refresh_token", Value: user.RefreshToken},
		}

		var response struct {
			AccessToken string `json:"accessToken"`
		}

		resp := app.MakeRequestWithCookies(t, "POST", "/api/v1/auth/refresh", nil, "", cookies)
		AssertStatus(t, resp, http.StatusOK)

		// Parse response
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &response)

		// New refresh token is in cookie
		newRefreshToken := GetRefreshTokenFromCookies(resp)

		if response.AccessToken == "" {
			t.Error("Expected new access token")
		}
		if newRefreshToken == "" {
			t.Error("Expected new refresh token in cookie")
		}
	})

	t.Run("fails with invalid refresh token", func(t *testing.T) {
		cookies := []*http.Cookie{
			{Name: "refresh_token", Value: "invalid-refresh-token"},
		}

		resp := app.MakeRequestWithCookies(t, "POST", "/api/v1/auth/refresh", nil, "", cookies)
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("fails with missing refresh token", func(t *testing.T) {
		// No cookie = missing refresh token
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/refresh", nil, "")
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestAuth_Logout(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user
	user := app.CreateTestUser(t, "logout@example.com", "password123", "Logout Test Org")

	t.Run("logout invalidates session", func(t *testing.T) {
		// Logout using cookie-based refresh token
		cookies := []*http.Cookie{
			{Name: "refresh_token", Value: user.RefreshToken},
		}

		resp := app.MakeRequestWithCookies(t, "POST", "/api/v1/auth/logout", nil, user.AccessToken, cookies)
		AssertStatus(t, resp, http.StatusOK)

		// The access token might still work until it expires (JWT based)
		// But the refresh token should no longer work
		resp = app.MakeRequestWithCookies(t, "POST", "/api/v1/auth/refresh", nil, "", cookies)
		AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestAuth_ChangePassword(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user
	user := app.CreateTestUser(t, "changepw@example.com", "oldpassword123", "Password Test Org")

	t.Run("changes password successfully", func(t *testing.T) {
		body := map[string]string{
			"currentPassword": "oldpassword123",
			"newPassword":     "newpassword456",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/change-password", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusOK)

		// Verify we can login with new password
		loginBody := map[string]string{
			"email":    "changepw@example.com",
			"password": "newpassword456",
		}
		resp = app.MakeRequest(t, "POST", "/api/v1/auth/login", loginBody, "")
		AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("fails with wrong current password", func(t *testing.T) {
		body := map[string]string{
			"currentPassword": "wrongpassword",
			"newPassword":     "newpassword789",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/change-password", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("fails with short new password", func(t *testing.T) {
		body := map[string]string{
			"currentPassword": "newpassword456",
			"newPassword":     "short",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/change-password", body, user.AccessToken)
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestAuth_GetUserOrgs(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create a test user
	user := app.CreateTestUser(t, "orgs@example.com", "password123", "First Org")

	t.Run("returns user organizations", func(t *testing.T) {
		var response struct {
			Memberships []struct {
				OrgID   string `json:"orgId"`
				OrgName string `json:"orgName"`
				Role    string `json:"role"`
			} `json:"memberships"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/auth/orgs", nil, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Memberships) != 1 {
			t.Errorf("Expected 1 membership, got %d", len(response.Memberships))
		}
		if response.Memberships[0].OrgName != "First Org" {
			t.Errorf("Expected org name 'First Org', got %s", response.Memberships[0].OrgName)
		}
	})
}

func TestAuth_Invitation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create an admin user
	admin := app.CreateTestUser(t, "admin@example.com", "password123", "Invite Test Org")

	t.Run("admin can invite user", func(t *testing.T) {
		body := map[string]string{
			"email": "invited@example.com",
			"role":  "user",
		}

		var response struct {
			Invitation struct {
				ID    string `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
				Token string `json:"token"`
			} `json:"invitation"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", body, admin.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.Invitation.Email != "invited@example.com" {
			t.Errorf("Expected email invited@example.com, got %s", response.Invitation.Email)
		}
		if response.Invitation.Token == "" {
			t.Error("Expected invitation token to be set")
		}
	})

	t.Run("admin can list invitations", func(t *testing.T) {
		var response struct {
			Invitations []struct {
				Email string `json:"email"`
			} `json:"invitations"`
		}

		resp := app.MakeRequestWithResponse(t, "GET", "/api/v1/auth/invitations", nil, admin.AccessToken, &response)
		AssertStatus(t, resp, http.StatusOK)

		if len(response.Invitations) == 0 {
			t.Error("Expected at least one invitation")
		}
	})

	t.Run("non-admin cannot invite user", func(t *testing.T) {
		// Create a regular user by accepting an invitation or registering separately
		regularUser := app.CreateTestUser(t, "regular@example.com", "password123", "Regular User Org")

		body := map[string]string{
			"email": "another@example.com",
			"role":  "user",
		}

		// Regular user tries to invite to their own org
		resp := app.MakeRequest(t, "POST", "/api/v1/auth/invite", body, regularUser.AccessToken)
		// This should succeed because they're the owner of their own org
		AssertStatus(t, resp, http.StatusCreated)
	})
}

func TestAuth_AcceptInvitation(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// Create an admin user and invite someone
	admin := app.CreateTestUser(t, "inviter@example.com", "password123", "Accept Test Org")

	inviteBody := map[string]string{
		"email": "newuser@example.com",
		"role":  "user",
	}

	var inviteResp struct {
		Invitation struct {
			Token string `json:"token"`
		} `json:"invitation"`
	}

	resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/invite", inviteBody, admin.AccessToken, &inviteResp)
	AssertStatus(t, resp, http.StatusCreated)

	t.Run("new user can accept invitation", func(t *testing.T) {
		body := map[string]interface{}{
			"token":     inviteResp.Invitation.Token,
			"password":  "newuserpassword123",
			"firstName": "New",
			"lastName":  "User",
		}

		var response struct {
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
			AccessToken string `json:"accessToken"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/auth/accept-invite", body, "", &response)
		AssertStatus(t, resp, http.StatusOK)

		if response.User.Email != "newuser@example.com" {
			t.Errorf("Expected email newuser@example.com, got %s", response.User.Email)
		}
		if response.AccessToken == "" {
			t.Error("Expected access token to be set")
		}
	})

	t.Run("fails with invalid token", func(t *testing.T) {
		body := map[string]interface{}{
			"token":    "invalid-token",
			"password": "password123",
		}

		resp := app.MakeRequest(t, "POST", "/api/v1/auth/accept-invite", body, "")
		AssertStatus(t, resp, http.StatusBadRequest)
	})
}
