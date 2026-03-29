package cliconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	path := writeConfig(t, `{"apiUrl":"https://custom.ai","defaultProject":"my-proj","outputFormat":"json","noColor":true}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIURL != "https://custom.ai" {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, "https://custom.ai")
	}
	if cfg.DefaultProject != "my-proj" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "my-proj")
	}
	if cfg.OutputFormat != "json" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "json")
	}
	if !cfg.NoColor {
		t.Fatal("NoColor = false, want true")
	}
}

func TestLoad_Partial(t *testing.T) {
	path := writeConfig(t, `{"defaultProject":"partial-proj"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// apiUrl should remain at default since file didn't set it and unmarshaling leaves it empty
	// but the code ensures default if empty
	if cfg.APIURL != "https://idapt.ai" {
		t.Fatalf("APIURL = %q, want default %q", cfg.APIURL, "https://idapt.ai")
	}
	if cfg.DefaultProject != "partial-proj" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "partial-proj")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("missing file should return defaults, got: %v", err)
	}
	if cfg.APIURL != "https://idapt.ai" {
		t.Fatalf("APIURL = %q, want default %q", cfg.APIURL, "https://idapt.ai")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeConfig(t, "")
	// Overwrite to truly empty (writeConfig writes content, need 0 bytes)
	os.WriteFile(path, []byte{}, 0600)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("empty file should return defaults, got: %v", err)
	}
	if cfg.APIURL != "https://idapt.ai" {
		t.Fatalf("APIURL = %q, want default", cfg.APIURL)
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	path := writeConfig(t, "{bad json!}")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestLoad_ExtraFieldsIgnored(t *testing.T) {
	path := writeConfig(t, `{"apiUrl":"https://example.com","unknownField":"ignored","num":99}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIURL != "https://example.com" {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, "https://example.com")
	}
}

func TestLoad_EnvOverride_APIURL(t *testing.T) {
	path := writeConfig(t, `{"apiUrl":"https://file.ai"}`)
	t.Setenv("IDAPT_API_URL", "https://env.ai")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://env.ai" {
		t.Fatalf("APIURL = %q, want env override %q", cfg.APIURL, "https://env.ai")
	}
}

func TestLoad_EnvOverride_Project(t *testing.T) {
	path := writeConfig(t, `{}`)
	t.Setenv("IDAPT_PROJECT", "env-project")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProject != "env-project" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "env-project")
	}
}

func TestLoad_EnvOverride_Output(t *testing.T) {
	path := writeConfig(t, `{"outputFormat":"table"}`)
	t.Setenv("IDAPT_OUTPUT", "jsonl")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputFormat != "jsonl" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "jsonl")
	}
}

func TestLoad_EnvOverride_NoColor(t *testing.T) {
	path := writeConfig(t, `{}`)
	t.Setenv("NO_COLOR", "1")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.NoColor {
		t.Fatal("NoColor = false, want true when NO_COLOR is set")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.json")

	err := Save(path, Defaults())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Config{
		APIURL:         "https://roundtrip.ai",
		DefaultProject: "test-proj",
		OutputFormat:   "jsonl",
		NoColor:        true,
	}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}

	// Clear env vars so they don't interfere
	t.Setenv("IDAPT_API_URL", "")
	t.Setenv("IDAPT_PROJECT", "")
	t.Setenv("IDAPT_OUTPUT", "")

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIURL != original.APIURL {
		t.Fatalf("APIURL = %q, want %q", loaded.APIURL, original.APIURL)
	}
	if loaded.DefaultProject != original.DefaultProject {
		t.Fatalf("DefaultProject = %q, want %q", loaded.DefaultProject, original.DefaultProject)
	}
	if loaded.OutputFormat != original.OutputFormat {
		t.Fatalf("OutputFormat = %q, want %q", loaded.OutputFormat, original.OutputFormat)
	}
	if loaded.NoColor != original.NoColor {
		t.Fatalf("NoColor = %v, want %v", loaded.NoColor, original.NoColor)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := Save(path, Defaults()); err != nil {
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

func TestGet_ValidKey(t *testing.T) {
	cfg := Config{APIURL: "https://test.ai", DefaultProject: "proj", OutputFormat: "json", NoColor: true}

	tests := []struct {
		key  string
		want string
	}{
		{"apiUrl", "https://test.ai"},
		{"defaultProject", "proj"},
		{"outputFormat", "json"},
		{"noColor", "true"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			val, ok := cfg.Get(tt.key)
			if !ok {
				t.Fatalf("Get(%q) returned not ok", tt.key)
			}
			if val != tt.want {
				t.Fatalf("Get(%q) = %q, want %q", tt.key, val, tt.want)
			}
		})
	}
}

func TestGet_InvalidKey(t *testing.T) {
	cfg := Config{}
	_, ok := cfg.Get("nonexistent")
	if ok {
		t.Fatal("Get for invalid key should return false")
	}
}

func TestSet_ValidKey(t *testing.T) {
	cfg := Config{}

	if err := cfg.Set("apiUrl", "https://new.ai"); err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://new.ai" {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, "https://new.ai")
	}

	if err := cfg.Set("defaultProject", "new-proj"); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProject != "new-proj" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "new-proj")
	}

	if err := cfg.Set("outputFormat", "quiet"); err != nil {
		t.Fatal(err)
	}
	if cfg.OutputFormat != "quiet" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "quiet")
	}
}

func TestSet_InvalidKey(t *testing.T) {
	cfg := Config{}
	err := cfg.Set("badKey", "value")
	if err == nil {
		t.Fatal("Set with invalid key should error")
	}
}

func TestSet_BoolKey(t *testing.T) {
	cfg := Config{}

	if err := cfg.Set("noColor", "true"); err != nil {
		t.Fatal(err)
	}
	if !cfg.NoColor {
		t.Fatal("NoColor = false after Set true")
	}

	if err := cfg.Set("noColor", "false"); err != nil {
		t.Fatal(err)
	}
	if cfg.NoColor {
		t.Fatal("NoColor = true after Set false")
	}

	err := cfg.Set("noColor", "invalid")
	if err == nil {
		t.Fatal("Set noColor with invalid bool should error")
	}
}

func TestKeys(t *testing.T) {
	keys := Keys()
	if len(keys) != 4 {
		t.Fatalf("len(Keys()) = %d, want 4", len(keys))
	}
	expected := map[string]bool{"apiUrl": true, "defaultProject": true, "outputFormat": true, "noColor": true}
	for _, k := range keys {
		if !expected[k] {
			t.Fatalf("unexpected key %q in Keys()", k)
		}
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.APIURL != "https://idapt.ai" {
		t.Fatalf("Defaults().APIURL = %q, want %q", d.APIURL, "https://idapt.ai")
	}
	if d.DefaultProject != "" {
		t.Fatalf("Defaults().DefaultProject = %q, want empty", d.DefaultProject)
	}
	if d.OutputFormat != "" {
		t.Fatalf("Defaults().OutputFormat = %q, want empty", d.OutputFormat)
	}
	if d.NoColor {
		t.Fatal("Defaults().NoColor = true, want false")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if filepath.Base(path) != "config.json" {
		t.Fatalf("DefaultPath base = %q, want config.json", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != ".idapt" {
		t.Fatalf("DefaultPath parent dir = %q, want .idapt", filepath.Base(filepath.Dir(path)))
	}
}
