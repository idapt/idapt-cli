//go:build daemontest

package daemon

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestConcurrentJWTRequests(t *testing.T) {
	// Get a valid JWT
	jwt := issueJWTViaApp(t, "/")

	const goroutines = 10
	var wg sync.WaitGroup
	errors := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			resp := daemonRequest(t, "GET", fmt.Sprintf("/test-concurrent-%d", idx),
				withCookie("idapt_machine_token", jwt))

			// 200 (backend) or 502 (no backend) are both acceptable — proves auth passed
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
				errors <- fmt.Sprintf("goroutine %d: expected 200 or 502, got %d", idx, resp.StatusCode)
			}
			resp.Body.Close()
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentMixedAuth(t *testing.T) {
	// Bearer tokens pass through to app — no daemon-side registration needed
	apiKey := fmt.Sprintf("mk_test_concurrent_%d", time.Now().UnixNano())

	// Get a valid JWT
	jwt := issueJWTViaApp(t, "/")

	const (
		jwtCount    = 5
		bearerCount = 5
		noAuthCount = 5
		total       = jwtCount + bearerCount + noAuthCount
	)

	type result struct {
		authType string
		index    int
		status   int
	}

	var wg sync.WaitGroup
	results := make(chan result, total)

	// Launch JWT-authenticated goroutines
	for i := 0; i < jwtCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := daemonRequest(t, "GET", fmt.Sprintf("/test-jwt-%d", idx),
				withCookie("idapt_machine_token", jwt))
			results <- result{authType: "jwt", index: idx, status: resp.StatusCode}
			resp.Body.Close()
		}(i)
	}

	// Launch Bearer API key goroutines
	for i := 0; i < bearerCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := daemonRequest(t, "GET", fmt.Sprintf("/test-bearer-%d", idx),
				withBearer(apiKey))
			results <- result{authType: "bearer", index: idx, status: resp.StatusCode}
			resp.Body.Close()
		}(i)
	}

	// Launch no-auth goroutines
	for i := 0; i < noAuthCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := daemonRequest(t, "GET", fmt.Sprintf("/test-noauth-%d", idx))
			results <- result{authType: "noauth", index: idx, status: resp.StatusCode}
			resp.Body.Close()
		}(i)
	}

	wg.Wait()
	close(results)

	for r := range results {
		switch r.authType {
		case "jwt":
			// JWT auth should pass (200 or 502)
			if r.status != http.StatusOK && r.status != http.StatusBadGateway {
				t.Errorf("JWT goroutine %d: expected 200 or 502, got %d", r.index, r.status)
			}
		case "bearer":
			// Bearer API key should pass (200 or 502)
			if r.status != http.StatusOK && r.status != http.StatusBadGateway {
				t.Errorf("Bearer goroutine %d: expected 200 or 502, got %d", r.index, r.status)
			}
		case "noauth":
			// No-auth should be rejected (401 or 302)
			if r.status != http.StatusUnauthorized && r.status != http.StatusFound {
				t.Errorf("NoAuth goroutine %d: expected 401 or 302, got %d", r.index, r.status)
			}
		}
	}
}

func TestConcurrentProxyConfigAndRequests(t *testing.T) {
	jwt := issueJWTViaApp(t, "/")

	var wg sync.WaitGroup

	// Goroutine 1: update proxy config multiple times
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			config := map[string]interface{}{
				"ports": []map[string]interface{}{
					{"port": 9001, "authMode": "authenticated"},
				},
			}
			hmacOpts := withHMAC("POST", "/api/proxy", machineToken)
			opts := append(hmacOpts, withJSON(config))
			resp := daemonRequest(t, "POST", "/api/proxy", opts...)
			resp.Body.Close()

			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Goroutines 2-6: send concurrent requests while config is being updated
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				resp := daemonRequest(t, "GET", fmt.Sprintf("/test-concurrent-config-%d-%d", idx, j),
					withCookie("idapt_machine_token", jwt))
				// Any valid HTTP response is acceptable — no panics is the goal
				resp.Body.Close()

				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	// If we reach here without panics, the test passes
}
