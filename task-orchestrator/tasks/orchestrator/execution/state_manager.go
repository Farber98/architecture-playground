package execution

import (
	"context"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/store"
)

// StateManager coordinates task state transitions with persistence.
// This abstraction enables different persistence strategies and state validation
// without coupling execution logic to storage implementation details.
type StateManager interface {
	TransitionToRunning(ctx context.Context, execCtx *ExecutionContext) error
	TransitionToFailed(ctx context.Context, execCtx *ExecutionContext) error
	TransitionToCompleted(ctx context.Context, execCtx *ExecutionContext) error
}

// DefaultStateManager implements the default state management strategy
// Persistence failures are logged but don't halt execution
type DefaultStateManager struct {
	store  store.TaskStore
	logger *logger.Logger
}

// NewDefaultStateManager creates a default state manager
func NewDefaultStateManager(store store.TaskStore, logger *logger.Logger) *DefaultStateManager {
	return &DefaultStateManager{
		store:  store,
		logger: logger,
	}
}

// TransitionToRunning marks execution start for progress tracking and monitoring.
// Store failures are logged but don't prevent execution
func (sm *DefaultStateManager) TransitionToRunning(ctx context.Context, execCtx *ExecutionContext) error {
	if err := execCtx.Task.SetStatus(tasks.StatusRunning); err != nil {
		return err
	}

	// Attempt persistence for durability, but don't block execution on failure
	if err := sm.store.Update(ctx, execCtx.Task.ID, execCtx.Task.Status, execCtx.Task.Result); err != nil {
		sm.logger.Task("failed to update running status", execCtx.Task.ID, map[string]any{
			"error": err.Error(),
		})
		// Continue execution even if store update fails
	}

	return nil
}

// TransitionToFailed ensures error states are captured even when persistence fails.
// Graceful handling prevents cascading failures in error scenarios.
func (sm *DefaultStateManager) TransitionToFailed(ctx context.Context, execCtx *ExecutionContext) error {
	if setErr := execCtx.Task.SetStatus(tasks.StatusFailed); setErr != nil {
		sm.logger.Error("failed to set task status to failed", map[string]any{
			"task_id": execCtx.Task.ID,
			"error":   setErr.Error(),
		})
	}

	if updateErr := sm.store.Update(ctx, execCtx.Task.ID, execCtx.Task.Status, execCtx.Task.Result); updateErr != nil {
		sm.logger.Task("failed to update task failure state", execCtx.Task.ID, map[string]any{
			"update_error":   updateErr.Error(),
			"original_error": execCtx.Error.Error(),
		})
	}

	return nil
}

// TransitionToCompleted finalizes successful execution with persistence validation.
// Invalid transitions are caught early, but persistence failures are gracefully handled.
func (sm *DefaultStateManager) TransitionToCompleted(ctx context.Context, execCtx *ExecutionContext) error {
	if err := execCtx.Task.SetStatus(tasks.StatusDone); err != nil {
		return err
	}

	// Attempt final persistence but don't fail successful execution for storage issues
	if err := sm.store.Update(ctx, execCtx.Task.ID, execCtx.Task.Status, execCtx.Task.Result); err != nil {
		sm.logger.Task("failed to update final task state", execCtx.Task.ID, map[string]any{
			"error":        err.Error(),
			"final_status": execCtx.Task.Status.String(),
		})
		// the task exec was successful, what failed was the update. We continue.
	}

	return nil
}
