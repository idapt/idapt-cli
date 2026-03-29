package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func cols(headers ...string) []Column {
	c := make([]Column, len(headers))
	for i, h := range headers {
		c[i] = Column{Header: h, Field: strings.ToLower(h)}
	}
	return c
}

func items(maps ...map[string]interface{}) []map[string]interface{} {
	return maps
}

// --- TableFormatter ---

func TestTableWriteList_Basic(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"name": "alice", "age": 30},
		map[string]interface{}{"name": "bob", "age": 25},
	), []Column{
		{Header: "NAME", Field: "name"},
		{Header: "AGE", Field: "age"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header, separator, rows), got %d", len(lines))
	}
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "AGE") {
		t.Fatalf("header line missing columns: %q", lines[0])
	}
	if !strings.Contains(lines[2], "alice") {
		t.Fatalf("first data row missing 'alice': %q", lines[2])
	}
	if !strings.Contains(lines[3], "bob") {
		t.Fatalf("second data row missing 'bob': %q", lines[3])
	}
}

func TestTableWriteList_Alignment(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"id": "short", "value": "x"},
		map[string]interface{}{"id": "a-longer-id", "value": "y"},
	), []Column{
		{Header: "ID", Field: "id"},
		{Header: "VALUE", Field: "value"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Both data rows should have VALUE column at the same position
	if len(lines) < 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	pos2 := strings.Index(lines[2], "x")
	pos3 := strings.Index(lines[3], "y")
	if pos2 != pos3 {
		t.Fatalf("columns not aligned: 'x' at %d, 'y' at %d", pos2, pos3)
	}
}

func TestTableWriteList_Truncation(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"name": "this-is-a-very-long-name-that-should-be-truncated"},
	), []Column{
		{Header: "NAME", Field: "name", Width: 10},
	})
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Data row (index 2)
	dataLine := lines[2]
	if !strings.Contains(dataLine, "...") {
		t.Fatalf("truncated value should contain '...': %q", dataLine)
	}
}

func TestTableWriteList_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(nil, []Column{{Header: "NAME", Field: "name"}})
	if err != nil {
		t.Fatal(err)
	}

	// Should still print header and separator
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("empty list should print header + separator, got %d lines", len(lines))
	}
}

func TestTableWriteList_NilFieldValue(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"name": nil},
	), []Column{{Header: "NAME", Field: "name"}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "-") {
		t.Fatalf("nil field should render as '-', got: %q", buf.String())
	}
}

func TestTableWriteList_Unicode(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"name": "cafe\u0301"},
	), []Column{{Header: "NAME", Field: "name"}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "cafe\u0301") {
		t.Fatalf("unicode not preserved: %q", buf.String())
	}
}

func TestTableWriteItem(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	err := f.WriteItem(map[string]interface{}{
		"name":   "test-agent",
		"status": "active",
	}, []Column{
		{Header: "Name", Field: "name"},
		{Header: "Status", Field: "status"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Name:") {
		t.Fatalf("WriteItem missing 'Name:' label: %q", out)
	}
	if !strings.Contains(out, "test-agent") {
		t.Fatalf("WriteItem missing value 'test-agent': %q", out)
	}
	if !strings.Contains(out, "Status:") {
		t.Fatalf("WriteItem missing 'Status:' label: %q", out)
	}
}

// --- JSONFormatter ---

func TestJSONWriteList_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"id": "1", "name": "a"},
		map[string]interface{}{"id": "2", "name": "b"},
	), nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 items, got %d", len(parsed))
	}
}

func TestJSONWriteList_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{w: &buf}

	err := f.WriteList([]map[string]interface{}{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	trimmed := strings.TrimSpace(buf.String())
	if trimmed != "[]" {
		t.Fatalf("empty list should output '[]', got %q", trimmed)
	}
}

func TestJSONWriteList_SpecialCharsEscaped(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"msg": "hello \"world\" <>&"},
	), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it parses back correctly
	var parsed []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON with special chars: %v", err)
	}
	if parsed[0]["msg"] != "hello \"world\" <>&" {
		t.Fatalf("special chars not preserved: %q", parsed[0]["msg"])
	}
}

func TestJSONWriteItem(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{w: &buf}

	err := f.WriteItem(map[string]interface{}{"id": "123", "name": "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["id"] != "123" {
		t.Fatalf("id = %q, want %q", parsed["id"], "123")
	}
}

// --- JSONLFormatter ---

func TestJSONLWriteList(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONLFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"id": "1"},
		map[string]interface{}{"id": "2"},
		map[string]interface{}{"id": "3"},
	), nil)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
	}
}

func TestJSONLWriteList_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONLFormatter{w: &buf}

	err := f.WriteList([]map[string]interface{}{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Fatalf("empty JSONL should produce no output, got %q", buf.String())
	}
}

// --- QuietFormatter ---

func TestQuietWriteList_IDsOnly(t *testing.T) {
	var buf bytes.Buffer
	f := &QuietFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"id": "abc-123", "name": "agent1"},
		map[string]interface{}{"id": "def-456", "name": "agent2"},
	), nil)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "abc-123" {
		t.Fatalf("line 0 = %q, want %q", lines[0], "abc-123")
	}
	if lines[1] != "def-456" {
		t.Fatalf("line 1 = %q, want %q", lines[1], "def-456")
	}
}

func TestQuietWriteList_MissingIDField(t *testing.T) {
	var buf bytes.Buffer
	f := &QuietFormatter{w: &buf}

	err := f.WriteList(items(
		map[string]interface{}{"name": "no-id-here"},
	), nil)
	if err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Fatalf("items without 'id' should produce no output, got %q", buf.String())
	}
}

// --- New() factory ---

func TestNew_Table(t *testing.T) {
	var buf bytes.Buffer
	f := New(FormatTable, &buf, false)
	if _, ok := f.(*TableFormatter); !ok {
		t.Fatalf("New(table) returned %T, want *TableFormatter", f)
	}
}

func TestNew_JSON(t *testing.T) {
	var buf bytes.Buffer
	f := New(FormatJSON, &buf, false)
	if _, ok := f.(*JSONFormatter); !ok {
		t.Fatalf("New(json) returned %T, want *JSONFormatter", f)
	}
}

func TestNew_JSONL(t *testing.T) {
	var buf bytes.Buffer
	f := New(FormatJSONL, &buf, false)
	if _, ok := f.(*JSONLFormatter); !ok {
		t.Fatalf("New(jsonl) returned %T, want *JSONLFormatter", f)
	}
}

func TestNew_Quiet(t *testing.T) {
	var buf bytes.Buffer
	f := New(FormatQuiet, &buf, false)
	if _, ok := f.(*QuietFormatter); !ok {
		t.Fatalf("New(quiet) returned %T, want *QuietFormatter", f)
	}
}

func TestNew_UnknownFallsBackToTable(t *testing.T) {
	var buf bytes.Buffer
	f := New("unknown", &buf, false)
	if _, ok := f.(*TableFormatter); !ok {
		t.Fatalf("New(unknown) returned %T, want *TableFormatter (fallback)", f)
	}
}

// --- WriteError ---

func TestWriteError_TableMode(t *testing.T) {
	var buf bytes.Buffer
	WriteError(FormatTable, &buf, errors.New("something failed"))

	out := buf.String()
	if !strings.Contains(out, "Error:") {
		t.Fatalf("table WriteError should contain 'Error:': %q", out)
	}
	if !strings.Contains(out, "something failed") {
		t.Fatalf("table WriteError should contain message: %q", out)
	}
}

func TestWriteError_JSONMode(t *testing.T) {
	var buf bytes.Buffer
	WriteError(FormatJSON, &buf, errors.New("json failure"))

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON WriteError output not valid JSON: %v\noutput: %s", err, buf.String())
	}
	errObj, ok := parsed["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got %T", parsed["error"])
	}
	if errObj["message"] != "json failure" {
		t.Fatalf("error message = %q, want %q", errObj["message"], "json failure")
	}
}

func TestWriteError_JSONLMode(t *testing.T) {
	var buf bytes.Buffer
	WriteError(FormatJSONL, &buf, errors.New("jsonl failure"))

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSONL WriteError output not valid JSON: %v", err)
	}
}

// --- WriteRaw ---

func TestWriteRaw(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{w: &buf}

	data := []byte("raw output data\n")
	if err := f.WriteRaw(data); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "raw output data\n" {
		t.Fatalf("WriteRaw output = %q, want %q", buf.String(), "raw output data\n")
	}
}

func TestWriteRaw_JSON(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{w: &buf}

	data := []byte(`{"raw":true}`)
	if err := f.WriteRaw(data); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `{"raw":true}` {
		t.Fatalf("WriteRaw output = %q", buf.String())
	}
}

// --- Flush ---

func TestFlush_AllFormatters(t *testing.T) {
	formatters := []Formatter{
		&TableFormatter{w: &bytes.Buffer{}},
		&JSONFormatter{w: &bytes.Buffer{}},
		&JSONLFormatter{w: &bytes.Buffer{}},
		&QuietFormatter{w: &bytes.Buffer{}},
	}
	for _, f := range formatters {
		if err := f.Flush(); err != nil {
			t.Fatalf("Flush() on %T returned error: %v", f, err)
		}
	}
}
