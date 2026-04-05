package fuse

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/idapt/idapt-cli/internal/cache"
	isync "github.com/idapt/idapt-cli/internal/sync"
)

// MountConfig holds configuration for a FUSE mount.
type MountConfig struct {
	ProjectID       string
	MountPoint      string
	CacheDir        string
	MaxCacheSize    int64 // bytes, default 10GB
	ExcludePatterns []string
}

// FuseFS is the shared filesystem context for all FUSE nodes.
type FuseFS struct {
	APIClient     *FuseAPIClient
	MetadataCache *cache.MetadataCache
	DiskCache     *cache.DiskCache
	Exclusion     *isync.ExclusionEngine
	Router        *isync.WriteRouter
	Flusher       *isync.BackgroundFlusher
	SSE           *SSESubscriber // SSE cache invalidation (near-instant cross-mount visibility)
	ProjectID     string
	MountPoint    string
}

// MountManager manages active FUSE mounts.
type MountManager struct {
	mu     sync.Mutex
	mounts map[string]*activeMount // mountPoint → mount
}

type activeMount struct {
	server   *gofuse.Server
	fuseFS   *FuseFS
	cancel   context.CancelFunc
	lockFile *os.File // flock guard against concurrent mounts
}

// NewMountManager creates a new mount manager.
func NewMountManager() *MountManager {
	return &MountManager{
		mounts: make(map[string]*activeMount),
	}
}

// Mount creates and starts a FUSE mount.
func (mm *MountManager) Mount(ctx context.Context, cfg MountConfig, apiClient *FuseAPIClient) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.mounts[cfg.MountPoint]; exists {
		return fmt.Errorf("already mounted at %s", cfg.MountPoint)
	}

	// Try to clean up stale FUSE mount BEFORE mkdir (crash recovery).
	// A stale mount causes MkdirAll to fail with ENOTCONN/"file exists",
	// so we must detect and clean up first.
	if isStaleMount(cfg.MountPoint) {
		log.Printf("fuse-mount: cleaning up stale mount at %s", cfg.MountPoint)
		forceUnmount(cfg.MountPoint)
		// Give the kernel a moment to release the mount point
		time.Sleep(100 * time.Millisecond)
	}

	// Ensure mount point exists (succeeds for existing directories)
	if err := os.MkdirAll(cfg.MountPoint, 0755); err != nil {
		return fmt.Errorf("create mount point: %w", err)
	}

	// Concurrent mount guard: flock on cache directory
	lockPath := filepath.Join(cfg.CacheDir, ".fuse.lock")
	os.MkdirAll(filepath.Dir(lockPath), 0755)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("create lock file: %w", err)
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		return fmt.Errorf("another process has this project mounted (lock held on %s)", lockPath)
	}

	// Default cache size: 10GB
	if cfg.MaxCacheSize == 0 {
		cfg.MaxCacheSize = 10 * 1024 * 1024 * 1024
	}

	// Default cache dir
	if cfg.CacheDir == "" {
		cfg.CacheDir = fmt.Sprintf("/var/cache/idapt/%s", cfg.ProjectID)
	}

	// Initialize subsystems.
	// 60s TTL is a safety net — SSE invalidation handles most cache updates within ~100ms.
	metaCache := cache.NewMetadataCache(60 * time.Second)
	diskCache, err := cache.NewDiskCache(cfg.CacheDir, cfg.MaxCacheSize)
	if err != nil {
		metaCache.Stop()
		return fmt.Errorf("init disk cache: %w", err)
	}

	exclusion := isync.LoadExclusionEngine(cfg.MountPoint, cfg.ExcludePatterns)
	router := isync.NewWriteRouter(exclusion)

	fuseFS := &FuseFS{
		APIClient:     apiClient,
		MetadataCache: metaCache,
		DiskCache:     diskCache,
		Exclusion:     exclusion,
		Router:        router,
		ProjectID:     cfg.ProjectID,
		MountPoint:    cfg.MountPoint,
	}

	// Background flusher for dirty files
	flusher := isync.NewBackgroundFlusher(diskCache, func(fctx context.Context, fileID, localPath string, openVersion int) error {
		content, err := os.ReadFile(localPath)
		if err != nil {
			return err
		}
		return apiClient.UpdateFileContent(fctx, fileID, string(content), openVersion)
	}, 60*time.Second)
	fuseFS.Flusher = flusher

	// SSE subscriber for near-instant cache invalidation on remote changes
	sseSubscriber := NewSSESubscriber(apiClient, metaCache, diskCache, cfg.ProjectID)
	fuseFS.SSE = sseSubscriber

	// Create root node
	root := &IdaptNode{
		entry: &FileEntry{
			ID:       "",
			Name:     "",
			IsFolder: true,
		},
		fuseFS: fuseFS,
	}

	// Mount options
	opts := &fs.Options{
		MountOptions: gofuse.MountOptions{
			FsName:        "idapt",
			Name:          "idapt",
			DisableXAttrs: false,
			MaxBackground: 64,
			Debug:         os.Getenv("IDAPT_FUSE_DEBUG") == "1",
		},
		EntryTimeout: func() *time.Duration { d := 5 * time.Second; return &d }(),
		AttrTimeout:  func() *time.Duration { d := 5 * time.Second; return &d }(),
		NullPermissions: true,
	}

	// Create FUSE server
	server, err := fs.Mount(cfg.MountPoint, root, opts)
	if err != nil {
		metaCache.Stop()
		return fmt.Errorf("fuse mount: %w", err)
	}

	mountCtx, mountCancel := context.WithCancel(ctx)

	// Start background flusher + SSE cache invalidation
	go flusher.Start(mountCtx)
	go sseSubscriber.Start(mountCtx)

	mm.mounts[cfg.MountPoint] = &activeMount{
		server:   server,
		fuseFS:   fuseFS,
		cancel:   mountCancel,
		lockFile: lockFile,
	}

	log.Printf("fuse-mount: mounted project %s at %s", cfg.ProjectID, cfg.MountPoint)

	// Serve in background
	go func() {
		server.Wait()
		log.Printf("fuse-mount: server exited for %s", cfg.MountPoint)
	}()

	return nil
}

// Unmount stops a FUSE mount.
func (mm *MountManager) Unmount(mountPoint string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mount, exists := mm.mounts[mountPoint]
	if !exists {
		return fmt.Errorf("not mounted at %s", mountPoint)
	}

	// Flush dirty files before unmount
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if mount.fuseFS.Flusher != nil {
		if err := mount.fuseFS.Flusher.FlushAll(ctx); err != nil {
			log.Printf("fuse-mount: flush errors during unmount: %v", err)
		}
		mount.fuseFS.Flusher.Stop()
	}

	// Stop SSE subscriber + metadata cache
	if mount.fuseFS.SSE != nil {
		mount.fuseFS.SSE.Stop()
	}
	mount.fuseFS.MetadataCache.Stop()

	// Cancel mount context
	mount.cancel()

	// Unmount FUSE
	if err := mount.server.Unmount(); err != nil {
		return fmt.Errorf("fuse unmount: %w", err)
	}

	// Release flock
	if mount.lockFile != nil {
		syscall.Flock(int(mount.lockFile.Fd()), syscall.LOCK_UN)
		mount.lockFile.Close()
	}

	delete(mm.mounts, mountPoint)
	log.Printf("fuse-mount: unmounted %s", mountPoint)
	return nil
}

// Shutdown unmounts all active mounts. Called during daemon graceful shutdown.
func (mm *MountManager) Shutdown(ctx context.Context) {
	mm.mu.Lock()
	mountPoints := make([]string, 0, len(mm.mounts))
	for mp := range mm.mounts {
		mountPoints = append(mountPoints, mp)
	}
	mm.mu.Unlock()

	for _, mp := range mountPoints {
		if err := mm.Unmount(mp); err != nil {
			log.Printf("fuse-mount: shutdown unmount %s failed: %v", mp, err)
		}
	}
}

// ActiveMounts returns a list of active mount points.
func (mm *MountManager) ActiveMounts() []string {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mounts := make([]string, 0, len(mm.mounts))
	for mp := range mm.mounts {
		mounts = append(mounts, mp)
	}
	return mounts
}

// isStaleMount checks if a mount point has a stale FUSE mount (from a crash).
func isStaleMount(mountPoint string) bool {
	// Try to stat the mount point — if it returns "transport endpoint is not connected",
	// the FUSE daemon crashed and left a stale mount.
	_, err := os.Stat(mountPoint)
	if err == nil {
		return false
	}
	// Check for ENOTCONN (stale FUSE mount)
	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return errno == syscall.ENOTCONN
		}
	}
	return false
}

// forceUnmount cleans up a stale FUSE mount using fusermount3 -uz (lazy unmount).
func forceUnmount(mountPoint string) {
	// Try fusermount3 first (modern), fall back to fusermount
	for _, cmd := range []string{"fusermount3", "fusermount"} {
		if err := exec.Command(cmd, "-uz", mountPoint).Run(); err == nil {
			log.Printf("fuse-mount: force-unmounted stale mount at %s via %s", mountPoint, cmd)
			return
		}
	}
	// Last resort: umount -l
	if err := exec.Command("umount", "-l", mountPoint).Run(); err != nil {
		log.Printf("fuse-mount: failed to force-unmount %s: %v", mountPoint, err)
	}
}
