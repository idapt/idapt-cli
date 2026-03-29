package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- ParseJSONFlag ---

func TestParseJSONFlag_InlineString(t *testing.T) {
	result, err := ParseJSONFlag(`{"name":"alice","age":30}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "alice" {
		t.Fatalf("name = %q, want %q", result["name"], "alice")
	}
	if result["age"] != float64(30) {
		t.Fatalf("age = %v, want 30", result["age"])
	}
}

func TestParseJSONFlag_EmptyObject(t *testing.T) {
	result, err := ParseJSONFlag(`{}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map, got %d keys", len(result))
	}
}

func TestParseJSONFlag_InvalidJSON(t *testing.T) {
	_, err := ParseJSONFlag(`{not valid}`, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseJSONFlag_PartialJSON(t *testing.T) {
	_, err := ParseJSONFlag(`{"name":`, nil)
	if err == nil {
		t.Fatal("expected error for partial JSON")
	}
}

func TestParseJSONFlag_StdinDash(t *testing.T) {
	stdin := strings.NewReader(`{"from":"stdin"}`)
	result, err := ParseJSONFlag("-", stdin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["from"] != "stdin" {
		t.Fatalf("from = %q, want %q", result["from"], "stdin")
	}
}

func TestParseJSONFlag_StdinEmpty(t *testing.T) {
	stdin := strings.NewReader("")
	_, err := ParseJSONFlag("-", stdin)
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
}

func TestParseJSONFlag_StdinNil(t *testing.T) {
	_, err := ParseJSONFlag("-", nil)
	if err == nil {
		t.Fatal("expected error for nil stdin")
	}
}

func TestParseJSONFlag_StdinInvalidJSON(t *testing.T) {
	stdin := strings.NewReader("not json")
	_, err := ParseJSONFlag("-", stdin)
	if err == nil {
		t.Fatal("expected error for invalid JSON on stdin")
	}
}

func TestParseJSONFlag_StdinLargePayload(t *testing.T) {
	// 1MB JSON payload
	large := `{"data":"` + strings.Repeat("x", 1024*1024) + `"}`
	stdin := strings.NewReader(large)
	result, err := ParseJSONFlag("-", stdin)
	if err != nil {
		t.Fatalf("unexpected error for large payload: %v", err)
	}
	if result["data"] == nil {
		t.Fatal("expected data field in large payload")
	}
}

func TestParseJSONFlag_NestedObjects(t *testing.T) {
	result, err := ParseJSONFlag(`{"outer":{"inner":"val"}}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	outer, ok := result["outer"].(map[string]interface{})
	if !ok {
		t.Fatalf("outer is %T, want map", result["outer"])
	}
	if outer["inner"] != "val" {
		t.Fatalf("inner = %q, want %q", outer["inner"], "val")
	}
}

func TestParseJSONFlag_Arrays(t *testing.T) {
	result, err := ParseJSONFlag(`{"tags":["a","b","c"]}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	tags, ok := result["tags"].([]interface{})
	if !ok {
		t.Fatalf("tags is %T, want []interface{}", result["tags"])
	}
	if len(tags) != 3 {
		t.Fatalf("len(tags) = %d, want 3", len(tags))
	}
}

func TestParseJSONFlag_Numerics(t *testing.T) {
	result, err := ParseJSONFlag(`{"int":42,"float":3.14}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["int"] != float64(42) {
		t.Fatalf("int = %v (type %T), want float64(42)", result["int"], result["int"])
	}
	if result["float"] != float64(3.14) {
		t.Fatalf("float = %v, want 3.14", result["float"])
	}
}

func TestParseJSONFlag_Booleans(t *testing.T) {
	result, err := ParseJSONFlag(`{"active":true,"deleted":false}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["active"] != true {
		t.Fatalf("active = %v, want true", result["active"])
	}
	if result["deleted"] != false {
		t.Fatalf("deleted = %v, want false", result["deleted"])
	}
}

func TestParseJSONFlag_NullValues(t *testing.T) {
	result, err := ParseJSONFlag(`{"value":null}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["value"] != nil {
		t.Fatalf("value = %v, want nil", result["value"])
	}
}

// --- ReadFileFlag ---

func TestReadFileFlag_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("file content here"), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := ReadFileFlag(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "file content here" {
		t.Fatalf("content = %q, want %q", content, "file content here")
	}
}

func TestReadFileFlag_MissingFile(t *testing.T) {
	_, err := ReadFileFlag(filepath.Join(t.TempDir(), "nonexistent.txt"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadFileFlag_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := ReadFileFlag(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Fatalf("content = %q, want empty", content)
	}
}

func TestReadFileFlag_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	data := strings.Repeat("A", 1024*1024) // 1MB
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := ReadFileFlag(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) != 1024*1024 {
		t.Fatalf("content length = %d, want %d", len(content), 1024*1024)
	}
}

// --- MergeFlags ---

func TestMergeFlags_OverrideJSONField(t *testing.T) {
	base := map[string]interface{}{"name": "old", "status": "active"}
	overrides := map[string]interface{}{"name": "new"}

	result := MergeFlags(base, overrides)
	if result["name"] != "new" {
		t.Fatalf("name = %q, want %q", result["name"], "new")
	}
	if result["status"] != "active" {
		t.Fatalf("status = %q, want %q (unchanged)", result["status"], "active")
	}
}

func TestMergeFlags_NilBase(t *testing.T) {
	overrides := map[string]interface{}{"key": "val"}
	result := MergeFlags(nil, overrides)
	if result["key"] != "val" {
		t.Fatalf("key = %q, want %q", result["key"], "val")
	}
}

func TestMergeFlags_NilOverrides(t *testing.T) {
	base := map[string]interface{}{"key": "val"}
	result := MergeFlags(base, nil)
	if result["key"] != "val" {
		t.Fatalf("key = %q, want %q", result["key"], "val")
	}
}

func TestMergeFlags_EmptyStringSkipped(t *testing.T) {
	base := map[string]interface{}{"name": "original"}
	overrides := map[string]interface{}{"name": ""}

	result := MergeFlags(base, overrides)
	if result["name"] != "original" {
		t.Fatalf("name = %q, want %q (empty string should not override)", result["name"], "original")
	}
}

func TestMergeFlags_NilValueSkipped(t *testing.T) {
	base := map[string]interface{}{"name": "original"}
	overrides := map[string]interface{}{"name": nil}

	result := MergeFlags(base, overrides)
	if result["name"] != "original" {
		t.Fatalf("name = %q, want %q (nil should not override)", result["name"], "original")
	}
}

func TestMergeFlags_NewFieldAdded(t *testing.T) {
	base := map[string]interface{}{"existing": "val"}
	overrides := map[string]interface{}{"newField": "newVal"}

	result := MergeFlags(base, overrides)
	if result["newField"] != "newVal" {
		t.Fatalf("newField = %q, want %q", result["newField"], "newVal")
	}
	if result["existing"] != "val" {
		t.Fatalf("existing = %q, want %q", result["existing"], "val")
	}
}

func TestMergeFlags_BothEmpty(t *testing.T) {
	result := MergeFlags(map[string]interface{}{}, map[string]interface{}{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d keys", len(result))
	}
}
