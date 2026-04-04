//go:build daemontest

package daemon

import (
	"net/http"
	"strings"
	"testing"
)

func TestInstallScriptContent(t *testing.T) {
	resp := appRequest(t, "GET", "/cli/install")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 from /cli/install, got %d: %s", resp.StatusCode, body)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/") {
		t.Errorf("Expected Content-Type containing text/, got: %s", contentType)
	}

	body := readBody(t, resp)
	if !strings.HasPrefix(body, "#!") {
		t.Errorf("Expected script to start with #!, got: %.50s", body)
	}
	if !strings.Contains(body, "idapt") {
		t.Error("Expected install script to contain 'idapt'")
	}
}

func TestVersionEndpoint(t *testing.T) {
	resp := appRequest(t, "GET", "/api/cli/version")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 from /api/cli/version, got %d: %s", resp.StatusCode, body)
	}

	result := readJSON(t, resp)

	if _, ok := result["version"]; !ok {
		t.Error("Response missing 'version' field")
	}
	if _, ok := result["downloadUrl"]; !ok {
		t.Error("Response missing 'downloadUrl' field")
	}
}

func TestSetupScriptContent(t *testing.T) {
	t.Skip("setup script endpoint not yet implemented")
}

func TestSetupScriptExecution(t *testing.T) {
	t.Skip("setup script endpoint not yet implemented")
}

func TestSetupScriptIdempotency(t *testing.T) {
	t.Skip("setup script endpoint not yet implemented")
}
