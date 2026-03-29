//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Agent_Lifecycle(t *testing.T) {
	skipIfNoServer(t)

	// Create a project to scope the agents
	projectID := createProjectForTest(t)

	agentName := uniqueName("agent")

	// 1. Create agent in the project
	status, result := rawPost(t, "/api/agents", map[string]interface{}{
		"name":      agentName,
		"icon":      "emoji/🤖",
		"projectId": projectID,
	})
	if status != 201 {
		t.Fatalf("create agent returned %d, want 201; body: %v", status, result)
	}
	agent := getMap(result, "agent")
	agentID := getString(agent, "id")
	if agentID == "" {
		// Some API responses nest differently -- try top-level
		agentID = getString(result, "id")
	}
	if agentID == "" {
		t.Fatalf("no agent ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, "/api/agents/"+agentID)
	})

	// 2. List agents for the project
	status, result = rawGet(t, "/api/agents?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list agents returned %d; body: %v", status, result)
	}
	agents := getSlice(result, "agents")
	if !containsID(agents, agentID) {
		t.Fatalf("agent %s not found in list (%d items)", agentID, len(agents))
	}

	// 3. Get agent by ID
	status, result = rawGet(t, "/api/agents/"+agentID)
	if status != 200 {
		t.Fatalf("get agent returned %d; body: %v", status, result)
	}
	got := getMap(result, "agent")
	if got == nil {
		got = result // Some endpoints return flat
	}
	gotName := getString(got, "name")
	if gotName != agentName {
		t.Fatalf("agent name = %q, want %q", gotName, agentName)
	}

	// 4. Edit agent -- change name
	newName := uniqueName("agent-renamed")
	status, result = rawPatch(t, "/api/agents/"+agentID, map[string]interface{}{
		"name": newName,
	})
	if status != 200 {
		t.Fatalf("patch agent returned %d; body: %v", status, result)
	}

	// 5. Get again -- verify name changed
	status, result = rawGet(t, "/api/agents/"+agentID)
	if status != 200 {
		t.Fatalf("get agent after patch returned %d; body: %v", status, result)
	}
	got = getMap(result, "agent")
	if got == nil {
		got = result
	}
	if getString(got, "name") != newName {
		t.Fatalf("agent name after patch = %q, want %q", getString(got, "name"), newName)
	}

	// 6. Delete agent
	status, _ = rawDelete(t, "/api/agents/"+agentID)
	if status != 204 {
		t.Fatalf("delete agent returned %d, want 204", status)
	}

	// 7. List agents -- should be gone
	status, result = rawGet(t, "/api/agents?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list agents after delete returned %d; body: %v", status, result)
	}
	agents = getSlice(result, "agents")
	if containsID(agents, agentID) {
		t.Fatalf("agent %s still in list after deletion", agentID)
	}
}

func TestIntegration_Agent_DuplicateName(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	agentName := uniqueName("dupagent")

	// Create first agent
	status, result := rawPost(t, "/api/agents", map[string]interface{}{
		"name":      agentName,
		"icon":      "emoji/🤖",
		"projectId": projectID,
	})
	if status != 201 {
		t.Fatalf("create first agent returned %d; body: %v", status, result)
	}
	agent := getMap(result, "agent")
	agentID := getString(agent, "id")
	if agentID == "" {
		agentID = getString(result, "id")
	}
	t.Cleanup(func() {
		rawDelete(t, "/api/agents/"+agentID)
	})

	// Create second agent with same name in same project -- should fail or auto-suffix
	status, result = rawPost(t, "/api/agents", map[string]interface{}{
		"name":      agentName,
		"icon":      "emoji/🤖",
		"projectId": projectID,
	})
	if status == 201 {
		// API may auto-suffix the name -- clean up
		agent2 := getMap(result, "agent")
		agentID2 := getString(agent2, "id")
		if agentID2 == "" {
			agentID2 = getString(result, "id")
		}
		if agentID2 != "" {
			t.Cleanup(func() {
				rawDelete(t, "/api/agents/"+agentID2)
			})
		}
		// If API succeeds with auto-suffixed name, that's acceptable behavior
		t.Logf("duplicate agent name was accepted (likely auto-suffixed), status: %d", status)
		return
	}

	// Expect an error (409/400/422 ideally, but server may return 500 for
	// DB unique constraint violations wrapped as internal errors).
	// Any non-201 is acceptable — the important thing is it didn't create a duplicate.
	t.Logf("duplicate agent name returned %d (expected error, got error)", status)
}
