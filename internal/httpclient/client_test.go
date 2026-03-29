package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVersionTransport_SetsUserAgent(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("1.2.0", 5*time.Second)
	resp, err := client.Get(server.URL + "/api/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	expected := "idapt-cli/1.2.0"
	if gotUserAgent != expected {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, expected)
	}
}

func TestVersionTransport_SetsApiVersion(t *testing.T) {
	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("X-Idapt-Version")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("1.0.0", 5*time.Second)
	resp, err := client.Get(server.URL + "/api/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if gotVersion != APIVersion {
		t.Errorf("X-Idapt-Version = %q, want %q", gotVersion, APIVersion)
	}
}

func TestVersionTransport_DoesNotOverwriteExistingVersion(t *testing.T) {
	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("X-Idapt-Version")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("1.0.0", 5*time.Second)
	req, err := http.NewRequest("GET", server.URL+"/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req.Header.Set("X-Idapt-Version", "2025-01-01")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if gotVersion != "2025-01-01" {
		t.Errorf("X-Idapt-Version = %q, want %q (should not overwrite)", gotVersion, "2025-01-01")
	}
}

func TestVersionTransport_CustomTimeout(t *testing.T) {
	client := New("1.0.0", 42*time.Second)
	if client.Timeout != 42*time.Second {
		t.Errorf("Timeout = %v, want %v", client.Timeout, 42*time.Second)
	}
}

func TestVersionTransport_DefaultAPIVersion(t *testing.T) {
	// APIVersion should have a valid default
	if APIVersion == "" {
		t.Error("APIVersion should not be empty")
	}
	// Should match YYYY-MM-DD format
	if len(APIVersion) != 10 || APIVersion[4] != '-' || APIVersion[7] != '-' {
		t.Errorf("APIVersion = %q, want YYYY-MM-DD format", APIVersion)
	}
}
