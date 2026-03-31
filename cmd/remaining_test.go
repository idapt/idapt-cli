package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// --- exec ---

func TestExecCode(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/exec/code": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["language"] != "python" {
				t.Errorf("expected language=python, got %v", body["language"])
			}
			if body["code"] != "print('hi')" {
				t.Errorf("expected code=print('hi'), got %v", body["code"])
			}
			jsonResponse(w, 200, map[string]interface{}{"output": "hi\n", "exitCode": 0})
		},
	})
	stdout, _, err := runCmd(t, h, "exec", "code", "python", "--code", "print('hi')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "hi") {
		t.Errorf("expected 'hi' in output, got: %s", stdout)
	}
}

func TestExecCodeMissingCode(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "exec", "code", "python")
	if err == nil {
		t.Fatal("expected error when --code and --file are missing")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestExecBash(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/exec/bash": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["command"] != "echo hello" {
				t.Errorf("expected command='echo hello', got %v", body["command"])
			}
			jsonResponse(w, 200, map[string]interface{}{"output": "hello\n", "exitCode": 0})
		},
	})
	stdout, _, err := runCmd(t, h, "exec", "bash", "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", stdout)
	}
}

// --- web ---

func TestWebSearch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/web/search": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") != "golang" {
				t.Errorf("expected q=golang, got %v", r.URL.Query().Get("q"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"results": []map[string]interface{}{
					{"title": "Go Programming", "url": "https://go.dev", "snippet": "The Go language"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "web", "search", "golang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebFetch(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/web/fetch": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["url"] != "https://example.com" {
				t.Errorf("expected url=https://example.com, got %v", body["url"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"content": "Example Domain\nThis domain is for examples.", "title": "Example",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "web", "fetch", "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Example Domain") {
		t.Errorf("expected fetched content, got: %s", stdout)
	}
}

// --- profile ---

func TestProfileGet(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/auth/account": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": "u1", "email": "alice@example.com", "name": "Alice", "slug": "alice",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "profile", "get", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", got["email"])
	}
}

func TestProfileEdit(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/auth/account": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "Alice B" {
				t.Errorf("expected name='Alice B', got %v", body["name"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "profile", "edit", "--name", "Alice B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "updated") {
		t.Errorf("expected 'updated' message, got: %s", stdout)
	}
}

func TestProfileEditMissingFlags(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "profile", "edit")
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

// --- subscription ---

func TestSubscriptionStatus(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/subscription": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"plan": "pro", "status": "active", "balance": "$12.50",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "subscription", "status", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["plan"] != "pro" {
		t.Errorf("expected plan=pro, got %v", got["plan"])
	}
}

func TestSubscriptionUsage(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/usage/rate-limits": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"messagesUsed": 100, "messagesLimit": 1000,
				"storageUsed": "500MB", "storageLimit": "10GB",
			})
		},
	})
	_, _, err := runCmd(t, h, "subscription", "usage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- apikey ---

func TestApikeyList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/api-keys": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"keys": []map[string]interface{}{
					{"id": "k1", "name": "CI Key", "prefix": "idpt_", "permissions": "read"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "apikey", "list", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}
}

func TestApikeyCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/api-keys": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "CI Key" {
				t.Errorf("expected name='CI Key', got %v", body["name"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"id": "k2", "name": "CI Key", "key": "idpt_abc123",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "apikey", "create", "--name", "CI Key", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["key"] != "idpt_abc123" {
		t.Errorf("expected key=idpt_abc123, got %v", got["key"])
	}
}

func TestApikeyCreateMissingName(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "apikey", "create")
	if err == nil {
		t.Fatal("expected error when --name is missing")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestApikeyDelete(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/api-keys/k1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "apikey", "delete", "k1", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", stdout)
	}
}

// --- share ---

func TestShareList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/shared-with-me": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"shares": []map[string]interface{}{
					{"id": "sh1", "resourceType": "task-board", "resourceName": "Sprint Board", "access": "read", "sharedBy": "bob"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "share", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShareCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/tasks/boards/board-1/shares": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["email"] != "bob@example.com" {
				t.Errorf("expected email=bob@example.com, got %v", body["email"])
			}
			if body["access"] != "write" {
				t.Errorf("expected access=write, got %v", body["access"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "sh2"})
		},
	})
	stdout, _, err := runCmd(t, h, "share", "create", "task-board", "board-1", "--email", "bob@example.com", "--access", "write")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Shared") {
		t.Errorf("expected 'Shared' message, got: %s", stdout)
	}
}

func TestShareCreateMissingEmail(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "share", "create", "task-board", "board-1")
	if err == nil {
		t.Fatal("expected error when --email is missing")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestShareDelete(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/tasks/boards/board-1/shares/sh1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "share", "delete", "task-board", "board-1", "sh1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' message, got: %s", stdout)
	}
}

func TestShareCreateUnsupportedType(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
	_, _, err := runCmd(t, h, "share", "create", "invalid-type", "id1", "--email", "x@x.com")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' error, got: %v", err)
	}
}

// --- notification ---

func TestNotificationList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/notifications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"notifications": []map[string]interface{}{
					{"id": "n1", "type": "share", "message": "Alice shared Wiki", "read": false},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "notification", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationReadOne(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/notifications/n1": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["read"] != true {
				t.Errorf("expected read=true, got %v", body["read"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "notification", "read", "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "marked as read") {
		t.Errorf("expected 'marked as read' message, got: %s", stdout)
	}
}

func TestNotificationReadAll(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/notifications/read-all": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "notification", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "All notifications") {
		t.Errorf("expected 'All notifications' message, got: %s", stdout)
	}
}

// --- multi-agent ---

func TestMultiAgentChatCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/multi-agent/chat": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["parentChatId"] != "chat-1" {
				t.Errorf("expected parentChatId=chat-1, got %v", body["parentChatId"])
			}
			if body["agentId"] != "agent-1" {
				t.Errorf("expected agentId=agent-1, got %v", body["agentId"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"chatId": "child-1", "agentId": "agent-1",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "multi-agent", "chat", "create", "--parent-chat", "chat-1", "--agent", "agent-1", "--message", "Hello", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["chatId"] != "child-1" {
		t.Errorf("expected chatId=child-1, got %v", got["chatId"])
	}
}

func TestMultiAgentChatList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/multi-agent/chat/chat-1/children": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"chats": []map[string]interface{}{
					{"id": "child-1", "agentId": "agent-1", "status": "active"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "multi-agent", "chat", "list", "chat-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultiAgentMessageSendNoStream(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/multi-agent/chat/child-1/message": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			content := body["content"].(map[string]interface{})
			if content["text"] != "Do the task" {
				t.Errorf("expected text='Do the task', got %v", content["text"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "multi-agent", "message", "send", "child-1", "Do the task", "--no-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "sent") {
		t.Errorf("expected 'sent' message, got: %s", stdout)
	}
}

func TestMultiAgentMessageList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/multi-agent/chat/child-1/messages": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"messages": []map[string]interface{}{
					{"id": "msg-1", "type": "user", "userText": "Hello"},
					{"id": "msg-2", "type": "agent", "assistantText": "Hi there"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "multi-agent", "message", "list", "child-1", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got))
	}
}

func TestMultiAgentMessageGet(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/multi-agent/chat/child-1/messages/msg-1": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": "msg-1", "type": "user", "userText": "Hello",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "multi-agent", "message", "get", "child-1", "msg-1", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["userText"] != "Hello" {
		t.Errorf("expected userText=Hello, got %v", got["userText"])
	}
}
