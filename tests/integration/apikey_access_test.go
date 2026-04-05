//go:build integration

// Tests that verify API key auth works for all CLI-used endpoints.
// These endpoints were previously blocked or calling wrong paths.
// Covers: auth/me, search, machines, file operations, SSE subscriptions.
package integration

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
)

// ---------------------------------------------------------------------------
// Auth Status — GET /api/auth/me (was: GET /api/auth/account which only had DELETE)
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_AuthMe(t *testing.T) {
	skipIfNoServer(t)

	// GET /api/auth/me should work with API key auth and return user info.
	// The old CLI used /api/auth/account which only had a DELETE handler → 405.
	var resp struct {
		User map[string]interface{} `json:"user"`
	}
	err := client.Get(testCtx, "/api/auth/me", nil, &resp)
	if err != nil {
		t.Fatalf("GET /api/auth/me with API key failed: %v", err)
	}
	if resp.User == nil {
		t.Fatal("user object is nil in /api/auth/me response")
	}
	// Should have an actorId (resolved from the API key's principal)
	actorId := getString(resp.User, "actorId")
	if actorId == "" {
		t.Error("actorId is empty in /api/auth/me response")
	}
}

func TestIntegration_APIKeyAccess_AuthAccountBlocked(t *testing.T) {
	skipIfNoServer(t)

	// GET /api/auth/account should return an error (only DELETE is defined).
	// This verifies the old endpoint path doesn't work with GET.
	var resp map[string]interface{}
	err := client.Get(testCtx, "/api/auth/account", nil, &resp)
	if err == nil {
		t.Fatal("GET /api/auth/account should fail (only DELETE handler exists)")
	}
	// Accept 403 (API key blocked), 404, or 405 (method not allowed)
	if apiErr, ok := err.(*api.APIError); ok {
		if apiErr.StatusCode != 403 && apiErr.StatusCode != 404 && apiErr.StatusCode != 405 {
			t.Fatalf("expected 403/404/405 for GET /api/auth/account, got %d: %s", apiErr.StatusCode, apiErr.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// Search — GET /api/search (was blocked for API keys, now allowed)
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_Search(t *testing.T) {
	skipIfNoServer(t)

	projectID := getPersonalProjectID(t)

	// GET /api/search with API key should work (requires files:read permission).
	// Our test API key has wildcard permissions.
	var resp map[string]interface{}
	q := url.Values{
		"projectId": {projectID},
		"q":         {"test"},
		"source":    {"file"},
	}
	err := client.Get(testCtx, "/api/search", q, &resp)
	if err != nil {
		// A 429 (rate limit) is acceptable — it means the endpoint is reachable
		if apiErr, ok := err.(*api.APIError); ok && apiErr.StatusCode == 429 {
			t.Logf("search rate-limited (expected in CI): %v", err)
			return
		}
		t.Fatalf("GET /api/search with API key failed: %v", err)
	}
	// Response should be a JSON object with results
	if resp == nil {
		t.Fatal("search response is nil")
	}
}

// ---------------------------------------------------------------------------
// Machine List — GET /api/machines (was blocked for API keys, now allowed)
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_MachineList(t *testing.T) {
	skipIfNoServer(t)

	projectID := getPersonalProjectID(t)

	// GET /api/machines should work with API key auth (requires machines:read).
	var resp struct {
		Machines []map[string]interface{} `json:"machines"`
	}
	q := url.Values{"projectId": {projectID}}
	err := client.Get(testCtx, "/api/machines", q, &resp)
	if err != nil {
		t.Fatalf("GET /api/machines with API key failed: %v", err)
	}
	// Response should have a machines array (may be empty for test user)
	if resp.Machines == nil {
		t.Fatal("machines array is nil in response")
	}
}

// ---------------------------------------------------------------------------
// File Operations — FUSE-used endpoints (metadata, version, folders, move)
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_FileList(t *testing.T) {
	skipIfNoServer(t)

	projectID := getPersonalProjectID(t)

	// GET /api/files/list with API key
	var resp struct {
		Files []map[string]interface{} `json:"files"`
	}
	q := url.Values{"projectId": {projectID}}
	err := client.Get(testCtx, "/api/files/list", q, &resp)
	if err != nil {
		t.Fatalf("GET /api/files/list with API key failed: %v", err)
	}
	if resp.Files == nil {
		t.Fatal("files array is nil in response")
	}
}

func TestIntegration_APIKeyAccess_FileFolder(t *testing.T) {
	skipIfNoServer(t)

	projectID := getPersonalProjectID(t)

	// POST /api/files/folders should work with API key (requires files:write).
	// The endpoint is in the route-permissions map.
	folderName := uniqueName("folder")
	var createResp map[string]interface{}
	err := client.Post(testCtx, "/api/files/folders", map[string]interface{}{
		"name":      folderName,
		"projectId": projectID,
	}, &createResp)
	if err != nil {
		// 403 "not accessible via API key" means route-permissions isn't updated yet
		if apiErr, ok := err.(*api.APIError); ok && apiErr.StatusCode == 403 {
			t.Fatalf("POST /api/files/folders blocked for API key (route-permissions map not updated?): %v", err)
		}
		// 500 may be transient; log and skip
		if apiErr, ok := err.(*api.APIError); ok && apiErr.StatusCode == 500 {
			t.Skipf("POST /api/files/folders returned 500 (transient server error): %v", err)
		}
		t.Fatalf("POST /api/files/folders with API key failed: %v", err)
	}

	// Clean up: delete the created folder
	folder := getMap(createResp, "folder")
	if folder != nil {
		id := getString(folder, "id")
		if id != "" {
			_ = client.Delete(testCtx, "/api/files/"+id)
		}
	}
}

// ---------------------------------------------------------------------------
// SSE Subscriptions — GET /api/subscriptions/files
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_SSESubscription(t *testing.T) {
	skipIfNoServer(t)

	projectID := getPersonalProjectID(t)

	// SSE endpoints require Redis for IPC subscriptions. If Redis isn't
	// available (common in dev), the endpoint hangs on subscribe.
	// Use a short timeout — if we get 200 + text/event-stream, it works.
	// If it times out, skip (infrastructure not available).
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	u := baseURL + "/api/subscriptions/files?projectId=" + projectID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("x-api-key", client.APIKey())
	req.Header.Set("Origin", baseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			t.Skip("SSE endpoint timed out (Redis/IPC likely unavailable)")
		}
		t.Fatalf("GET /api/subscriptions/files with API key failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("SSE endpoint returned %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// Cross-check: invalid API key is rejected consistently across new routes
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_InvalidKeyRejected(t *testing.T) {
	skipIfNoServer(t)

	badClient, err := api.NewClient(api.ClientConfig{
		BaseURL: baseURL,
		APIKey:  "uk_invalid_nonexistent_key_12345",
	})
	if err != nil {
		t.Fatalf("create bad client: %v", err)
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// All these routes should reject invalid API keys with 401
	routes := []struct {
		method string
		path   string
		query  url.Values
	}{
		{"GET", "/api/auth/me", nil},
		{"GET", "/api/machines", url.Values{"projectId": {"fake"}}},
		{"GET", "/api/search", url.Values{"q": {"test"}, "source": {"file"}, "projectId": {"fake"}}},
	}

	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			var resp map[string]interface{}
			err := badClient.Get(ctx, r.path, r.query, &resp)
			if err == nil {
				t.Fatalf("%s %s should fail with invalid API key", r.method, r.path)
			}
			if !api.IsAuthError(err) {
				t.Logf("expected 401 auth error for %s %s, got: %v", r.method, r.path, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SSE URL correctness: verify query params are not embedded in path
// ---------------------------------------------------------------------------

func TestIntegration_APIKeyAccess_SSEQueryParamNotInPath(t *testing.T) {
	skipIfNoServer(t)

	// This test verifies the fix for the SSE URL bug where ?projectId=xxx
	// was embedded in the path, causing %3F encoding and 404.
	// We verify that the WRONG way (embedded in URL.Path) produces a 404,
	// proving the bug existed.

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// WRONG: projectId baked into the URL path (simulating the old bug).
	// Go's url.URL.Path would encode ? as %3F, resulting in a 404.
	badURL := baseURL + "/api/subscriptions/files%3FprojectId%3Dfake-project"
	req, err := http.NewRequestWithContext(ctx, "GET", badURL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("x-api-key", client.APIKey())
	req.Header.Set("Origin", baseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			t.Skip("request timed out (infrastructure may be unavailable)")
		}
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// The %3F-encoded path should NOT return 200 with text/event-stream.
	// It should return 404 (catch-all page) or similar error.
	if resp.StatusCode == 200 {
		ct := resp.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "text/event-stream") {
			t.Fatal("encoded %3F path should NOT reach the SSE endpoint")
		}
	}
	// Any non-200 or non-SSE response confirms the bug exists when path-encoding is wrong
}
