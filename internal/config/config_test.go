package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtSecret": "test-secret",
		"machineToken": "test-token"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MachineID != "mm-123" {
		t.Errorf("MachineID = %q, want %q", cfg.MachineID, "mm-123")
	}
	if cfg.AppURL != "https://idapt.ai" {
		t.Errorf("AppURL = %q, want %q", cfg.AppURL, "https://idapt.ai")
	}
	if cfg.Domain != "my-machine.idapt.app" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "my-machine.idapt.app")
	}
	if cfg.DefaultBackendPort != 80 {
		t.Errorf("DefaultBackendPort = %d, want 80", cfg.DefaultBackendPort)
	}
	if cfg.ACMEEmail != "machines@idapt.ai" {
		t.Errorf("ACMEEmail = %q, want %q", cfg.ACMEEmail, "machines@idapt.ai")
	}
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-original",
		"appUrl": "https://original.ai",
		"domain": "original.idapt.app",
		"jwtSecret": "original-secret",
		"machineToken": "original-token"
	}`)

	t.Setenv("IDAPT_MACHINE_ID", "mm-override")
	t.Setenv("IDAPT_APP_URL", "https://override.ai")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MachineID != "mm-override" {
		t.Errorf("MachineID = %q, want %q (env override)", cfg.MachineID, "mm-override")
	}
	if cfg.AppURL != "https://override.ai" {
		t.Errorf("AppURL = %q, want %q (env override)", cfg.AppURL, "https://override.ai")
	}
}

func TestLoad_MissingMachineID(t *testing.T) {
	path := writeTestConfig(t, `{
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtSecret": "test-secret",
		"machineToken": "test-token"
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing machineId")
	}
	if err.Error() != "machineId is required" {
		t.Errorf("error = %q, want 'machineId is required'", err.Error())
	}
}

func TestLoad_MissingAppURL(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"domain": "my-machine.idapt.app",
		"jwtSecret": "test-secret",
		"machineToken": "test-token"
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing appUrl")
	}
}

func TestLoad_WildcardDomain(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "*.idapt.app",
		"jwtSecret": "test-secret",
		"machineToken": "test-token"
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for wildcard domain")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeTestConfig(t, `{invalid json}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeTestConfig(t, `{}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty config (missing required fields)")
	}
}
