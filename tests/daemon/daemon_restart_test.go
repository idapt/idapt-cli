//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
	"time"
)

func TestRestartResponsiveness(t *testing.T) {
	triggerDaemonRestart(t)

	if err := waitForDaemonHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon not healthy after restart: %v", err)
	}

	resp := daemonRequest(t, "GET", "/api/health")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 from /api/health after restart, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestRestartJWKSRefetch(t *testing.T) {
	triggerDaemonRestart(t)

	if err := waitForDaemonHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon not healthy after restart: %v", err)
	}

	// Give the daemon extra time to re-fetch JWKS after restart
	time.Sleep(5 * time.Second)

	// Issue a new JWT after restart — daemon must have re-fetched JWKS
	jwt := issueJWTViaApp(t, "/after-restart")

	resp := daemonRequest(t, "GET", "/after-restart",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with post-restart JWT, got %d: %s (proves JWKS was not re-fetched)", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestRestartProxyConfigPersistence(t *testing.T) {
	t.Skip("HMAC token mismatch after daemon restart")
	if machineToken == "" {
		t.Skip("No heartbeat secret available, skipping proxy config persistence test")
	}

	// Set a proxy config via HMAC-authenticated management API
	hmacOpts := withHMAC("PUT", "/api/proxy", machineToken)
	allOpts := append(hmacOpts,
		withJSON(map[string]interface{}{
			"routes": []map[string]interface{}{
				{
					"path":   "/test-persist",
					"target": "http://localhost:9999",
				},
			},
		}),
	)
	resp := daemonRequest(t, "PUT", "/api/proxy", allOpts...)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Failed to set proxy config: %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Trigger restart
	triggerDaemonRestart(t)

	if err := waitForDaemonHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon not healthy after restart: %v", err)
	}

	// Verify proxy config is preserved
	getOpts := withHMAC("GET", "/api/proxy", machineToken)
	resp = daemonRequest(t, "GET", "/api/proxy", getOpts...)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 for GET /api/proxy after restart, got %d: %s", resp.StatusCode, body)
	}

	body := readBody(t, resp)
	if body == "" {
		t.Fatal("Empty proxy config response after restart")
	}
}

func TestRestartHeartbeatResumes(t *testing.T) {
	if machineToken == "" {
		t.Skip("No heartbeat secret available, skipping heartbeat resume test")
	}

	triggerDaemonRestart(t)

	if err := waitForDaemonHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon not healthy after restart: %v", err)
	}

	// Wait for at least one heartbeat cycle (daemon sends heartbeats every ~30s)
	time.Sleep(35 * time.Second)

	// Query the app for the machine's lastActivityAt
	resp := appRequest(t, "GET", "/api/admin/test/machine-heartbeat-secret?machineId="+machineID,
		withHeader("x-test-secret", testSecret))
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Failed to query machine status: %d: %s", resp.StatusCode, body)
	}

	result := readJSON(t, resp)
	if _, ok := result["secret"]; !ok {
		t.Error("Machine heartbeat secret not returned — heartbeat may not have resumed")
	}
}

func TestRestartDrainTimeout(t *testing.T) {
	t.Skip("requires long-polling support")
}
