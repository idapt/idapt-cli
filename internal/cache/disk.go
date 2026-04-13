package cache

import (
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DiskCache struct {
	mu          sync.Mutex
	baseDir     string
	maxSize     int64
	currentSize int64
	lru         *list.List
	items       map[string]*list.Element // fileID → LRU element
}

type lruEntry struct {
	fileID     string
	size       int64
	lastAccess time.Time
	dirty      bool
}

type BlobMeta struct {
	Version    int       `json:"version"`
	Size       int64     `json:"size"`
	LastAccess time.Time `json:"lastAccess"`
}

func NewDiskCache(baseDir string, maxSize int64) (*DiskCache, error) {
	blobDir := filepath.Join(baseDir, "blobs")
	localDir := filepath.Join(baseDir, "local")

	for _, dir := range []string{blobDir, localDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create cache dir %s: %w", dir, err)
		}
	}

	dc := &DiskCache{
		baseDir: baseDir,
		maxSize: maxSize,
		lru:     list.New(),
		items:   make(map[string]*list.Element),
	}

	dc.scanExisting()
	return dc, nil
}

func (dc *DiskCache) blobPath(fileID string) string {
	return filepath.Join(dc.baseDir, "blobs", fileID)
}

func (dc *DiskCache) metaPath(fileID string) string {
	return filepath.Join(dc.baseDir, "blobs", fileID+".meta")
}

func (dc *DiskCache) LocalPath(relativePath string) string {
	return filepath.Join(dc.baseDir, "local", relativePath)
}

func (dc *DiskCache) Get(fileID string) (io.ReadCloser, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	elem, ok := dc.items[fileID]
	if !ok {
		return nil, nil
	}

	dc.lru.MoveToFront(elem)
	entry := elem.Value.(*lruEntry)
	entry.lastAccess = time.Now()

	f, err := os.Open(dc.blobPath(fileID))
	if err != nil {
		if os.IsNotExist(err) {
			dc.removeLocked(fileID)
			return nil, nil
		}
		return nil, err
	}

	return f, nil
}

func (dc *DiskCache) Put(fileID string, version int, r io.Reader) (string, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	path := dc.blobPath(fileID)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create cache file: %w", err)
	}

	n, err := io.Copy(f, r)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("write cache file: %w", err)
	}

	meta := BlobMeta{Version: version, Size: n, LastAccess: time.Now()}
	metaData, _ := json.Marshal(meta)
	if err := os.WriteFile(dc.metaPath(fileID), metaData, 0644); err != nil {
		log.Printf("fuse-cache: failed to write meta for %s: %v", fileID, err)
	}

	if elem, ok := dc.items[fileID]; ok {
		old := elem.Value.(*lruEntry)
		dc.currentSize -= old.size
		old.size = n
		old.lastAccess = time.Now()
		old.dirty = false
		dc.lru.MoveToFront(elem)
	} else {
		entry := &lruEntry{fileID: fileID, size: n, lastAccess: time.Now()}
		elem := dc.lru.PushFront(entry)
		dc.items[fileID] = elem
	}
	dc.currentSize += n

	dc.evictIfNeeded()

	return path, nil
}

func (dc *DiskCache) GetVersion(fileID string) int {
	data, err := os.ReadFile(dc.metaPath(fileID))
	if err != nil {
		return -1
	}

	var meta BlobMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return -1
	}
	return meta.Version
}

func (dc *DiskCache) MarkDirty(fileID string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if elem, ok := dc.items[fileID]; ok {
		elem.Value.(*lruEntry).dirty = true
	}
}

type DirtyFile struct {
	FileID  string
	Path    string
	Version int
}

func (dc *DiskCache) DirtyFiles() []DirtyFile {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	var files []DirtyFile
	for e := dc.lru.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*lruEntry)
		if entry.dirty {
			files = append(files, DirtyFile{
				FileID:  entry.fileID,
				Path:    dc.blobPath(entry.fileID),
				Version: dc.GetVersion(entry.fileID),
			})
		}
	}
	return files
}

func (dc *DiskCache) ClearDirty(fileID string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if elem, ok := dc.items[fileID]; ok {
		elem.Value.(*lruEntry).dirty = false
	}
}

func (dc *DiskCache) CachedFileIDs() []string {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	ids := make([]string, 0, len(dc.items))
	for id := range dc.items {
		ids = append(ids, id)
	}
	return ids
}

func (dc *DiskCache) Evict(fileID string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.removeLocked(fileID)
}

func (dc *DiskCache) removeLocked(fileID string) {
	if elem, ok := dc.items[fileID]; ok {
		entry := elem.Value.(*lruEntry)
		dc.currentSize -= entry.size
		dc.lru.Remove(elem)
		delete(dc.items, fileID)

		os.Remove(dc.blobPath(fileID))
		os.Remove(dc.metaPath(fileID))
	}
}

func (dc *DiskCache) evictIfNeeded() {
	for dc.currentSize > dc.maxSize {
		back := dc.lru.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*lruEntry)

		if entry.dirty {
			dc.lru.MoveToFront(back) // protect from future eviction
			break
		}

		dc.removeLocked(entry.fileID)
	}
}

func (dc *DiskCache) scanExisting() {
	blobDir := filepath.Join(dc.baseDir, "blobs")
	entries, err := os.ReadDir(blobDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".meta" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		fileID := e.Name()
		size := info.Size()
		entry := &lruEntry{
			fileID:     fileID,
			size:       size,
			lastAccess: info.ModTime(),
		}

		elem := dc.lru.PushBack(entry)
		dc.items[fileID] = elem
		dc.currentSize += size
	}
}
