//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
	"time"
)

func TestPublicPortBypassesAuth(t *testing.T) {
	t.Skip("public port bypass requires multi-port TLS listener")
	// Configure port 9000 as public
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9000, "authMode": "public"},
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

	// Allow time for the proxy config change to propagate to the listener
	time.Sleep(1 * time.Second)

	// Make an unauthenticated request — should succeed because port is public
	resp := daemonRequest(t, "GET", "/")
	statusOK := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadGateway
	if !statusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 or 502 (no backend) for public port, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAuthenticatedPortRequiresAuth(t *testing.T) {
	// Configure port 9000 as authenticated
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9000, "authMode": "authenticated"},
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

	// Unauthenticated request should be rejected (401 or 302 redirect)
	resp := daemonRequest(t, "GET", "/")
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusFound {
		body := readBody(t, resp)
		t.Fatalf("Expected 401 or 302 for authenticated port without auth, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestDefaultPortRequiresAuth(t *testing.T) {
	// Clear proxy config — no ports configured
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

	// Unauthenticated request should be rejected (default is authenticated)
	resp := daemonRequest(t, "GET", "/")
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusFound {
		body := readBody(t, resp)
		t.Fatalf("Expected 401 or 302 for default (no config) port without auth, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}
