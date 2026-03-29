package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDaemonCommands_SkipFactory(t *testing.T) {
	t.Run("version_runs_without_error", func(t *testing.T) {
		// version is a daemon command — no factory setup or auth needed.
		// Note: versionCmd uses fmt.Printf (OS stdout), so we only verify
		// it runs without error; actual output goes to OS stdout.
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, h, "version")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGlobalFlag_Output_JSON(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{
					{"id": "p1", "name": "My Project", "slug": "myproj"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "project", "list", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify valid JSON
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %s", err, stdout)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 project, got %d", len(got))
	}
	if got[0]["slug"] != "myproj" {
		t.Errorf("expected slug=myproj, got %v", got[0]["slug"])
	}
}

func TestGlobalFlag_Output_Table(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{
					{"id": "p1", "name": "My Project", "slug": "myproj", "role": "owner"},
				},
			})
		},
	})
	// Create env with table format since the test env defaults to JSON
	env := newTestEnv(t, h)
	env.factory.Format = "table"
	stdout, _, err := runCmdWithEnv(t, env, "project", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Table output should contain headers
	if !strings.Contains(stdout, "ID") {
		t.Errorf("expected table header 'ID' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "myproj") {
		t.Errorf("expected 'myproj' in table output, got: %s", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "nonexistent-cmd")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestNoArgs(t *testing.T) {
	// Running with no args should show help text (no error since SilenceErrors)
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	cmd, stdout, _ := setupTestCmd(t, h)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
	output := stdout.String()
	if !strings.Contains(output, "idapt") {
		t.Errorf("expected help text containing 'idapt', got: %s", output)
	}
}

func TestGlobalFlag_Confirm(t *testing.T) {
	t.Run("confirm_skips_prompt", func(t *testing.T) {
		kbID := "11111111-1111-1111-1111-111111111111"
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"DELETE /api/kb/" + kbID: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
		})
		_, _, err := runCmd(t, h, "kb", "delete", kbID, "--project", "myproj", "--confirm")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no_confirm_aborts_delete", func(t *testing.T) {
		kbID := "11111111-1111-1111-1111-111111111111"
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
		})
		_, _, err := runCmd(t, h, "kb", "delete", kbID, "--project", "myproj")
		if err == nil {
			t.Fatal("expected abort error without --confirm")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected 'aborted' error, got: %v", err)
		}
	})
}

func TestGlobalFlag_Verbose(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "p1", "slug": "test"}},
			})
		},
	})
	// Verbose flag should not cause errors
	_, _, err := runCmd(t, h, "project", "list", "--verbose")
	if err != nil {
		t.Fatalf("unexpected error with --verbose: %v", err)
	}
}

func TestAPIErrorPropagation(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "internal server error",
			})
		},
	})
	_, _, err := runCmd(t, h, "project", "list")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
