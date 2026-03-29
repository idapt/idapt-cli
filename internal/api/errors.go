// Package api provides an HTTP client for the idapt REST API.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Structured exit codes for CLI error reporting.
const (
	ExitOK         = 0
	ExitError      = 1
	ExitAuth       = 2
	ExitForbidden  = 3
	ExitNotFound   = 4
	ExitValidation = 5
	ExitRateLimit  = 10
)

// APIError represents a structured API error response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
	RetryAfter int // seconds, from Retry-After header
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Code)
}

// ExitCode maps the HTTP status to a structured CLI exit code.
func (e *APIError) ExitCode() int {
	switch {
	case e.StatusCode == 401:
		return ExitAuth
	case e.StatusCode == 403:
		return ExitForbidden
	case e.StatusCode == 404:
		return ExitNotFound
	case e.StatusCode == 422 || e.StatusCode == 400:
		return ExitValidation
	case e.StatusCode == 429:
		return ExitRateLimit
	default:
		return ExitError
	}
}

// IsAuthError returns true if the error is an authentication error.
func IsAuthError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 401
	}
	return false
}

// IsNotFoundError returns true if the error is a not-found error.
func IsNotFoundError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 404
	}
	return false
}

// IsRateLimitError returns true if the error is a rate-limit error.
func IsRateLimitError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 429
	}
	return false
}

// parseErrorResponse reads the JSON error body and returns an *APIError.
func parseErrorResponse(resp *http.Response) *APIError {
	apiErr := &APIError{StatusCode: resp.StatusCode}

	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			apiErr.RetryAfter = secs
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil || len(body) == 0 {
		apiErr.Message = http.StatusText(resp.StatusCode)
		return apiErr
	}

	// Try { "error": { "code": "...", "message": "..." } } format
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		apiErr.Code = envelope.Error.Code
		apiErr.Message = envelope.Error.Message
		apiErr.Retryable = envelope.Error.Retryable
		return apiErr
	}

	// Try { "error": "string message" } format
	var simple struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &simple) == nil && simple.Error != "" {
		apiErr.Message = simple.Error
		return apiErr
	}

	// Fallback: truncate raw body
	msg := string(body)
	if len(msg) > 256 {
		msg = msg[:256] + "..."
	}
	apiErr.Message = msg
	return apiErr
}
