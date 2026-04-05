//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

// Daemon passes through all Bearer tokens to the upstream app for validation.
// These tests verify the pass-through behavior — the daemon does NOT validate API keys itself.

func TestBearerTokenPassthrough(t *testing.T) {
	// Any Bearer token passes through the daemon to the backend
	resp := daemonRequest(t, "GET", "/",
		withBearer("mk_testkey123"))

	// 200 (backend) or 502 (no backend) both prove auth passed through
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 or 502 (pass-through), got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestBearerTokenInvalidPassthrough(t *testing.T) {
	// Invalid tokens also pass through — app validates, not daemon
	resp := daemonRequest(t, "GET", "/",
		withBearer("mk_wrongkey"))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 or 502 (pass-through), got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestBearerTokenWrongPrefixPassthrough(t *testing.T) {
	// Non-mk_ prefixed tokens also pass through
	resp := daemonRequest(t, "GET", "/",
		withBearer("sk_something"))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 or 502 (pass-through), got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestBearerTokenEmptyPassthrough(t *testing.T) {
	// Empty Bearer token passes through — backend decides response
	resp := daemonRequest(t, "GET", "/",
		withHeader("Authorization", "Bearer "))

	// Must not get a browser redirect (302) — that would mean daemon treated it as no-auth
	if resp.StatusCode == http.StatusFound {
		resp.Body.Close()
		t.Fatalf("Expected non-redirect for Bearer header, got 302")
	}
	resp.Body.Close()
}
