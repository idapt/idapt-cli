//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SecondReadFromCache(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cache-hit.txt")
	env.createServerFile(name, "cached content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// First read — fetches from server
	data1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	// Second read — should come from cache (much faster)
	start := time.Now()
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	elapsed := time.Since(start)

	if string(data1) != string(data2) {
		t.Error("first and second reads returned different content")
	}

	// Cache hit should be sub-millisecond (kernel page cache)
	if elapsed > 100*time.Millisecond {
		t.Logf("WARN: second read took %v (expected < 100ms for cache hit)", elapsed)
	}
}

func TestCache_WriteInvalidatesCache(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("cache-inval.txt")
	env.createServerFile(name, "original")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Read → cache
	os.ReadFile(path)

	// Write new content
	os.WriteFile(path, []byte("updated"), 0644)

	// Read should return new content
	data, _ := os.ReadFile(path)
	if string(data) != "updated" {
		t.Errorf("expected 'updated' after write, got %q", string(data))
	}
}
