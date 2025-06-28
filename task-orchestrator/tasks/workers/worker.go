package workers

import (
	"context"
	"fmt"
	"sync"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/orchestrator/execution"
	"task-orchestrator/tasks/queue"
	"task-orchestrator/tasks/registry"
)

type Worker struct {
	id           int
	queue        queue.TaskQueue
	stateManager execution.StateManager
	registry     *registry.HandlerRegistry
	logger       *logger.Logger
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func NewWorker(id int, queue queue.TaskQueue, stateManager execution.StateManager, registry *registry.HandlerRegistry, logger *logger.Logger) *Worker {
	return &Worker{
		id:           id,
		queue:        queue,
		stateManager: stateManager,
		registry:     registry,
		logger:       logger,
		stopCh:       make(chan struct{}),
		stopOnce:     sync.Once{},
	}
}

// Start begins worker's processing loop
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("worker starting", map[string]any{
		"worker_id": w.id,
	})

	defer w.logger.Info("worker stopped", map[string]any{
		"worker_id": w.id,
	})

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopping due to context cancellation", map[string]any{
				"worker_id": w.id,
			})

			return

		case <-w.stopCh:
			w.logger.Info("worker stopping", map[string]any{
				"worker_id": w.id,
			})

			return

		default:
			// process next task (blocking)
			w.processNextTask(ctx)
		}
	}
}

// Stop signals the worker to stop gracefully
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *Worker) processNextTask(ctx context.Context) {
	// dequeue next task (blocking)
	execCtx, err := w.queue.Dequeue(ctx)
	if err != nil {
		// Check if context was cancelled (normal shutdown)
		if ctx.Err() != nil {
			return
		}

		w.logger.Error("failed to dequeue task", map[string]any{
			"worker_id": w.id,
			"error":     err.Error(),
		})
		return
	}

	w.logger.Task(execCtx.Task.ID, "worker processing task", map[string]any{
		"worker_id": w.id,
		"task_type": execCtx.Task.Type,
	})

	if err := w.stateManager.TransitionToRunning(ctx, execCtx); err != nil {
		w.logger.Error("failed to update task status to running", map[string]any{
			"task_id":   execCtx.Task.ID,
			"worker_id": w.id,
			"error":     err.Error(),
		})
		// Can't process if we can't update state - return without processing
		return
	}

	// Execute the actual task
	if err := w.executeTask(ctx, execCtx.Task); err != nil {
		w.handleTaskFailure(ctx, execCtx, err)
		return
	}

	w.handleTaskSuccess(ctx, execCtx)
}

func (w *Worker) executeTask(ctx context.Context, task *tasks.Task) error {
	handler, ok := w.registry.Get(task.Type)
	if !ok {
		return fmt.Errorf("no handler registered for task type: %s", task.Type)
	}

	// Execute the handler (this is where the actual work happens)
	return handler.Run(ctx, task)
}

// handleTaskSuccess manages successful task completion
func (w *Worker) handleTaskSuccess(ctx context.Context, execCtx *taskContext.ExecutionContext) {
	if err := w.stateManager.TransitionToCompleted(ctx, execCtx); err != nil {
		w.logger.Error("failed to update task status to completed", map[string]any{
			"task_id":   execCtx.Task.ID,
			"worker_id": w.id,
			"error":     err.Error(),
		})
	}

	w.logger.Task(execCtx.Task.ID, "task completed successfully", map[string]any{
		"worker_id": w.id,
		"result":    execCtx.Task.Result,
	})
}

// handleTaskFailure manages task execution failures
func (w *Worker) handleTaskFailure(ctx context.Context, execCtx *taskContext.ExecutionContext, taskErr error) {
	w.logger.Task(execCtx.Task.ID, "task execution failed", map[string]any{
		"worker_id": w.id,
		"error":     taskErr.Error(),
	})

	// Set error details
	execCtx.SetError(taskErr)
	execCtx.Task.Result = taskErr.Error()

	if err := w.stateManager.TransitionToFailed(ctx, execCtx); err != nil {
		w.logger.Error("failed to update task status to failed", map[string]any{
			"task_id":   execCtx.Task.ID,
			"worker_id": w.id,
			"error":     err.Error(),
		})
	}
}
