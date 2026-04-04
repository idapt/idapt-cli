// Package cache provides caching for the FUSE filesystem.
//
// MetadataCache is an in-memory TTL cache for file/folder metadata.
// DiskCache is a local LRU disk cache for file content.
package cache

import (
	"sync"
	"time"
)

// MetadataEntry holds cached metadata for a file or directory listing.
type MetadataEntry struct {
	Data      interface{} // FileEntry or []FileEntry
	ExpiresAt time.Time
}

// MetadataCache is a thread-safe in-memory cache with TTL eviction.
type MetadataCache struct {
	mu      sync.RWMutex
	entries map[string]*MetadataEntry
	ttl     time.Duration
	stopCh  chan struct{}
}

// NewMetadataCache creates a new metadata cache with the given TTL.
func NewMetadataCache(ttl time.Duration) *MetadataCache {
	mc := &MetadataCache{
		entries: make(map[string]*MetadataEntry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go mc.sweepLoop()
	return mc
}

// Get returns a cached entry if it exists and hasn't expired.
func (mc *MetadataCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

// Put stores a value in the cache with the configured TTL.
func (mc *MetadataCache) Put(key string, data interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries[key] = &MetadataEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(mc.ttl),
	}
}

// Invalidate removes a single key from the cache.
func (mc *MetadataCache) Invalidate(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.entries, key)
}

// InvalidatePrefix removes all keys with the given prefix.
func (mc *MetadataCache) InvalidatePrefix(prefix string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key := range mc.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(mc.entries, key)
		}
	}
}

// InvalidateAll clears all entries.
func (mc *MetadataCache) InvalidateAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*MetadataEntry)
}

// Stop terminates the background sweep goroutine.
func (mc *MetadataCache) Stop() {
	close(mc.stopCh)
}

// sweepLoop runs every 30 seconds to remove expired entries.
func (mc *MetadataCache) sweepLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.sweep()
		}
	}
}

func (mc *MetadataCache) sweep() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, entry := range mc.entries {
		if now.After(entry.ExpiresAt) {
			delete(mc.entries, key)
		}
	}
}
