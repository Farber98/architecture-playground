package runners

import (
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	handlerRegistry "task-orchestrator/tasks/registry"
	"time"
)

var _ Runner = (*SynchronousRunner)(nil)

// SynchronousRunner executes tasks in-process and blocks until completion.
// This implementation is used in V1 to provide minimal orchestration overhead.
// It delegates execution to the appropriate TaskHandler based on task.Type,
// and logs lifecycle events such as start, duration, and result.
type SynchronousRunner struct {
	registry *handlerRegistry.HandlerRegistry
	logger   *logger.Logger
}

// NewSynchronousRunner constructs a new SynchronousRunner with a handler registry.
func NewSynchronousRunner(r *handlerRegistry.HandlerRegistry, lg *logger.Logger) *SynchronousRunner {
	return &SynchronousRunner{registry: r, logger: lg}
}

func (r *SynchronousRunner) Run(task *tasks.Task) error {
	elapsed := time.Now()
	defer func() {
		r.logger.Task(task.ID, "task completed", map[string]any{
			"duration_ns": time.Since(elapsed).Nanoseconds(),
			"status":      task.Status,
			"result":      task.Result,
		})
	}()

	handler, ok := r.registry.Get(task.Type)
	if !ok {
		task.Status = "failed"
		task.Result = "no handler registered for task type: " + task.Type
		return errors.NewNotFoundError("no handler registered for task type: " + task.Type)
	}

	r.logger.Task(task.ID, "starting task", map[string]any{
		"task_type": task.Type,
	})

	if err := handler.Run(task); err != nil {
		task.Status = "failed"
		task.Result = "task execution failed: " + err.Error()
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
