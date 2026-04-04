package cache

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskCache_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	content := "hello world"
	if _, err := dc.Put("file1", 1, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	reader, err := dc.Get("file1")
	if err != nil || reader == nil {
		t.Fatal("expected cache hit")
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != content {
		t.Fatalf("expected %q, got %q", content, string(data))
	}
}

func TestDiskCache_GetVersion(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 5, strings.NewReader("content"))

	v := dc.GetVersion("file1")
	if v != 5 {
		t.Fatalf("expected version 5, got %d", v)
	}
}

func TestDiskCache_GetMiss(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := dc.Get("nonexistent")
	if err != nil || reader != nil {
		t.Fatal("expected nil reader for cache miss")
	}
}

func TestDiskCache_GetVersionMiss(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	v := dc.GetVersion("nonexistent")
	if v != -1 {
		t.Fatalf("expected -1 for missing file, got %d", v)
	}
}

func TestDiskCache_LRUEviction(t *testing.T) {
	dir := t.TempDir()
	// Max 100 bytes
	dc, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Put 3 files of 50 bytes each (total 150 > 100)
	data := bytes.Repeat([]byte("x"), 50)

	dc.Put("file1", 1, bytes.NewReader(data))
	dc.Put("file2", 1, bytes.NewReader(data))
	dc.Put("file3", 1, bytes.NewReader(data))

	// file1 should have been evicted (oldest)
	r, _ := dc.Get("file1")
	if r != nil {
		r.Close()
		t.Fatal("expected file1 evicted")
	}

	// file3 should still be cached (most recent)
	r, _ = dc.Get("file3")
	if r == nil {
		t.Fatal("expected file3 cached")
	}
	r.Close()
}

func TestDiskCache_DirtyNotEvicted(t *testing.T) {
	dir := t.TempDir()
	// Max 60 bytes — only room for 1 file
	dc, err := NewDiskCache(dir, 60)
	if err != nil {
		t.Fatal(err)
	}

	data := bytes.Repeat([]byte("x"), 50)

	dc.Put("dirty-file", 1, bytes.NewReader(data))
	dc.MarkDirty("dirty-file")

	// Try to add another file that would cause eviction
	dc.Put("new-file", 1, bytes.NewReader(data))

	// Dirty file should NOT be evicted
	r, _ := dc.Get("dirty-file")
	if r == nil {
		t.Fatal("dirty file should not be evicted")
	}
	r.Close()
}

func TestDiskCache_DirtyFiles(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 1, strings.NewReader("a"))
	dc.Put("file2", 2, strings.NewReader("b"))
	dc.MarkDirty("file1")

	dirty := dc.DirtyFiles()
	if len(dirty) != 1 {
		t.Fatalf("expected 1 dirty file, got %d", len(dirty))
	}
	if dirty[0].FileID != "file1" {
		t.Fatalf("expected dirty file1, got %s", dirty[0].FileID)
	}
}

func TestDiskCache_ClearDirty(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 1, strings.NewReader("content"))
	dc.MarkDirty("file1")
	dc.ClearDirty("file1")

	dirty := dc.DirtyFiles()
	if len(dirty) != 0 {
		t.Fatalf("expected 0 dirty files after clear, got %d", len(dirty))
	}
}

func TestDiskCache_Evict(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 1, strings.NewReader("content"))
	dc.Evict("file1")

	r, _ := dc.Get("file1")
	if r != nil {
		r.Close()
		t.Fatal("expected miss after evict")
	}

	// Verify file deleted from disk
	if _, err := os.Stat(dc.blobPath("file1")); !os.IsNotExist(err) {
		t.Fatal("expected blob file deleted")
	}
}

func TestDiskCache_ZeroByteFile(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("empty", 1, strings.NewReader(""))

	r, err := dc.Get("empty")
	if err != nil || r == nil {
		t.Fatal("expected cache hit for empty file")
	}

	data, _ := io.ReadAll(r)
	r.Close()
	if len(data) != 0 {
		t.Fatalf("expected 0 bytes, got %d", len(data))
	}
}

func TestDiskCache_LocalPath(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dir, "local", "node_modules", "express")
	got := dc.LocalPath("node_modules/express")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDiskCache_ScanExisting(t *testing.T) {
	dir := t.TempDir()
	blobDir := filepath.Join(dir, "blobs")
	os.MkdirAll(blobDir, 0755)
	os.MkdirAll(filepath.Join(dir, "local"), 0755)

	// Pre-populate with a file
	os.WriteFile(filepath.Join(blobDir, "existing-file"), []byte("existing"), 0644)

	dc, err := NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Should find the existing file
	r, _ := dc.Get("existing-file")
	if r == nil {
		t.Fatal("expected to find pre-existing file in cache")
	}
	r.Close()
}
