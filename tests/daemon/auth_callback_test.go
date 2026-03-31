//go:build daemontest

package daemon

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestAuthCallbackValidJWT(t *testing.T) {
	jwt := issueJWTViaApp(t, "/dashboard")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/dashboard")
	if resp.StatusCode != http.StatusFound {
		body := readBody(t, resp)
		t.Fatalf("Expected 302 redirect, got %d: %s", resp.StatusCode, body)
	}

	location := resp.Header.Get("Location")
	if !strings.HasSuffix(location, "/dashboard") {
		t.Errorf("Expected redirect to /dashboard, got Location: %s", location)
	}

	setCookie := resp.Header.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("Expected Set-Cookie header, got none")
	}
	resp.Body.Close()
}

func TestAuthCallbackCookieHttpOnly(t *testing.T) {
	jwt := issueJWTViaApp(t, "/test")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/test")
	setCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	if !strings.Contains(setCookie, "HttpOnly") {
		t.Errorf("Set-Cookie missing HttpOnly flag: %s", setCookie)
	}
}

func TestAuthCallbackCookieSecure(t *testing.T) {
	jwt := issueJWTViaApp(t, "/test")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/test")
	setCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	if !strings.Contains(setCookie, "Secure") {
		t.Errorf("Set-Cookie missing Secure flag: %s", setCookie)
	}
}

func TestAuthCallbackCookieSameSite(t *testing.T) {
	jwt := issueJWTViaApp(t, "/test")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/test")
	setCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	if !strings.Contains(setCookie, "SameSite=Lax") {
		t.Errorf("Set-Cookie missing SameSite=Lax: %s", setCookie)
	}
}

func TestAuthCallbackCookiePath(t *testing.T) {
	jwt := issueJWTViaApp(t, "/test")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/test")
	setCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	if !strings.Contains(setCookie, "Path=/") {
		t.Errorf("Set-Cookie missing Path=/: %s", setCookie)
	}
}

func TestAuthCallbackCookieMaxAge(t *testing.T) {
	jwt := issueJWTViaApp(t, "/test")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=/test")
	setCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	// Extract Max-Age value
	if !strings.Contains(setCookie, "Max-Age=") {
		t.Fatalf("Set-Cookie missing Max-Age: %s", setCookie)
	}

	for _, part := range strings.Split(setCookie, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Max-Age=") {
			val := strings.TrimPrefix(part, "Max-Age=")
			maxAge, err := strconv.Atoi(val)
			if err != nil {
				t.Fatalf("Invalid Max-Age value %q: %v", val, err)
			}
			// Should be around 86400 (24h) — allow some tolerance
			if maxAge < 3600 || maxAge > 172800 {
				t.Errorf("Max-Age %d seems unreasonable (expected ~86400)", maxAge)
			}
			return
		}
	}
	t.Error("Could not find Max-Age in Set-Cookie parts")
}

func TestAuthCallbackMissingToken(t *testing.T) {
	resp := daemonRequest(t, "GET", "/__auth_callback")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing token, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuthCallbackInvalidToken(t *testing.T) {
	resp := daemonRequest(t, "GET", "/__auth_callback?token=garbage.invalid.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuthCallbackOpenRedirect(t *testing.T) {
	jwt := issueJWTViaApp(t, "/safe")

	resp := daemonRequest(t, "GET", "/__auth_callback?token="+jwt+"&path=//evil.com")

	location := resp.Header.Get("Location")
	resp.Body.Close()

	// Must NOT redirect to evil.com — should redirect to / or strip the double-slash
	if strings.Contains(location, "evil.com") {
		t.Errorf("Open redirect vulnerability: redirected to %s", location)
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		// If it redirects, it must be to a safe path (/ or similar)
		if strings.HasPrefix(location, "//") || strings.Contains(location, "://") {
			// Allow https://daemon-test-api:8443/ but not //evil.com
			if !strings.Contains(location, daemonURL) && strings.HasPrefix(location, "//") {
				t.Errorf("Redirect to protocol-relative URL not allowed: %s", location)
			}
		}
	}
}
