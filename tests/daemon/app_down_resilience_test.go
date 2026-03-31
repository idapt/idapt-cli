//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
	"time"
)

func TestAppDownJWTStillWorks(t *testing.T) {
	// Issue JWT while app is up
	jwt := issueJWTViaApp(t, "/resilience-jwt")

	// Block app connectivity
	blockApp(t, true)
	t.Cleanup(func() { blockApp(t, false) })

	// Wait briefly for block to take effect
	time.Sleep(2 * time.Second)

	// JWT validation is local (ES256 with cached public key) — should still work
	resp := daemonRequest(t, "GET", "/resilience-jwt",
		withCookie("idapt_machine_token", jwt))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with JWT while app is down (ES256 validation is local), got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAppDownDaemonHealthy(t *testing.T) {
	blockApp(t, true)
	t.Cleanup(func() { blockApp(t, false) })

	// Wait for block to take effect
	time.Sleep(5 * time.Second)

	// Daemon health endpoint should still respond — it does not depend on app
	resp := daemonRequest(t, "GET", "/api/health")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 from /api/health while app is down, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAppDownAuthCallbackStillWorks(t *testing.T) {
	t.Skip("blockApp uses mock iptables in test mode — cannot actually block outbound connections")
	// Issue JWT while app is up
	jwt := issueJWTViaApp(t, "/resilience-callback")

	// Block app connectivity
	blockApp(t, true)
	t.Cleanup(func() { blockApp(t, false) })

	// Wait briefly for block to take effect
	time.Sleep(2 * time.Second)

	// Auth callback validates JWT locally — should still set cookie and redirect
	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/resilience-callback")

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusTemporaryRedirect {
		body := readBody(t, resp)
		t.Fatalf("Expected redirect from __auth_callback while app is down, got %d: %s", resp.StatusCode, body)
	}

	setCookie := resp.Header.Get("Set-Cookie")
	if setCookie == "" {
		t.Error("Expected Set-Cookie header from __auth_callback while app is down")
	}
	resp.Body.Close()
}
