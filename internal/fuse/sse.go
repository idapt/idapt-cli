package fuse

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/idapt/idapt-cli/internal/cache"
)

// SSESubscriber subscribes to server-sent events for a project and invalidates
// the local metadata and disk caches when remote changes are detected.
// This enables near-instant cross-mount visibility (~100ms) instead of waiting
// for the TTL safety net (60s).
type SSESubscriber struct {
	apiClient     *FuseAPIClient
	metadataCache *cache.MetadataCache
	diskCache     *cache.DiskCache
	projectID     string
	stopCh        chan struct{}
}

// NewSSESubscriber creates a subscriber for project file change events.
func NewSSESubscriber(apiClient *FuseAPIClient, metadataCache *cache.MetadataCache, diskCache *cache.DiskCache, projectID string) *SSESubscriber {
	return &SSESubscriber{
		apiClient:     apiClient,
		metadataCache: metadataCache,
		diskCache:     diskCache,
		projectID:     projectID,
		stopCh:        make(chan struct{}),
	}
}

// Start connects to the SSE endpoint and processes events until stopped.
// Reconnects with exponential backoff on connection loss.
func (s *SSESubscriber) Start(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		err := s.connect(ctx)
		if err != nil {
			log.Printf("fuse-sse: connection lost: %v — reconnecting in %v", err, backoff)
		}

		// On disconnect, invalidate all metadata to ensure freshness via TTL
		s.metadataCache.InvalidateAll()

		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-time.After(backoff):
		}

		// Exponential backoff, capped
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Stop signals the subscriber to disconnect and exit.
func (s *SSESubscriber) Stop() {
	close(s.stopCh)
}

// connect establishes an SSE connection and reads events until error or stop.
func (s *SSESubscriber) connect(ctx context.Context) error {
	// Build SSE URL — uses the existing subscriptions endpoint
	resp, err := s.apiClient.client.Do(ctx, "GET", "/api/subscriptions/files?projectId="+s.projectID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &sseError{status: resp.StatusCode}
	}

	log.Printf("fuse-sse: connected to project %s event stream", s.projectID)

	scanner := bufio.NewScanner(resp.Body)
	// SSE lines can be long (file content in events)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			eventData.WriteString(line[6:])
			continue
		}

		// Empty line = end of event
		if line == "" && eventData.Len() > 0 {
			s.processEvent(eventData.String())
			eventData.Reset()
			// Reset backoff on successful event
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil // connection closed cleanly
}

// sseEvent is the minimal structure we need from file SSE events.
type sseEvent struct {
	Type     string `json:"type"`
	FileID   string `json:"fileId"`
	ParentID string `json:"parentId"`
	Version  int    `json:"version"`
}

// processEvent handles a single SSE event by invalidating relevant caches.
func (s *SSESubscriber) processEvent(data string) {
	var event sseEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// Silently ignore unparseable events (could be heartbeat/ping)
		return
	}

	switch event.Type {
	case "files:updated":
		s.handleFileUpdated(event)
	case "files:deleted":
		s.handleFileDeleted(event)
	case "files:created":
		s.handleFileCreated(event)
	default:
		// Ignore unknown event types (batch, live:update, etc.)
	}
}

func (s *SSESubscriber) handleFileUpdated(event sseEvent) {
	// Only evict disk cache if server version is newer than cached
	if event.Version > 0 {
		cachedVersion := s.diskCache.GetVersion(event.FileID)
		if cachedVersion >= 0 && event.Version > cachedVersion {
			s.diskCache.Evict(event.FileID)
			log.Printf("fuse-sse: evicted %s (cached v%d < server v%d)", event.FileID, cachedVersion, event.Version)
		}
	}

	// Invalidate metadata cache for this file and parent's children list
	s.metadataCache.InvalidatePrefix("lookup:" + event.ParentID + ":")
	s.metadataCache.Invalidate("children:" + event.ParentID)
}

func (s *SSESubscriber) handleFileDeleted(event sseEvent) {
	s.diskCache.Evict(event.FileID)
	s.metadataCache.InvalidatePrefix("lookup:" + event.ParentID + ":")
	s.metadataCache.Invalidate("children:" + event.ParentID)
	log.Printf("fuse-sse: evicted deleted file %s", event.FileID)
}

func (s *SSESubscriber) handleFileCreated(event sseEvent) {
	// New file — invalidate parent's children list so next readdir includes it
	s.metadataCache.Invalidate("children:" + event.ParentID)
	log.Printf("fuse-sse: invalidated children of %s (new file created)", event.ParentID)
}

type sseError struct {
	status int
}

func (e *sseError) Error() string {
	return "SSE connection failed with status " + http.StatusText(e.status)
}
