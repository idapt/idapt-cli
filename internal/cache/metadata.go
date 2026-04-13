package cache

import (
	"sync"
	"time"
)

type MetadataEntry struct {
	Data      interface{} // FileEntry or []FileEntry
	ExpiresAt time.Time
}

type MetadataCache struct {
	mu      sync.RWMutex
	entries map[string]*MetadataEntry
	ttl     time.Duration
	stopCh  chan struct{}
}

func NewMetadataCache(ttl time.Duration) *MetadataCache {
	mc := &MetadataCache{
		entries: make(map[string]*MetadataEntry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go mc.sweepLoop()
	return mc
}

func (mc *MetadataCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

func (mc *MetadataCache) Put(key string, data interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries[key] = &MetadataEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(mc.ttl),
	}
}

func (mc *MetadataCache) Invalidate(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.entries, key)
}

func (mc *MetadataCache) InvalidatePrefix(prefix string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key := range mc.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(mc.entries, key)
		}
	}
}

func (mc *MetadataCache) InvalidateAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*MetadataEntry)
}

func (mc *MetadataCache) Stop() {
	close(mc.stopCh)
}

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
