package runners

import (
	"task-orchestrator/errors"
	"task-orchestrator/tasks"
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

func (r *SynchronousRunner) Run(task *tasks.Task) error {
	handler, ok := r.registry.Get(task.Type)
	if !ok {
		return errors.NewNotFoundError("no handler registered for task type: " + task.Type)
	}

	if err := handler.Run(task); err != nil {
		// Preserve structured errors, wrap others as execution errors
		if _, ok := errors.IsTaskError(err); ok {
			return err
		}
		return errors.NewExecutionError("task execution failed", map[string]any{
			"task_id":   task.ID,
			"task_type": task.Type,
			"error":     err.Error(),
		})
	}

	return nil
}
