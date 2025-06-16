package execution

import (
	"context"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
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
	execCtx := NewExecutionContext(task)

	// Prepare for execution by transitioning state to running
	if err := w.stateManager.TransitionToRunning(ctx, execCtx); err != nil {
		return err
	}

	// Execute the actual business logic via configured runner
	if err := w.runner.Run(ctx, task); err != nil {
		w.logger.Task(task.ID, "execution failed", map[string]any{
			"error": err.Error(),
		})

		// Update context with error details
		execCtx.SetError(err)
		w.resultHandler.HandleFailure(execCtx)

		// Attempt state cleanup, but preserve the original execution error
		if transitionErr := w.stateManager.TransitionToFailed(ctx, execCtx); transitionErr != nil {
			w.logger.Error("failed to transition task to failed state", map[string]any{
				"task_id":          task.ID,
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
			"task_id": task.ID,
			"error":   err.Error(),
		})
		// Execution succeeded, so we don't fail the entire operation for persistence issues
	}

	// Record successful completion for monitoring and debugging
	w.logger.Task(task.ID, "task completed", map[string]any{
		"status": task.Status.String(),
	})

	return nil
}
