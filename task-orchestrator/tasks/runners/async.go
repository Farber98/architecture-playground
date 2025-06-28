package runners

import (
	"context"
	"task-orchestrator/errors"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/queue"
	"task-orchestrator/tasks/registry"
)

// AsynchronousRunner validates tasks and enqueues them for background processing.
type AsynchronousRunner struct {
	queue    queue.TaskQueue
	registry *registry.HandlerRegistry
}

var _ Runner = (*AsynchronousRunner)(nil)

func NewAsynchronousRunner(queue queue.TaskQueue, registry *registry.HandlerRegistry) *AsynchronousRunner {
	return &AsynchronousRunner{queue: queue, registry: registry}
}

// StateManager handles all status updates
func (r *AsynchronousRunner) Run(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	// Validate handler exists before queuing
	_, ok := r.registry.Get(execCtx.Task.Type)
	if !ok {
		return errors.NewNotFoundError("no handler registered for task type: " + execCtx.Task.Type)
	}

	// Handler exists, safe to queue for background processing
	if err := r.queue.Enqueue(ctx, execCtx); err != nil {
		// Preserve structured errors, wrap others as execution errors
		if _, ok := errors.IsTaskError(err); ok {
			return err
		}
		return errors.NewExecutionError("failed to enqueue task", map[string]any{
			"task_id":   execCtx.Task.ID,
			"task_type": execCtx.Task.Type,
			"error":     err.Error(),
		})
	}

	return nil
}
