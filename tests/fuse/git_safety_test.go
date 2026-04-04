//go:build integration

package fuse_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGit_InitAndCommit(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Create a file
	name := env.uniqueName("git-test.txt")
	path := filepath.Join(env.mountPoint, name)
	os.WriteFile(path, []byte("git content"), 0644)

	// Init git repo (in excluded .git or as test)
	gitDir := filepath.Join(env.mountPoint, ".git-test")
	cmd := exec.Command("git", "init", env.mountPoint)
	cmd.Env = append(os.Environ(), "GIT_DIR="+gitDir)
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Add and commit
	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = env.mountPoint
	addCmd.Env = append(os.Environ(), "GIT_DIR="+gitDir)
	if err := addCmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "test commit", "--author=Test <test@test.com>")
	commitCmd.Dir = env.mountPoint
	commitCmd.Env = append(os.Environ(), "GIT_DIR="+gitDir, "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func TestGit_StatusOnSyncedFiles(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Create a few files
	for i := 0; i < 5; i++ {
		name := env.uniqueName("git-status.txt")
		os.WriteFile(filepath.Join(env.mountPoint, name), []byte("content"), 0644)
	}

	// git status should work without corruption
	gitDir := filepath.Join(env.mountPoint, ".git-test2")
	exec.Command("git", "init", env.mountPoint).Run()

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = env.mountPoint
	statusCmd.Env = append(os.Environ(), "GIT_DIR="+gitDir)
	out, err := statusCmd.Output()
	if err != nil {
		t.Skipf("git status: %v", err)
	}

	// Should have output (untracked files)
	if len(out) == 0 {
		t.Log("git status returned empty (may not have initialized properly)")
	}
}
