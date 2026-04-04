package sync

import "testing"

func TestWriteRouter_Excluded(t *testing.T) {
	exclusion := NewExclusionEngine("node_modules/\n", "", nil)
	router := NewWriteRouter(exclusion)

	if got := router.Route("node_modules/express/index.js"); got != WriteModeExcluded {
		t.Errorf("expected WriteModeExcluded for excluded path, got %v", got)
	}
}

func TestWriteRouter_OCC(t *testing.T) {
	exclusion := NewExclusionEngine("node_modules/\n", "", nil)
	router := NewWriteRouter(exclusion)

	if got := router.Route("src/app.ts"); got != WriteModeOCC {
		t.Errorf("expected WriteModeOCC for non-excluded path, got %v", got)
	}
}

func TestWriteRouter_EmptyExclusion(t *testing.T) {
	exclusion := NewExclusionEngine("", "", nil)
	router := NewWriteRouter(exclusion)

	if got := router.Route("anything"); got != WriteModeOCC {
		t.Errorf("expected WriteModeOCC with empty exclusion, got %v", got)
	}
}

func TestWriteRouter_NegatedPath(t *testing.T) {
	exclusion := NewExclusionEngine("*.log\n!important.log\n", "", nil)
	router := NewWriteRouter(exclusion)

	if got := router.Route("debug.log"); got != WriteModeExcluded {
		t.Errorf("expected WriteModeExcluded for debug.log, got %v", got)
	}

	if got := router.Route("important.log"); got != WriteModeOCC {
		t.Errorf("expected WriteModeOCC for negated important.log, got %v", got)
	}
}
