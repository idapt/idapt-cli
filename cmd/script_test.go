package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestScriptList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/scripts": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"scripts": []map[string]interface{}{
					{"id": "s1", "name": "deploy", "language": "bash", "createdAt": "2024-01-01"},
					{"id": "s2", "name": "test", "language": "python", "createdAt": "2024-01-02"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "script", "list", "--project", "myproj", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(got))
	}
}

func TestScriptCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/scripts": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "deploy" {
				t.Errorf("expected name=deploy, got %v", body["name"])
			}
			if body["language"] != "bash" {
				t.Errorf("expected language=bash, got %v", body["language"])
			}
			if body["content"] != "#!/bin/bash\necho hello" {
				t.Errorf("unexpected content: %v", body["content"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "s3", "name": "deploy"})
		},
	})
	_, _, err := runCmd(t, h, "script", "create", "--project", "myproj", "--name", "deploy", "--language", "bash", "--content", "#!/bin/bash\necho hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptGet(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/scripts/" + scriptID: func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": scriptID, "name": "deploy", "language": "bash", "content": "echo hi",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "script", "get", scriptID, "--project", "myproj", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["language"] != "bash" {
		t.Errorf("expected language=bash, got %v", got["language"])
	}
}

func TestScriptEdit(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"PATCH /api/scripts/" + scriptID: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "deploy-v2" {
				t.Errorf("expected name=deploy-v2, got %v", body["name"])
			}
			jsonResponse(w, 200, map[string]interface{}{"id": scriptID, "name": "deploy-v2"})
		},
	})
	_, _, err := runCmd(t, h, "script", "edit", scriptID, "--project", "myproj", "--name", "deploy-v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptDelete(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"DELETE /api/scripts/" + scriptID: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "script", "delete", scriptID, "--project", "myproj", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", stdout)
	}
}

func TestScriptRun(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/scripts/" + scriptID + "/execute": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			jsonResponse(w, 200, map[string]interface{}{
				"id": "exec-1", "status": "completed", "output": "hello world",
			})
		},
	})
	_, _, err := runCmd(t, h, "script", "run", scriptID, "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptRunWithMachine(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	machineID := "22222222-2222-2222-2222-222222222222"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/scripts/" + scriptID + "/execute": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["machineId"] != machineID {
				t.Errorf("expected machineId=%s, got %v", machineID, body["machineId"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"id": "exec-2", "status": "running",
			})
		},
	})
	_, _, err := runCmd(t, h, "script", "run", scriptID, "--project", "myproj", "--machine", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptRuns(t *testing.T) {
	scriptID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/scripts/" + scriptID + "/runs": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"runs": []map[string]interface{}{
					{"id": "exec-1", "status": "completed", "exitCode": 0},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "script", "runs", scriptID, "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptRunOutput(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/scripts/runs/exec-1": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": "exec-1", "status": "completed", "output": "script output here", "exitCode": 0,
			})
		},
	})
	stdout, _, err := runCmd(t, h, "script", "run-output", "exec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "script output here") {
		t.Errorf("expected script output, got: %s", stdout)
	}
}

func TestScriptInterrupt(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/scripts/runs/exec-1/interrupt": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["action"] != "interrupt" {
				t.Errorf("expected action=interrupt, got %v", body["action"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "script", "interrupt", "exec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Interrupt") {
		t.Errorf("expected 'Interrupt' message, got: %s", stdout)
	}
}
