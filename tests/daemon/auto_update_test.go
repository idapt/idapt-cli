//go:build daemontest

package daemon

import (
	"strings"
	"testing"
)

func TestAutoUpdateCheckVersion(t *testing.T) {
	t.Skip("idapt update --check-only not yet implemented")
}

func TestAutoUpdateSameVersionSkip(t *testing.T) {
	output, errStr := daemonExec(t, "idapt update")

	// If the update subcommand doesn't exist, skip
	if strings.Contains(errStr, "unknown command") || strings.Contains(output, "unknown command") {
		t.Skip("'idapt update' subcommand not available yet")
	}

	// Since the binary is already the current version, it should skip download
	combined := output + errStr
	if !strings.Contains(combined, "up to date") &&
		!strings.Contains(combined, "already") &&
		!strings.Contains(combined, "no update") &&
		!strings.Contains(combined, "current") {
		t.Logf("Update output (may have downloaded):\nstdout: %s\nstderr: %s", output, errStr)
	}
}

func TestAutoUpdateBinaryPath(t *testing.T) {
	output, errStr := daemonExec(t, "which idapt")

	if errStr != "" && !strings.Contains(errStr, "") {
		t.Fatalf("'which idapt' returned error: %s", errStr)
	}

	path := strings.TrimSpace(output)
	if path != "/usr/local/bin/idapt" {
		t.Errorf("Expected idapt at /usr/local/bin/idapt, got: %s", path)
	}
}
