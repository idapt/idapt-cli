package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	ExitOK         = 0
	ExitError      = 1
	ExitAuth       = 2
	ExitForbidden  = 3
	ExitNotFound   = 4
	ExitValidation = 5
	ExitRateLimit  = 10
)

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

func IsAuthError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 401
	}
	return false
}

func IsNotFoundError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 404
	}
	return false
}

func IsRateLimitError(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 429
	}
	return false
}

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

	var simple struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &simple) == nil && simple.Error != "" {
		apiErr.Message = simple.Error
		return apiErr
	}

	msg := string(body)
	if len(msg) > 256 {
		msg = msg[:256] + "..."
	}
	apiErr.Message = msg
	return apiErr
}
