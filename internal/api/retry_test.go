package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_429ThenSuccess(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	resp, err := c.Do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("got %d requests, want 2", count)
	}
}

func TestRetry_502ThenSuccess(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n == 1 {
			w.WriteHeader(502)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	resp, err := c.Do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("got %d requests, want 2", count)
	}
}

func TestRetry_400NotRetried(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	_, err := c.Do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("got %d requests, want 1 (no retry for 400)", count)
	}
}

func TestRetry_POSTNotRetried(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(429)
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	_, err := c.Do(context.Background(), "POST", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 429 POST")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("got %d requests, want 1 (POST not retried)", count)
	}
}

func TestRetry_ContextCancelledDuringWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetryWait_Calculation(t *testing.T) {
	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{1, 1 * time.Second, 1 * time.Second},
		{2, 2 * time.Second, 2 * time.Second},
		{3, 4 * time.Second, 4 * time.Second},
	}
	for _, tt := range tests {
		got := retryWait(tt.attempt)
		if got < tt.min || got > tt.max {
			t.Errorf("retryWait(%d) = %v, want between %v and %v", tt.attempt, got, tt.min, tt.max)
		}
	}
}

func TestRetryWait_CappedAtMax(t *testing.T) {
	got := retryWait(100)
	if got > maxRetryWait {
		t.Errorf("retryWait(100) = %v, want <= %v", got, maxRetryWait)
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, false},
		{502, true},
		{503, true},
		{504, true},
	}
	for _, tt := range tests {
		if got := shouldRetry(tt.code); got != tt.want {
			t.Errorf("shouldRetry(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
