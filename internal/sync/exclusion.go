package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

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

func NewExclusionEngine(idaptsyncContent, gitignoreContent string, extraPatterns []string) *ExclusionEngine {
	var patterns []pattern

	content := idaptsyncContent
	if content == "" {
		content = gitignoreContent
	}

	if content != "" {
		patterns = append(patterns, parsePatterns(content)...)
	}

	for _, p := range extraPatterns {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, parsePattern(p))
		}
	}

	return &ExclusionEngine{patterns: patterns}
}

func LoadExclusionEngine(projectRoot string, extraPatterns []string) *ExclusionEngine {
	idaptsync := readFileContent(filepath.Join(projectRoot, ".idaptsync"))
	gitignore := readFileContent(filepath.Join(projectRoot, ".gitignore"))
	return NewExclusionEngine(idaptsync, gitignore, extraPatterns)
}

func (e *ExclusionEngine) IsExcluded(path string) bool {
	path = strings.TrimPrefix(path, "/")

	excluded := false
	for _, p := range e.patterns {
		if p.matches(path) {
			excluded = !p.negation
		}
	}

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

func (e *ExclusionEngine) Reload(idaptsyncContent, gitignoreContent string, extraPatterns []string) {
	*e = *NewExclusionEngine(idaptsyncContent, gitignoreContent, extraPatterns)
}

func parsePatterns(content string) []pattern {
	var patterns []pattern
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimRight(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, parsePattern(line))
	}
	return patterns
}

func parsePattern(line string) pattern {
	p := pattern{raw: line}

	if strings.HasPrefix(line, "!") {
		p.negation = true
		line = line[1:]
	}

	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	if strings.Contains(line, "/") {
		p.anchored = true
		line = strings.TrimPrefix(line, "/")
	}

	p.hasDouble = strings.Contains(line, "**")
	p.segments = strings.Split(line, "/")

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
	if !p.anchored && len(p.segments) == 1 {
		basename := filepath.Base(path)
		return matchGlob(p.segments[0], basename)
	}

	pathSegments := strings.Split(path, "/")

	if p.hasDouble {
		return matchDoubleGlob(p.segments, pathSegments)
	}

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

func matchGlob(pattern, name string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(name, pattern[1:])
	}

	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}

	return pattern == name
}

func matchDoubleGlob(patternSegs, pathSegs []string) bool {
	pi, si := 0, 0
	for pi < len(patternSegs) && si < len(pathSegs) {
		if patternSegs[pi] == "**" {
			pi++
			if pi >= len(patternSegs) {
				return true // ** at end matches everything
			}
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

	return pi >= len(patternSegs) && si >= len(pathSegs)
}

func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
