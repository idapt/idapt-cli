package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestStoreSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/explore/search": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") != "deploy" {
				t.Errorf("expected q=deploy, got %v", r.URL.Query().Get("q"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "r1", "type": "script", "name": "Deploy Script", "authorName": "alice"},
					{"id": "r2", "type": "agent", "name": "Deploy Agent", "authorName": "bob"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "store", "search", "deploy", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestStoreSkillSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/skill-store": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") != "code" {
				t.Errorf("expected q=code, got %v", r.URL.Query().Get("q"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "sk1", "name": "Code Review", "installCount": 42},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "store", "skill", "search", "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreSkillInstall(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/skill-store/sk1/install": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["projectId"] != "proj-1" {
				t.Errorf("expected projectId=proj-1, got %v", body["projectId"])
			}
			jsonResponse(w, 200, map[string]interface{}{"id": "installed-1"})
		},
	})
	stdout, _, err := runCmd(t, h, "store", "skill", "install", "sk1", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("expected 'installed' message, got: %s", stdout)
	}
}

func TestStoreKBSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/kb-store": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "kb-s1", "name": "API Docs KB", "installCount": 100},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "store", "kb", "search", "api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreKBInstall(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/kb-store/kb-s1/install": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"id": "installed-kb"})
		},
	})
	stdout, _, err := runCmd(t, h, "store", "kb", "install", "kb-s1", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("expected 'installed' message, got: %s", stdout)
	}
}

func TestStoreAgentSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/agent-store": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "ag-s1", "name": "Coding Assistant", "installCount": 200},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "store", "agent", "search", "coding")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreAgentInstall(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/agent-store/ag-s1/install": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"id": "installed-agent"})
		},
	})
	stdout, _, err := runCmd(t, h, "store", "agent", "install", "ag-s1", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("expected 'installed' message, got: %s", stdout)
	}
}

func TestStoreScriptSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/script-store": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "sc-s1", "name": "Setup Script", "installCount": 50},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "store", "script", "search", "setup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreScriptInstall(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/script-store/sc-s1/install": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"id": "installed-script"})
		},
	})
	stdout, _, err := runCmd(t, h, "store", "script", "install", "sc-s1", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("expected 'installed' message, got: %s", stdout)
	}
}
