package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeBody returns an io.ReadCloser from a string.
func fakeBody(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

func TestParseErrorResponse_StructuredEnvelope(t *testing.T) {
	t.Helper()
	resp := &http.Response{
		StatusCode: 422,
		Header:     http.Header{},
		Body:       fakeBody(`{"error":{"code":"validation_error","message":"name is required","retryable":false}}`),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.StatusCode != 422 {
		t.Fatalf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Code != "validation_error" {
		t.Fatalf("Code = %q, want %q", apiErr.Code, "validation_error")
	}
	if apiErr.Message != "name is required" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "name is required")
	}
	if apiErr.Retryable {
		t.Fatal("Retryable = true, want false")
	}
}

func TestParseErrorResponse_StructuredRetryable(t *testing.T) {
	resp := &http.Response{
		StatusCode: 503,
		Header:     http.Header{},
		Body:       fakeBody(`{"error":{"code":"overloaded","message":"try again later","retryable":true}}`),
	}
	apiErr := parseErrorResponse(resp)
	if !apiErr.Retryable {
		t.Fatal("Retryable = false, want true")
	}
	if apiErr.Message != "try again later" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "try again later")
	}
}

func TestParseErrorResponse_SimpleStringError(t *testing.T) {
	resp := &http.Response{
		StatusCode: 400,
		Header:     http.Header{},
		Body:       fakeBody(`{"error":"bad request buddy"}`),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.Message != "bad request buddy" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "bad request buddy")
	}
	if apiErr.Code != "" {
		t.Fatalf("Code = %q, want empty", apiErr.Code)
	}
}

func TestParseErrorResponse_HTMLBody(t *testing.T) {
	html := `<html><body><h1>502 Bad Gateway</h1></body></html>`
	resp := &http.Response{
		StatusCode: 502,
		Header:     http.Header{},
		Body:       fakeBody(html),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.StatusCode != 502 {
		t.Fatalf("StatusCode = %d, want 502", apiErr.StatusCode)
	}
	// HTML body should be used as-is (not valid JSON)
	if !strings.Contains(apiErr.Message, "502 Bad Gateway") {
		t.Fatalf("Message = %q, want it to contain HTML body", apiErr.Message)
	}
}

func TestParseErrorResponse_EmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Header:     http.Header{},
		Body:       fakeBody(""),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.Message != http.StatusText(500) {
		t.Fatalf("Message = %q, want %q", apiErr.Message, http.StatusText(500))
	}
}

func TestParseErrorResponse_InvalidJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: 400,
		Header:     http.Header{},
		Body:       fakeBody("{not-json"),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.Message != "{not-json" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "{not-json")
	}
}

func TestParseErrorResponse_LargeBodyTruncation(t *testing.T) {
	// Body larger than 8192 bytes should be truncated by LimitReader.
	// The resulting non-JSON message should be capped at 256 + "...".
	large := strings.Repeat("x", 10000)
	resp := &http.Response{
		StatusCode: 500,
		Header:     http.Header{},
		Body:       fakeBody(large),
	}
	apiErr := parseErrorResponse(resp)
	// The body read is limited to 8192, then truncated at 256 for display
	if len(apiErr.Message) > 260 {
		t.Fatalf("Message length = %d, want <= 260 (256 + '...')", len(apiErr.Message))
	}
	if !strings.HasSuffix(apiErr.Message, "...") {
		t.Fatalf("Message should end with '...' for truncated body")
	}
}

func TestParseErrorResponse_RetryAfterHeader(t *testing.T) {
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"60"}},
		Body:       fakeBody(`{"error":"rate limited"}`),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.RetryAfter != 60 {
		t.Fatalf("RetryAfter = %d, want 60", apiErr.RetryAfter)
	}
}

func TestParseErrorResponse_RetryAfterHeaderNonNumeric(t *testing.T) {
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"not-a-number"}},
		Body:       fakeBody(`{"error":"rate limited"}`),
	}
	apiErr := parseErrorResponse(resp)
	if apiErr.RetryAfter != 0 {
		t.Fatalf("RetryAfter = %d, want 0 for non-numeric header", apiErr.RetryAfter)
	}
}

func TestAPIError_Error(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		e := &APIError{StatusCode: 400, Code: "bad_request", Message: "field required"}
		if e.Error() != "field required" {
			t.Fatalf("Error() = %q, want %q", e.Error(), "field required")
		}
	})

	t.Run("without message", func(t *testing.T) {
		e := &APIError{StatusCode: 500, Code: "internal"}
		got := e.Error()
		want := "API error 500: internal"
		if got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})
}

func TestAPIError_ExitCode(t *testing.T) {
	tests := []struct {
		status   int
		wantExit int
	}{
		{400, ExitValidation},
		{401, ExitAuth},
		{403, ExitForbidden},
		{404, ExitNotFound},
		{422, ExitValidation},
		{429, ExitRateLimit},
		{500, ExitError},
		{502, ExitError},
		{503, ExitError},
		{418, ExitError}, // unknown status falls to default
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			e := &APIError{StatusCode: tt.status}
			if got := e.ExitCode(); got != tt.wantExit {
				t.Fatalf("ExitCode() for status %d = %d, want %d", tt.status, got, tt.wantExit)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	t.Run("true for 401", func(t *testing.T) {
		if !IsAuthError(&APIError{StatusCode: 401}) {
			t.Fatal("IsAuthError should return true for 401")
		}
	})
	t.Run("false for 403", func(t *testing.T) {
		if IsAuthError(&APIError{StatusCode: 403}) {
			t.Fatal("IsAuthError should return false for 403")
		}
	})
	t.Run("false for non-APIError", func(t *testing.T) {
		if IsAuthError(errors.New("plain error")) {
			t.Fatal("IsAuthError should return false for non-APIError")
		}
	})
	t.Run("false for nil", func(t *testing.T) {
		if IsAuthError(nil) {
			t.Fatal("IsAuthError should return false for nil")
		}
	})
}

func TestIsNotFoundError(t *testing.T) {
	t.Run("true for 404", func(t *testing.T) {
		if !IsNotFoundError(&APIError{StatusCode: 404}) {
			t.Fatal("IsNotFoundError should return true for 404")
		}
	})
	t.Run("false for 400", func(t *testing.T) {
		if IsNotFoundError(&APIError{StatusCode: 400}) {
			t.Fatal("IsNotFoundError should return false for 400")
		}
	})
	t.Run("false for non-APIError", func(t *testing.T) {
		if IsNotFoundError(errors.New("oops")) {
			t.Fatal("IsNotFoundError should return false for non-APIError")
		}
	})
}

func TestIsRateLimitError(t *testing.T) {
	t.Run("true for 429", func(t *testing.T) {
		if !IsRateLimitError(&APIError{StatusCode: 429}) {
			t.Fatal("IsRateLimitError should return true for 429")
		}
	})
	t.Run("false for 500", func(t *testing.T) {
		if IsRateLimitError(&APIError{StatusCode: 500}) {
			t.Fatal("IsRateLimitError should return false for 500")
		}
	})
	t.Run("false for non-APIError", func(t *testing.T) {
		if IsRateLimitError(errors.New("timeout")) {
			t.Fatal("IsRateLimitError should return false for non-APIError")
		}
	})
}
