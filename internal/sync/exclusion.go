// Package sync implements the file sync protocol for the FUSE mount.
package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ExclusionEngine parses .idaptsync/.gitignore-style patterns and determines
// which paths are excluded from sync (local-only).
type ExclusionEngine struct {
	patterns []pattern
}

type pattern struct {
	raw       string
	negation  bool
	dirOnly   bool
	anchored  bool
	segments  []string // path segments for matching
	hasDouble bool     // contains ** glob
}

// NewExclusionEngine creates an exclusion engine from pattern sources.
// Patterns from .idaptsync take priority over .gitignore.
// Extra patterns (from --exclude flag) are appended.
func NewExclusionEngine(idaptsyncContent, gitignoreContent string, extraPatterns []string) *ExclusionEngine {
	var patterns []pattern

	// Use .idaptsync if available, else .gitignore
	content := idaptsyncContent
	if content == "" {
		content = gitignoreContent
	}

	if content != "" {
		patterns = append(patterns, parsePatterns(content)...)
	}

	// Add --exclude patterns
	for _, p := range extraPatterns {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, parsePattern(p))
		}
	}

	return &ExclusionEngine{patterns: patterns}
}

// LoadExclusionEngine loads patterns from files on disk + extra CLI patterns.
func LoadExclusionEngine(projectRoot string, extraPatterns []string) *ExclusionEngine {
	idaptsync := readFileContent(filepath.Join(projectRoot, ".idaptsync"))
	gitignore := readFileContent(filepath.Join(projectRoot, ".gitignore"))
	return NewExclusionEngine(idaptsync, gitignore, extraPatterns)
}

// IsExcluded returns true if the path matches an exclusion pattern.
// Last matching pattern wins (like gitignore).
func (e *ExclusionEngine) IsExcluded(path string) bool {
	// Normalize: remove leading slash, ensure no trailing slash for matching
	path = strings.TrimPrefix(path, "/")

	excluded := false
	for _, p := range e.patterns {
		if p.matches(path) {
			excluded = !p.negation
		}
	}

	// Check ancestors: if /foo/ is excluded, /foo/bar/baz is too
	if !excluded {
		parts := strings.Split(path, "/")
		for i := 1; i < len(parts); i++ {
			ancestor := strings.Join(parts[:i], "/")
			for _, p := range e.patterns {
				if p.matches(ancestor) || p.matches(ancestor+"/") {
					excluded = !p.negation
				}
			}
			if excluded {
				break
			}
		}
	}

	return excluded
}

// Reload replaces all patterns from new content.
func (e *ExclusionEngine) Reload(idaptsyncContent, gitignoreContent string, extraPatterns []string) {
	*e = *NewExclusionEngine(idaptsyncContent, gitignoreContent, extraPatterns)
}

func parsePatterns(content string) []pattern {
	var patterns []pattern
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		// Strip trailing whitespace
		line = strings.TrimRight(line, " \t")
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, parsePattern(line))
	}
	return patterns
}

func parsePattern(line string) pattern {
	p := pattern{raw: line}

	// Negation
	if strings.HasPrefix(line, "!") {
		p.negation = true
		line = line[1:]
	}

	// Directory-only
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	// Anchored if contains / (not just trailing)
	if strings.Contains(line, "/") {
		p.anchored = true
		// Strip leading / (e.g., "/build" → "build")
		line = strings.TrimPrefix(line, "/")
	}

	p.hasDouble = strings.Contains(line, "**")
	p.segments = strings.Split(line, "/")

	// Remove empty segments from splitting
	filtered := p.segments[:0]
	for _, s := range p.segments {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	p.segments = filtered

	return p
}

func (p *pattern) matches(path string) bool {
	// Simple name-only pattern (no /) — match basename at any depth
	if !p.anchored && len(p.segments) == 1 {
		basename := filepath.Base(path)
		return matchGlob(p.segments[0], basename)
	}

	// Anchored pattern — match from root
	pathSegments := strings.Split(path, "/")

	if p.hasDouble {
		return matchDoubleGlob(p.segments, pathSegments)
	}

	// Simple anchored match
	if len(p.segments) > len(pathSegments) {
		return false
	}

	for i, seg := range p.segments {
		if !matchGlob(seg, pathSegments[i]) {
			return false
		}
	}
	return true
}

// matchGlob implements simple glob matching (* matches any chars, ? matches one).
func matchGlob(pattern, name string) bool {
	if pattern == "*" {
		return true
	}

	// Handle *.ext patterns
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(name, pattern[1:])
	}

	// Handle prefix* patterns
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}

	// Exact match
	return pattern == name
}

// matchDoubleGlob handles ** patterns that match across directories.
func matchDoubleGlob(patternSegs, pathSegs []string) bool {
	pi, si := 0, 0
	for pi < len(patternSegs) && si < len(pathSegs) {
		if patternSegs[pi] == "**" {
			pi++
			if pi >= len(patternSegs) {
				return true // ** at end matches everything
			}
			// Try matching remaining pattern at each position
			for si < len(pathSegs) {
				if matchDoubleGlob(patternSegs[pi:], pathSegs[si:]) {
					return true
				}
				si++
			}
			return false
		}

		if !matchGlob(patternSegs[pi], pathSegs[si]) {
			return false
		}
		pi++
		si++
	}

	// Both consumed
	return pi >= len(patternSegs) && si >= len(pathSegs)
}

func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
