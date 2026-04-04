package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/idapt/idapt-cli/internal/cache"
)

// FlushFunc is the callback used to upload dirty file content to the server.
// It receives the fileID, local file path, and the version the file was opened at.
// Returns nil on success, or an error (including ESTALE for version conflicts).
type FlushFunc func(ctx context.Context, fileID string, localPath string, openVersion int) error

// BackgroundFlusher periodically uploads dirty cached files to the server.
type BackgroundFlusher struct {
	diskCache *cache.DiskCache
	flushFn   FlushFunc
	interval  time.Duration
	stopCh    chan struct{}
}

// NewBackgroundFlusher creates a background flusher.
func NewBackgroundFlusher(diskCache *cache.DiskCache, flushFn FlushFunc, interval time.Duration) *BackgroundFlusher {
	return &BackgroundFlusher{
		diskCache: diskCache,
		flushFn:   flushFn,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the background flush loop.
func (f *BackgroundFlusher) Start(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("fuse-flush: stopping (context cancelled)")
			return
		case <-f.stopCh:
			log.Printf("fuse-flush: stopping (stop requested)")
			return
		case <-ticker.C:
			f.flushDirty(ctx)
		}
	}
}

// FlushAll forces an immediate flush of all dirty files. Called on unmount.
func (f *BackgroundFlusher) FlushAll(ctx context.Context) error {
	dirty := f.diskCache.DirtyFiles()
	if len(dirty) == 0 {
		return nil
	}

	log.Printf("fuse-flush: flushing %d dirty files", len(dirty))
	var lastErr error
	for _, d := range dirty {
		if err := f.flushOne(ctx, d); err != nil {
			log.Printf("fuse-flush: failed to flush %s: %v", d.FileID, err)
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("flush had errors (last: %w)", lastErr)
	}
	return nil
}

// Stop signals the background loop to exit.
func (f *BackgroundFlusher) Stop() {
	close(f.stopCh)
}

func (f *BackgroundFlusher) flushDirty(ctx context.Context) {
	dirty := f.diskCache.DirtyFiles()
	if len(dirty) == 0 {
		return
	}

	log.Printf("fuse-flush: found %d dirty files", len(dirty))
	for _, d := range dirty {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := f.flushOne(ctx, d); err != nil {
			log.Printf("fuse-flush: failed to flush %s: %v", d.FileID, err)
		}
	}
}

func (f *BackgroundFlusher) flushOne(ctx context.Context, d cache.DirtyFile) error {
	if err := f.flushFn(ctx, d.FileID, d.Path, d.Version); err != nil {
		// On OCC conflict: save as .conflict file
		if isStaleError(err) {
			conflictPath := d.Path + ".conflict"
			if copyErr := copyFile(d.Path, conflictPath); copyErr != nil {
				log.Printf("fuse-flush: failed to save conflict file for %s: %v", d.FileID, copyErr)
			} else {
				log.Printf("fuse-flush: OCC conflict for %s — saved as %s", d.FileID, conflictPath)
			}
			f.diskCache.ClearDirty(d.FileID)
			return nil
		}
		return err
	}

	f.diskCache.ClearDirty(d.FileID)
	return nil
}

func isStaleError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "stale" || fmt.Sprintf("%v", err) == "stale NFS file handle"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Sync()
}
