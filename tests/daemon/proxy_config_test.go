//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

func TestProxyConfigSetValid(t *testing.T) {
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9000, "authMode": "authenticated"},
		},
	}

	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	resp := daemonRequest(t, "POST", "/api/proxy", opts...)

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	result := readJSON(t, resp)
	if accepted, ok := result["accepted"].(bool); !ok || !accepted {
		t.Errorf("Expected accepted=true, got %v", result["accepted"])
	}
	if count, ok := result["count"].(float64); !ok || count != 1 {
		t.Errorf("Expected count=1, got %v", result["count"])
	}
}

func TestProxyConfigGet(t *testing.T) {
	// Set a config first
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 7000, "authMode": "public"},
			{"port": 7001, "authMode": "authenticated"},
		},
	}
	setOpts := append(withHMAC("POST", "/api/proxy", machineToken), withJSON(config))
	setResp := daemonRequest(t, "POST", "/api/proxy", setOpts...)
	if setResp.StatusCode != http.StatusOK {
		body := readBody(t, setResp)
		t.Fatalf("Failed to set config: %d: %s", setResp.StatusCode, body)
	}
	setResp.Body.Close()

	// GET the config
	getOpts := withHMAC("GET", "/api/proxy", machineToken)
	resp := daemonRequest(t, "GET", "/api/proxy", getOpts...)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	result := readJSON(t, resp)
	ports, ok := result["ports"].([]interface{})
	if !ok {
		t.Fatalf("Expected ports array in response, got %v", result)
	}
	if len(ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(ports))
	}
}

func TestProxyConfigInvalidAuthMode(t *testing.T) {
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9000, "authMode": "invalid"},
		},
	}

	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	resp := daemonRequest(t, "POST", "/api/proxy", opts...)

	if resp.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp)
		t.Fatalf("Expected 400 for invalid authMode, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestProxyConfigInvalidPort(t *testing.T) {
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 0, "authMode": "authenticated"},
		},
	}

	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	opts := append(hmacOpts, withJSON(config))
	resp := daemonRequest(t, "POST", "/api/proxy", opts...)

	if resp.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp)
		t.Fatalf("Expected 400 for port 0, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}
