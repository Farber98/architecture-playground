package runners

import (
	"context"
	"task-orchestrator/errors"
	taskContext "task-orchestrator/tasks/context"
	handlerRegistry "task-orchestrator/tasks/registry"
)

var _ Runner = (*SynchronousRunner)(nil)

// SynchronousRunner executes tasks in-process and blocks until completion.
// This implementation is used in V1 to provide minimal orchestration overhead.
// It delegates execution to the appropriate TaskHandler based on task.Type,
// and logs lifecycle events such as start, duration, and result.
type SynchronousRunner struct {
	registry *handlerRegistry.HandlerRegistry
}

// NewSynchronousRunner constructs a new SynchronousRunner with a handler registry.
func NewSynchronousRunner(r *handlerRegistry.HandlerRegistry) *SynchronousRunner {
	return &SynchronousRunner{registry: r}
}

func (r *SynchronousRunner) Run(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	handler, ok := r.registry.Get(execCtx.Task.Type)
	if !ok {
		return errors.NewNotFoundError("no handler registered for task type: " + execCtx.Task.Type)
	}

	if err := handler.Run(ctx, execCtx.Task); err != nil {
		// Preserve structured errors, wrap others as execution errors
		if _, ok := errors.IsTaskError(err); ok {
			return err
		}
		return errors.NewExecutionError("task execution failed", map[string]any{
			"task_id":   execCtx.Task.ID,
			"task_type": execCtx.Task.Type,
			"error":     err.Error(),
		})
	}

	return nil
}
