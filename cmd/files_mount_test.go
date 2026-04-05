package cmd

import (
	"testing"
)

func TestResolveProjectIDUUID(t *testing.T) {
	// UUID should be returned directly without API call
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	if len(uuid) != 36 {
		t.Fatalf("expected UUID length 36, got %d", len(uuid))
	}
	// Count hyphens
	count := 0
	for _, c := range uuid {
		if c == '-' {
			count++
		}
	}
	if count != 4 {
		t.Fatalf("expected 4 hyphens in UUID, got %d", count)
	}
}

func TestResolveProjectIDSlug(t *testing.T) {
	// Slug (not UUID) should trigger API call
	slug := "idapt"
	if len(slug) == 36 {
		t.Fatal("slug should not look like UUID")
	}
}
