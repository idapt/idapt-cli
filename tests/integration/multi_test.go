//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Workflow_AgentWithKB(t *testing.T) {
	skipIfNoServer(t)

	// Create a project to scope all resources
	projectID := createProjectForTest(t)

	// Create a KB in the project
	kbName := uniqueName("workflow-kb")
	status, result := rawPost(t, "/api/kb", map[string]interface{}{
		"name":      kbName,
		"projectId": projectID,
	})
	if status != 201 {
		t.Fatalf("create KB returned %d; body: %v", status, result)
	}
	kb := getMap(result, "kb")
	if kb == nil {
		kb = result
	}
	kbID := getString(kb, "id")
	if kbID == "" {
		t.Fatalf("no KB ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, "/api/kb/"+kbID)
	})

	// Add a note to the KB
	status, result = rawPost(t, "/api/kb/"+kbID+"/notes", map[string]interface{}{
		"title":   "Integration Test Note",
		"content": "This note was created during the cross-resource workflow test.",
	})
	if status != 201 {
		t.Fatalf("create note returned %d; body: %v", status, result)
	}

	// Create an agent in the same project
	agentName := uniqueName("workflow-agent")
	status, result = rawPost(t, "/api/agents", map[string]interface{}{
		"name":      agentName,
		"icon":      "emoji/🤖",
		"projectId": projectID,
	})
	if status != 201 {
		t.Fatalf("create agent returned %d; body: %v", status, result)
	}
	agent := getMap(result, "agent")
	if agent == nil {
		agent = result
	}
	agentID := getString(agent, "id")
	if agentID == "" {
		agentID = getString(result, "id")
	}
	if agentID == "" {
		t.Fatalf("no agent ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, "/api/agents/"+agentID)
	})

	// Verify both resources exist in the project

	// List agents for the project
	status, result = rawGet(t, "/api/agents?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list agents returned %d; body: %v", status, result)
	}
	agents := getSlice(result, "agents")
	if !containsID(agents, agentID) {
		t.Fatalf("agent %s not found in project %s", agentID, projectID)
	}

	// List KBs for the project
	status, result = rawGet(t, "/api/kb?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list KBs returned %d; body: %v", status, result)
	}
	kbs := getSlice(result, "kbs")
	if kbs == nil {
		kbs = getSlice(result, "knowledgeBases")
	}
	if !containsID(kbs, kbID) {
		t.Fatalf("KB %s not found in project %s", kbID, projectID)
	}
}

func TestIntegration_Workflow_ProjectWithMultipleResources(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)

	// Create multiple agents
	var agentIDs []string
	for i := 0; i < 3; i++ {
		name := uniqueName("multi-agent")
		status, result := rawPost(t, "/api/agents", map[string]interface{}{
			"name":      name,
			"icon":      "emoji/🤖",
			"projectId": projectID,
		})
		if status != 201 {
			t.Fatalf("create agent %d returned %d; body: %v", i, status, result)
		}
		agent := getMap(result, "agent")
		if agent == nil {
			agent = result
		}
		id := getString(agent, "id")
		if id == "" {
			id = getString(result, "id")
		}
		if id == "" {
			t.Fatalf("no agent ID for agent %d: %v", i, result)
		}
		agentIDs = append(agentIDs, id)
	}
	t.Cleanup(func() {
		for _, id := range agentIDs {
			rawDelete(t, "/api/agents/"+id)
		}
	})

	// Verify all 3 agents exist in the project
	status, result := rawGet(t, "/api/agents?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list agents returned %d; body: %v", status, result)
	}
	agents := getSlice(result, "agents")
	for _, id := range agentIDs {
		if !containsID(agents, id) {
			t.Fatalf("agent %s not found in project's agent list", id)
		}
	}

	// Create a task for the project
	boardID := getPersonalBoardID(t)
	status, result = rawPost(t, "/api/tasks/boards/"+boardID+"/items", map[string]interface{}{
		"title": uniqueName("multi-task"),
	})
	if status == 201 {
		item := getMap(result, "item")
		if item == nil {
			item = result
		}
		itemID := getString(item, "id")
		if itemID != "" {
			t.Cleanup(func() {
				rawDelete(t, "/api/tasks/items/"+itemID)
			})
		}
	}
}
