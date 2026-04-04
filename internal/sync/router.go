package sync

// WriteMode determines how a write operation should be handled.
type WriteMode int

const (
	// WriteModeExcluded means the path is local-only (no sync).
	WriteModeExcluded WriteMode = iota
	// WriteModeOCC means the path uses OCC version checking on every write.
	WriteModeOCC
)

// WriteRouter determines the sync mode for a given path.
type WriteRouter struct {
	exclusion *ExclusionEngine
}

// NewWriteRouter creates a write router with the given exclusion engine.
func NewWriteRouter(exclusion *ExclusionEngine) *WriteRouter {
	return &WriteRouter{exclusion: exclusion}
}

// Route returns the write mode for a given path.
func (r *WriteRouter) Route(path string) WriteMode {
	if r.exclusion.IsExcluded(path) {
		return WriteModeExcluded
	}
	return WriteModeOCC
}
