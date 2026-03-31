//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

func TestAPIKeyValid(t *testing.T) {
	apiKey := "mk_testkey123"
	hash := hashAPIKey(apiKey)
	registerAPIKeyHash(t, hash)

	resp := daemonRequest(t, "GET", "/",
		withBearer(apiKey))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with valid API key, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAPIKeyInvalid(t *testing.T) {
	resp := daemonRequest(t, "GET", "/",
		withBearer("mk_wrongkey"))

	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("Expected 401 for invalid API key, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIKeyWrongPrefix(t *testing.T) {
	resp := daemonRequest(t, "GET", "/",
		withBearer("sk_something"))

	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("Expected 401 for wrong-prefix API key, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIKeyEmpty(t *testing.T) {
	resp := daemonRequest(t, "GET", "/",
		withHeader("Authorization", "Bearer "))

	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("Expected 401 for empty Bearer token, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
