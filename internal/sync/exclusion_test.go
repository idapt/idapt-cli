package sync

import "testing"

func TestExclusion_NodeModules(t *testing.T) {
	e := NewExclusionEngine("node_modules/\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{"node_modules", true},
		{"node_modules/express/index.js", true},
		{"src/node_modules/foo", true}, // unanchored
		{"src/app.ts", false},
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestExclusion_GlobExtension(t *testing.T) {
	e := NewExclusionEngine("*.log\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{"debug.log", true},
		{"a/b/debug.log", true},
		{"logfile", false}, // no extension match
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestExclusion_AnchoredPattern(t *testing.T) {
	e := NewExclusionEngine("/build/\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{"build", true},
		{"build/output.js", true},
		{"src/build", false}, // anchored: only top-level
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestExclusion_Negation(t *testing.T) {
	e := NewExclusionEngine("*.log\n!important.log\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{"debug.log", true},
		{"important.log", false}, // negated
		{"a/important.log", false},
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestExclusion_DoubleGlob(t *testing.T) {
	e := NewExclusionEngine("**/*.tmp\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{"file.tmp", true},
		{"a/file.tmp", true},
		{"a/b/c/file.tmp", true},
		{"file.txt", false},
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestExclusion_Empty(t *testing.T) {
	e := NewExclusionEngine("", "", nil)

	if e.IsExcluded("anything") {
		t.Error("empty engine should not exclude anything")
	}
}

func TestExclusion_CommentsAndBlankLines(t *testing.T) {
	content := `# Comment
node_modules/

# Another comment
*.log
`
	e := NewExclusionEngine(content, "", nil)

	if !e.IsExcluded("node_modules") {
		t.Error("node_modules should be excluded")
	}
	if !e.IsExcluded("debug.log") {
		t.Error("*.log should be excluded")
	}
}

func TestExclusion_GitignoreFallback(t *testing.T) {
	// No .idaptsync, but .gitignore has dist/
	e := NewExclusionEngine("", "dist/\n", nil)

	if !e.IsExcluded("dist") {
		t.Error("dist should be excluded via gitignore fallback")
	}
	if e.IsExcluded("src") {
		t.Error("src should not be excluded")
	}
}

func TestExclusion_IdaptsyncOverridesGitignore(t *testing.T) {
	// .idaptsync only excludes node_modules, .gitignore also has dist
	e := NewExclusionEngine("node_modules/\n", "dist/\nnode_modules/\n", nil)

	if !e.IsExcluded("node_modules") {
		t.Error("node_modules should be excluded")
	}
	// dist is NOT excluded because .idaptsync takes priority
	if e.IsExcluded("dist") {
		t.Error("dist should NOT be excluded (idaptsync overrides gitignore)")
	}
}

func TestExclusion_ExtraPatterns(t *testing.T) {
	e := NewExclusionEngine("", "", []string{"tmp", "cache"})

	if !e.IsExcluded("tmp") {
		t.Error("tmp should be excluded via extra patterns")
	}
	if !e.IsExcluded("cache") {
		t.Error("cache should be excluded via extra patterns")
	}
	if e.IsExcluded("src") {
		t.Error("src should not be excluded")
	}
}

func TestExclusion_TrailingWhitespace(t *testing.T) {
	e := NewExclusionEngine("node_modules/   \n", "", nil)

	if !e.IsExcluded("node_modules") {
		t.Error("trailing whitespace should be stripped")
	}
}

func TestExclusion_AncestorCheck(t *testing.T) {
	e := NewExclusionEngine("foo/\n", "", nil)

	if !e.IsExcluded("foo/bar/baz.txt") {
		t.Error("descendant of excluded dir should be excluded")
	}
}

func TestExclusion_GitFolder(t *testing.T) {
	e := NewExclusionEngine(".git/\n", "", nil)

	tests := []struct {
		path     string
		excluded bool
	}{
		{".git", true},
		{".git/objects/abc", true},
		{".git/HEAD", true},
		{".gitignore", false}, // .gitignore != .git
	}

	for _, tt := range tests {
		if got := e.IsExcluded(tt.path); got != tt.excluded {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}
