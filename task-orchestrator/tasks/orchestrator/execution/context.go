package execution

import (
	"fmt"
	"task-orchestrator/tasks"
	"time"
)

// ExecutionContext tracks execution state and timing for observability and debugging.
// This enables detailed monitoring, metrics collection, and future execution strategies
// like retries or timeouts that need execution history.
type ExecutionContext struct {
	Task      *tasks.Task
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Metadata  map[string]any
}

// NewExecutionContext initializes tracking for a new execution attempt.
func NewExecutionContext(task *tasks.Task) *ExecutionContext {
	return &ExecutionContext{
		Task:      task,
		StartTime: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// SetError captures failure details for consistent error handling across strategies.
func (ctx *ExecutionContext) SetError(err error) {
	ctx.Error = err
	ctx.EndTime = time.Now()
	ctx.Metadata["has_error"] = true
	ctx.Metadata["error_type"] = fmt.Sprintf("%T", err)
}

// SetSuccess marks successful completion for monitoring and metrics collection.
func (ctx *ExecutionContext) SetSuccess() {
	ctx.EndTime = time.Now()
	ctx.Metadata["has_error"] = false
}

// IsSuccess provides a simple way to check execution outcome.
func (ctx *ExecutionContext) IsSuccess() bool {
	return ctx.Error == nil
}

// Duration calculates execution time
func (ctx *ExecutionContext) Duration() time.Duration {
	if ctx.EndTime.IsZero() {
		// For in-progress executions, show current elapsed time
		return time.Since(ctx.StartTime)
	}
	// For completed executions, show fixed duration
	return ctx.EndTime.Sub(ctx.StartTime)
}
