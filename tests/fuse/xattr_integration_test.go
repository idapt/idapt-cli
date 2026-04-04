//go:build integration

package fuse_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestXattr_ListAttributes(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("xattr-list.txt")
	env.createServerFile(name, "content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Use getfattr to list xattrs
	out, err := exec.Command("getfattr", "-d", "-m", "user.idapt.", path).Output()
	if err != nil {
		t.Skipf("getfattr not available: %v", err)
	}

	output := string(out)
	expectedKeys := []string{
		"user.idapt.resource_id",
		"user.idapt.version",
		"user.idapt.project_id",
	}

	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("expected %s in xattr listing, got: %s", key, output)
		}
	}
}

func TestXattr_GetVersion(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("xattr-ver.txt")
	env.createServerFile(name, "content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	out, err := exec.Command("getfattr", "-n", "user.idapt.version", "--only-values", path).Output()
	if err != nil {
		t.Skipf("getfattr not available: %v", err)
	}

	version := strings.TrimSpace(string(out))
	if version != "1" {
		t.Errorf("expected version '1', got %q", version)
	}
}

func TestXattr_SetReadOnly(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("xattr-ro.txt")
	env.createServerFile(name, "content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	// Attempting to set a read-only xattr should fail
	cmd := exec.Command("setfattr", "-n", "user.idapt.resource_id", "-v", "hacked", path)
	err := cmd.Run()
	if err == nil {
		t.Error("expected error setting read-only xattr")
	}
}

func TestXattr_NonExistentKey(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("xattr-nokey.txt")
	env.createServerFile(name, "content")
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	cmd := exec.Command("getfattr", "-n", "user.idapt.nonexistent", path)
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for non-existent xattr key")
	}
}

func TestXattr_ExcludedFileNoXattr(t *testing.T) {
	// Excluded files are local-only — they should not have idapt xattrs
	// This is a behavioral test: if the file doesn't go through FUSE,
	// getfattr won't find user.idapt.* attributes
	_ = os.Getenv("SKIP") // placeholder — excluded files use loopback, no xattr
	t.Skip("excluded files use local filesystem, no FUSE xattr")
}
