//go:build integration

package integration

import (
	"fmt"
	"testing"
)

// getPersonalBoardID fetches the personal task board ID for the test user.
func getPersonalBoardID(t *testing.T) string {
	t.Helper()
	status, result := rawGet(t, "/api/tasks/boards/personal")
	if status != 200 {
		t.Fatalf("GET /api/tasks/boards/personal returned %d; body: %v", status, result)
	}
	boardID := getString(result, "id")
	if boardID == "" {
		board := getMap(result, "board")
		boardID = getString(board, "id")
	}
	if boardID == "" {
		t.Fatalf("no board ID in personal board response: %v", result)
	}
	return boardID
}

func TestIntegration_Task_Lifecycle(t *testing.T) {
	skipIfNoServer(t)

	boardID := getPersonalBoardID(t)
	taskTitle := uniqueName("task")

	// 1. Create task item
	status, result := rawPost(t, fmt.Sprintf("/api/tasks/boards/%s/items", boardID), map[string]interface{}{
		"title": taskTitle,
	})
	if status != 201 {
		t.Fatalf("create task returned %d, want 201; body: %v", status, result)
	}
	item := getMap(result, "item")
	if item == nil {
		item = result
	}
	itemID := getString(item, "id")
	if itemID == "" {
		t.Fatalf("no task item ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, fmt.Sprintf("/api/tasks/items/%s", itemID))
	})

	// 2. List task items
	status, result = rawGet(t, fmt.Sprintf("/api/tasks/boards/%s/items", boardID))
	if status != 200 {
		t.Fatalf("list tasks returned %d; body: %v", status, result)
	}
	items := getSlice(result, "items")
	if !containsID(items, itemID) {
		t.Logf("task %s not found in items list; response: %v", itemID, result)
	}

	// 3. Get task item
	status, result = rawGet(t, fmt.Sprintf("/api/tasks/items/%s", itemID))
	if status != 200 {
		t.Fatalf("get task returned %d; body: %v", status, result)
	}
	got := getMap(result, "item")
	if got == nil {
		got = result
	}
	if getString(got, "title") != taskTitle {
		t.Fatalf("task title = %q, want %q", getString(got, "title"), taskTitle)
	}

	// 4. Edit task -- change status
	status, result = rawPatch(t, fmt.Sprintf("/api/tasks/items/%s", itemID), map[string]interface{}{
		"status": "in_progress",
	})
	if status != 200 {
		t.Fatalf("patch task returned %d; body: %v", status, result)
	}

	// 5. Verify status changed
	status, result = rawGet(t, fmt.Sprintf("/api/tasks/items/%s", itemID))
	if status != 200 {
		t.Fatalf("get task after patch returned %d; body: %v", status, result)
	}
	got = getMap(result, "item")
	if got == nil {
		got = result
	}
	gotStatus := getString(got, "status")
	if gotStatus != "in_progress" {
		t.Fatalf("task status after patch = %q, want %q", gotStatus, "in_progress")
	}

	// 6. Delete task item
	status, _ = rawDelete(t, fmt.Sprintf("/api/tasks/items/%s", itemID))
	if status != 204 {
		t.Fatalf("delete task returned %d, want 204", status)
	}
}

func TestIntegration_Task_Labels(t *testing.T) {
	skipIfNoServer(t)

	boardID := getPersonalBoardID(t)
	labelName := uniqueName("label")

	// 1. Create label
	status, result := rawPost(t, fmt.Sprintf("/api/tasks/boards/%s/labels", boardID), map[string]interface{}{
		"name":  labelName,
		"color": "#ff5733",
	})
	if status != 201 && status != 200 {
		t.Fatalf("create label returned %d; body: %v", status, result)
	}
	label := getMap(result, "label")
	if label == nil {
		label = result
	}
	labelID := getString(label, "id")
	if labelID == "" {
		t.Fatalf("no label ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, fmt.Sprintf("/api/tasks/boards/%s/labels/%s", boardID, labelID))
	})

	// 2. List labels
	status, result = rawGet(t, fmt.Sprintf("/api/tasks/boards/%s/labels", boardID))
	if status != 200 {
		t.Fatalf("list labels returned %d; body: %v", status, result)
	}
	labels := getSlice(result, "labels")
	if !containsID(labels, labelID) {
		t.Logf("label %s not found in labels list; response: %v", labelID, result)
	}

	// 3. Delete label
	status, _ = rawDelete(t, fmt.Sprintf("/api/tasks/boards/%s/labels/%s", boardID, labelID))
	if status != 204 && status != 200 {
		t.Fatalf("delete label returned %d, want 204 or 200", status)
	}
}
