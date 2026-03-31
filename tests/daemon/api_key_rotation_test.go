//go:build daemontest

package daemon

import (
	"net/http"
	"testing"
)

func TestAPIKeyRotationOldKeyWorks(t *testing.T) {
	key := "mk_testrotation1_" + t.Name()
	hash := hashAPIKey(key)
	registerAPIKeyHash(t, hash)

	resp := daemonRequest(t, "GET", "/",
		withBearer(key))

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with first API key, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAPIKeyRotationNewKeyWorks(t *testing.T) {
	// Register two keys to simulate rotation
	key1 := "mk_testrotation_old_" + t.Name()
	key2 := "mk_testrotation_new_" + t.Name()
	registerAPIKeyHash(t, hashAPIKey(key1))
	registerAPIKeyHash(t, hashAPIKey(key2))

	// Old key still works
	resp1 := daemonRequest(t, "GET", "/",
		withBearer(key1))
	if resp1.StatusCode != http.StatusOK {
		body := readBody(t, resp1)
		t.Fatalf("Expected 200 with old API key after rotation, got %d: %s", resp1.StatusCode, body)
	}
	resp1.Body.Close()

	// New key also works
	resp2 := daemonRequest(t, "GET", "/",
		withBearer(key2))
	if resp2.StatusCode != http.StatusOK {
		body := readBody(t, resp2)
		t.Fatalf("Expected 200 with new API key after rotation, got %d: %s", resp2.StatusCode, body)
	}
	resp2.Body.Close()
}

func TestAPIKeyRotationNewKeyIndependent(t *testing.T) {
	// Register only the new key — verify it works on its own
	key := "mk_testrotation_independent_" + t.Name()
	registerAPIKeyHash(t, hashAPIKey(key))

	resp := daemonRequest(t, "GET", "/",
		withBearer(key))
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("Expected 200 with independently registered API key, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// An unregistered key must NOT work
	unregistered := "mk_testrotation_unregistered_" + t.Name()
	resp2 := daemonRequest(t, "GET", "/",
		withBearer(unregistered))
	if resp2.StatusCode == http.StatusOK {
		resp2.Body.Close()
		t.Fatal("Expected rejection for unregistered API key, got 200")
	}
	resp2.Body.Close()
}
