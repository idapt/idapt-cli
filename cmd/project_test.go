package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestProjectList(t *testing.T) {
	t.Run("success returns projects", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{
						{"id": "p1", "name": "Alpha", "slug": "alpha", "role": "owner", "icon": "A"},
						{"id": "p2", "name": "Beta", "slug": "beta", "role": "editor", "icon": "B"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "list")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(items))
		}
		if items[0]["slug"] != "alpha" {
			t.Errorf("expected slug alpha, got: %v", items[0]["slug"])
		}
	})

	t.Run("empty list", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "list")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 0 {
			t.Errorf("expected empty list, got %d items", len(items))
		}
	})

	t.Run("server error propagated", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 500, "Internal Server Error")
			},
		})
		_, _, err := runCmd(t, handler, "project", "list")
		if err == nil {
			t.Fatal("expected error for server failure")
		}
	})
}

func TestProjectCreate(t *testing.T) {
	t.Run("with name and slug flags", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "My Project" {
					t.Errorf("expected name 'My Project', got: %v", body["name"])
				}
				if body["slug"] != "my-project" {
					t.Errorf("expected slug 'my-project', got: %v", body["slug"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "new-id", "name": "My Project", "slug": "my-project",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "create", "--name", "My Project", "--slug", "my-project")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != "new-id" {
			t.Errorf("expected id new-id, got: %v", parsed["id"])
		}
	})

	t.Run("all flags", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "Test" {
					t.Errorf("expected name Test, got: %v", body["name"])
				}
				if body["description"] != "A description" {
					t.Errorf("expected description, got: %v", body["description"])
				}
				if body["icon"] != "T" {
					t.Errorf("expected icon T, got: %v", body["icon"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "id-1", "name": "Test", "slug": "test",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"project", "create",
			"--name", "Test",
			"--slug", "test",
			"--description", "A description",
			"--icon", "T",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "Test" {
			t.Errorf("expected name Test, got: %v", parsed["name"])
		}
	})

	t.Run("json input", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "From JSON" {
					t.Errorf("expected name 'From JSON', got: %v", body["name"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "j1", "name": "From JSON", "slug": "from-json",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"project", "create",
			"--json", `{"name":"From JSON","slug":"from-json"}`,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["slug"] != "from-json" {
			t.Errorf("expected slug from-json, got: %v", parsed["slug"])
		}
	})

	t.Run("conflict 409", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 409, "Project slug already exists")
			},
		})
		_, _, err := runCmd(t, handler, "project", "create", "--name", "Dup", "--slug", "dup")
		if err == nil {
			t.Fatal("expected error for 409 conflict")
		}
		if !strings.Contains(err.Error(), "slug already exists") {
			t.Errorf("expected conflict message, got: %v", err)
		}
	})

	t.Run("rate limit 429", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "60")
				jsonErrorResponse(w, 429, "Rate limit exceeded")
			},
		})
		_, _, err := runCmd(t, handler, "project", "create", "--name", "Fast")
		if err == nil {
			t.Fatal("expected error for rate limit")
		}
		if !strings.Contains(err.Error(), "Rate limit") {
			t.Errorf("expected rate limit message, got: %v", err)
		}
	})
}

func TestProjectGet(t *testing.T) {
	t.Run("by UUID", func(t *testing.T) {
		projectID := "11111111-1111-1111-1111-111111111111"
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": projectID, "name": "My Proj", "slug": "my-proj",
					"description": "Desc", "icon": "P", "createdAt": "2025-01-01",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "get", projectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "My Proj" {
			t.Errorf("expected name, got: %v", parsed["name"])
		}
	})

	t.Run("by slug resolved", func(t *testing.T) {
		resolvedID := "22222222-2222-2222-2222-222222222222"
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				// Resolver calls GET /api/projects?slug=my-slug
				if r.URL.Query().Get("slug") == "my-slug" {
					jsonResponse(w, 200, map[string]interface{}{
						"projects": []map[string]interface{}{
							{"id": resolvedID, "slug": "my-slug"},
						},
					})
				}
			},
			"GET /api/projects/" + resolvedID: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": resolvedID, "name": "Resolved", "slug": "my-slug",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "get", "my-slug")
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
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{},
				})
			},
		})
		_, _, err := runCmd(t, handler, "project", "get", "nonexistent")
		if err == nil {
			t.Fatal("expected error for not found")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})

	t.Run("missing argument", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "project", "get")
		if err == nil {
			t.Fatal("expected error for missing arg")
		}
	})
}

func TestProjectEdit(t *testing.T) {
	projectID := "33333333-3333-3333-3333-333333333333"

	t.Run("single field update", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "Updated" {
					t.Errorf("expected name Updated, got: %v", body["name"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": projectID, "name": "Updated", "slug": "updated",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "edit", projectID, "--name", "Updated")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "Updated" {
			t.Errorf("expected Updated, got: %v", parsed["name"])
		}
	})

	t.Run("json input", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["description"] != "new desc" {
					t.Errorf("expected description 'new desc', got: %v", body["description"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": projectID, "name": "P", "slug": "p",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"project", "edit", projectID,
			"--json", `{"description":"new desc"}`,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("flag overrides json", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "FlagWins" {
					t.Errorf("expected name FlagWins (flag overrides json), got: %v", body["name"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": projectID, "name": "FlagWins", "slug": "fw",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"project", "edit", projectID,
			"--json", `{"name":"JsonName"}`,
			"--name", "FlagWins",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestProjectDelete(t *testing.T) {
	projectID := "44444444-4444-4444-4444-444444444444"

	t.Run("with confirm flag", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
		})
		stdout, _, err := runCmdConfirm(t, handler, "project", "delete", projectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "deleted") {
			t.Errorf("expected deletion message, got: %s", stdout)
		}
	})

	t.Run("without confirm aborts", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		// With non-TTY stdin and no --confirm, ConfirmAction returns false.
		_, _, err := runCmd(t, handler, "project", "delete", projectID)
		if err == nil {
			t.Fatal("expected error (aborted)")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected abort message, got: %v", err)
		}
	})

	t.Run("server 404", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/projects/" + projectID: func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 404, "Project not found")
			},
		})
		_, _, err := runCmdConfirm(t, handler, "project", "delete", projectID)
		if err == nil {
			t.Fatal("expected error for 404")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})
}

func TestProjectFork(t *testing.T) {
	projectID := "55555555-5555-5555-5555-555555555555"

	t.Run("success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/projects/" + projectID + "/fork": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 201, map[string]interface{}{
					"id": "forked-id", "name": "Fork of Alpha", "slug": "fork-of-alpha",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "project", "fork", projectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != "forked-id" {
			t.Errorf("expected forked id, got: %v", parsed["id"])
		}
	})
}

func TestProjectAlias(t *testing.T) {
	t.Run("proj alias works", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{},
				})
			},
		})
		_, _, err := runCmd(t, handler, "proj", "list")
		if err != nil {
			t.Fatalf("expected proj alias to work, got: %v", err)
		}
	})
}
