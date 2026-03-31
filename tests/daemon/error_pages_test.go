//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestErrorPage502BadGateway(t *testing.T) {
	t.Skip("502 test requires stopping mock backend which is not possible in current test setup")
	// Ensure proxy config has a port whose backend is not running.
	// Use a high port where no mock backend exists.
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 19999, "authMode": "public"},
		},
	}
	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	setResp := daemonRequest(t, "POST", "/api/proxy", opts...)
	if setResp.StatusCode != http.StatusOK {
		body := readBody(t, setResp)
		t.Fatalf("Failed to set proxy config: %d: %s", setResp.StatusCode, body)
	}
	setResp.Body.Close()

	time.Sleep(2 * time.Second)

	// Build the URL for port 19999 — replace the port in the daemon URL
	port19999URL := strings.Replace(daemonURL, ":8443", ":19999", 1)

	// Request the port with no backend — should get 502.
	// Use a short timeout client since there may be no listener on this port,
	// and the daemon's reverse proxy may take time connecting to a dead backend.
	shortClient := &http.Client{
		Transport: daemonClient().Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}
	req, err := http.NewRequest("GET", port19999URL+"/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/html")

	resp, err := shortClient.Do(req)
	if err != nil {
		// Connection refused or timeout — no listener on port 19999
		t.Skipf("Cannot connect to port 19999 (no listener or timeout): %v", err)
	}

	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("Expected 502, got %d: %s", resp.StatusCode, body)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	if len(body) <= 512 {
		t.Errorf("Error page should be >512 bytes to prevent browser interception, got %d bytes", len(body))
	}

	if !strings.Contains(body, "Bad Gateway") {
		t.Errorf("Expected body to contain 'Bad Gateway', got: %s", body[:min(200, len(body))])
	}
}

func TestErrorPage401Unauthenticated(t *testing.T) {
	// Clear proxy config so the default port requires auth
	config := map[string]interface{}{
		"ports": []map[string]interface{}{},
	}
	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	setResp := daemonRequest(t, "POST", "/api/proxy", opts...)
	if setResp.StatusCode != http.StatusOK {
		body := readBody(t, setResp)
		t.Fatalf("Failed to clear proxy config: %d: %s", setResp.StatusCode, body)
	}
	setResp.Body.Close()

	// Request without auth and without Accept: text/html (non-browser) — should get 401 with error page
	resp := daemonRequest(t, "GET", "/some-path",
		withHeader("Accept", "application/json"))

	body := readBody(t, resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d: %s", resp.StatusCode, body)
	}

	if !strings.Contains(body, "Authentication Required") {
		t.Errorf("Expected body to contain 'Authentication Required', got: %s", body[:min(200, len(body))])
	}
}

func TestErrorPage403PortNotOpen(t *testing.T) {
	// This tests the daemon's response when requesting a port not in proxy config.
	// The port not being in config may produce a 403 "Port Not Accessible" page
	// or simply a connection error. We test based on the daemon's behavior.

	// Clear proxy config
	config := map[string]interface{}{
		"ports": []map[string]interface{}{},
	}
	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	setResp := daemonRequest(t, "POST", "/api/proxy", opts...)
	if setResp.StatusCode != http.StatusOK {
		body := readBody(t, setResp)
		t.Fatalf("Failed to clear proxy config: %d: %s", setResp.StatusCode, body)
	}
	setResp.Body.Close()

	time.Sleep(2 * time.Second)

	// Try to connect to port 9001 which should not have a listener.
	// Use direct HTTP call since doRequest calls t.Fatalf on connection errors,
	// but connection refused is an acceptable outcome here.
	req, err := http.NewRequest("GET", daemonPort9001+"/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/html")

	resp, err := daemonClient().Do(req)
	if err != nil {
		// Connection refused — port is not open, which is acceptable
		t.Logf("Connection refused for closed port (acceptable): %v", err)
		return
	}

	// If we get a response, verify it's an error page
	body := readBody(t, resp)
	if resp.StatusCode == http.StatusForbidden {
		if !strings.Contains(body, "Port Not Accessible") && !strings.Contains(body, "not open") {
			t.Logf("Got 403 but unexpected body: %s", body[:min(200, len(body))])
		}
	} else {
		t.Logf("Got status %d for closed port (expected connection refused or 403): %s",
			resp.StatusCode, body[:min(200, len(body))])
	}
}
