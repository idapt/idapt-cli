package fuse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cache"
)

type SSESubscriber struct {
	apiClient     *FuseAPIClient
	metadataCache *cache.MetadataCache
	diskCache     *cache.DiskCache
	projectID     string
	stopCh        chan struct{}
}

func NewSSESubscriber(apiClient *FuseAPIClient, metadataCache *cache.MetadataCache, diskCache *cache.DiskCache, projectID string) *SSESubscriber {
	return &SSESubscriber{
		apiClient:     apiClient,
		metadataCache: metadataCache,
		diskCache:     diskCache,
		projectID:     projectID,
		stopCh:        make(chan struct{}),
	}
}

func (s *SSESubscriber) Start(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	consecutiveFailures := 0
	const maxConsecutiveFailures = 10

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
			consecutiveFailures++
			if consecutiveFailures <= 3 || consecutiveFailures%10 == 0 {
				log.Printf("fuse-sse: connection lost (%d failures): %v — reconnecting in %v", consecutiveFailures, err, backoff)
			}
			if consecutiveFailures == maxConsecutiveFailures {
				log.Printf("fuse-sse: %d consecutive failures — suppressing further logs (TTL cache still active)", maxConsecutiveFailures)
			}
		} else {
			backoff = 1 * time.Second
			consecutiveFailures = 0
		}

		s.metadataCache.InvalidateAll()
		s.reconcileDiskCache(ctx)

		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (s *SSESubscriber) Stop() {
	close(s.stopCh)
}

func (s *SSESubscriber) reconcileDiskCache(ctx context.Context) {
	cachedIDs := s.diskCache.CachedFileIDs()
	if len(cachedIDs) == 0 {
		return
	}

	serverVersions, err := s.apiClient.GetFileVersionsBatch(ctx, cachedIDs)
	if err != nil {
		log.Printf("fuse-sse: reconcile failed (will rely on TTL): %v", err)
		return
	}

	evicted := 0
	for fileID, serverVersion := range serverVersions {
		cachedVersion := s.diskCache.GetVersion(fileID)
		if cachedVersion >= 0 && serverVersion > cachedVersion {
			s.diskCache.Evict(fileID)
			evicted++
		}
	}

	if evicted > 0 {
		log.Printf("fuse-sse: reconciled %d/%d cached files (%d evicted as stale)", len(cachedIDs), len(cachedIDs), evicted)
	}
}

func (s *SSESubscriber) connect(ctx context.Context) error {
	resp, err := s.apiClient.client.Do(ctx, "GET", "/api/subscriptions/files", nil,
		api.WithQuery(url.Values{"projectId": {s.projectID}}))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		return fmt.Errorf("unexpected content-type %q (expected text/event-stream)", ct)
	}

	log.Printf("fuse-sse: connected to project %s event stream", s.projectID)

	scanner := bufio.NewScanner(resp.Body)
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

		if line == "" && eventData.Len() > 0 {
			s.processEvent(eventData.String())
			eventData.Reset()
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type sseEnvelope struct {
	Channel string          `json:"channel"`
	Message json.RawMessage `json:"message"`
}

type sseEvent struct {
	Type     string `json:"type"`
	FileID   string `json:"fileId"`
	ParentID string `json:"parentId"`
	Version  int    `json:"version"`
}

type sseBatchEvent struct {
	Type       string     `json:"type"`
	Operations []sseEvent `json:"operations"`
}

func (s *SSESubscriber) processEvent(data string) {
	var envelope sseEnvelope
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return // silently ignore unparseable (heartbeat/ping)
	}

	if len(envelope.Message) == 0 {
		return
	}

	var event sseEvent
	if err := json.Unmarshal(envelope.Message, &event); err != nil {
		return
	}

	switch event.Type {
	case "files:updated":
		s.handleFileUpdated(event)
	case "files:deleted":
		s.handleFileDeleted(event)
	case "files:created":
		s.handleFileCreated(event)
	case "files:batch":
		s.handleBatch(envelope.Message)
	default:
	}
}

func (s *SSESubscriber) handleFileUpdated(event sseEvent) {
	s.diskCache.Evict(event.FileID)

	s.metadataCache.InvalidatePrefix("lookup:" + event.ParentID + ":")
	s.metadataCache.Invalidate("children:" + event.ParentID)
}

func (s *SSESubscriber) handleFileDeleted(event sseEvent) {
	s.diskCache.Evict(event.FileID)
	s.metadataCache.InvalidatePrefix("lookup:" + event.ParentID + ":")
	s.metadataCache.Invalidate("children:" + event.ParentID)
}

func (s *SSESubscriber) handleFileCreated(event sseEvent) {
	s.metadataCache.Invalidate("children:" + event.ParentID)
}

func (s *SSESubscriber) handleBatch(raw json.RawMessage) {
	var batch sseBatchEvent
	if err := json.Unmarshal(raw, &batch); err != nil {
		return
	}
	for _, op := range batch.Operations {
		switch op.Type {
		case "files:updated":
			s.handleFileUpdated(op)
		case "files:deleted":
			s.handleFileDeleted(op)
		case "files:created":
			s.handleFileCreated(op)
		}
	}
}
