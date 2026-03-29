package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSettingsGetAll(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/settings": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"theme": "dark", "defaultModel": "gpt-4", "slug": "alice",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "settings", "get", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["theme"] != "dark" {
		t.Errorf("expected theme=dark, got %v", got["theme"])
	}
	if got["defaultModel"] != "gpt-4" {
		t.Errorf("expected defaultModel=gpt-4, got %v", got["defaultModel"])
	}
}

func TestSettingsGetSingleKey(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/settings": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"theme": "dark", "defaultModel": "gpt-4", "slug": "alice",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "settings", "get", "theme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.TrimSpace(stdout), "dark") {
		t.Errorf("expected 'dark', got: %s", stdout)
	}
}

func TestSettingsGetUnknownKey(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/settings": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"theme": "dark",
			})
		},
	})
	_, _, err := runCmd(t, h, "settings", "get", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown setting")
	}
	if !strings.Contains(err.Error(), "unknown setting") {
		t.Errorf("expected 'unknown setting' error, got: %v", err)
	}
}

func TestSettingsSet(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/settings": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["theme"] != "light" {
				t.Errorf("expected theme=light, got %v", body["theme"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "settings", "set", "theme", "light")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "updated") {
		t.Errorf("expected 'updated' message, got: %s", stdout)
	}
}
