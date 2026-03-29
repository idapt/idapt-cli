package api

import (
	"context"
	"net/http"
	"time"
)

const (
	maxRetries       = 3
	defaultRetryWait = 1 * time.Second
	maxRetryWait     = 30 * time.Second
)

// doWithRetry wraps an HTTP request with retry logic for transient errors.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Only retry idempotent methods
	if req.Method != "GET" && req.Method != "PUT" && req.Method != "DELETE" && req.Method != "HEAD" {
		return c.http.Do(req)
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			wait := retryWait(attempt)
			if lastResp != nil && lastResp.Header.Get("Retry-After") != "" {
				if parsed, err := time.ParseDuration(lastResp.Header.Get("Retry-After") + "s"); err == nil && parsed > 0 {
					wait = parsed
				}
			}
			if wait > maxRetryWait {
				wait = maxRetryWait
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			lastResp = nil
			continue
		}

		if !shouldRetry(resp.StatusCode) {
			return resp, nil
		}

		lastResp = resp
		lastErr = nil
		resp.Body.Close()
	}

	if lastErr != nil {
		return nil, lastErr
	}
	// All retries exhausted with retryable status
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

func shouldRetry(statusCode int) bool {
	return statusCode == 429 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

func retryWait(attempt int) time.Duration {
	wait := defaultRetryWait
	for i := 1; i < attempt; i++ {
		wait *= 2
	}
	if wait > maxRetryWait {
		wait = maxRetryWait
	}
	return wait
}
