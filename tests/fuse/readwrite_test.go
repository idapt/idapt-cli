//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite_CreateFile(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	name := env.uniqueName("write-create.txt")
	path := filepath.Join(env.mountPoint, name)

	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Verify readable
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected %q, got %q", "hello", string(data))
	}
}

func TestWrite_Mkdir(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	name := env.uniqueName("write-mkdir")
	path := filepath.Join(env.mountPoint, name)

	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestWrite_RemoveFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("write-rm.txt")
	env.createServerFile(name, "to be deleted")
	env.mount()

	path := filepath.Join(env.mountPoint, name)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be gone after remove")
	}
}

func TestWrite_RenameFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("write-rename.txt")
	newName := env.uniqueName("write-renamed.txt")
	env.createServerFile(name, "rename me")
	env.mount()

	oldPath := filepath.Join(env.mountPoint, name)
	newPath := filepath.Join(env.mountPoint, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected old path gone")
	}

	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read renamed: %v", err)
	}
	if string(data) != "rename me" {
		t.Errorf("expected %q, got %q", "rename me", string(data))
	}
}

func TestWrite_OverwriteFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("write-overwrite.txt")
	env.createServerFile(name, "original")
	env.mount()

	path := filepath.Join(env.mountPoint, name)
	if err := os.WriteFile(path, []byte("updated"), 0644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "updated" {
		t.Errorf("expected %q, got %q", "updated", string(data))
	}
}

func TestWrite_ZeroByteFile(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	name := env.uniqueName("write-empty.txt")
	path := filepath.Join(env.mountPoint, name)

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat empty: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected 0 bytes, got %d", info.Size())
	}
}

func TestWrite_Symlink(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	target := env.uniqueName("write-symtarget.txt")
	link := env.uniqueName("write-symlink")

	// Create target
	targetPath := filepath.Join(env.mountPoint, target)
	os.WriteFile(targetPath, []byte("target content"), 0644)

	// Create symlink
	linkPath := filepath.Join(env.mountPoint, link)
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Read via symlink
	resolved, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if resolved != target {
		t.Errorf("expected link target %q, got %q", target, resolved)
	}
}
