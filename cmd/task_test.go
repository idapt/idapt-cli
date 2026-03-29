package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTaskList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/tasks/boards/board-1/items": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"items": []map[string]interface{}{
						{"id": "t1", "number": 1, "title": "Fix bug", "status": "open", "priority": "high"},
						{"id": "t2", "number": 2, "title": "Add tests", "status": "done", "priority": "low"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, h, "task", "list", "board-1", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got []map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(got))
		}
	})

	t.Run("with_status_filter", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/tasks/boards/board-1/items": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("status") != "open" {
					t.Errorf("expected status=open, got %v", r.URL.Query().Get("status"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"items": []map[string]interface{}{
						{"id": "t1", "number": 1, "title": "Fix bug", "status": "open"},
					},
				})
			},
		})
		_, _, err := runCmd(t, h, "task", "list", "board-1", "--status", "open")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestTaskCreate(t *testing.T) {
	t.Run("with_flags", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/tasks/boards/board-1/items": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["title"] != "New task" {
					t.Errorf("expected title='New task', got %v", body["title"])
				}
				if body["priority"] != "high" {
					t.Errorf("expected priority=high, got %v", body["priority"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "t3", "number": 3, "title": "New task"})
			},
		})
		stdout, _, err := runCmd(t, h, "task", "create", "board-1", "--title", "New task", "--priority", "high", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got["number"].(float64) != 3 {
			t.Errorf("expected number=3, got %v", got["number"])
		}
	})

	t.Run("json_input", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/tasks/boards/board-1/items": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["title"] != "JSON task" {
					t.Errorf("expected title='JSON task', got %v", body["title"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "t4", "number": 4, "title": "JSON task"})
			},
		})
		_, _, err := runCmd(t, h, "task", "create", "board-1", "--json", `{"title":"JSON task","status":"open"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestTaskGet(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/tasks/items/t1": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": "t1", "number": 1, "title": "Fix bug", "status": "open", "priority": "high",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "task", "get", "t1", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["title"] != "Fix bug" {
		t.Errorf("expected title='Fix bug', got %v", got["title"])
	}
}

func TestTaskEdit(t *testing.T) {
	t.Run("status_and_priority", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/tasks/items/t1": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["status"] != "done" {
					t.Errorf("expected status=done, got %v", body["status"])
				}
				if body["priority"] != "low" {
					t.Errorf("expected priority=low, got %v", body["priority"])
				}
				jsonResponse(w, 200, map[string]interface{}{"id": "t1", "title": "Fix bug", "status": "done"})
			},
		})
		_, _, err := runCmd(t, h, "task", "edit", "t1", "--status", "done", "--priority", "low")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestTaskDelete(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/tasks/items/t1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "task", "delete", "t1", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected deletion message")
	}
}

func TestTaskComment(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/tasks/items/t1/comments": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["content"] != "looks good" {
				t.Errorf("expected content='looks good', got %v", body["content"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "c1"})
		},
	})
	stdout, _, err := runCmd(t, h, "task", "comment", "t1", "looks good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected comment message")
	}
}

func TestTaskLabelList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/tasks/items/t1/labels": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"labels": []map[string]interface{}{
					{"id": "l1", "name": "bug", "color": "red"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "task", "label", "list", "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskLabelCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/tasks/items/t1/labels": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "enhancement" {
				t.Errorf("expected name=enhancement, got %v", body["name"])
			}
			if body["color"] != "blue" {
				t.Errorf("expected color=blue, got %v", body["color"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "l2"})
		},
	})
	_, _, err := runCmd(t, h, "task", "label", "create", "t1", "--name", "enhancement", "--color", "blue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskLabelEdit(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/tasks/items/t1/labels": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "bugfix" {
				t.Errorf("expected name=bugfix, got %v", body["name"])
			}
			w.WriteHeader(200)
		},
	})
	_, _, err := runCmd(t, h, "task", "label", "edit", "t1", "l1", "--name", "bugfix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskLabelDelete(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/tasks/items/t1/labels": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"labels": []map[string]interface{}{}})
		},
		"POST /api/tasks/items/t1/labels": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["action"] != "remove" {
				t.Errorf("expected action=remove, got %v", body["action"])
			}
			if body["labelId"] != "l1" {
				t.Errorf("expected labelId=l1, got %v", body["labelId"])
			}
			w.WriteHeader(200)
		},
	})
	_, _, err := runCmd(t, h, "task", "label", "delete", "t1", "l1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
