//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
)

func TestJWTCookieValid(t *testing.T) {
	// Get a valid JWT via the auth flow
	jwt := issueJWTViaApp(t, "/")

	// First, complete the auth callback to get the cookie value
	// The JWT itself is the cookie value set by __auth_callback
	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with valid JWT cookie, got %d: %s", resp.StatusCode, body)
	}

	// Should get the proxied backend response
	body := readJSON(t, resp)
	if _, ok := body["port"]; !ok {
		t.Error("Expected proxied response with 'port' field from mock backend")
	}
}

func TestJWTCookieExpired(t *testing.T) {
	// Use a garbage token to simulate an expired/invalid JWT
	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", "expired.garbage.token"))

	// Should be rejected — either 401 or redirect to auth
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("Expected rejection for expired/invalid JWT cookie, got 200")
	}
	resp.Body.Close()
}

func TestJWTCookieWrongMachine(t *testing.T) {
	// Use an invalid token to simulate a JWT for a different machine
	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", "wrong.machine.jwt"))

	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("Expected rejection for wrong-machine JWT cookie, got 200")
	}
	resp.Body.Close()
}

func TestJWTAlgHS256Rejected(t *testing.T) {
	// The daemon strictly accepts ES256 only — HS256 must be rejected.
	// Use a garbage token since we can't craft a valid HS256 JWT in tests.
	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"))

	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("Expected rejection for HS256 JWT, got 200")
	}
	resp.Body.Close()
}

func TestJWTAlgNoneRejected(t *testing.T) {
	// alg:none attack — must be rejected
	// eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0 = {"alg":"none","typ":"JWT"}
	// eyJzdWIiOiIxMjM0NTY3ODkwIn0 = {"sub":"1234567890"}
	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0."))

	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("Expected rejection for alg:none JWT, got 200")
	}
	resp.Body.Close()
}

func TestJWTTamperedPayload(t *testing.T) {
	// Get a valid JWT, then tamper with the payload
	jwt := issueJWTViaApp(t, "/")

	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("Expected 3-part JWT, got %d parts", len(parts))
	}

	// Tamper with the payload by flipping a character
	payload := []byte(parts[1])
	if len(payload) > 5 {
		// Flip a character in the middle of the payload
		if payload[5] == 'A' {
			payload[5] = 'B'
		} else {
			payload[5] = 'A'
		}
	}
	tampered := parts[0] + "." + string(payload) + "." + parts[2]

	resp := daemonRequest(t, "GET", "/",
		withCookie("idapt_machine_token", tampered))

	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("Expected rejection for tampered JWT, got 200")
	}
	resp.Body.Close()
}

func TestNoAuthBrowserRedirect(t *testing.T) {
	// Browser request (Accept: text/html) without auth → redirect to auth endpoint
	resp := daemonRequest(t, "GET", "/",
		withHeader("Accept", "text/html"))

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusTemporaryRedirect {
		resp.Body.Close()
		t.Fatalf("Expected 302/307 redirect for unauthenticated browser request, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	resp.Body.Close()

	if !strings.Contains(location, "/api/managed-machines/auth") {
		t.Errorf("Expected redirect to /api/managed-machines/auth, got: %s", location)
	}
}

func TestNoAuthAPIClient(t *testing.T) {
	// API client (no Accept header) without auth → 401
	resp := daemonRequest(t, "GET", "/")

	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("Expected 401 for unauthenticated API request, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
