package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestKBList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"GET /api/kb": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"knowledgeBases": []map[string]interface{}{
						{"id": "kb-1", "name": "wiki", "icon": "📖", "noteCount": 5, "createdAt": "2024-01-01"},
						{"id": "kb-2", "name": "docs", "icon": "📄", "noteCount": 3, "createdAt": "2024-01-02"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, h, "kb", "list", "--project", "myproj", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got []map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 KBs, got %d", len(got))
		}
	})
}

func TestKBCreate(t *testing.T) {
	t.Run("with_flags", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"POST /api/kb": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["name"] != "wiki" {
					t.Errorf("expected name=wiki, got %v", body["name"])
				}
				if body["icon"] != "📖" {
					t.Errorf("expected icon=📖, got %v", body["icon"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "kb-1", "name": "wiki"})
			},
		})
		stdout, _, err := runCmd(t, h, "kb", "create", "--project", "myproj", "--name", "wiki", "--icon", "📖", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got["id"] != "kb-1" {
			t.Errorf("expected id=kb-1, got %v", got["id"])
		}
	})

	t.Run("json_input", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"POST /api/kb": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["description"] != "my knowledge base" {
					t.Errorf("expected description='my knowledge base', got %v", body["description"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "kb-2", "name": "test"})
			},
		})
		_, _, err := runCmd(t, h, "kb", "create", "--project", "myproj", "--json", `{"name":"test","description":"my knowledge base"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestKBGet(t *testing.T) {
	t.Run("by_uuid", func(t *testing.T) {
		kbID := "11111111-1111-1111-1111-111111111111"
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"GET /api/kb/" + kbID: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": kbID, "name": "wiki", "noteCount": 10,
				})
			},
		})
		stdout, _, err := runCmd(t, h, "kb", "get", kbID, "--project", "myproj", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got map[string]interface{}
		json.Unmarshal([]byte(stdout), &got)
		if got["name"] != "wiki" {
			t.Errorf("expected name=wiki, got %v", got["name"])
		}
	})
}

func TestKBEdit(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"PATCH /api/kb/" + kbID: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "renamed" {
				t.Errorf("expected name=renamed, got %v", body["name"])
			}
			jsonResponse(w, 200, map[string]interface{}{"id": kbID, "name": "renamed"})
		},
	})
	_, _, err := runCmd(t, h, "kb", "edit", kbID, "--project", "myproj", "--name", "renamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBDelete(t *testing.T) {
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
	stdout, _, err := runCmd(t, h, "kb", "delete", kbID, "--project", "myproj", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected confirmation message")
	}
}

func TestKBAsk(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/kb/" + kbID + "/ask": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["question"] != "What is AI?" {
				t.Errorf("expected question='What is AI?', got %v", body["question"])
			}
			jsonResponse(w, 200, map[string]interface{}{"answer": "AI is...", "sources": []string{"note1"}})
		},
	})
	_, _, err := runCmd(t, h, "kb", "ask", kbID, "What is AI?", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBSearch(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/kb/" + kbID + "/search": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") != "neural" {
				t.Errorf("expected q=neural, got %v", r.URL.Query().Get("q"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"results": []map[string]interface{}{
					{"noteId": "n1", "title": "Neural Nets", "score": 0.95},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "kb", "search", kbID, "neural", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBIngest(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/kb/" + kbID + "/ingest": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["filePath"] != "/tmp/data.pdf" {
				t.Errorf("expected filePath='/tmp/data.pdf', got %v", body["filePath"])
			}
			jsonResponse(w, 200, map[string]interface{}{"status": "ok"})
		},
	})
	stdout, _, err := runCmd(t, h, "kb", "ingest", kbID, "/tmp/data.pdf", "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected ingestion message")
	}
}

func TestKBNoteList(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/kb/" + kbID + "/notes": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"notes": []map[string]interface{}{
					{"id": "note-1", "title": "Index", "updatedAt": "2024-01-01"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "kb", "note", "list", kbID, "--project", "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBNoteGet(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/kb/notes/note-1": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": "note-1", "title": "Index", "content": "# Index\nHello",
			})
		},
	})
	_, _, err := runCmd(t, h, "kb", "note", "get", "note-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBNoteCreate(t *testing.T) {
	kbID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/kb/" + kbID + "/notes": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["title"] != "New Note" {
				t.Errorf("expected title='New Note', got %v", body["title"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "note-2", "title": "New Note"})
		},
	})
	_, _, err := runCmd(t, h, "kb", "note", "create", kbID, "--project", "myproj", "--title", "New Note", "--content", "Hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBNoteEdit(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/kb/notes/note-1": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["title"] != "Updated" {
				t.Errorf("expected title=Updated, got %v", body["title"])
			}
			jsonResponse(w, 200, map[string]interface{}{"id": "note-1", "title": "Updated"})
		},
	})
	_, _, err := runCmd(t, h, "kb", "note", "edit", "note-1", "--title", "Updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKBNoteDelete(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/kb/notes/note-1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "kb", "note", "delete", "note-1", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected deletion message")
	}
}
