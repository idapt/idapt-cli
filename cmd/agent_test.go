package cmd

import (
	"net/http"
	"strings"
	"testing"
)

const testProjectID = "00000000-0000-0000-0000-000000000001"

func TestAgentList(t *testing.T) {
	t.Run("success returns agents", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("projectId") == "" {
					t.Error("expected projectId query param")
				}
				jsonResponse(w, 200, map[string]interface{}{
					"agents": []map[string]interface{}{
						{"id": "a1", "name": "Coder", "icon": "C", "modelId": "gpt-4", "createdAt": "2025-01-01"},
						{"id": "a2", "name": "Writer", "icon": "W", "modelId": "claude-3", "createdAt": "2025-01-02"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "agent", "list", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 2 {
			t.Fatalf("expected 2 agents, got %d", len(items))
		}
		if items[0]["name"] != "Coder" {
			t.Errorf("expected first agent Coder, got: %v", items[0]["name"])
		}
	})

	t.Run("empty list", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"agents": []map[string]interface{}{},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "agent", "list", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 0 {
			t.Errorf("expected empty list, got %d items", len(items))
		}
	})

	t.Run("uses default project from config", func(t *testing.T) {
		// The test env sets DefaultProject = "proj-uuid-1234" in the config.
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("projectId") != "00000000-0000-0000-0000-000000000001" {
					t.Errorf("expected default project ID, got: %v", r.URL.Query().Get("projectId"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"agents": []map[string]interface{}{},
				})
			},
		})
		// No --project flag; should use config default.
		_, _, err := runCmd(t, handler, "agent", "list")
		if err != nil {
			t.Fatalf("expected no error with default project, got: %v", err)
		}
	})
}

func TestAgentCreate(t *testing.T) {
	t.Run("with all flags", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/agents": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "MyAgent" {
					t.Errorf("expected name MyAgent, got: %v", body["name"])
				}
				if body["icon"] != "A" {
					t.Errorf("expected icon A, got: %v", body["icon"])
				}
				if body["systemPrompt"] != "You are helpful" {
					t.Errorf("expected systemPrompt, got: %v", body["systemPrompt"])
				}
				if body["permissionPreset"] != "full" {
					t.Errorf("expected permissionPreset full, got: %v", body["permissionPreset"])
				}
				if body["projectId"] == nil || body["projectId"] == "" {
					t.Error("expected projectId to be set")
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "agent-new", "name": "MyAgent", "icon": "A",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"agent", "create",
			"--project", testProjectID,
			"--name", "MyAgent",
			"--icon", "A",
			"--system-prompt", "You are helpful",
			"--permission-preset", "full",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != "agent-new" {
			t.Errorf("expected id agent-new, got: %v", parsed["id"])
		}
	})

	t.Run("json input with flag override", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/agents": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				// --name flag should override the JSON name
				if body["name"] != "OverrideName" {
					t.Errorf("expected flag to override json name, got: %v", body["name"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "a-j", "name": "OverrideName", "icon": "J",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"agent", "create",
			"--project", testProjectID,
			"--json", `{"name":"JsonName","icon":"J"}`,
			"--name", "OverrideName",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/agents": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 422, "Name is required")
			},
		})
		_, _, err := runCmd(t, handler,
			"agent", "create",
			"--project", testProjectID,
		)
		if err == nil {
			t.Fatal("expected error for 422")
		}
		if !strings.Contains(err.Error(), "Name is required") {
			t.Errorf("expected validation message, got: %v", err)
		}
	})
}

func TestAgentGet(t *testing.T) {
	agentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	t.Run("by UUID", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents/" + agentID: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": agentID, "name": "TestAgent", "icon": "T",
					"modelId": "gpt-4", "systemPrompt": "Be nice", "createdAt": "2025-01-01",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "agent", "get", agentID, "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "TestAgent" {
			t.Errorf("expected name TestAgent, got: %v", parsed["name"])
		}
	})

	t.Run("by name resolved", func(t *testing.T) {
		resolvedID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("name") == "MyBot" {
					jsonResponse(w, 200, map[string]interface{}{
						"agents": []map[string]interface{}{
							{"id": resolvedID, "name": "MyBot"},
						},
					})
					return
				}
				jsonResponse(w, 200, map[string]interface{}{"agents": []map[string]interface{}{}})
			},
			"GET /api/agents/" + resolvedID: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": resolvedID, "name": "MyBot", "icon": "B",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "agent", "get", "MyBot", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != resolvedID {
			t.Errorf("expected resolved ID, got: %v", parsed["id"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/agents": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"agents": []map[string]interface{}{},
				})
			},
		})
		_, _, err := runCmd(t, handler, "agent", "get", "NoSuchAgent", "--project", testProjectID)
		if err == nil {
			t.Fatal("expected error for not found")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})
}

func TestAgentEdit(t *testing.T) {
	agentID := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	t.Run("single field", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/agents/" + agentID: func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "Renamed" {
					t.Errorf("expected name Renamed, got: %v", body["name"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": agentID, "name": "Renamed",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"agent", "edit", agentID,
			"--project", testProjectID,
			"--name", "Renamed",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "Renamed" {
			t.Errorf("expected Renamed, got: %v", parsed["name"])
		}
	})

	t.Run("json input", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/agents/" + agentID: func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["systemPrompt"] != "New prompt" {
					t.Errorf("expected systemPrompt from json, got: %v", body["systemPrompt"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": agentID, "name": "A",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"agent", "edit", agentID,
			"--project", testProjectID,
			"--json", `{"systemPrompt":"New prompt"}`,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestAgentDelete(t *testing.T) {
	agentID := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	t.Run("with confirm", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/agents/" + agentID: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
		})
		stdout, _, err := runCmdConfirm(t, handler,
			"agent", "delete", agentID,
			"--project", testProjectID,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "deleted") {
			t.Errorf("expected deletion message, got: %s", stdout)
		}
	})

	t.Run("without confirm aborts", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler,
			"agent", "delete", agentID,
			"--project", testProjectID,
		)
		if err == nil {
			t.Fatal("expected error (aborted)")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected abort, got: %v", err)
		}
	})

	t.Run("not found on server", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/agents/" + agentID: func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 404, "Agent not found")
			},
		})
		_, _, err := runCmdConfirm(t, handler,
			"agent", "delete", agentID,
			"--project", testProjectID,
		)
		if err == nil {
			t.Fatal("expected error for 404")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})
}
