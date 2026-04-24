package uws1

import "context"

type executionContextKey struct{}

// ExecutionContext carries runtime-only orchestration state into runtime hooks.
type ExecutionContext struct {
	Iteration *IterationContext
	Records   map[string]ExecutionRecord
	Current   *CurrentExecutionContext
}

// IterationContext describes the current orchestrator-owned iteration scope.
type IterationContext struct {
	Item       any
	Index      int
	Batch      []any
	BatchIndex int
}

// ExecutionRecord is the orchestrator-owned summary of one construct execution.
type ExecutionRecord struct {
	ID     string
	Kind   string
	Status string
	Error  string
	Result any
	Outputs map[string]any
}

// CurrentExecutionContext describes the construct currently being evaluated.
type CurrentExecutionContext struct {
	Key     string
	ID      string
	Kind    string
	ResponseID string
	Outputs map[string]any
}

// WithExecutionContext returns a new context carrying the given execution state.
func WithExecutionContext(ctx context.Context, state *ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey{}, state)
}

// ExecutionContextFromContext returns the current execution state, if any.
func ExecutionContextFromContext(ctx context.Context) (*ExecutionContext, bool) {
	state, ok := ctx.Value(executionContextKey{}).(*ExecutionContext)
	return state, ok
}
