package cache

import (
	"sync"
	"testing"
	"time"
)

func TestMetadataCache_PutAndGet(t *testing.T) {
	mc := NewMetadataCache(1 * time.Second)
	defer mc.Stop()

	mc.Put("key1", "value1")

	val, ok := mc.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val.(string) != "value1" {
		t.Fatalf("expected value1, got %v", val)
	}
}

func TestMetadataCache_Expiry(t *testing.T) {
	mc := NewMetadataCache(50 * time.Millisecond)
	defer mc.Stop()

	mc.Put("key1", "value1")

	// Should be available immediately
	if _, ok := mc.Get("key1"); !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	if _, ok := mc.Get("key1"); ok {
		t.Fatal("expected cache miss after expiry")
	}
}

func TestMetadataCache_Invalidate(t *testing.T) {
	mc := NewMetadataCache(10 * time.Second)
	defer mc.Stop()

	mc.Put("key1", "value1")
	mc.Put("key2", "value2")

	mc.Invalidate("key1")

	if _, ok := mc.Get("key1"); ok {
		t.Fatal("expected miss after invalidation")
	}
	if _, ok := mc.Get("key2"); !ok {
		t.Fatal("expected hit for non-invalidated key")
	}
}

func TestMetadataCache_InvalidatePrefix(t *testing.T) {
	mc := NewMetadataCache(10 * time.Second)
	defer mc.Stop()

	mc.Put("children:abc", "data1")
	mc.Put("children:def", "data2")
	mc.Put("lookup:abc:file.txt", "data3")

	mc.InvalidatePrefix("children:")

	if _, ok := mc.Get("children:abc"); ok {
		t.Fatal("expected miss for children:abc")
	}
	if _, ok := mc.Get("children:def"); ok {
		t.Fatal("expected miss for children:def")
	}
	if _, ok := mc.Get("lookup:abc:file.txt"); !ok {
		t.Fatal("expected hit for lookup key (different prefix)")
	}
}

func TestMetadataCache_ConcurrentAccess(t *testing.T) {
	mc := NewMetadataCache(1 * time.Second)
	defer mc.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			mc.Put(key, n)
			mc.Get(key)
			mc.Invalidate(key)
		}(i)
	}
	wg.Wait()
	// No panics = success
}

func TestMetadataCache_InvalidateAll(t *testing.T) {
	mc := NewMetadataCache(10 * time.Second)
	defer mc.Stop()

	mc.Put("a", 1)
	mc.Put("b", 2)
	mc.Put("c", 3)

	mc.InvalidateAll()

	if _, ok := mc.Get("a"); ok {
		t.Fatal("expected miss after InvalidateAll")
	}
	if _, ok := mc.Get("b"); ok {
		t.Fatal("expected miss after InvalidateAll")
	}
}
