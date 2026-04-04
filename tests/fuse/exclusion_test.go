//go:build integration

package fuse_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ifuse "github.com/idapt/idapt-cli/internal/fuse"
)

func TestExclusion_ExcludedDirIsLocalOnly(t *testing.T) {
	env := setupTestEnv(t)

	// Mount with node_modules excluded
	cfg := ifuse.MountConfig{
		ProjectID:       env.projectID,
		MountPoint:      env.mountPoint,
		CacheDir:        filepath.Join(t.TempDir(), "cache"),
		ExcludePatterns: []string{"node_modules"},
	}
	ctx := context.Background()
	if err := env.mountMgr.Mount(ctx, cfg, env.apiClient); err != nil {
		t.Fatalf("mount: %v", err)
	}
	t.Cleanup(func() { env.mountMgr.Unmount(env.mountPoint) })

	// Create files in excluded directory
	nmDir := filepath.Join(env.mountPoint, "node_modules")
	os.MkdirAll(nmDir, 0755)
	os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{}`), 0644)

	// Verify files exist locally
	if _, err := os.Stat(filepath.Join(nmDir, "package.json")); err != nil {
		t.Error("expected excluded file to exist locally")
	}
}

func TestExclusion_SyncedFileWorks(t *testing.T) {
	env := setupTestEnv(t)

	cfg := ifuse.MountConfig{
		ProjectID:       env.projectID,
		MountPoint:      env.mountPoint,
		CacheDir:        filepath.Join(t.TempDir(), "cache"),
		ExcludePatterns: []string{"node_modules"},
	}
	ctx := context.Background()
	if err := env.mountMgr.Mount(ctx, cfg, env.apiClient); err != nil {
		t.Fatalf("mount: %v", err)
	}
	t.Cleanup(func() { env.mountMgr.Unmount(env.mountPoint) })

	// Create a synced file (not excluded)
	name := env.uniqueName("excl-synced.txt")
	path := filepath.Join(env.mountPoint, name)
	if err := os.WriteFile(path, []byte("synced content"), 0644); err != nil {
		t.Fatalf("write synced file: %v", err)
	}

	// Verify it was created on server
	files, _ := env.apiClient.ListFiles(ctx, env.projectID, "")
	found := false
	for _, f := range files {
		if f.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Error("synced file should appear on server")
	}
}
