package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/idapt/idapt-cli/internal/cache"
)

type FlushFunc func(ctx context.Context, fileID string, localPath string, openVersion int) error

type BackgroundFlusher struct {
	diskCache *cache.DiskCache
	flushFn   FlushFunc
	interval  time.Duration
	stopCh    chan struct{}
}

func NewBackgroundFlusher(diskCache *cache.DiskCache, flushFn FlushFunc, interval time.Duration) *BackgroundFlusher {
	return &BackgroundFlusher{
		diskCache: diskCache,
		flushFn:   flushFn,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

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
		if IsConflictError(err) {
			conflictPath := d.Path + ".conflict"
			if copyErr := copyFile(d.Path, conflictPath); copyErr != nil {
				log.Printf("fuse-flush: failed to save conflict file for %s: %v", d.FileID, copyErr)
			} else {
				log.Printf("fuse-flush: OCC conflict for %s — local copy saved as %s", d.FileID, conflictPath)
			}
			f.diskCache.ClearDirty(d.FileID)
			f.diskCache.Evict(d.FileID)
			return nil
		}
		return err
	}

	f.diskCache.ClearDirty(d.FileID)
	return nil
}

func IsConflictError(err error) bool {
	if err == nil {
		return false
	}
	if err == syscall.ESTALE {
		return true
	}
	s := err.Error()
	return s == "stale" || strings.Contains(s, "conflict") || strings.Contains(s, "stale NFS file handle")
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
