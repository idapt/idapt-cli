//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
)

func TestFirewallSetRulesValid(t *testing.T) {
	rules := []map[string]interface{}{
		{"port": 8080, "protocol": "tcp", "source": "public"},
		{"port": 3000, "protocol": "tcp", "source": "public"},
	}

	hmacOpts := withHMAC("POST", "/api/firewall", machineToken)
	opts := append(hmacOpts, withJSON(rules))
	resp := daemonRequest(t, "POST", "/api/firewall", opts...)

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	result := readJSON(t, resp)
	if accepted, ok := result["accepted"].(bool); !ok || !accepted {
		t.Errorf("Expected accepted=true, got %v", result["accepted"])
	}
	if count, ok := result["count"].(float64); !ok || count != 2 {
		t.Errorf("Expected count=2, got %v", result["count"])
	}
}

func TestFirewallGetRules(t *testing.T) {
	// First, set some rules so we have something to retrieve
	rules := []map[string]interface{}{
		{"port": 4000, "protocol": "tcp", "source": "public"},
		{"port": 5000, "protocol": "udp", "source": "public"},
	}

	setOpts := append(withHMAC("POST", "/api/firewall", machineToken), withJSON(rules))
	setResp := daemonRequest(t, "POST", "/api/firewall", setOpts...)
	if setResp.StatusCode != http.StatusOK {
		body := readBody(t, setResp)
		t.Fatalf("Failed to set rules: %d: %s", setResp.StatusCode, body)
	}
	setResp.Body.Close()

	// Now GET the rules
	getOpts := withHMAC("GET", "/api/firewall", machineToken)
	resp := daemonRequest(t, "GET", "/api/firewall", getOpts...)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	body := readBody(t, resp)
	if !strings.Contains(body, "4000") {
		t.Errorf("Expected rules to contain port 4000, got: %s", body)
	}
	if !strings.Contains(body, "5000") {
		t.Errorf("Expected rules to contain port 5000, got: %s", body)
	}
}

func TestFirewallMissingHMAC(t *testing.T) {
	rules := []map[string]interface{}{
		{"port": 8080, "protocol": "tcp", "source": "public"},
	}

	// Send without any HMAC headers
	resp := daemonRequest(t, "POST", "/api/firewall", withJSON(rules))
	if resp.StatusCode != http.StatusForbidden {
		body := readBody(t, resp)
		t.Fatalf("Expected 403 without HMAC, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestFirewallWrongHMAC(t *testing.T) {
	rules := []map[string]interface{}{
		{"port": 8080, "protocol": "tcp", "source": "public"},
	}

	// Use a bogus secret for HMAC
	wrongOpts := append(withHMAC("POST", "/api/firewall", "deadbeefdeadbeefdeadbeefdeadbeef"), withJSON(rules))
	resp := daemonRequest(t, "POST", "/api/firewall", wrongOpts...)
	if resp.StatusCode != http.StatusForbidden {
		body := readBody(t, resp)
		t.Fatalf("Expected 403 with wrong HMAC, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestFirewallInvalidRules(t *testing.T) {
	rules := []map[string]interface{}{
		{"port": 70000, "protocol": "tcp", "source": "public"},
	}

	hmacOpts := withHMAC("POST", "/api/firewall", machineToken)
	opts := append(hmacOpts, withJSON(rules))
	resp := daemonRequest(t, "POST", "/api/firewall", opts...)

	if resp.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp)
		t.Fatalf("Expected 400 for invalid port, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestFirewallMockIptablesLog(t *testing.T) {
	// Set rules to trigger iptables execution
	rules := []map[string]interface{}{
		{"port": 9999, "protocol": "tcp", "source": "public"},
	}

	hmacOpts := withHMAC("POST", "/api/firewall", machineToken)
	opts := append(hmacOpts, withJSON(rules))
	resp := daemonRequest(t, "POST", "/api/firewall", opts...)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Check the mock iptables log inside the daemon container
	output, errStr := daemonExec(t, "cat /var/log/idapt/iptables.log")
	if errStr != "" {
		t.Logf("daemonExec error output: %s", errStr)
	}

	if !strings.Contains(output, "iptables") {
		t.Errorf("Expected iptables.log to contain iptables commands, got: %s", output)
	}
}
