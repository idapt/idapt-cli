//go:build integration

package fuse_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ifuse "github.com/idapt/idapt-cli/internal/fuse"
)

func TestMount_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Verify mount point is a FUSE filesystem
	cmd := exec.Command("mountpoint", "-q", env.mountPoint)
	if err := cmd.Run(); err != nil {
		t.Errorf("expected %s to be a mountpoint", env.mountPoint)
	}
}

func TestMount_UnmountSuccess(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Unmount
	if err := env.mountMgr.Unmount(env.mountPoint); err != nil {
		t.Fatalf("unmount failed: %v", err)
	}

	// Verify no longer a mountpoint
	cmd := exec.Command("mountpoint", "-q", env.mountPoint)
	if err := cmd.Run(); err == nil {
		t.Errorf("expected %s to NOT be a mountpoint after unmount", env.mountPoint)
	}
}

func TestMount_DoubleMountError(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Second mount should fail
	cfg := ifuse.MountConfig{
		ProjectID:  env.projectID,
		MountPoint: env.mountPoint,
		CacheDir:   filepath.Join(t.TempDir(), "cache2"),
	}
	err := env.mountMgr.Mount(context.Background(), cfg, env.apiClient)
	if err == nil {
		t.Error("expected error on double mount")
	}
}

func TestMount_InvalidMountPoint(t *testing.T) {
	env := setupTestEnv(t)

	cfg := ifuse.MountConfig{
		ProjectID:  env.projectID,
		MountPoint: "/proc/nonexistent/path", // cannot create
	}
	err := env.mountMgr.Mount(context.Background(), cfg, env.apiClient)
	if err == nil {
		t.Error("expected error for invalid mount point")
	}
}

func TestMount_ActiveMountsList(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	mounts := env.mountMgr.ActiveMounts()
	if len(mounts) != 1 {
		t.Fatalf("expected 1 active mount, got %d", len(mounts))
	}
	if mounts[0] != env.mountPoint {
		t.Errorf("expected mount point %s, got %s", env.mountPoint, mounts[0])
	}
}

func TestMount_ShutdownUnmountsAll(t *testing.T) {
	env := setupTestEnv(t)

	// Mount two projects (using different mount points)
	mp1 := filepath.Join(t.TempDir(), "mnt1")
	mp2 := filepath.Join(t.TempDir(), "mnt2")
	os.MkdirAll(mp1, 0755)
	os.MkdirAll(mp2, 0755)

	ctx := context.Background()
	env.mountMgr.Mount(ctx, ifuse.MountConfig{
		ProjectID: env.projectID, MountPoint: mp1,
		CacheDir: filepath.Join(t.TempDir(), "c1"),
	}, env.apiClient)
	env.mountMgr.Mount(ctx, ifuse.MountConfig{
		ProjectID: env.projectID, MountPoint: mp2,
		CacheDir: filepath.Join(t.TempDir(), "c2"),
	}, env.apiClient)

	// Shutdown should unmount all
	env.mountMgr.Shutdown(ctx)

	if len(env.mountMgr.ActiveMounts()) != 0 {
		t.Error("expected 0 active mounts after shutdown")
	}
}
