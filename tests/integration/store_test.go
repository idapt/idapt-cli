//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Store_SearchTemplates(t *testing.T) {
	skipIfNoServer(t)

	// Search unified template store with empty query
	status, result := rawGet(t, "/api/templates?limit=5")
	if status != 200 {
		t.Fatalf("GET /api/templates returned %d, want 200; body: %v", status, result)
	}
	items := getSlice(result, "items")
	if items == nil {
		t.Fatal("response missing 'items' field")
	}
	t.Logf("template store returned %d items", len(items))
}

func TestIntegration_Store_SearchByType(t *testing.T) {
	skipIfNoServer(t)

	for _, typ := range []string{"skill", "agent", "machine"} {
		t.Run(typ, func(t *testing.T) {
			status, result := rawGet(t, "/api/templates?type="+typ+"&limit=5")
			if status != 200 {
				t.Fatalf("GET /api/templates?type=%s returned %d, want 200; body: %v", typ, status, result)
			}
			items := getSlice(result, "items")
			if items == nil {
				t.Fatal("response missing 'items' field")
			}
			// All returned items should be of the requested type
			for i, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if m["type"] != typ {
					t.Errorf("item %d has type %v, want %s", i, m["type"], typ)
				}
			}
		})
	}
}
