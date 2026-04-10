package sync

type WriteMode int

const (
	WriteModeExcluded WriteMode = iota
	WriteModeOCC
)

type WriteRouter struct {
	exclusion *ExclusionEngine
}

func NewWriteRouter(exclusion *ExclusionEngine) *WriteRouter {
	return &WriteRouter{exclusion: exclusion}
}

func (r *WriteRouter) Route(path string) WriteMode {
	if r.exclusion.IsExcluded(path) {
		return WriteModeExcluded
	}
	return WriteModeOCC
}
