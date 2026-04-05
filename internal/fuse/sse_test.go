package fuse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cache"
)

func TestSSE_ParseFilesUpdated(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := &SSESubscriber{
		metadataCache: mc,
		diskCache:     dc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Populate cache
	mc.Put("children:parent-1", "cached-children")
	mc.Put("lookup:parent-1:test.txt", "cached-lookup")

	// Process update event
	s.processEvent(`{"type":"files:updated","fileId":"file-1","parentId":"parent-1","version":5}`)

	// Children cache should be invalidated
	if _, ok := mc.Get("children:parent-1"); ok {
		t.Error("expected children cache invalidated after files:updated")
	}
	// Lookup cache should be invalidated
	if _, ok := mc.Get("lookup:parent-1:test.txt"); ok {
		t.Error("expected lookup cache invalidated after files:updated")
	}
}

func TestSSE_ParseFilesDeleted(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := &SSESubscriber{
		metadataCache: mc,
		diskCache:     dc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Populate disk cache
	dc.Put("file-1", 3, strings.NewReader("content"))

	// Process delete event
	s.processEvent(`{"type":"files:deleted","fileId":"file-1","parentId":"parent-1"}`)

	// Disk cache should be evicted
	r, _ := dc.Get("file-1")
	if r != nil {
		r.Close()
		t.Error("expected disk cache evicted after files:deleted")
	}
}

func TestSSE_ParseFilesCreated(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()

	s := &SSESubscriber{
		metadataCache: mc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	mc.Put("children:parent-1", "old-children-list")

	s.processEvent(`{"type":"files:created","fileId":"new-file","parentId":"parent-1"}`)

	// Parent's children cache should be invalidated
	if _, ok := mc.Get("children:parent-1"); ok {
		t.Error("expected children cache invalidated after files:created")
	}
}

func TestSSE_VersionCompare_NoEvictIfCurrent(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := &SSESubscriber{
		metadataCache: mc,
		diskCache:     dc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Cache file at version 5
	dc.Put("file-1", 5, strings.NewReader("v5 content"))

	// SSE says version 5 (same as cached) — should NOT evict
	s.processEvent(`{"type":"files:updated","fileId":"file-1","parentId":"parent-1","version":5}`)

	r, _ := dc.Get("file-1")
	if r == nil {
		t.Error("should NOT evict disk cache when SSE version == cached version")
	} else {
		r.Close()
	}
}

func TestSSE_VersionCompare_EvictIfNewer(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := &SSESubscriber{
		metadataCache: mc,
		diskCache:     dc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Cache file at version 3
	dc.Put("file-1", 3, strings.NewReader("v3 content"))

	// SSE says version 5 (newer) — should evict
	s.processEvent(`{"type":"files:updated","fileId":"file-1","parentId":"parent-1","version":5}`)

	r, _ := dc.Get("file-1")
	if r != nil {
		r.Close()
		t.Error("should evict disk cache when SSE version > cached version")
	}
}

func TestSSE_UnknownEventType(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()

	s := &SSESubscriber{
		metadataCache: mc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Should not panic
	s.processEvent(`{"type":"unknown:event","fileId":"x"}`)
	s.processEvent(`{"type":"files:batch","operations":[]}`)
	s.processEvent(`{"type":"live:update","fileId":"x"}`)
}

func TestSSE_MalformedJSON(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()

	s := &SSESubscriber{
		metadataCache: mc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// Should not panic on any of these
	s.processEvent("")
	s.processEvent("not json")
	s.processEvent("{invalid")
	s.processEvent(`{"type":123}`)
}

// TestSSE_ConnectSendsProjectIDAsQueryParam verifies that the SSE connect sends
// projectId as a proper query parameter (not embedded in the path which causes %3F encoding).
func TestSSE_ConnectSendsProjectIDAsQueryParam(t *testing.T) {
	var capturedPath string
	var capturedQuery string
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedAPIKey = r.Header.Get("x-api-key")
		// Return SSE content-type with immediate close
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, ": heartbeat\n\n")
	}))
	defer server.Close()

	client, _ := api.NewClient(api.ClientConfig{
		BaseURL: server.URL,
		APIKey:  "uk_test_key",
	})
	fuseClient := NewFuseAPIClient(client)

	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := NewSSESubscriber(fuseClient, mc, dc, "proj-123")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// connect will return when the server closes the response
	_ = s.connect(ctx)

	// Path must NOT contain %3F (encoded ?) — projectId must be a query param
	if strings.Contains(capturedPath, "projectId") {
		t.Errorf("projectId should NOT be in path, got path: %s", capturedPath)
	}
	if capturedPath != "/api/subscriptions/files" {
		t.Errorf("path = %q, want /api/subscriptions/files", capturedPath)
	}
	if !strings.Contains(capturedQuery, "projectId=proj-123") {
		t.Errorf("query = %q, want projectId=proj-123", capturedQuery)
	}
	if capturedAPIKey != "uk_test_key" {
		t.Errorf("x-api-key = %q, want uk_test_key", capturedAPIKey)
	}
}

// TestSSE_ConnectRejectsHTMLResponse verifies that the SSE connect returns an error
// when the server returns HTML instead of text/event-stream (e.g., catch-all page).
func TestSSE_ConnectRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprint(w, "<!DOCTYPE html><html><body>Not found</body></html>")
	}))
	defer server.Close()

	client, _ := api.NewClient(api.ClientConfig{
		BaseURL: server.URL,
		APIKey:  "uk_test",
	})
	fuseClient := NewFuseAPIClient(client)
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()

	s := &SSESubscriber{
		apiClient:     fuseClient,
		metadataCache: mc,
		projectID:     "proj-1",
		stopCh:        make(chan struct{}),
	}

	ctx := context.Background()
	err := s.connect(ctx)

	if err == nil {
		t.Fatal("expected error when server returns HTML, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected content-type") {
		t.Errorf("error = %q, want 'unexpected content-type'", err.Error())
	}
}

// TestSSE_ConnectProcessesEvents verifies end-to-end SSE event processing.
func TestSSE_ConnectProcessesEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		// Send one SSE event then close
		fmt.Fprint(w, "data: {\"type\":\"files:created\",\"fileId\":\"new-file\",\"parentId\":\"parent-1\"}\n\n")
	}))
	defer server.Close()

	client, _ := api.NewClient(api.ClientConfig{BaseURL: server.URL, APIKey: "uk_test"})
	fuseClient := NewFuseAPIClient(client)
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := NewSSESubscriber(fuseClient, mc, dc, "proj-1")

	// Pre-populate cache
	mc.Put("children:parent-1", "old-list")

	ctx := context.Background()
	err := s.connect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Event should have invalidated the children cache
	if _, ok := mc.Get("children:parent-1"); ok {
		t.Error("children cache should be invalidated after files:created event")
	}
}

func TestSSE_EventForUncachedFile(t *testing.T) {
	mc := cache.NewMetadataCache(60 * time.Second)
	defer mc.Stop()
	dir := t.TempDir()
	dc, _ := cache.NewDiskCache(dir, 100*1024*1024)

	s := &SSESubscriber{
		metadataCache: mc,
		diskCache:     dc,
		projectID:     "test",
		stopCh:        make(chan struct{}),
	}

	// SSE for file not in cache — should not crash or error
	s.processEvent(`{"type":"files:updated","fileId":"not-cached","parentId":"p","version":10}`)
	// No panic = success
}
