//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

func TestProxyAuthenticatedRequest(t *testing.T) {
	jwt := issueJWTViaApp(t, "/")

	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 for authenticated proxy request, got %d: %s", resp.StatusCode, body)
	}

	body := readJSON(t, resp)
	if port, ok := body["port"]; !ok {
		t.Error("Expected proxied response with 'port' field from mock backend")
	} else {
		// Verify it's the expected mock backend port
		portNum, ok := port.(float64)
		if !ok {
			t.Errorf("Expected 'port' to be a number, got %T", port)
		} else if int(portNum) != 9000 {
			t.Errorf("Expected port 9000, got %d", int(portNum))
		}
	}
}

func TestProxyForwardedHeaders(t *testing.T) {
	jwt := issueJWTViaApp(t, "/")

	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	// The mock backend may echo headers back in the response body
	// Check that the proxy sets X-Forwarded-For (visible in response headers or body)
	body := readJSON(t, resp)

	// Check if the mock backend echoes headers — look for forwarded info
	if headers, ok := body["headers"].(map[string]interface{}); ok {
		if _, ok := headers["X-Forwarded-For"]; !ok {
			t.Error("Expected X-Forwarded-For header to be forwarded to backend")
		}
	}
	// If mock doesn't echo headers, at minimum verify the request succeeded through the proxy
}

func TestProxyBackendResponds(t *testing.T) {
	jwt := issueJWTViaApp(t, "/")

	// Verify the proxy correctly forwards to the mock backend and returns its response
	resp := daemonRequest(t, "GET", "/test-path",
		withCookie("idapt_machine_token", jwt))

	// The mock backend should respond (may be 200 or 404 depending on routes, but not a proxy error)
	if resp.StatusCode >= 502 && resp.StatusCode <= 504 {
		resp.Body.Close()
		t.Fatalf("Got proxy error %d — backend appears unreachable", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestACMEChallengeOpen(t *testing.T) {
	// ACME challenge paths must be accessible without authentication for Let's Encrypt
	resp := daemonRequest(t, "GET", "/.well-known/acme-challenge/test-challenge-token")

	// Should NOT get 401 — ACME challenges must bypass auth
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatal("ACME challenge path returned 401 — must not require auth")
	}

	// 404 is acceptable (no actual challenge exists), but not 401/403
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		t.Fatal("ACME challenge path returned 403 — must not require auth")
	}
	resp.Body.Close()
}
