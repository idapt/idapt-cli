//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Store_SearchSkills(t *testing.T) {
	skipIfNoServer(t)

	// Search skill store with empty query -- should return without error
	status, result := rawGet(t, "/api/skill-store")
	if status != 200 {
		t.Fatalf("GET /api/skill-store returned %d, want 200; body: %v", status, result)
	}
	// The response should have an items or data array
	items := getSlice(result, "items")
	if items == nil {
		items = getSlice(result, "data")
	}
	// Even if empty, the endpoint should respond with valid JSON
	t.Logf("skill store returned %d items", len(items))
}

func TestIntegration_Store_SearchAgents(t *testing.T) {
	skipIfNoServer(t)

	// Search agent store with empty query -- should return without error
	status, result := rawGet(t, "/api/agent-store")
	if status != 200 {
		t.Fatalf("GET /api/agent-store returned %d, want 200; body: %v", status, result)
	}
	items := getSlice(result, "items")
	if items == nil {
		items = getSlice(result, "data")
	}
	t.Logf("agent store returned %d items", len(items))
}

func TestIntegration_Store_SearchScripts(t *testing.T) {
	skipIfNoServer(t)
	t.Skip("skipping: /api/script-store route not yet implemented")

	// Search script store with empty query -- should return without error
	status, result := rawGet(t, "/api/script-store")
	if status != 200 {
		t.Fatalf("GET /api/script-store returned %d, want 200; body: %v", status, result)
	}
	items := getSlice(result, "items")
	if items == nil {
		items = getSlice(result, "data")
	}
	t.Logf("script store returned %d items", len(items))
}
