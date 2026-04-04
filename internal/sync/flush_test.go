package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/cache"
)

func TestBackgroundFlusher_FlushAll(t *testing.T) {
	dir := t.TempDir()
	dc, err := cache.NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Put a dirty file
	dc.Put("file1", 1, strings.NewReader("dirty content"))
	dc.MarkDirty("file1")

	flushed := make(map[string]bool)
	flushFn := func(ctx context.Context, fileID string, localPath string, openVersion int) error {
		flushed[fileID] = true
		return nil
	}

	flusher := NewBackgroundFlusher(dc, flushFn, 1*time.Hour)

	ctx := context.Background()
	if err := flusher.FlushAll(ctx); err != nil {
		t.Fatal(err)
	}

	if !flushed["file1"] {
		t.Error("expected file1 to be flushed")
	}
}

func TestBackgroundFlusher_NoDirtyFiles(t *testing.T) {
	dir := t.TempDir()
	dc, err := cache.NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Put a non-dirty file
	dc.Put("file1", 1, strings.NewReader("clean content"))

	flushFn := func(ctx context.Context, fileID string, localPath string, openVersion int) error {
		t.Error("should not flush non-dirty file")
		return nil
	}

	flusher := NewBackgroundFlusher(dc, flushFn, 1*time.Hour)

	ctx := context.Background()
	if err := flusher.FlushAll(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundFlusher_FlushError(t *testing.T) {
	dir := t.TempDir()
	dc, err := cache.NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 1, strings.NewReader("content"))
	dc.MarkDirty("file1")

	flushFn := func(ctx context.Context, fileID string, localPath string, openVersion int) error {
		return errors.New("upload failed")
	}

	flusher := NewBackgroundFlusher(dc, flushFn, 1*time.Hour)

	ctx := context.Background()
	err = flusher.FlushAll(ctx)
	if err == nil {
		t.Error("expected error from FlushAll")
	}
}

func TestBackgroundFlusher_ConflictSavesFile(t *testing.T) {
	dir := t.TempDir()
	dc, err := cache.NewDiskCache(dir, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	dc.Put("file1", 1, strings.NewReader("local content"))
	dc.MarkDirty("file1")

	flushFn := func(ctx context.Context, fileID string, localPath string, openVersion int) error {
		return errors.New("stale")
	}

	flusher := NewBackgroundFlusher(dc, flushFn, 1*time.Hour)

	ctx := context.Background()
	// Conflict should not return error (handled by saving .conflict)
	if err := flusher.FlushAll(ctx); err != nil {
		t.Fatal(err)
	}

	// Check conflict file exists
	conflictPath := filepath.Join(dir, "blobs", "file1.conflict")
	if _, err := os.Stat(conflictPath); os.IsNotExist(err) {
		t.Error("expected .conflict file to be created")
	}

	// File should no longer be dirty
	dirty := dc.DirtyFiles()
	if len(dirty) != 0 {
		t.Errorf("expected 0 dirty files after conflict, got %d", len(dirty))
	}
}
