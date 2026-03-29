//go:build integration

package integration

import (
	"testing"
)

func TestIntegration_Project_Lifecycle(t *testing.T) {
	skipIfNoServer(t)

	name := uniqueName("proj")
	slug := uniqueSlug("proj")

	// 1. Create project
	status, result := rawPost(t, "/api/projects", map[string]interface{}{
		"name": name,
		"slug": slug,
	})
	if status != 201 {
		t.Fatalf("create project returned %d, want 201; body: %v", status, result)
	}
	proj := getMap(result, "project")
	projectID := getString(proj, "id")
	if projectID == "" {
		t.Fatalf("no project ID in create response: %v", result)
	}

	// 2. List projects -- should contain the new project
	status, result = rawGet(t, "/api/projects")
	if status != 200 {
		t.Fatalf("list projects returned %d; body: %v", status, result)
	}
	projects := getSlice(result, "projects")
	if !containsID(projects, projectID) {
		t.Fatalf("project %s not found in list (%d items)", projectID, len(projects))
	}

	// 3. Get project by ID
	status, result = rawGet(t, "/api/projects/"+projectID)
	if status != 200 {
		t.Fatalf("get project returned %d; body: %v", status, result)
	}
	got := getMap(result, "project")
	if getString(got, "name") != name {
		t.Fatalf("project name = %q, want %q", getString(got, "name"), name)
	}

	// 4. Edit project -- change name
	newName := uniqueName("proj-renamed")
	status, result = rawPatch(t, "/api/projects/"+projectID, map[string]interface{}{
		"name": newName,
	})
	if status != 200 {
		t.Fatalf("patch project returned %d; body: %v", status, result)
	}

	// 5. Get again -- verify name changed
	status, result = rawGet(t, "/api/projects/"+projectID)
	if status != 200 {
		t.Fatalf("get project after patch returned %d; body: %v", status, result)
	}
	got = getMap(result, "project")
	if getString(got, "name") != newName {
		t.Fatalf("project name after patch = %q, want %q", getString(got, "name"), newName)
	}

	// 6. Delete project
	status, _ = rawDelete(t, "/api/projects/"+projectID)
	if status != 204 {
		t.Fatalf("delete project returned %d, want 204", status)
	}

	// 7. List -- project should be gone
	status, result = rawGet(t, "/api/projects")
	if status != 200 {
		t.Fatalf("list projects after delete returned %d; body: %v", status, result)
	}
	projects = getSlice(result, "projects")
	if containsID(projects, projectID) {
		t.Fatalf("project %s still found in list after deletion", projectID)
	}
}

func TestIntegration_Project_DuplicateSlug(t *testing.T) {
	skipIfNoServer(t)

	slug := uniqueSlug("dupslug")

	// Create first project
	status, result := rawPost(t, "/api/projects", map[string]interface{}{
		"name": uniqueName("proj1"),
		"slug": slug,
	})
	if status != 201 {
		t.Fatalf("create first project returned %d; body: %v", status, result)
	}
	proj := getMap(result, "project")
	projectID := getString(proj, "id")
	t.Cleanup(func() {
		rawDelete(t, "/api/projects/"+projectID)
	})

	// Try to create second project with same slug -- should fail
	status, result = rawPost(t, "/api/projects", map[string]interface{}{
		"name": uniqueName("proj2"),
		"slug": slug,
	})
	// Expect 409 Conflict or 400 Bad Request
	if status == 201 {
		// Clean up the accidentally created project
		proj2 := getMap(result, "project")
		if id := getString(proj2, "id"); id != "" {
			rawDelete(t, "/api/projects/"+id)
		}
		t.Fatalf("expected error for duplicate slug, but got 201")
	}
	// Server may return 409 (proper conflict), 400/422 (validation), or 500
	// (DB constraint violation wrapped as internal error). All are acceptable
	// as long as it's not 201.
	if status != 409 && status != 400 && status != 422 && status != 500 {
		t.Fatalf("duplicate slug returned %d, want error status; body: %v", status, result)
	}
}
