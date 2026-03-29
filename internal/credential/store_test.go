package credential

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte(`{"apiKey":"sk-test-123"}`), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.APIKey != "sk-test-123" {
		t.Fatalf("APIKey = %q, want %q", creds.APIKey, "sk-test-123")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	creds, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if creds.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty for missing file", creds.APIKey)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := Load(path)
	if err != nil {
		t.Fatalf("empty file should not error, got: %v", err)
	}
	if creds.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty for empty file", creds.APIKey)
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestLoad_ExtraFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte(`{"apiKey":"sk-123","extra":"ignored","count":42}`), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.APIKey != "sk-123" {
		t.Fatalf("APIKey = %q, want %q", creds.APIKey, "sk-123")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "credentials.json")

	err := Save(path, Credentials{APIKey: "sk-new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fi, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if fi.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions = %o, want 0700", fi.Mode().Perm())
	}
}

func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	if err := Save(path, Credentials{APIKey: "sk-secret"}); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("file permissions = %o, want 0600", fi.Mode().Perm())
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	if err := Save(path, Credentials{APIKey: "old-key"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Credentials{APIKey: "new-key"}); err != nil {
		t.Fatal(err)
	}

	creds, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if creds.APIKey != "new-key" {
		t.Fatalf("APIKey = %q, want %q after overwrite", creds.APIKey, "new-key")
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	original := Credentials{APIKey: "sk-roundtrip-test-456"}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != original.APIKey {
		t.Fatalf("round-trip failed: got %q, want %q", loaded.APIKey, original.APIKey)
	}

	// Verify saved file is valid indented JSON with trailing newline
	data, _ := os.ReadFile(path)
	if data[len(data)-1] != '\n' {
		t.Fatal("saved file should end with newline")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
}

func TestClear_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	if err := Save(path, Credentials{APIKey: "to-delete"}); err != nil {
		t.Fatal(err)
	}

	if err := Clear(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be removed after Clear")
	}
}

func TestClear_MissingFileIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	if err := Clear(path); err != nil {
		t.Fatalf("Clear on missing file should not error, got: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Fatal("DefaultPath should not be empty")
	}
	if filepath.Base(path) != "credentials.json" {
		t.Fatalf("DefaultPath base = %q, want %q", filepath.Base(path), "credentials.json")
	}
	if filepath.Base(filepath.Dir(path)) != ".idapt" {
		t.Fatalf("DefaultPath dir = %q, want .idapt", filepath.Base(filepath.Dir(path)))
	}
}
