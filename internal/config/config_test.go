package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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

// generateTestPublicKeyPEM generates an ECDSA P-256 public key PEM for tests.
func generateTestPublicKeyPEM(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return string(pubPEM)
}

// writeTestKeyFile writes a PEM public key to a temp file and returns the path.
func writeTestKeyFile(t *testing.T, pemContent string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt-public-key.pem")
	if err := os.WriteFile(path, []byte(pemContent), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidConfig_WithPEM(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
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
	if cfg.JWTPublicKeyPEM == "" {
		t.Error("JWTPublicKeyPEM should be populated")
	}
	if cfg.DefaultBackendPort != 80 {
		t.Errorf("DefaultBackendPort = %d, want 80", cfg.DefaultBackendPort)
	}
	if cfg.ACMEEmail != "machines@idapt.ai" {
		t.Errorf("ACMEEmail = %q, want %q", cfg.ACMEEmail, "machines@idapt.ai")
	}
}

func TestLoad_ValidConfig_WithFile(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	keyFile := writeTestKeyFile(t, pubPEM)

	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyFile": `+jsonEscape(keyFile)+`
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.JWTPublicKeyPEM == "" {
		t.Error("JWTPublicKeyPEM should be populated from file")
	}
	if cfg.JWTPublicKeyPEM != pubPEM {
		t.Error("JWTPublicKeyPEM should match file contents")
	}
}

func TestLoad_ValidConfig_NoMachineToken(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v (machineToken should be optional)", err)
	}
	if cfg.MachineToken != "" {
		t.Errorf("MachineToken = %q, want empty (optional)", cfg.MachineToken)
	}
}

func TestLoad_MissingBothPEMAndFile(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app"
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when both jwtPublicKeyPEM and jwtPublicKeyFile are missing")
	}
}

func TestLoad_EmptyPEM(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": ""
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty jwtPublicKeyPEM")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyFile": "/does/not/exist/jwt-public-key.pem"
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for nonexistent key file")
	}
}

func TestLoad_FileOverridesPEM(t *testing.T) {
	inlinePEM := "inline-pem-content"
	filePEM := generateTestPublicKeyPEM(t)
	keyFile := writeTestKeyFile(t, filePEM)

	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(inlinePEM)+`,
		"jwtPublicKeyFile": `+jsonEscape(keyFile)+`
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should take precedence over inline PEM
	if cfg.JWTPublicKeyPEM != filePEM {
		t.Error("jwtPublicKeyFile should override jwtPublicKeyPEM")
	}
}

func TestLoad_EnvOverride_PublicKeyPEM(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": "original"
	}`)

	t.Setenv("IDAPT_JWT_PUBLIC_KEY_PEM", pubPEM)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.JWTPublicKeyPEM != pubPEM {
		t.Error("IDAPT_JWT_PUBLIC_KEY_PEM env should override config file value")
	}
}

func TestLoad_EnvOverride_PublicKeyFile(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	keyFile := writeTestKeyFile(t, pubPEM)

	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app"
	}`)

	t.Setenv("IDAPT_JWT_PUBLIC_KEY_FILE", keyFile)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.JWTPublicKeyPEM != pubPEM {
		t.Error("IDAPT_JWT_PUBLIC_KEY_FILE env should populate JWTPublicKeyPEM")
	}
}

func TestLoad_BackwardCompat_OldFieldsIgnored(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`,
		"jwtSecret": "old-secret-value",
		"machineToken": "old-token-value"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("old fields should be ignored, got error: %v", err)
	}
	if cfg.MachineID != "mm-123" {
		t.Error("config should load successfully with old fields present")
	}
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-original",
		"appUrl": "https://original.ai",
		"domain": "original.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
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
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"appUrl": "https://idapt.ai",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
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
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"domain": "my-machine.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing appUrl")
	}
}

func TestLoad_WildcardDomain(t *testing.T) {
	pubPEM := generateTestPublicKeyPEM(t)
	path := writeTestConfig(t, `{
		"machineId": "mm-123",
		"appUrl": "https://idapt.ai",
		"domain": "*.idapt.app",
		"jwtPublicKeyPEM": `+jsonEscape(pubPEM)+`
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

// jsonEscape wraps a string in JSON quotes with proper escaping.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
