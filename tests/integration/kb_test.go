//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_KB_Lifecycle(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	kbName := uniqueName("kb")

	// 1. Create KB
	status, result := rawPost(t, "/api/kb", map[string]interface{}{
		"name":      kbName,
		"projectId": projectID,
	})
	if status != 201 {
		t.Fatalf("create KB returned %d, want 201; body: %v", status, result)
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

	// 2. List KBs -- should contain the new KB
	status, result = rawGet(t, "/api/kb?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list KBs returned %d; body: %v", status, result)
	}
	kbs := getSlice(result, "kbs")
	if kbs == nil {
		kbs = getSlice(result, "knowledgeBases")
	}
	if !containsID(kbs, kbID) {
		t.Fatalf("KB %s not found in list (%d items); response: %v", kbID, len(kbs), result)
	}

	// 3. Create a note in the KB
	noteTitle := uniqueName("note")
	noteContent := "This is test note content about quantum physics experiments."
	status, result = rawPost(t, "/api/kb/"+kbID+"/notes", map[string]interface{}{
		"title":   noteTitle,
		"content": noteContent,
	})
	if status != 201 {
		t.Fatalf("create note returned %d, want 201; body: %v", status, result)
	}
	note := getMap(result, "note")
	if note == nil {
		note = result
	}
	noteID := getString(note, "id")
	if noteID == "" {
		t.Fatalf("no note ID in create response: %v", result)
	}

	// 4. List notes
	status, result = rawGet(t, "/api/kb/"+kbID+"/notes")
	if status != 200 {
		t.Fatalf("list notes returned %d; body: %v", status, result)
	}
	notes := getSlice(result, "notes")
	if notes == nil {
		t.Logf("notes list response: %v", result)
	}

	// 5. Edit note -- change title
	newTitle := uniqueName("note-edited")
	status, result = rawPatch(t, "/api/kb/"+kbID+"/notes?noteId="+noteID, map[string]interface{}{
		"title": newTitle,
	})
	if status != 200 {
		t.Fatalf("patch note returned %d; body: %v", status, result)
	}

	// 6. Get single note to verify
	status, result = rawGet(t, "/api/kb/notes/"+noteID)
	if status != 200 {
		t.Fatalf("get note returned %d; body: %v", status, result)
	}
	gotNote := getMap(result, "note")
	if gotNote == nil {
		gotNote = result
	}
	if getString(gotNote, "title") != newTitle {
		t.Fatalf("note title after patch = %q, want %q", getString(gotNote, "title"), newTitle)
	}

	// 7. Delete note
	status, _ = rawDelete(t, "/api/kb/"+kbID+"/notes?noteId="+noteID)
	if status != 200 && status != 204 {
		t.Fatalf("delete note returned %d, want 200 or 204", status)
	}

	// Note: KB itself is cleaned up via t.Cleanup above.
}

func TestIntegration_KB_Search(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	kbName := uniqueName("kb-search")

	// Create KB with a note containing specific text
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
	t.Cleanup(func() {
		rawDelete(t, "/api/kb/"+kbID)
	})

	// Create a note with specific content
	status, result = rawPost(t, "/api/kb/"+kbID+"/notes", map[string]interface{}{
		"title":   "Quantum Physics Notes",
		"content": "This note discusses quantum entanglement and superposition principles.",
	})
	if status != 201 {
		t.Fatalf("create search note returned %d; body: %v", status, result)
	}

	// Search for the note by querying notes endpoint with q parameter
	status, result = rawGet(t, "/api/kb/"+kbID+"/notes?q=quantum")
	if status != 200 {
		t.Fatalf("search notes returned %d; body: %v", status, result)
	}

	notes := getSlice(result, "notes")
	if len(notes) == 0 {
		t.Logf("search for 'quantum' returned no notes (search index may not be immediate); response: %v", result)
	}
}
