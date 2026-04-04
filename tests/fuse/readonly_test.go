//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRead_ListRootFiles(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("read-list.txt")
	env.createServerFile(name, "content")
	env.mount()

	entries, err := os.ReadDir(env.mountPoint)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find %s in directory listing", name)
	}
}

func TestRead_CatFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("read-cat.txt")
	content := "hello from server"
	env.createServerFile(name, content)
	env.mount()

	data, err := os.ReadFile(filepath.Join(env.mountPoint, name))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestRead_StatFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("read-stat.txt")
	content := "stat test content"
	env.createServerFile(name, content)
	env.mount()

	info, err := os.Stat(filepath.Join(env.mountPoint, name))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Size() != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), info.Size())
	}
	if info.IsDir() {
		t.Error("expected file, got directory")
	}
}

func TestRead_NonExistentFile(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	_, err := os.ReadFile(filepath.Join(env.mountPoint, "nonexistent-file.txt"))
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected ENOENT, got %v", err)
	}
}

func TestRead_EmptyFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("read-empty.txt")
	env.createServerFile(name, "")
	env.mount()

	data, err := os.ReadFile(filepath.Join(env.mountPoint, name))
	if err != nil {
		t.Fatalf("read empty file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}
}

func TestRead_LargeFile(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("read-large.txt")
	content := strings.Repeat("x", 100*1024) // 100KB
	env.createServerFile(name, content)
	env.mount()

	data, err := os.ReadFile(filepath.Join(env.mountPoint, name))
	if err != nil {
		t.Fatalf("read large file: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(data))
	}
}
