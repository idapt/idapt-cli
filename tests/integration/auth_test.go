//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
)

func TestIntegration_Auth_SessionWorks(t *testing.T) {
	skipIfNoServer(t)

	// Use session cookie to call GET /api/settings (returns user settings).
	// This validates that the test user session is valid.
	status, result := rawGet(t, "/api/settings")
	if status != 200 {
		t.Fatalf("GET /api/settings returned %d, want 200; body: %v", status, result)
	}
	// Settings response should be a JSON object (not empty).
	if result == nil {
		t.Fatal("settings response is nil")
	}
}

func TestIntegration_Auth_APIKeyWorks(t *testing.T) {
	skipIfNoServer(t)

	// Use the api.Client (which sends x-api-key) to call a route
	// that supports API key auth: GET /api/projects
	var resp struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	err := client.Get(testCtx, "/api/projects", nil, &resp)
	if err != nil {
		t.Fatalf("GET /api/projects with API key failed: %v", err)
	}
	// Response should have a projects array (could be empty for new user).
	if resp.Projects == nil {
		t.Fatal("projects response is nil, expected array")
	}
}

func TestIntegration_Auth_InvalidAPIKey(t *testing.T) {
	skipIfNoServer(t)

	badClient, err := api.NewClient(api.ClientConfig{
		BaseURL: baseURL,
		APIKey:  "idapt_invalid_key_that_does_not_exist",
	})
	if err != nil {
		t.Fatalf("create bad client: %v", err)
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	var resp map[string]interface{}
	err = badClient.Get(ctx, "/api/projects", nil, &resp)
	if err == nil {
		t.Fatal("expected error for invalid API key, got nil")
	}

	// Should be an auth error (401)
	if !api.IsAuthError(err) {
		t.Fatalf("expected auth error (401), got: %v", err)
	}
}
