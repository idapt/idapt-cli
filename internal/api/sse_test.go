package api

import (
	"io"
	"strings"
	"testing"
)

func sseReader(data string) *SSEReader {
	return NewSSEReaderFromReader(io.NopCloser(strings.NewReader(data)))
}

func TestSSEReader_SingleEvent(t *testing.T) {
	r := sseReader("data: hello world\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Data != "hello world" {
		t.Fatalf("Data = %q, want %q", ev.Data, "hello world")
	}

	_, err = r.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestSSEReader_EventWithType(t *testing.T) {
	r := sseReader("event: status\ndata: connected\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Event != "status" {
		t.Fatalf("Event = %q, want %q", ev.Event, "status")
	}
	if ev.Data != "connected" {
		t.Fatalf("Data = %q, want %q", ev.Data, "connected")
	}
}

func TestSSEReader_EventWithID(t *testing.T) {
	r := sseReader("id: 42\ndata: payload\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.ID != "42" {
		t.Fatalf("ID = %q, want %q", ev.ID, "42")
	}
	if ev.Data != "payload" {
		t.Fatalf("Data = %q, want %q", ev.Data, "payload")
	}
}

func TestSSEReader_MultiLineData(t *testing.T) {
	r := sseReader("data: line1\ndata: line2\ndata: line3\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	want := "line1\nline2\nline3"
	if ev.Data != want {
		t.Fatalf("Data = %q, want %q", ev.Data, want)
	}
}

func TestSSEReader_MultipleEvents(t *testing.T) {
	r := sseReader("data: first\n\ndata: second\n\ndata: third\n\n")
	expected := []string{"first", "second", "third"}
	for i, want := range expected {
		ev, err := r.Next()
		if err != nil {
			t.Fatalf("event %d: Next error: %v", i, err)
		}
		if ev.Data != want {
			t.Fatalf("event %d: Data = %q, want %q", i, ev.Data, want)
		}
	}
	_, err := r.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after all events, got %v", err)
	}
}

func TestSSEReader_CommentLines(t *testing.T) {
	r := sseReader(": this is a comment\ndata: real data\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Data != "real data" {
		t.Fatalf("Data = %q, want %q", ev.Data, "real data")
	}
}

func TestSSEReader_EmptyData(t *testing.T) {
	r := sseReader("data:\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Data != "" {
		t.Fatalf("Data = %q, want empty", ev.Data)
	}
}

func TestSSEReader_NoSpaceAfterColon(t *testing.T) {
	r := sseReader("data:no-space\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Data != "no-space" {
		t.Fatalf("Data = %q, want %q", ev.Data, "no-space")
	}
}

func TestSSEReader_EmptyStream(t *testing.T) {
	r := sseReader("")
	_, err := r.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF for empty stream, got %v", err)
	}
}

func TestSSEReader_OnlyComments(t *testing.T) {
	r := sseReader(": comment one\n: comment two\n")
	_, err := r.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF for comment-only stream, got %v", err)
	}
}

func TestSSEReader_ConsecutiveNewlines(t *testing.T) {
	// Multiple blank lines between events should be skipped
	r := sseReader("data: first\n\n\n\ndata: second\n\n")
	ev1, err := r.Next()
	if err != nil {
		t.Fatalf("event 1: Next error: %v", err)
	}
	if ev1.Data != "first" {
		t.Fatalf("event 1: Data = %q, want %q", ev1.Data, "first")
	}
	ev2, err := r.Next()
	if err != nil {
		t.Fatalf("event 2: Next error: %v", err)
	}
	if ev2.Data != "second" {
		t.Fatalf("event 2: Data = %q, want %q", ev2.Data, "second")
	}
}

func TestSSEReader_JSONPayload(t *testing.T) {
	r := sseReader(`data: {"type":"message","content":"hello"}` + "\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	want := `{"type":"message","content":"hello"}`
	if ev.Data != want {
		t.Fatalf("Data = %q, want %q", ev.Data, want)
	}
}

func TestSSEReader_LargePayload(t *testing.T) {
	// bufio.Scanner default max token size is 64KB. Use a payload that
	// fits within that limit (including the "data: " prefix and newline).
	large := strings.Repeat("x", 60*1024) // 60KB, well within 64KB limit
	r := sseReader("data: " + large + "\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if len(ev.Data) != 60*1024 {
		t.Fatalf("Data length = %d, want %d", len(ev.Data), 60*1024)
	}
}

func TestSSEReader_UTF8Data(t *testing.T) {
	r := sseReader("data: こんにちは世界 🌍\n\n")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	want := "こんにちは世界 🌍"
	if ev.Data != want {
		t.Fatalf("Data = %q, want %q", ev.Data, want)
	}
}

func TestSSEReader_Close(t *testing.T) {
	body := io.NopCloser(strings.NewReader("data: test\n\n"))
	r := NewSSEReaderFromReader(body)
	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestSSEReader_StreamEndWithPartialEvent(t *testing.T) {
	// Stream ends without trailing blank line — partial event should be returned
	r := sseReader("event: done\ndata: final")
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ev.Event != "done" {
		t.Fatalf("Event = %q, want %q", ev.Event, "done")
	}
	if ev.Data != "final" {
		t.Fatalf("Data = %q, want %q", ev.Data, "final")
	}

	_, err = r.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after partial event, got %v", err)
	}
}
