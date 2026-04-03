//go:build integration

package integration

import (
	"testing"
)

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
}
