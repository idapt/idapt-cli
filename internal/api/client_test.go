package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestClient creates a Client pointing at the given httptest.Server.
// It bypasses NewClient to avoid the httpclient transport and directly
// uses http.DefaultClient so httptest works without TLS issues.
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return &Client{
		baseURL: u,
		apiKey:  "test-key-123",
		http:    &http.Client{Timeout: 5 * time.Second},
		errOut:  io.Discard,
	}
}

func TestNewClient(t *testing.T) {
	t.Run("default base URL", func(t *testing.T) {
		c, err := NewClient(ClientConfig{APIKey: "k"})
		if err != nil {
			t.Fatalf("NewClient error: %v", err)
		}
		if c.baseURL.String() != "https://idapt.ai" {
			t.Fatalf("baseURL = %q, want %q", c.baseURL.String(), "https://idapt.ai")
		}
	})

	t.Run("custom base URL with trailing slash", func(t *testing.T) {
		c, err := NewClient(ClientConfig{BaseURL: "https://custom.example.com/", APIKey: "k"})
		if err != nil {
			t.Fatalf("NewClient error: %v", err)
		}
		if c.baseURL.String() != "https://custom.example.com" {
			t.Fatalf("baseURL = %q, want trailing slash stripped", c.baseURL.String())
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, err := NewClient(ClientConfig{BaseURL: "://bad"})
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})
}

func TestDo_AuthHeaderInjection(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-api-key")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if gotHeader != "test-key-123" {
		t.Fatalf("x-api-key = %q, want %q", gotHeader, "test-key-123")
	}
}

func TestDo_NoAuthHeaderWhenEmpty(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-api-key")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := &Client{
		baseURL: u,
		apiKey:  "",
		http:    &http.Client{Timeout: 5 * time.Second},
		errOut:  io.Discard,
	}
	_, err := c.Do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if gotHeader != "" {
		t.Fatalf("x-api-key should be empty when apiKey is empty, got %q", gotHeader)
	}
}

func TestDo_URLConstruction(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Do(context.Background(), "GET", "/api/v1/agents", nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if gotPath != "/api/v1/agents" {
		t.Fatalf("path = %q, want %q", gotPath, "/api/v1/agents")
	}
}

func TestDo_WithQuery(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	q := url.Values{"search": []string{"hello"}, "page": []string{"2"}}
	_, err := c.Do(context.Background(), "GET", "/items", nil, WithQuery(q))
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if gotQuery.Get("search") != "hello" {
		t.Fatalf("search = %q, want %q", gotQuery.Get("search"), "hello")
	}
	if gotQuery.Get("page") != "2" {
		t.Fatalf("page = %q, want %q", gotQuery.Get("page"), "2")
	}
}

func TestDo_WithHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Do(context.Background(), "GET", "/test", nil, WithHeader("X-Custom", "value123"))
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if gotHeader != "value123" {
		t.Fatalf("X-Custom = %q, want %q", gotHeader, "value123")
	}
}

func TestDo_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"resource missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Do(context.Background(), "GET", "/missing", nil)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "resource missing" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "resource missing")
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Do(ctx, "GET", "/slow", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDoJSON_MarshalAndDecode(t *testing.T) {
	type reqPayload struct {
		Name string `json:"name"`
	}
	type respPayload struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var req reqPayload
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respPayload{ID: 42, Name: req.Name})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	var resp respPayload
	err := c.DoJSON(context.Background(), "POST", "/create", reqPayload{Name: "test"}, &resp)
	if err != nil {
		t.Fatalf("DoJSON error: %v", err)
	}
	if resp.ID != 42 {
		t.Fatalf("resp.ID = %d, want 42", resp.ID)
	}
	if resp.Name != "test" {
		t.Fatalf("resp.Name = %q, want %q", resp.Name, "test")
	}
}

func TestDoJSON_UnknownFieldsIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"name":"test","extra_field":"ignored","nested":{"a":1}}`))
	}))
	defer srv.Close()

	type resp struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	c := newTestClient(t, srv.URL)
	var got resp
	err := c.DoJSON(context.Background(), "GET", "/data", nil, &got)
	if err != nil {
		t.Fatalf("DoJSON error: %v", err)
	}
	if got.ID != 1 || got.Name != "test" {
		t.Fatalf("got = %+v, want {ID:1, Name:test}", got)
	}
}

func TestDoJSON_NilBody(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	err := c.DoJSON(context.Background(), "DELETE", "/item/1", nil, nil)
	if err != nil {
		t.Fatalf("DoJSON error: %v", err)
	}
	// No Content-Type header should be set when body is nil
	if gotContentType != "" {
		t.Fatalf("Content-Type = %q, want empty for nil body", gotContentType)
	}
}

func TestDoJSON_204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	type resp struct {
		ID int `json:"id"`
	}
	c := newTestClient(t, srv.URL)
	var got resp
	// Should not attempt to decode body on 204
	err := c.DoJSON(context.Background(), "DELETE", "/item/1", nil, &got)
	if err != nil {
		t.Fatalf("DoJSON error: %v", err)
	}
	if got.ID != 0 {
		t.Fatalf("resp should be zero value on 204, got %+v", got)
	}
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "10")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":["a","b"]}`))
	}))
	defer srv.Close()

	type resp struct {
		Items []string `json:"items"`
	}
	c := newTestClient(t, srv.URL)
	var got resp
	err := c.Get(context.Background(), "/list", url.Values{"limit": []string{"10"}}, &got)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items count = %d, want 2", len(got.Items))
	}
}

func TestGet_NilQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want empty", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	var got map[string]interface{}
	err := c.Get(context.Background(), "/test", nil, &got)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
}

func TestPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello") {
			t.Errorf("body = %q, want it to contain 'hello'", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	type resp struct {
		OK bool `json:"ok"`
	}
	c := newTestClient(t, srv.URL)
	var got resp
	err := c.Post(context.Background(), "/create", map[string]string{"msg": "hello"}, &got)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if !got.OK {
		t.Fatal("ok = false, want true")
	}
}

func TestPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer srv.Close()

	type resp struct {
		Updated bool `json:"updated"`
	}
	c := newTestClient(t, srv.URL)
	var got resp
	err := c.Patch(context.Background(), "/update/1", map[string]string{"name": "new"}, &got)
	if err != nil {
		t.Fatalf("Patch error: %v", err)
	}
	if !got.Updated {
		t.Fatal("updated = false, want true")
	}
}

func TestDelete(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	err := c.Delete(context.Background(), "/item/42")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Fatalf("method = %s, want DELETE", gotMethod)
	}
}
