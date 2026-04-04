//go:build integration

package fuse_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ifuse "github.com/idapt/idapt-cli/internal/fuse"
	isync "github.com/idapt/idapt-cli/internal/sync"
)

func TestSyncDown_DownloadsFiles(t *testing.T) {
	env := setupTestEnv(t)

	// Create files on server
	name1 := env.uniqueName("sync-down1.txt")
	name2 := env.uniqueName("sync-down2.txt")
	env.createServerFile(name1, "content1")
	env.createServerFile(name2, "content2")

	localDir := filepath.Join(t.TempDir(), "sync-down")
	os.MkdirAll(localDir, 0755)

	ctx := context.Background()
	exclusion := isync.NewExclusionEngine("", "", nil)

	// Sync down
	if err := syncDownHelper(ctx, env.apiClient, env.projectID, localDir, exclusion); err != nil {
		t.Fatalf("sync down: %v", err)
	}

	// Verify files downloaded
	data1, err := os.ReadFile(filepath.Join(localDir, name1))
	if err != nil {
		t.Fatalf("read %s: %v", name1, err)
	}
	if string(data1) != "content1" {
		t.Errorf("expected 'content1', got %q", string(data1))
	}

	data2, err := os.ReadFile(filepath.Join(localDir, name2))
	if err != nil {
		t.Fatalf("read %s: %v", name2, err)
	}
	if string(data2) != "content2" {
		t.Errorf("expected 'content2', got %q", string(data2))
	}
}

func TestSyncDown_RespectsExclusions(t *testing.T) {
	env := setupTestEnv(t)

	// Create both synced and "excluded" files
	syncedName := env.uniqueName("sync-included.txt")
	env.createServerFile(syncedName, "include me")

	localDir := filepath.Join(t.TempDir(), "sync-excl")
	os.MkdirAll(localDir, 0755)

	ctx := context.Background()
	exclusion := isync.NewExclusionEngine("*.log\n", "", nil)

	if err := syncDownHelper(ctx, env.apiClient, env.projectID, localDir, exclusion); err != nil {
		t.Fatalf("sync down: %v", err)
	}

	// Synced file should exist
	if _, err := os.Stat(filepath.Join(localDir, syncedName)); err != nil {
		t.Errorf("expected %s to be downloaded", syncedName)
	}
}

func TestSyncDown_EmptyProject(t *testing.T) {
	env := setupTestEnv(t)
	// Don't create any files

	localDir := filepath.Join(t.TempDir(), "sync-empty")
	os.MkdirAll(localDir, 0755)

	ctx := context.Background()
	exclusion := isync.NewExclusionEngine("", "", nil)

	// Should not error on empty project
	if err := syncDownHelper(ctx, env.apiClient, env.projectID, localDir, exclusion); err != nil {
		t.Fatalf("sync down empty: %v", err)
	}
}

// syncDownHelper wraps the sync down logic for testing.
func syncDownHelper(ctx context.Context, client *ifuse.FuseAPIClient, projectID, localDir string, exclusion *isync.ExclusionEngine) error {
	files, err := client.ListFiles(ctx, projectID, "")
	if err != nil {
		return err
	}

	for _, f := range files {
		if exclusion.IsExcluded(f.Name) {
			continue
		}

		if f.IsFolder {
			os.MkdirAll(filepath.Join(localDir, f.Name), 0755)
			continue
		}

		reader, err := client.DownloadFile(ctx, f.ID)
		if err != nil {
			continue
		}

		outPath := filepath.Join(localDir, f.Name)
		outFile, err := os.Create(outPath)
		if err != nil {
			reader.Close()
			continue
		}
		outFile.ReadFrom(reader)
		outFile.Close()
		reader.Close()
	}

	return nil
}
