//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Script_Lifecycle(t *testing.T) {
	skipIfNoServer(t)
	t.Skip("skipping: /api/scripts CRUD routes not yet implemented")

	projectID := createProjectForTest(t)
	scriptName := uniqueName("script")

	// 1. Create script
	status, result := rawPost(t, "/api/scripts", map[string]interface{}{
		"name":      scriptName,
		"projectId": projectID,
		"content":   "#!/bin/bash\necho 'hello from test'",
		"language":  "bash",
	})
	if status != 201 {
		t.Fatalf("create script returned %d, want 201; body: %v", status, result)
	}
	script := getMap(result, "script")
	if script == nil {
		script = result
	}
	scriptID := getString(script, "id")
	if scriptID == "" {
		t.Fatalf("no script ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, "/api/scripts/"+scriptID)
	})

	// 2. List scripts for the project
	status, result = rawGet(t, "/api/scripts?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list scripts returned %d; body: %v", status, result)
	}
	scripts := getSlice(result, "scripts")
	if !containsID(scripts, scriptID) {
		t.Logf("script %s not found in list (%d items); response: %v", scriptID, len(scripts), result)
	}

	// 3. Get script by ID
	status, result = rawGet(t, "/api/scripts/"+scriptID)
	if status != 200 {
		t.Fatalf("get script returned %d; body: %v", status, result)
	}
	got := getMap(result, "script")
	if got == nil {
		got = result
	}
	if getString(got, "name") != scriptName {
		t.Fatalf("script name = %q, want %q", getString(got, "name"), scriptName)
	}

	// 4. Edit script -- change name
	newName := uniqueName("script-renamed")
	status, result = rawPatch(t, "/api/scripts/"+scriptID, map[string]interface{}{
		"name": newName,
	})
	if status != 200 {
		t.Fatalf("patch script returned %d; body: %v", status, result)
	}

	// 5. Verify name changed
	status, result = rawGet(t, "/api/scripts/"+scriptID)
	if status != 200 {
		t.Fatalf("get script after patch returned %d; body: %v", status, result)
	}
	got = getMap(result, "script")
	if got == nil {
		got = result
	}
	if getString(got, "name") != newName {
		t.Fatalf("script name after patch = %q, want %q", getString(got, "name"), newName)
	}

	// 6. Delete script
	status, _ = rawDelete(t, "/api/scripts/"+scriptID)
	if status != 204 {
		t.Fatalf("delete script returned %d, want 204", status)
	}
}
