package execution

import (
	"context"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/runners"
)

// ExecutionWorkflow orchestrates the complete task execution lifecycle.
// This abstraction enables different execution strategies (sync, async, retry, etc.)
// without changing the orchestrator's interface.
type ExecutionWorkflow interface {
	Execute(ctx context.Context, task *tasks.Task) error
}

// DefaultExecutionWorkflow implements synchronous execution with proper error handling.
// Components are injected to enable testing and future strategy variations.
type DefaultExecutionWorkflow struct {
	runner        runners.Runner // Executes the actual business logic
	stateManager  StateManager   // Manages persistence and state transitions
	resultHandler ResultHandler  // Formats results and handles error messages
	logger        *logger.Logger
}

// NewDefaultExecutionWorkflow creates a workflow with dependency injection.
// This pattern enables easy testing and future configuration flexibility.
func NewDefaultExecutionWorkflow(
	runner runners.Runner,
	stateManager StateManager,
	resultHandler ResultHandler,
	logger *logger.Logger,
) *DefaultExecutionWorkflow {
	return &DefaultExecutionWorkflow{
		runner:        runner,
		stateManager:  stateManager,
		resultHandler: resultHandler,
		logger:        logger,
	}
}

// Execute is isolated to enable future enhancement without breaking changes on the orchestrator. Contains error handling.
func (w *DefaultExecutionWorkflow) Execute(ctx context.Context, task *tasks.Task) error {
	execCtx := taskContext.NewExecutionContext(task)

	// Determine execution strategy based on runner type
	if isAsyncRunner(w.runner) {
		return w.executeAsync(ctx, execCtx)
	}

	return w.executeSync(ctx, execCtx)
}

// executeAsync handles V2 async execution (queue-based)
func (w *DefaultExecutionWorkflow) executeAsync(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	// V2 Async: Transition to QUEUED first, then enqueue
	if err := w.stateManager.TransitionToQueued(ctx, execCtx); err != nil {
		w.logger.Task(execCtx.Task.ID, "failed to transition to queued", map[string]any{
			"error": err.Error(),
		})
		execCtx.SetError(err)
		w.resultHandler.HandleFailure(execCtx)
		if transitionErr := w.stateManager.TransitionToFailed(ctx, execCtx); transitionErr != nil {
			w.logger.Error("failed to transition task to failed state", map[string]any{
				"task_id":          execCtx.Task.ID,
				"transition_error": transitionErr.Error(),
				"original_error":   err.Error(),
			})
		}
		return err
	}

	// Enqueue for background processing
	if err := w.runner.Run(ctx, execCtx); err != nil {
		w.logger.Task(execCtx.Task.ID, "failed to enqueue task", map[string]any{
			"error": err.Error(),
		})
		execCtx.SetError(err)
		execCtx.Task.Result = err.Error()
		w.resultHandler.HandleFailure(execCtx)
		w.stateManager.TransitionToFailed(ctx, execCtx)
		if transitionErr := w.stateManager.TransitionToFailed(ctx, execCtx); transitionErr != nil {
			w.logger.Error("failed to transition task to failed state after enqueue failure", map[string]any{
				"task_id":          execCtx.Task.ID,
				"transition_error": transitionErr.Error(),
				"original_error":   err.Error(),
			})
		}
		return err
	}

	w.logger.Task(execCtx.Task.ID, "task successfully queued", nil)
	return nil
}

func (w *DefaultExecutionWorkflow) executeSync(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	// Prepare for execution by transitioning state to running
	if err := w.stateManager.TransitionToRunning(ctx, execCtx); err != nil {
		return err
	}

	if err := w.runner.Run(ctx, execCtx); err != nil {
		w.logger.Task(execCtx.Task.ID, "execution failed", map[string]any{
			"error": err.Error(),
		})

		// Update context with error details
		execCtx.SetError(err)
		w.resultHandler.HandleFailure(execCtx)

		// Attempt state cleanup, but preserve the original execution error
		if transitionErr := w.stateManager.TransitionToFailed(ctx, execCtx); transitionErr != nil {
			w.logger.Error("failed to transition task to failed state", map[string]any{
				"task_id":          execCtx.Task.ID,
				"transition_error": transitionErr.Error(),
				"original_error":   err.Error(),
			})
		}

		// Always return the execution error
		return err
	}

	// Finalize successful execution
	w.resultHandler.HandleSuccess(execCtx)
	if err := w.stateManager.TransitionToCompleted(ctx, execCtx); err != nil {
		// Log state transition failures but don't fail the operation since the business logic succeeded
		w.logger.Error("failed to transition task to completed state after successful execution", map[string]any{
			"task_id": execCtx.Task.ID,
			"error":   err.Error(),
		})
		// Execution succeeded, so we don't fail the entire operation for persistence issues
	}

	// Record successful completion for monitoring and debugging
	w.logger.Task(execCtx.Task.ID, "task completed", map[string]any{
		"status": execCtx.Task.Status.String(),
	})

	return nil
}

// Helper function to detect AsyncRunner
func isAsyncRunner(runner runners.Runner) bool {
	// Check if runner is AsynchronousRunner by type assertion
	_, ok := runner.(*runners.AsynchronousRunner)
	return ok
}
