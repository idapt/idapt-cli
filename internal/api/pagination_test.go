package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestListIterator_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PageResponse{
			Data:    []map[string]interface{}{{"id": "1"}, {"id": "2"}},
			HasMore: false,
		})
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{Limit: 10}, nil)

	var items []string
	for iter.Next(context.Background()) {
		items = append(items, iter.Item()["id"].(string))
	}
	if iter.Err() != nil {
		t.Fatalf("unexpected error: %v", iter.Err())
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestListIterator_MultiPage(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqCount, 1)
		if n == 1 {
			json.NewEncoder(w).Encode(PageResponse{
				Data:    []map[string]interface{}{{"id": "a"}, {"id": "b"}},
				HasMore: true,
				LastID:  "b",
			})
		} else {
			json.NewEncoder(w).Encode(PageResponse{
				Data:    []map[string]interface{}{{"id": "c"}},
				HasMore: false,
			})
		}
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{Limit: 2}, nil)

	var items []string
	for iter.Next(context.Background()) {
		items = append(items, iter.Item()["id"].(string))
	}
	if iter.Err() != nil {
		t.Fatalf("unexpected error: %v", iter.Err())
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3: %v", len(items), items)
	}
	if items[0] != "a" || items[1] != "b" || items[2] != "c" {
		t.Errorf("got items %v, want [a b c]", items)
	}
}

func TestListIterator_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PageResponse{
			Data:    []map[string]interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{}, nil)

	if iter.Next(context.Background()) {
		t.Error("expected Next() to return false for empty result")
	}
	if iter.Err() != nil {
		t.Errorf("unexpected error: %v", iter.Err())
	}
}

func TestListIterator_CursorPassed(t *testing.T) {
	var secondCursor string
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqCount, 1)
		if n == 1 {
			json.NewEncoder(w).Encode(PageResponse{
				Data:    []map[string]interface{}{{"id": "1"}},
				HasMore: true,
				LastID:  "cursor-123",
			})
		} else {
			secondCursor = r.URL.Query().Get("starting_after")
			json.NewEncoder(w).Encode(PageResponse{
				Data:    []map[string]interface{}{},
				HasMore: false,
			})
		}
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{Limit: 1}, nil)

	for iter.Next(context.Background()) {
	}
	if secondCursor != "cursor-123" {
		t.Errorf("second request cursor = %q, want %q", secondCursor, "cursor-123")
	}
}

func TestListIterator_ErrorOnFirstPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"message":"server error"}}`))
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{}, nil)

	if iter.Next(context.Background()) {
		t.Error("expected Next() to return false on error")
	}
	if iter.Err() == nil {
		t.Error("expected error, got nil")
	}
}

func TestListIterator_LimitQuery(t *testing.T) {
	var gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		json.NewEncoder(w).Encode(PageResponse{Data: []map[string]interface{}{}, HasMore: false})
	}))
	defer server.Close()

	c, _ := NewClient(ClientConfig{BaseURL: server.URL, CLIVersion: "test"})
	iter := NewListIterator(c, "/api/items", PageParams{Limit: 5}, nil)
	iter.Next(context.Background())

	if gotLimit != "5" {
		t.Errorf("limit query = %q, want %q", gotLimit, "5")
	}
}

func TestPageParams_Query(t *testing.T) {
	p := PageParams{Limit: 10, StartingAfter: "abc"}
	q := p.Query()
	if q.Get("limit") != "10" {
		t.Errorf("limit = %q, want %q", q.Get("limit"), "10")
	}
	if q.Get("starting_after") != "abc" {
		t.Errorf("starting_after = %q, want %q", q.Get("starting_after"), "abc")
	}
}
