package fuse

import (
	"net/http"
	"syscall"
	"testing"

	"github.com/idapt/idapt-cli/internal/api"
)

func TestMapAPIError_NotFound(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusNotFound})
	if err != syscall.ENOENT {
		t.Errorf("expected ENOENT, got %v", err)
	}
}

func TestMapAPIError_Forbidden(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusForbidden})
	if err != syscall.EACCES {
		t.Errorf("expected EACCES, got %v", err)
	}
}

func TestMapAPIError_Conflict(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusConflict})
	if err != syscall.ESTALE {
		t.Errorf("expected ESTALE, got %v", err)
	}
}

func TestMapAPIError_TooManyRequests(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusTooManyRequests})
	if err != syscall.EAGAIN {
		t.Errorf("expected EAGAIN, got %v", err)
	}
}

func TestMapAPIError_Unauthorized(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusUnauthorized})
	if err != syscall.EACCES {
		t.Errorf("expected EACCES, got %v", err)
	}
}

func TestMapAPIError_ServerError(t *testing.T) {
	err := mapAPIError(&api.APIError{StatusCode: http.StatusInternalServerError})
	if err != syscall.EIO {
		t.Errorf("expected EIO, got %v", err)
	}
}

func TestMapAPIError_Nil(t *testing.T) {
	err := mapAPIError(nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestMimeFromExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{"txt", "text/plain"},
		{"md", "text/markdown"},
		{"json", "application/json"},
		{"js", "application/javascript"},
		{"ts", "application/typescript"},
		{"html", "text/html"},
		{"css", "text/css"},
		{"png", "image/png"},
		{"jpg", "image/jpeg"},
		{"pdf", "application/pdf"},
		{"go", "text/x-go"},
		{"unknown", "application/x-unknown"},
	}

	for _, tt := range tests {
		got := mimeFromExt(tt.ext)
		if got != tt.expected {
			t.Errorf("mimeFromExt(%q) = %q, want %q", tt.ext, got, tt.expected)
		}
	}
}

func TestGetExtension(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"file.txt", "txt"},
		{"archive.tar.gz", "gz"},
		{"Makefile", ""},
		{".gitignore", "gitignore"},
		{"noext", ""},
	}

	for _, tt := range tests {
		got := getExtension(tt.name)
		if got != tt.expected {
			t.Errorf("getExtension(%q) = %q, want %q", tt.name, got, tt.expected)
		}
	}
}
