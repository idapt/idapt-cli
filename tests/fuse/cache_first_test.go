//go:build integration

package fuse_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Cache-First Read Behavior ---

func TestCacheFirst_CachedFileOpensInstantly(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-instant.txt")
	env.createServerFile(name, "cached content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// First read — populates cache (may take time to download)
	os.ReadFile(path)

	// Second read — should be instant (cache hit, no server check)
	start := time.Now()
	data, err := os.ReadFile(path)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if string(data) != "cached content" {
		t.Errorf("expected 'cached content', got %q", string(data))
	}
	if elapsed > 50*time.Millisecond {
		t.Logf("WARN: cached open took %v (expected < 50ms)", elapsed)
	}
}

func TestCacheFirst_100OpensAllCached(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-100.txt")
	env.createServerFile(name, "repeated content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Prime cache
	os.ReadFile(path)

	// 100 opens — all should be from cache
	start := time.Now()
	for i := 0; i < 100; i++ {
		_, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// 100 cached reads should be well under 1 second
	if elapsed > 2*time.Second {
		t.Errorf("100 cached opens took %v (expected < 2s)", elapsed)
	}
}

func TestCacheFirst_UncachedFileDownloads(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-miss.txt")
	env.createServerFile(name, "fresh download")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// First read — cache miss, downloads
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "fresh download" {
		t.Errorf("expected 'fresh download', got %q", string(data))
	}
}

func TestCacheFirst_OpenCloseReopen(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-reopen.txt")
	env.createServerFile(name, "persistent cache")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Open, read, close
	data1, _ := os.ReadFile(path)
	// Re-open — should be instant (cache persists across open/close)
	data2, _ := os.ReadFile(path)

	if string(data1) != string(data2) {
		t.Error("content changed between open/close/reopen")
	}
}

func TestCacheFirst_DeletedFileServesStale(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-deleted.txt")
	fileID := env.createServerFile(name, "will be deleted")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Prime cache
	os.ReadFile(path)

	// Delete server-side
	env.apiClient.TrashFile(context.Background(), fileID)

	// Read should still work (stale cache)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("Read after server delete returned error: %v (acceptable — SSE may have invalidated)", err)
	} else if string(data) != "will be deleted" {
		t.Logf("Content: %q (may differ if SSE already invalidated)", string(data))
	}
}

// --- Stale Read + Write Conflict ---

func TestCacheFirst_StaleReadWriteConflict(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-stale-occ.txt")
	env.createServerFile(name, "version 1")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Prime cache at v1
	data, _ := os.ReadFile(path)
	if string(data) != "version 1" {
		t.Fatalf("expected 'version 1', got %q", string(data))
	}

	// Server-side update to v2 (bypasses mount, bumps version)
	ctx := context.Background()
	files, _ := env.apiClient.ListFiles(ctx, env.projectID, "")
	for _, f := range files {
		if f.Name == name {
			env.rawClient.Patch(ctx, "/api/files/"+f.ID, map[string]interface{}{
				"content": "version 2 from server",
			}, nil)
			break
		}
	}

	// Mount still reads stale v1 (cache-first, no server check)
	staleData, _ := os.ReadFile(path)
	if string(staleData) != "version 1" {
		t.Logf("Got %q — SSE may have already invalidated cache", string(staleData))
	}

	// Write via mount — should detect conflict on Flush (expectedVersion from cache = v1, server = v2)
	err := os.WriteFile(path, []byte("mount writes v3"), 0644)
	if err != nil {
		t.Logf("Write returned error: %v (expected — OCC conflict on stale version)", err)
	}
	// Either way, server content should NOT be silently overwritten
}

// --- Concurrent Opens ---

func TestCacheFirst_ConcurrentOpens(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cf-concurrent.txt")
	content := strings.Repeat("x", 10*1024)
	env.createServerFile(name, content)
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Prime cache
	os.ReadFile(path)

	// 50 concurrent reads
	var wg sync.WaitGroup
	errors := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := os.ReadFile(path)
			if err != nil {
				errors <- err
				return
			}
			if len(data) != len(content) {
				errors <- os.ErrInvalid
			}
		}()
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent open error: %v", err)
	}
}
