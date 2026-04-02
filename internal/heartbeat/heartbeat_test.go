package heartbeat

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// capturedRequest stores fields from an incoming heartbeat request for
// inspection by tests.
type capturedRequest struct {
	Method    string
	URL       string
	Headers   http.Header
	Body      []byte
	BodyJSON  map[string]interface{}
}

// newTestServer creates an httptest.Server that captures every request it
// receives and responds with the given status code.
func newTestServer(t *testing.T, statusCode int) (*httptest.Server, *[]capturedRequest, *sync.Mutex) {
	t.Helper()
	var captured []capturedRequest
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var bodyJSON map[string]interface{}
		_ = json.Unmarshal(body, &bodyJSON)

		mu.Lock()
		captured = append(captured, capturedRequest{
			Method:   r.Method,
			URL:      r.URL.String(),
			Headers:  r.Header.Clone(),
			Body:     body,
			BodyJSON: bodyJSON,
		})
		mu.Unlock()

		w.WriteHeader(statusCode)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured, &mu
}

// waitForRequests polls until at least n requests have been captured or the
// timeout elapses.
func waitForRequests(mu *sync.Mutex, captured *[]capturedRequest, n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(*captured)
		mu.Unlock()
		if count >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHeartbeat_MessageFormat(t *testing.T) {
	// The HMAC message must follow the format:
	// POST:/api/managed-machines/{machineId}/heartbeat:{timestamp}
	// Token is a plain (non-hex) string — falls back to raw bytes.
	machineID := "test-machine-123"
	token := "secret-token"

	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, machineID, token, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat request")
	}
	cancel()

	mu.Lock()
	req := (*captured)[0]
	mu.Unlock()

	// Verify the HMAC can be reconstructed from the expected message format.
	// Token "secret-token" is not valid hex, so heartbeat falls back to []byte(token).
	ts := req.Headers.Get("X-Machine-Timestamp")
	if ts == "" {
		t.Fatal("X-Machine-Timestamp header missing")
	}

	expectedMessage := fmt.Sprintf("POST:/api/managed-machines/%s/heartbeat:%s", machineID, ts)
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(expectedMessage))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	gotSig := req.Headers.Get("X-Machine-Signature")
	if gotSig != expectedSig {
		t.Errorf("HMAC mismatch\n  message: %s\n  got:      %s\n  expected: %s", expectedMessage, gotSig, expectedSig)
	}
}

func TestHeartbeat_Headers(t *testing.T) {
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, "machine-1", "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat request")
	}
	cancel()

	mu.Lock()
	req := (*captured)[0]
	mu.Unlock()

	if req.Headers.Get("X-Machine-Signature") == "" {
		t.Error("missing X-Machine-Signature header")
	}
	if req.Headers.Get("X-Machine-Timestamp") == "" {
		t.Error("missing X-Machine-Timestamp header")
	}
	if req.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", req.Headers.Get("Content-Type"), "application/json")
	}
}

func TestHeartbeat_URL(t *testing.T) {
	machineID := "abc-def-456"
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, machineID, "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat request")
	}
	cancel()

	mu.Lock()
	req := (*captured)[0]
	mu.Unlock()

	expectedPath := fmt.Sprintf("/api/managed-machines/%s/heartbeat", machineID)
	if req.URL != expectedPath {
		t.Errorf("URL path = %q, want %q", req.URL, expectedPath)
	}
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", req.Method, http.MethodPost)
	}
}

func TestHeartbeat_Payload(t *testing.T) {
	machineID := "payload-machine"
	cliVersion := "2.3.4"
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, machineID, "token", cliVersion)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat request")
	}
	cancel()

	mu.Lock()
	req := (*captured)[0]
	mu.Unlock()

	body := req.BodyJSON
	if body == nil {
		t.Fatal("body JSON is nil")
	}

	// machineId
	if got, ok := body["machineId"]; !ok {
		t.Error("payload missing machineId")
	} else if got != machineID {
		t.Errorf("machineId = %v, want %v", got, machineID)
	}

	// cliVersion
	if got, ok := body["cliVersion"]; !ok {
		t.Error("payload missing cliVersion")
	} else if got != cliVersion {
		t.Errorf("cliVersion = %v, want %v", got, cliVersion)
	}

	// uptime (should be a number >= 0)
	if got, ok := body["uptime"]; !ok {
		t.Error("payload missing uptime")
	} else {
		uptimeFloat, isNum := got.(float64)
		if !isNum {
			t.Errorf("uptime is not a number: %T", got)
		} else if uptimeFloat < 0 {
			t.Errorf("uptime = %v, want >= 0", uptimeFloat)
		}
	}

	// timestamp (should be a recent Unix timestamp)
	if got, ok := body["timestamp"]; !ok {
		t.Error("payload missing timestamp")
	} else {
		tsFloat, isNum := got.(float64)
		if !isNum {
			t.Errorf("timestamp is not a number: %T", got)
		} else {
			ts := int64(tsFloat)
			now := time.Now().Unix()
			if ts < now-60 || ts > now+60 {
				t.Errorf("timestamp %d is not within 60s of now (%d)", ts, now)
			}
		}
	}
}

func TestHeartbeat_30SecondInterval(t *testing.T) {
	if Interval != 30*time.Second {
		t.Errorf("Interval = %v, want %v", Interval, 30*time.Second)
	}
}

func TestHeartbeat_SendsImmediately(t *testing.T) {
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, "immediate-machine", "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for first heartbeat")
	}
	elapsed := time.Since(start)
	cancel()

	// The first heartbeat should fire well before the 30s interval.
	// Allow up to 2 seconds for scheduling jitter.
	if elapsed > 2*time.Second {
		t.Errorf("first heartbeat took %v, expected it to fire immediately (< 2s)", elapsed)
	}
}

func TestHeartbeat_MockServer_Success(t *testing.T) {
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, "success-machine", "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat")
	}
	cancel()

	mu.Lock()
	count := len(*captured)
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 request, got %d", count)
	}
}

func TestHeartbeat_MockServer_ServerError(t *testing.T) {
	// Server returns 500 for every request. The heartbeat sender should NOT
	// crash and should continue sending subsequent heartbeats.
	var requestCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	hb := New(srv.URL, "error-machine", "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	// Wait for at least the immediate heartbeat
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := requestCount
		mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	mu.Lock()
	n := requestCount
	mu.Unlock()

	if n < 1 {
		t.Fatalf("expected at least 1 request despite server errors, got %d", n)
	}
}

func TestHeartbeat_MockServer_VerifyHMAC(t *testing.T) {
	machineID := "hmac-verify-machine"
	token := "my-secret-hmac-token"

	var hmacValid bool
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Machine-Signature")
		ts := r.Header.Get("X-Machine-Timestamp")

		message := fmt.Sprintf("POST:/api/managed-machines/%s/heartbeat:%s", machineID, ts)
		mac := hmac.New(sha256.New, []byte(token))
		mac.Write([]byte(message))
		expected := hex.EncodeToString(mac.Sum(nil))

		mu.Lock()
		hmacValid = (sig == expected)
		mu.Unlock()

		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	hb := New(srv.URL, machineID, token, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	// Wait for at least one request to be processed
	time.Sleep(500 * time.Millisecond)
	cancel()

	mu.Lock()
	valid := hmacValid
	mu.Unlock()

	if !valid {
		t.Error("HMAC signature sent by heartbeat did not match expected value computed by mock server")
	}
}

func TestHeartbeat_HexEncodedToken(t *testing.T) {
	// Verify the heartbeat correctly decodes a hex-encoded machineToken
	// to binary before using it as HMAC key — matches the TypeScript server
	// which uses the raw binary Buffer from deriveHeartbeatSecret().
	machineID := "hex-test-machine"
	// Simulate a real provisioned token: 32 random bytes hex-encoded to 64 chars
	rawKey := []byte("this-is-a-32-byte-key-for-hmac!")
	hexToken := hex.EncodeToString(rawKey) // what gets written to config.json

	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, machineID, hexToken, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hb.Start(ctx)

	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for heartbeat request")
	}
	cancel()

	mu.Lock()
	req := (*captured)[0]
	mu.Unlock()

	ts := req.Headers.Get("X-Machine-Timestamp")
	if ts == "" {
		t.Fatal("X-Machine-Timestamp header missing")
	}

	// The heartbeat should have used the DECODED binary as the HMAC key,
	// NOT the hex string itself. Verify by recomputing with the raw key.
	message := fmt.Sprintf("POST:/api/managed-machines/%s/heartbeat:%s", machineID, ts)
	mac := hmac.New(sha256.New, rawKey) // use raw binary, not hex string
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	gotSig := req.Headers.Get("X-Machine-Signature")
	if gotSig != expectedSig {
		t.Errorf("hex-decoded HMAC mismatch\n  got:      %s\n  expected: %s\n  (using decoded binary key, not hex string)", gotSig, expectedSig)
	}

	// Also verify it does NOT match the old (buggy) behavior of using hex string as raw bytes
	oldMac := hmac.New(sha256.New, []byte(hexToken))
	oldMac.Write([]byte(message))
	oldSig := hex.EncodeToString(oldMac.Sum(nil))
	if gotSig == oldSig {
		t.Error("heartbeat is using hex string as raw bytes (old bug) instead of decoding to binary")
	}
}

func TestHeartbeat_ContextCancellation(t *testing.T) {
	srv, captured, mu := newTestServer(t, 200)

	hb := New(srv.URL, "cancel-machine", "token", "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hb.Start(ctx)
		close(done)
	}()

	// Wait for the first heartbeat to confirm it's running
	if !waitForRequests(mu, captured, 1, 5*time.Second) {
		t.Fatal("timed out waiting for first heartbeat")
	}

	// Cancel and verify Start() returns promptly
	cancel()

	select {
	case <-done:
		// Start returned as expected
	case <-time.After(5 * time.Second):
		t.Fatal("Start() did not return after context cancellation within 5 seconds")
	}
}
