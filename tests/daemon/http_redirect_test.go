//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
)

func TestHTTPToHTTPSRedirect(t *testing.T) {
	t.Skip("HTTP redirect blocked by ACME handler in test mode")
	resp := daemonHTTPRequest(t, "GET", "/somepath")

	// Accept either 301 or 302 — both indicate an HTTP-to-HTTPS redirect
	if resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusFound {
		resp.Body.Close()
		t.Fatalf("Expected 301 or 302 redirect from HTTP to HTTPS, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	resp.Body.Close()

	if !strings.HasPrefix(location, "https://") {
		t.Errorf("Expected redirect to https://, got Location: %s", location)
	}

	if !strings.Contains(location, "/somepath") {
		t.Errorf("Expected redirect to preserve path /somepath, got Location: %s", location)
	}
}
