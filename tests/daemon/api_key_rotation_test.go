//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

// Daemon passes through all Bearer tokens without validation.
// Key rotation is an app-level concern — daemon just forwards.

func TestBearerTokenMultipleKeysPassthrough(t *testing.T) {
	// Multiple different Bearer tokens all pass through
	keys := []string{
		"mk_testrotation_old",
		"mk_testrotation_new",
		"mk_testrotation_independent",
	}

	for _, key := range keys {
		resp := daemonRequest(t, "GET", "/",
			withBearer(key))

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			body := readBody(t, resp)
			t.Fatalf("Expected 200 or 502 for key %q, got %d: %s", key, resp.StatusCode, body)
		}
		resp.Body.Close()
	}
}
