package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestModelList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/models": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"models": []map[string]interface{}{
					{"id": "gpt-4", "name": "GPT-4", "provider": "openai", "contextLength": 128000},
					{"id": "claude-3", "name": "Claude 3", "provider": "anthropic", "contextLength": 200000},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "model", "list", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 models, got %d", len(got))
	}
}

func TestModelListWithProvider(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/models": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("provider") != "anthropic" {
				t.Errorf("expected provider=anthropic, got %v", r.URL.Query().Get("provider"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"models": []map[string]interface{}{
					{"id": "claude-3", "name": "Claude 3", "provider": "anthropic"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "model", "list", "--provider", "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/models": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") != "claude" {
				t.Errorf("expected q=claude, got %v", r.URL.Query().Get("q"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"models": []map[string]interface{}{
					{"id": "claude-3", "name": "Claude 3", "provider": "anthropic"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "model", "search", "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelFavoriteList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/model-favorites": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"favorites": []map[string]interface{}{
					{"modelId": "gpt-4", "name": "GPT-4"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "model", "favorite", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelFavoriteAdd(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/model-favorites": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["modelId"] != "claude-3" {
				t.Errorf("expected modelId=claude-3, got %v", body["modelId"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "model", "favorite", "add", "claude-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "added") {
		t.Errorf("expected 'added' message, got: %s", stdout)
	}
}

func TestModelFavoriteRemove(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/model-favorites": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["modelId"] != "claude-3" {
				t.Errorf("expected modelId=claude-3, got %v", body["modelId"])
			}
			if body["action"] != "remove" {
				t.Errorf("expected action=remove, got %v", body["action"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "model", "favorite", "remove", "claude-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' message, got: %s", stdout)
	}
}
