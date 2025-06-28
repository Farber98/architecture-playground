package context

import (
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
}

// NewExecutionContext initializes tracking for a new execution attempt.
func NewExecutionContext(task *tasks.Task) *ExecutionContext {
	return &ExecutionContext{
		Task:      task,
		StartTime: time.Now(),
	}
}

// SetError captures failure details for consistent error handling across strategies.
func (e *ExecutionContext) SetError(err error) {
	e.Error = err
	e.EndTime = time.Now()
}

// SetSuccess marks successful completion for monitoring and metrics collection.
func (e *ExecutionContext) SetSuccess() {
	e.EndTime = time.Now()
}
