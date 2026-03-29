package resolve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/idapt/idapt-cli/internal/api"
)

func testClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := api.NewClient(api.ClientConfig{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	return c
}

// --- IsUUID ---

func TestIsUUID_Valid(t *testing.T) {
	tests := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		"00000000-0000-0000-0000-000000000000",
		"abcdef12-3456-7890-abcd-ef1234567890",
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			if !IsUUID(s) {
				t.Fatalf("IsUUID(%q) = false, want true", s)
			}
		})
	}
}

func TestIsUUID_Invalid(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"empty", ""},
		{"short", "550e8400-e29b-41d4-a716"},
		{"no-dashes", "550e8400e29b41d4a716446655440000"},
		{"extra-char", "550e8400-e29b-41d4-a716-4466554400001"},
		{"slug", "my-project"},
		{"with-prefix", "uuid:550e8400-e29b-41d4-a716-446655440000"},
		{"wrong-section-lengths", "550e84-00e29b-41d4-a716-446655440000"},
		{"spaces", "550e8400 e29b 41d4 a716 446655440000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsUUID(tt.val) {
				t.Fatalf("IsUUID(%q) = true, want false", tt.val)
			}
		})
	}
}

// --- ResolveProject ---

func TestResolveProject_BySlug(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("slug") != "my-project" {
			t.Errorf("unexpected slug query: %s", r.URL.Query().Get("slug"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"projects": []map[string]interface{}{
				{"id": "proj-uuid-1234", "slug": "my-project"},
			},
		})
	}

	client := testClient(t, handler)
	resolver := New(client)

	id, err := resolver.ResolveProject(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "proj-uuid-1234" {
		t.Fatalf("id = %q, want %q", id, "proj-uuid-1234")
	}
}

func TestResolveProject_ByUUID(t *testing.T) {
	// UUID should be returned directly without HTTP call
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}
	client := testClient(t, handler)
	resolver := New(client)

	uuid := "550e8400-e29b-41d4-a716-446655440000"
	id, err := resolver.ResolveProject(context.Background(), uuid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != uuid {
		t.Fatalf("id = %q, want %q", id, uuid)
	}
	if callCount != 0 {
		t.Fatalf("expected no HTTP calls for UUID, got %d", callCount)
	}
}

func TestResolveProject_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"projects": []map[string]interface{}{},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)

	_, err := resolver.ResolveProject(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found project")
	}
}

func TestResolveProject_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "internal server error",
		})
	}
	client := testClient(t, handler)
	resolver := New(client)

	_, err := resolver.ResolveProject(context.Background(), "my-project")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestResolveProject_EmptySlug(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {})
	resolver := New(client)

	_, err := resolver.ResolveProject(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty slug")
	}
}

// --- Resolve (agent) ---

func TestResolve_Agent_ByName(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "agent-uuid-999", "name": "my-agent"},
			},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)

	id, err := resolver.Resolve(context.Background(), "agent", "my-agent", "proj-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "agent-uuid-999" {
		t.Fatalf("id = %q, want %q", id, "agent-uuid-999")
	}
}

func TestResolve_Agent_ByUUID(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}
	client := testClient(t, handler)
	resolver := New(client)

	uuid := "550e8400-e29b-41d4-a716-446655440000"
	id, err := resolver.Resolve(context.Background(), "agent", uuid, "proj-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != uuid {
		t.Fatalf("id = %q, want %q", id, uuid)
	}
	if callCount != 0 {
		t.Fatalf("expected no HTTP calls for UUID, got %d", callCount)
	}
}

func TestResolve_Agent_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": []map[string]interface{}{},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)

	_, err := resolver.Resolve(context.Background(), "agent", "ghost", "proj-id")
	if err == nil {
		t.Fatal("expected error for not found agent")
	}
}

func TestResolve_Agent_Ambiguous(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "id-1", "name": "dup"},
				{"id": "id-2", "name": "dup"},
			},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)

	_, err := resolver.Resolve(context.Background(), "agent", "dup", "proj-id")
	if err == nil {
		t.Fatal("expected error for ambiguous agent")
	}
}

func TestResolve_CacheHit(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "cached-id", "name": "cached-agent"},
			},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)
	ctx := context.Background()

	// First call should hit the server
	id1, err := resolver.Resolve(ctx, "agent", "cached-agent", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("first call: expected 1 HTTP call, got %d", callCount)
	}

	// Second call should use cache
	id2, err := resolver.Resolve(ctx, "agent", "cached-agent", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("second call: expected 1 HTTP call (cached), got %d", callCount)
	}
	if id1 != id2 {
		t.Fatalf("cached result mismatch: %q vs %q", id1, id2)
	}
}

func TestResolve_CacheDifferentProjects(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		projID := r.URL.Query().Get("projectId")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "agent-in-" + projID, "name": "same-name"},
			},
		})
	}
	client := testClient(t, handler)
	resolver := New(client)
	ctx := context.Background()

	id1, err := resolver.Resolve(ctx, "agent", "same-name", "proj-a")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := resolver.Resolve(ctx, "agent", "same-name", "proj-b")
	if err != nil {
		t.Fatal(err)
	}

	if callCount != 2 {
		t.Fatalf("expected 2 HTTP calls for different projects, got %d", callCount)
	}
	if id1 == id2 {
		t.Fatalf("expected different IDs for different projects, both got %q", id1)
	}
}

func TestResolve_EmptyName(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {})
	resolver := New(client)

	_, err := resolver.Resolve(context.Background(), "agent", "", "proj-id")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestResolve_ProjectRequired(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {})
	resolver := New(client)

	_, err := resolver.Resolve(context.Background(), "agent", "my-agent", "")
	if err == nil {
		t.Fatal("expected error when project is empty")
	}
}
