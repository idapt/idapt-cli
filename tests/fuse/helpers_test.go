//go:build integration

package fuse_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
	ifuse "github.com/idapt/idapt-cli/internal/fuse"
)

// testEnv holds shared test infrastructure.
type testEnv struct {
	apiClient  *ifuse.FuseAPIClient
	rawClient  *api.Client
	mountMgr   *ifuse.MountManager
	projectID  string
	mountPoint string
	t          *testing.T
}

func skipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("IDAPT_API_URL") == "" || os.Getenv("IDAPT_API_KEY") == "" {
		t.Skip("Skipping integration test: IDAPT_API_URL and IDAPT_API_KEY required")
	}
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	skipIfNoIntegration(t)

	apiURL := os.Getenv("IDAPT_API_URL")
	apiKey := os.Getenv("IDAPT_API_KEY")
	projectID := os.Getenv("IDAPT_TEST_PROJECT_ID")
	if projectID == "" {
		t.Fatal("IDAPT_TEST_PROJECT_ID required for FUSE integration tests")
	}

	rawClient, err := api.NewClient(api.ClientConfig{
		BaseURL: apiURL,
		APIKey:  apiKey,
	})
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}

	fuseClient := ifuse.NewFuseAPIClient(rawClient)
	mountMgr := ifuse.NewMountManager()

	mountPoint := filepath.Join(t.TempDir(), "mnt")
	os.MkdirAll(mountPoint, 0755)

	return &testEnv{
		apiClient:  fuseClient,
		rawClient:  rawClient,
		mountMgr:   mountMgr,
		projectID:  projectID,
		mountPoint: mountPoint,
		t:          t,
	}
}

func (env *testEnv) mount() {
	env.t.Helper()
	cfg := ifuse.MountConfig{
		ProjectID:  env.projectID,
		MountPoint: env.mountPoint,
		CacheDir:   filepath.Join(env.t.TempDir(), "cache"),
	}

	ctx := context.Background()
	if err := env.mountMgr.Mount(ctx, cfg, env.apiClient); err != nil {
		env.t.Fatalf("mount failed: %v", err)
	}

	env.t.Cleanup(func() {
		env.mountMgr.Unmount(env.mountPoint)
	})

	// Wait briefly for FUSE to be ready
	time.Sleep(100 * time.Millisecond)
}

func (env *testEnv) createServerFile(name, content string) string {
	env.t.Helper()
	ctx := context.Background()
	entry, err := env.apiClient.CreateFile(ctx, env.projectID, "", name, []byte(content), "text/plain")
	if err != nil {
		env.t.Fatalf("create server file %s: %v", name, err)
	}
	env.t.Cleanup(func() {
		env.apiClient.TrashFile(context.Background(), entry.ID)
	})
	return entry.ID
}

func (env *testEnv) uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
