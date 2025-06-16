package runners

import (
	"context"
	"task-orchestrator/tasks"
)

// Runner defines the interface for executing tasks.
type Runner interface {
	// Run executes the task using the registered handler for its type.
	// If no handler is found, an error is returned.
	Run(ctx context.Context, task *tasks.Task) error
}
