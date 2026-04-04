package fuse

import (
	"strings"
	"testing"
	"time"

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
