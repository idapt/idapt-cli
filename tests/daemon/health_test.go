//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

func TestHealthEndpointReturnsJSON(t *testing.T) {
	resp := daemonRequest(t, "GET", "/api/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	body := readJSON(t, resp)

	if _, ok := body["status"]; !ok {
		t.Error("Response missing 'status' field")
	}
	if _, ok := body["version"]; !ok {
		t.Error("Response missing 'version' field")
	}
	if _, ok := body["proxyPorts"]; !ok {
		t.Error("Response missing 'proxyPorts' field")
	}
}

func TestHealthEndpointNoAuth(t *testing.T) {
	// Request without any auth headers or cookies — should still succeed
	resp := daemonRequest(t, "GET", "/api/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 without auth, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
