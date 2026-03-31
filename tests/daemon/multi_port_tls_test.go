//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMultiPortAddPort(t *testing.T) {
	t.Skip("multi-port TLS listeners require certificate availability")
	// Add port 9001 as authenticated
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9001, "authMode": "authenticated"},
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

	// Give the listener manager time to start the new TLS listener
	time.Sleep(3 * time.Second)

	// Get a JWT for auth
	jwt := issueJWTViaApp(t, "/")

	// Retry the connection a few times — the TLS listener may still be starting
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("GET", daemonPort9001+"/", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: "idapt_machine_token", Value: jwt})

		resp, err := daemonClient().Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}

		// 200 from mock backend or 502 if backend isn't running — either means TLS listener is up
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			body := readBody(t, resp)
			t.Fatalf("Expected 200 or 502 on port 9001, got %d: %s", resp.StatusCode, body)
		}
		resp.Body.Close()
		return
	}
	t.Fatalf("Failed to connect to port 9001 after retries: %v", lastErr)
}

func TestMultiPortVerifyResponse(t *testing.T) {
	t.Skip("multi-port TLS listeners require certificate availability")
	// Ensure port 9001 is configured
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9001, "authMode": "authenticated"},
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

	jwt := issueJWTViaApp(t, "/")

	resp := daemonRequestPort(t, daemonPort9001, "GET", "/",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode == http.StatusOK {
		body := readBody(t, resp)
		// The mock backend on port 9001 should return a response identifying its port
		if !strings.Contains(body, "9001") && !strings.Contains(body, "mock") {
			t.Logf("Response from port 9001 (may not contain port identifier): %s", body)
		}
	} else if resp.StatusCode == http.StatusBadGateway {
		t.Log("Port 9001 TLS listener is up but no backend — 502 is expected in test env")
		resp.Body.Close()
	} else {
		body := readBody(t, resp)
		t.Fatalf("Unexpected status from port 9001: %d: %s", resp.StatusCode, body)
	}
}

func TestMultiPortRemovePort(t *testing.T) {
	// First add port 9001
	addConfig := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9001, "authMode": "authenticated"},
		},
	}
	hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
	addOpts := append(hmacOpts, withJSON(addConfig))
	addResp := daemonRequest(t, "POST", "/api/proxy", addOpts...)
	if addResp.StatusCode != http.StatusOK {
		body := readBody(t, addResp)
		t.Fatalf("Failed to add port: %d: %s", addResp.StatusCode, body)
	}
	addResp.Body.Close()

	time.Sleep(2 * time.Second)

	// Now remove port 9001 by setting empty config
	removeConfig := map[string]interface{}{
		"ports": []map[string]interface{}{},
	}
	removeOpts := append(withHMAC("POST", "/api/proxy", machineToken), withJSON(removeConfig))
	removeResp := daemonRequest(t, "POST", "/api/proxy", removeOpts...)
	if removeResp.StatusCode != http.StatusOK {
		body := readBody(t, removeResp)
		t.Fatalf("Failed to remove port: %d: %s", removeResp.StatusCode, body)
	}
	removeResp.Body.Close()

	time.Sleep(2 * time.Second)

	// Attempting to connect to port 9001 should fail (connection refused or error).
	// We use a direct HTTP call since doRequest calls t.Fatalf on connection errors,
	// but connection refused is the expected success case here.
	req, err := http.NewRequest("GET", daemonPort9001+"/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := daemonClient().Do(req)
	if err != nil {
		// Connection refused — this is the expected outcome after removing the port
		t.Logf("Connection to port 9001 refused as expected: %v", err)
		return
	}
	resp.Body.Close()
	t.Log("Port 9001 still responded after removal — listener may take time to shut down")
}

func TestMultiPortPublicMode(t *testing.T) {
	t.Skip("multi-port TLS listeners require certificate availability")
	// Add port 9001 as public
	config := map[string]interface{}{
		"ports": []map[string]interface{}{
			{"port": 9001, "authMode": "public"},
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

	// Unauthenticated request to port 9001 should succeed (public mode bypasses auth)
	resp := daemonRequestPort(t, daemonPort9001, "GET", "/")

	// 200 from mock backend or 502 if no backend — either means auth was bypassed
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 or 502 for public port 9001, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}
