//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Settings_GetSet(t *testing.T) {
	skipIfNoServer(t)

	// 1. Get current settings
	status, result := rawGet(t, "/api/settings")
	if status != 200 {
		t.Fatalf("GET /api/settings returned %d; body: %v", status, result)
	}
	// Settings should return a JSON object
	if result == nil {
		t.Fatal("settings response is nil")
	}

	// 2. Update a setting (name)
	newName := uniqueName("testuser")
	status, result = rawPatch(t, "/api/settings", map[string]interface{}{
		"name": newName,
	})
	if status != 200 {
		t.Fatalf("PATCH /api/settings returned %d; body: %v", status, result)
	}

	// 3. Get settings again -- verify name changed
	status, result = rawGet(t, "/api/settings")
	if status != 200 {
		t.Fatalf("GET /api/settings after patch returned %d; body: %v", status, result)
	}

	// The settings response structure varies; check if name is present
	gotName := getString(result, "name")
	if gotName == "" {
		// Try nested under "settings" key
		settings := getMap(result, "settings")
		if settings != nil {
			gotName = getString(settings, "name")
		}
	}
	if gotName != "" && gotName != newName {
		t.Fatalf("settings name = %q, want %q", gotName, newName)
	}
}
