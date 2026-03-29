package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChatList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"chats": []map[string]interface{}{
						{"id": "c1", "title": "First Chat", "agentId": "a1", "selectedContextModel": "gpt-4", "createdAt": "2025-01-01"},
						{"id": "c2", "title": "Second Chat", "agentId": "a2", "selectedContextModel": "claude-3", "createdAt": "2025-01-02"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "list")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 2 {
			t.Fatalf("expected 2 chats, got %d", len(items))
		}
		if items[0]["title"] != "First Chat" {
			t.Errorf("expected title 'First Chat', got: %v", items[0]["title"])
		}
	})

	t.Run("with agent filter", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("agentId") != "agent-x" {
					t.Errorf("expected agentId filter agent-x, got: %v", r.URL.Query().Get("agentId"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"chats": []map[string]interface{}{
						{"id": "c3", "title": "Filtered", "agentId": "agent-x"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "list", "--agent", "agent-x")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 1 {
			t.Errorf("expected 1 chat, got %d", len(items))
		}
	})

	t.Run("with project filter", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("projectId") == "" {
					t.Error("expected projectId query param")
				}
				jsonResponse(w, 200, map[string]interface{}{
					"chats": []map[string]interface{}{},
				})
			},
		})
		_, _, err := runCmd(t, handler, "chat", "list", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"chats": []map[string]interface{}{},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "list")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 0 {
			t.Errorf("expected empty list, got %d items", len(items))
		}
	})
}

func TestChatCreate(t *testing.T) {
	t.Run("with model and agent", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["agentId"] != "agent-123" {
					t.Errorf("expected agentId agent-123, got: %v", body["agentId"])
				}
				if body["selectedContextModel"] != "claude-3" {
					t.Errorf("expected model claude-3, got: %v", body["selectedContextModel"])
				}
				if body["title"] != "My Chat" {
					t.Errorf("expected title 'My Chat', got: %v", body["title"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "new-chat", "title": "My Chat",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"chat", "create",
			"--agent", "agent-123",
			"--model", "claude-3",
			"--title", "My Chat",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != "new-chat" {
			t.Errorf("expected id new-chat, got: %v", parsed["id"])
		}
	})

	t.Run("json input", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["title"] != "JSON Chat" {
					t.Errorf("expected title from JSON, got: %v", body["title"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "jc-1", "title": "JSON Chat",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"chat", "create",
			"--json", `{"title":"JSON Chat","agentId":"a1"}`,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 400, "Agent is required")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "create")
		if err == nil {
			t.Fatal("expected error for 400")
		}
		if !strings.Contains(err.Error(), "Agent is required") {
			t.Errorf("expected validation message, got: %v", err)
		}
	})
}

func TestChatGet(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat/chat-abc": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"id": "chat-abc", "title": "Hello", "agentId": "a1",
					"selectedContextModel": "gpt-4", "createdAt": "2025-03-01",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "get", "chat-abc")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["title"] != "Hello" {
			t.Errorf("expected title Hello, got: %v", parsed["title"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat/bad-id": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 404, "Chat not found")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "get", "bad-id")
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})

	t.Run("missing argument", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "chat", "get")
		if err == nil {
			t.Fatal("expected error for missing arg")
		}
	})
}

func TestChatEdit(t *testing.T) {
	t.Run("update title", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/chat/ce-1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["title"] != "New Title" {
					t.Errorf("expected title 'New Title', got: %v", body["title"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": "ce-1", "title": "New Title",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "edit", "ce-1", "--title", "New Title")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["title"] != "New Title" {
			t.Errorf("expected New Title, got: %v", parsed["title"])
		}
	})

	t.Run("update model", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/chat/ce-2": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["selectedContextModel"] != "gpt-5" {
					t.Errorf("expected model gpt-5, got: %v", body["selectedContextModel"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": "ce-2", "title": "C",
				})
			},
		})
		_, _, err := runCmd(t, handler, "chat", "edit", "ce-2", "--model", "gpt-5")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("json input", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/chat/ce-3": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["title"] != "From JSON" {
					t.Errorf("expected title from json, got: %v", body["title"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id": "ce-3", "title": "From JSON",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"chat", "edit", "ce-3",
			"--json", `{"title":"From JSON"}`,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestChatDelete(t *testing.T) {
	t.Run("with confirm", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/chat/cd-1": func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
		})
		stdout, _, err := runCmdConfirm(t, handler, "chat", "delete", "cd-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "deleted") {
			t.Errorf("expected deletion message, got: %s", stdout)
		}
	})

	t.Run("without confirm aborts", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "chat", "delete", "cd-2")
		if err == nil {
			t.Fatal("expected abort error")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected aborted, got: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/chat/cd-3": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 403, "Forbidden")
			},
		})
		_, _, err := runCmdConfirm(t, handler, "chat", "delete", "cd-3")
		if err == nil {
			t.Fatal("expected error for 403")
		}
	})
}

func TestChatSend(t *testing.T) {
	t.Run("streams SSE text deltas", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat/cs-1/pending": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				content, ok := body["content"].(map[string]interface{})
				if !ok {
					t.Errorf("expected content object, got: %v", body["content"])
				} else if content["text"] != "Hello" {
					t.Errorf("expected text Hello, got: %v", content["text"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "pending-1"})
			},
			"GET /api/chat/cs-1/stream": func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "text/event-stream" {
					t.Errorf("expected Accept: text/event-stream, got: %s", r.Header.Get("Accept"))
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Fatal("expected ResponseWriter to implement Flusher")
				}
				// Send text-delta events
				fmt.Fprint(w, "event: text-delta\ndata: {\"text\":\"Hi \"}\n\n")
				flusher.Flush()
				fmt.Fprint(w, "event: text-delta\ndata: {\"text\":\"there!\"}\n\n")
				flusher.Flush()
				fmt.Fprint(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "send", "cs-1", "Hello")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Hi there!") {
			t.Errorf("expected streamed text, got: %q", stdout)
		}
	})

	t.Run("with model override", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat/cs-2/pending": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["selectedContextModel"] != "gpt-5" {
					t.Errorf("expected model gpt-5, got: %v", body["selectedContextModel"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "p2"})
			},
			"GET /api/chat/cs-2/stream": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				fmt.Fprint(w, "event: done\ndata: {}\n\n")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "send", "cs-2", "Hi", "--model", "gpt-5")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("no-stream mode", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat/cs-3/pending": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 201, map[string]interface{}{"id": "p3"})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "send", "cs-3", "Msg", "--no-stream")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Message sent") {
			t.Errorf("expected no-stream message, got: %s", stdout)
		}
	})

	t.Run("stream error event", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat/cs-4/pending": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 201, map[string]interface{}{"id": "p4"})
			},
			"GET /api/chat/cs-4/stream": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				fmt.Fprint(w, "event: error\ndata: Model overloaded\n\n")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "send", "cs-4", "Msg")
		if err == nil {
			t.Fatal("expected error from SSE error event")
		}
		if !strings.Contains(err.Error(), "stream error") {
			t.Errorf("expected stream error, got: %v", err)
		}
	})

	t.Run("pending message creation fails", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/chat/cs-5/pending": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 403, "Chat archived")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "send", "cs-5", "Msg")
		if err == nil {
			t.Fatal("expected error for pending creation failure")
		}
		if !strings.Contains(err.Error(), "Chat archived") {
			t.Errorf("expected error message, got: %v", err)
		}
	})
}

func TestChatMessages(t *testing.T) {
	t.Run("lists messages", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat/cm-1/messages": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"messages": []map[string]interface{}{
						{"id": "m1", "type": "user", "userText": "Hello", "assistantText": ""},
						{"id": "m2", "type": "agent", "userText": "", "assistantText": "Hi there!"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "messages", "cm-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(items))
		}
		if items[0]["type"] != "user" {
			t.Errorf("expected first message type user, got: %v", items[0]["type"])
		}
	})

	t.Run("missing chat ID", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "chat", "messages")
		if err == nil {
			t.Fatal("expected error for missing argument")
		}
	})
}

func TestChatExport(t *testing.T) {
	t.Run("streams markdown export", func(t *testing.T) {
		markdown := "# Chat Export\n\n**User:** Hello\n\n**Agent:** Hi there!"
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/chat/ex-1/export": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/markdown")
				w.WriteHeader(200)
				_, _ = io.WriteString(w, markdown)
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "export", "ex-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if stdout != markdown {
			t.Errorf("expected markdown content, got: %q", stdout)
		}
	})
}

func TestChatStop(t *testing.T) {
	t.Run("sends stop request", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/chat/st-1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["stopRequestedAt"] == nil || body["stopRequestedAt"] == "" {
					t.Error("expected stopRequestedAt to be set")
				}
				w.WriteHeader(200)
				jsonResponse(w, 200, map[string]interface{}{})
			},
		})
		stdout, _, err := runCmd(t, handler, "chat", "stop", "st-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Stop requested") {
			t.Errorf("expected stop message, got: %s", stdout)
		}
	})

	t.Run("server error on stop", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/chat/st-2": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 404, "Chat not found")
			},
		})
		_, _, err := runCmd(t, handler, "chat", "stop", "st-2")
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})
}
