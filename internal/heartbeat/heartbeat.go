package heartbeat

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	// Interval between heartbeats.
	Interval = 30 * time.Second
	// Timeout for each heartbeat HTTP request.
	RequestTimeout = 5 * time.Second
)

// Heartbeat sends periodic heartbeats to the app server.
type Heartbeat struct {
	appURL       string
	machineID    string
	machineToken string
	cliVersion string
	client       *http.Client
	startTime    time.Time
}

// New creates a new heartbeat sender.
func New(appURL, machineID, machineToken, cliVersion string) *Heartbeat {
	return &Heartbeat{
		appURL:       appURL,
		machineID:    machineID,
		machineToken: machineToken,
		cliVersion:   cliVersion,
		client:       &http.Client{Timeout: RequestTimeout},
		startTime:    time.Now(),
	}
}

// Start begins sending heartbeats. Blocks until context is cancelled.
func (h *Heartbeat) Start(ctx context.Context) {
	// Send first heartbeat immediately
	h.send(ctx)

	ticker := time.NewTicker(Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("heartbeat: stopping")
			return
		case <-ticker.C:
			h.send(ctx)
		}
	}
}

func (h *Heartbeat) send(ctx context.Context) {
	payload := map[string]interface{}{
		"machineId":    h.machineID,
		"cliVersion": h.cliVersion,
		"uptime":       int(time.Since(h.startTime).Seconds()),
		"timestamp":    time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("heartbeat: marshal error: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/managed-machines/%s/heartbeat", h.appURL, h.machineID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("heartbeat: request error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// HMAC signature
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	message := "POST:/api/managed-machines/" + h.machineID + "/heartbeat:" + timestamp
	mac := hmac.New(sha256.New, []byte(h.machineToken))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set("X-Machine-Signature", signature)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("heartbeat: send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("heartbeat: server returned %d", resp.StatusCode)
	}
}
