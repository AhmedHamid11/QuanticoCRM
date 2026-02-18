package turso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client handles Turso Platform API interactions
type Client struct {
	apiToken   string
	orgName    string
	httpClient *http.Client
	baseURL    string
}

// Database represents a Turso database
type Database struct {
	Name      string `json:"name"`
	DbID      string `json:"DbId"`
	Hostname  string `json:"hostname"`
	Group     string `json:"group"`
	IsSchema  bool   `json:"is_schema"`
	Schema    string `json:"schema,omitempty"`
	Regions   []string `json:"regions,omitempty"`
	Version   string `json:"version,omitempty"`
}

// CreateDatabaseRequest represents a database creation request
type CreateDatabaseRequest struct {
	Name  string `json:"name"`
	Group string `json:"group"`
}

// CreateDatabaseResponse represents the API response for database creation
type CreateDatabaseResponse struct {
	Database Database `json:"database"`
}

// ListDatabasesResponse represents the API response for listing databases
type ListDatabasesResponse struct {
	Databases []Database `json:"databases"`
}

// CreateTokenResponse represents the API response for token creation
type CreateTokenResponse struct {
	JWT string `json:"jwt"`
}

// NewClient creates a new Turso API client
func NewClient() (*Client, error) {
	// Try TURSO_API_TOKEN first, fall back to TURSO_AUTH_TOKEN
	apiToken := os.Getenv("TURSO_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("TURSO_AUTH_TOKEN")
	}
	if apiToken == "" {
		return nil, fmt.Errorf("TURSO_API_TOKEN or TURSO_AUTH_TOKEN environment variable is required")
	}

	orgName := os.Getenv("TURSO_ORG")
	if orgName == "" {
		return nil, fmt.Errorf("TURSO_ORG environment variable is required")
	}

	return &Client{
		apiToken: apiToken,
		orgName:  orgName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.turso.tech/v1",
	}, nil
}

// NewClientWithConfig creates a client with explicit config (for testing)
func NewClientWithConfig(apiToken, orgName string) *Client {
	return &Client{
		apiToken: apiToken,
		orgName:  orgName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.turso.tech/v1",
	}
}

// CreateDatabase creates a new Turso database
func (c *Client) CreateDatabase(ctx context.Context, name string) (*Database, error) {
	// Default group is usually "default" - can be configured
	group := os.Getenv("TURSO_GROUP")
	if group == "" {
		group = "default"
	}

	reqBody := CreateDatabaseRequest{
		Name:  name,
		Group: group,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/organizations/%s/databases", c.baseURL, c.orgName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create database: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result CreateDatabaseResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Database, nil
}

// DeleteDatabase deletes a Turso database
func (c *Client) DeleteDatabase(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/organizations/%s/databases/%s", c.baseURL, c.orgName, name)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete database: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetDatabase gets information about a database
func (c *Client) GetDatabase(ctx context.Context, name string) (*Database, error) {
	url := fmt.Sprintf("%s/organizations/%s/databases/%s", c.baseURL, c.orgName, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get database: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Database Database `json:"database"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Database, nil
}

// ListDatabases lists all databases in the organization
func (c *Client) ListDatabases(ctx context.Context) ([]Database, error) {
	url := fmt.Sprintf("%s/organizations/%s/databases", c.baseURL, c.orgName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list databases: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result ListDatabasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Databases, nil
}

// CreateAuthToken creates an auth token for a specific database
func (c *Client) CreateAuthToken(ctx context.Context, dbName string, expiration string) (string, error) {
	url := fmt.Sprintf("%s/organizations/%s/databases/%s/auth/tokens?expiration=%s",
		c.baseURL, c.orgName, dbName, expiration)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create token: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result CreateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.JWT, nil
}

// GetDatabaseURL returns the libsql URL for a database
func (c *Client) GetDatabaseURL(db *Database) string {
	return fmt.Sprintf("libsql://%s", db.Hostname)
}

// GetDatabaseURLByName constructs the URL from just the database name
func (c *Client) GetDatabaseURLByName(dbName string) string {
	return fmt.Sprintf("libsql://%s-%s.turso.io", dbName, c.orgName)
}

// UsageResponse represents Turso organization usage data
type UsageResponse struct {
	Organization struct {
		Name string `json:"name"`
	} `json:"organization"`
	Usage struct {
		RowsRead    int64 `json:"rows_read"`
		RowsWritten int64 `json:"rows_written"`
		StorageBytes int64 `json:"storage_bytes"`
	} `json:"usage"`
	Instances map[string]struct {
		Usage struct {
			RowsRead    int64 `json:"rows_read"`
			RowsWritten int64 `json:"rows_written"`
			StorageBytes int64 `json:"storage_bytes"`
		} `json:"usage"`
	} `json:"instances,omitempty"`
}

// GetUsage fetches the current billing period usage for the Turso organization.
func (c *Client) GetUsage(ctx context.Context) (*UsageResponse, error) {
	url := fmt.Sprintf("%s/organizations/%s/usage", c.baseURL, c.orgName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get usage: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result UsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse usage response: %w", err)
	}

	return &result, nil
}
