package fuse

import (
	"testing"
	"time"
)

func TestGetXattrValue(t *testing.T) {
	node := &IdaptNode{
		entry: &FileEntry{
			ID:               "file-123",
			ResourceID:       "res-abc",
			ProjectID:        "proj-456",
			CreatedByActorID: "actor-789",
			Icon:             "emoji/star",
			Prompt:           "a sunset",
			DurationMs:       5000,
			PublicAccess:     "read",
			Version:          7,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}

	tests := []struct {
		attr     string
		expected string
		ok       bool
	}{
		{xattrResID, "res-abc", true},
		{xattrProjectID, "proj-456", true},
		{xattrCreatedBy, "actor-789", true},
		{xattrIcon, "emoji/star", true},
		{xattrPrompt, "a sunset", true},
		{xattrDuration, "5000", true},
		{xattrPublic, "read", true},
		{xattrVersion, "7", true},
		{"user.idapt.nonexistent", "", false},
		{"security.selinux", "", false},
	}

	for _, tt := range tests {
		val, ok := node.getXattrValue(tt.attr)
		if ok != tt.ok {
			t.Errorf("getXattrValue(%q) ok = %v, want %v", tt.attr, ok, tt.ok)
		}
		if ok && val != tt.expected {
			t.Errorf("getXattrValue(%q) = %q, want %q", tt.attr, val, tt.expected)
		}
	}
}

func TestWritableXattrs(t *testing.T) {
	// icon, prompt, public_access are writable
	if !writableXattrs[xattrIcon] {
		t.Error("icon should be writable")
	}
	if !writableXattrs[xattrPrompt] {
		t.Error("prompt should be writable")
	}
	if !writableXattrs[xattrPublic] {
		t.Error("public_access should be writable")
	}

	// resource_id, project_id, version are NOT writable
	if writableXattrs[xattrResID] {
		t.Error("resource_id should NOT be writable")
	}
	if writableXattrs[xattrVersion] {
		t.Error("version should NOT be writable")
	}
}

func TestAllXattrKeysListed(t *testing.T) {
	// Ensure allXattrKeys contains all expected keys
	expected := map[string]bool{
		xattrResID: true, xattrProjectID: true, xattrCreatedBy: true,
		xattrIcon: true, xattrPrompt: true, xattrDuration: true,
		xattrPublic: true, xattrVersion: true,
	}

	for _, key := range allXattrKeys {
		if !expected[key] {
			t.Errorf("unexpected key in allXattrKeys: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing key in allXattrKeys: %s", key)
	}
}
