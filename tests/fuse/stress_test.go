//go:build integration

package fuse_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStress_CreateManyFiles(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Create 100 files
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("stress-create-%03d.txt", i)
		path := filepath.Join(env.mountPoint, name)
		if err := os.WriteFile(path, []byte(fmt.Sprintf("content-%d", i)), 0644); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	// List and verify count
	entries, err := os.ReadDir(env.mountPoint)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "stress-create-") {
			count++
		}
	}
	if count < 100 {
		t.Errorf("expected >= 100 stress files, found %d", count)
	}
}

func TestStress_ConcurrentReaders(t *testing.T) {
	env := setupTestEnv(t)
	name := env.uniqueName("stress-concurrent.txt")
	content := strings.Repeat("x", 10*1024) // 10KB
	env.createServerFile(name, content)
	env.mount()

	path := filepath.Join(env.mountPoint, name)

	var wg sync.WaitGroup
	errors := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := os.ReadFile(path)
			if err != nil {
				errors <- err
				return
			}
			if len(data) != len(content) {
				errors <- fmt.Errorf("expected %d bytes, got %d", len(content), len(data))
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent read error: %v", err)
	}
}

func TestStress_ConcurrentWritersDifferentFiles(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("stress-cwrite-%02d.txt", n)
			path := filepath.Join(env.mountPoint, name)
			if err := os.WriteFile(path, []byte(fmt.Sprintf("writer-%d", n)), 0644); err != nil {
				errors <- fmt.Errorf("writer %d: %w", n, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}
}

func TestStress_RapidCreateDelete(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("stress-cd-%03d.txt", i)
		path := filepath.Join(env.mountPoint, name)

		if err := os.WriteFile(path, []byte("temp"), 0644); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("delete %d: %v", i, err)
		}
	}
}

func TestStress_DeepNesting(t *testing.T) {
	env := setupTestEnv(t)
	env.mount()

	// Create 10-level deep directory
	dir := env.mountPoint
	for i := 0; i < 10; i++ {
		dir = filepath.Join(dir, fmt.Sprintf("level-%d", i))
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatalf("mkdir level %d: %v", i, err)
		}
	}

	// Create file at deepest level
	deepFile := filepath.Join(dir, "deep-file.txt")
	if err := os.WriteFile(deepFile, []byte("deep content"), 0644); err != nil {
		t.Fatalf("write deep file: %v", err)
	}

	data, err := os.ReadFile(deepFile)
	if err != nil {
		t.Fatalf("read deep file: %v", err)
	}
	if string(data) != "deep content" {
		t.Errorf("expected 'deep content', got %q", string(data))
	}
}
