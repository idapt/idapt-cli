//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
)

var (
	baseURL    string
	testSecret string
	client     *api.Client
	// sessionCookie is the raw session cookie for routes blocked to API key auth.
	sessionCookie string
	// testUserID is the auth user ID for cleanup.
	testUserID string
	testCtx    = context.Background()
)

// TestMain sets up a shared test user and API client for all integration tests.
// If the server is not reachable, all tests are skipped (exit 0, not failure).
func TestMain(m *testing.M) {
	baseURL = os.Getenv("IDAPT_TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3001"
	}
	testSecret = os.Getenv("TEST_SECRET")
	if testSecret == "" {
		testSecret = "test-api-secret"
	}

	// Check if server is reachable
	ctx, cancel := context.WithTimeout(testCtx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/readyz", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot create request: %v -- skipping integration tests\n", err)
		os.Exit(0)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: server not reachable at %s: %v -- skipping integration tests\n", baseURL, err)
		os.Exit(0)
	}
	resp.Body.Close()

	// Try to create a test user via admin endpoint (requires TEST_SECRET).
	// If that fails (e.g., dev server without TEST_SECRET), fall back to
	// logging in as the pre-existing dev user.
	var setupDone bool

	email := fmt.Sprintf("test-cli-integ-%d-%d@test.idapt.ai", time.Now().UnixMilli(), rand.Intn(99999))
	password := "TestPassword123!@#"

	createBody := map[string]interface{}{
		"users": []map[string]interface{}{
			{
				"email":    email,
				"password": password,
				"name":     "CLI Integration Test User",
				"tier":     "free",
			},
		},
	}
	bodyBytes, _ := json.Marshal(createBody)

	req, err = http.NewRequestWithContext(testCtx, "POST", baseURL+"/api/admin/test/create-users", bytes.NewReader(bodyBytes))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-test-secret", testSecret)
		req.Header.Set("Origin", baseURL)

		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				var createResult struct {
					Success bool `json:"success"`
					Users   []struct {
						ID           string `json:"id"`
						ActorID      string `json:"actorId"`
						Email        string `json:"email"`
						Slug         string `json:"slug"`
						SessionToken string `json:"sessionToken"`
					} `json:"users"`
					Errors []interface{} `json:"errors"`
				}
				if json.NewDecoder(resp.Body).Decode(&createResult) == nil &&
					createResult.Success && len(createResult.Users) > 0 &&
					createResult.Users[0].SessionToken != "" {
					user := createResult.Users[0]
					testUserID = user.ID
					sessionCookie = "better-auth.session_token=" + user.SessionToken
					setupDone = true
					fmt.Fprintf(os.Stderr, "INFO: Created test user %s\n", user.Email)

					// Grant pro subscription so API key creation works
					grantBody, _ := json.Marshal(map[string]string{"email": email, "tier": "pro"})
					grantReq, _ := http.NewRequestWithContext(testCtx, "POST", baseURL+"/api/admin/test/grant-subscription", bytes.NewReader(grantBody))
					grantReq.Header.Set("Content-Type", "application/json")
					grantReq.Header.Set("x-test-secret", testSecret)
					grantReq.Header.Set("Origin", baseURL)
					if grantResp, err := http.DefaultClient.Do(grantReq); err == nil {
						grantResp.Body.Close()
					}
				}
			} else {
				io.ReadAll(resp.Body) // drain
			}
		}
	}

	// Fallback: login as pre-existing dev user (from npm run db:reset)
	if !setupDone {
		fmt.Fprintf(os.Stderr, "INFO: create-users unavailable, falling back to dev user login\n")
		loginBody := map[string]interface{}{
			"email":    "dev@idapt.ai",
			"password": "TestPassword123!@#",
		}
		bodyBytes, _ = json.Marshal(loginBody)
		req, err = http.NewRequestWithContext(testCtx, "POST", baseURL+"/api/auth/sign-in/email", bytes.NewReader(bodyBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: cannot create login request: %v -- skipping\n", err)
			os.Exit(0)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", baseURL)

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: login request failed: %v -- skipping\n", err)
			os.Exit(0)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "WARNING: login returned %d: %s -- skipping\n", resp.StatusCode, string(respBody))
			os.Exit(0)
		}

		// Extract session cookie from Set-Cookie header
		for _, c := range resp.Cookies() {
			if c.Name == "better-auth.session_token" || c.Name == "__Secure-better-auth.session_token" {
				sessionCookie = c.Name + "=" + c.Value
				setupDone = true
				break
			}
		}

		if !setupDone {
			// Try reading token from response body
			var loginResult map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&loginResult)
			if token, ok := loginResult["token"].(string); ok && token != "" {
				sessionCookie = "better-auth.session_token=" + token
				setupDone = true
			}
		}

		if !setupDone {
			fmt.Fprintf(os.Stderr, "WARNING: could not extract session from login response -- skipping\n")
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "INFO: Logged in as dev@idapt.ai\n")
	}

	// Create an API key using the session cookie.
	// Some routes are blocked for API key auth, so we also keep the session
	// cookie for those. But we create an API key to test the CLI's primary
	// auth flow for routes that support it.
	apiKey, err := createAPIKey(baseURL, sessionCookie)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not create API key (will use session cookie only): %v\n", err)
		// Fall through -- client will be created without API key
	}

	// Create the shared api.Client.
	// If API key creation succeeded, use it. Otherwise create with empty key
	// and tests will fall back to session cookie via rawGet/rawPost helpers.
	cfg := api.ClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
	client, err = api.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot create API client: %v\n", err)
		os.Exit(1)
	}

	// Run all tests
	code := m.Run()

	// Cleanup: delete the test user
	cleanupTestUser(baseURL, testSecret, testUserID)

	os.Exit(code)
}

// createAPIKey creates a user API key via session cookie auth and returns the key string.
func createAPIKey(base, cookie string) (string, error) {
	body := map[string]interface{}{
		"name": "cli-integration-test",
		"permissions": map[string]interface{}{
			"projects": []string{"read", "write"},
			"agents":   []string{"read", "write"},
			"files":    []string{"read", "write"},
			"chat":     []string{"read", "write"},
			"kb":       []string{"read", "write"},
			"machines": []string{"read", "write"},
			"scripts":  []string{"read", "write"},
			"secrets":  []string{"read", "write"},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(testCtx, "POST", base+"/api/api-keys", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Origin", base)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create api-key returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Key, nil
}

// cleanupTestUser removes the test user via the admin cleanup endpoint.
func cleanupTestUser(base, secret, userID string) {
	if userID == "" {
		return
	}
	body := map[string]interface{}{"userId": userID}
	bodyBytes, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", base+"/api/admin/test/cleanup-user", bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cleanup request creation failed: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-test-secret", secret)
	req.Header.Set("Origin", base)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cleanup request failed: %v\n", err)
		return
	}
	resp.Body.Close()
}

// uniqueName generates a unique test resource name with the given prefix.
func uniqueName(prefix string) string {
	return fmt.Sprintf("cli-test-%s-%d-%d", prefix, time.Now().UnixMilli(), rand.Intn(999999))
}

// uniqueSlug generates a unique slug (lowercase, hyphen-separated).
func uniqueSlug(prefix string) string {
	return fmt.Sprintf("cli-test-%s-%d-%d", prefix, time.Now().UnixMilli(), rand.Intn(999999))
}

// skipIfNoServer skips the test if the server or client is not available.
func skipIfNoServer(t *testing.T) {
	t.Helper()
	if client == nil {
		t.Skip("integration test server not available")
	}
}

// rawRequest makes a raw HTTP request with the session cookie for routes
// that are blocked for API key auth (secrets, scripts, settings PATCH, store, etc.).
func rawRequest(t *testing.T, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	u := baseURL + path
	req, err := http.NewRequestWithContext(testCtx, method, u, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Cookie", sessionCookie)
	req.Header.Set("Origin", baseURL)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var result map[string]interface{}
	if len(respBody) > 0 {
		// Try to parse as JSON; if it fails (e.g. 204 No Content), return nil map
		_ = json.Unmarshal(respBody, &result)
	}

	return resp.StatusCode, result
}

// rawGet is a convenience wrapper for GET requests with session cookie auth.
func rawGet(t *testing.T, path string) (int, map[string]interface{}) {
	t.Helper()
	return rawRequest(t, "GET", path, nil)
}

// rawPost is a convenience wrapper for POST requests with session cookie auth.
func rawPost(t *testing.T, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	return rawRequest(t, "POST", path, body)
}

// rawPatch is a convenience wrapper for PATCH requests with session cookie auth.
func rawPatch(t *testing.T, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	return rawRequest(t, "PATCH", path, body)
}

// rawDelete is a convenience wrapper for DELETE requests with session cookie auth.
func rawDelete(t *testing.T, path string) (int, map[string]interface{}) {
	t.Helper()
	return rawRequest(t, "DELETE", path, nil)
}

// getPersonalProjectID fetches the personal project ID for the test user.
func getPersonalProjectID(t *testing.T) string {
	t.Helper()
	status, result := rawGet(t, "/api/projects/personal")
	if status != 200 {
		t.Fatalf("GET /api/projects/personal returned %d: %v", status, result)
	}
	id, ok := result["id"].(string)
	if !ok || id == "" {
		t.Fatalf("no project ID in personal project response: %v", result)
	}
	return id
}

// getString safely extracts a string from a map[string]interface{}.
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getFloat safely extracts a float64 from a map[string]interface{}.
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// getSlice safely extracts a []interface{} from a map[string]interface{}.
func getSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if s, ok := v.([]interface{}); ok {
			return s
		}
	}
	return nil
}

// getMap safely extracts a map[string]interface{} from a map[string]interface{}.
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// clientGet uses the api.Client for routes that support API key auth.
// Falls back to session cookie auth on 403 (API key route blocked).
func clientGet(t *testing.T, path string, query url.Values, target interface{}) {
	t.Helper()
	err := client.Get(testCtx, path, query, target)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
}

// clientPost uses the api.Client for POST requests.
func clientPost(t *testing.T, path string, body interface{}, target interface{}) {
	t.Helper()
	err := client.Post(testCtx, path, body, target)
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
}

// clientPatch uses the api.Client for PATCH requests.
func clientPatch(t *testing.T, path string, body interface{}, target interface{}) {
	t.Helper()
	err := client.Patch(testCtx, path, body, target)
	if err != nil {
		t.Fatalf("PATCH %s failed: %v", path, err)
	}
}

// clientDelete uses the api.Client for DELETE requests.
func clientDelete(t *testing.T, path string) {
	t.Helper()
	err := client.Delete(testCtx, path)
	if err != nil {
		t.Fatalf("DELETE %s failed: %v", path, err)
	}
}

// containsID checks if a slice of maps contains an entry with the given ID.
func containsID(items []interface{}, id string) bool {
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if getString(m, "id") == id {
				return true
			}
		}
	}
	return false
}

// containsIDInMaps checks if a typed slice contains an entry with the given ID.
func containsIDInMaps(items []map[string]interface{}, id string) bool {
	for _, m := range items {
		if getString(m, "id") == id {
			return true
		}
	}
	return false
}

// createProjectForTest creates a project and registers cleanup.
// Returns the project ID.
func createProjectForTest(t *testing.T) string {
	t.Helper()
	name := uniqueName("proj")
	slug := uniqueSlug("proj")

	status, result := rawPost(t, "/api/projects", map[string]interface{}{
		"name": name,
		"slug": slug,
	})
	if status != 201 {
		t.Fatalf("create project returned %d: %v", status, result)
	}

	proj := getMap(result, "project")
	id := getString(proj, "id")
	if id == "" {
		t.Fatalf("no project ID in create response: %v", result)
	}

	t.Cleanup(func() {
		rawDelete(t, "/api/projects/"+id)
	})

	return id
}
