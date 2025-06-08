package orchestrator

import (
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"
)

// Orchestrator defines the contract for task orchestration services.
type Orchestrator interface {
	// SubmitTask creates and executes a new task from the provided type and payload.
	SubmitTask(taskType string, payload json.RawMessage) (*tasks.Task, error)

	// GetTask retrieves a task by ID from the underlying storage.
	GetTask(taskID string) (*tasks.Task, error)

	// GetTaskStatus returns just the status of a task for lightweight queries.
	// This is more efficient than GetTask when you only need the status.
	GetTaskStatus(taskID string) (string, error)
}

// orchestrator is the single implementation that uses different runner strategies.
// The behavior changes based on which runner is injected (sync, async, distributed, etc.)
type orchestrator struct {
	store  store.TaskStore
	runner runners.Runner
	logger *logger.Logger
}

var _ Orchestrator = (*orchestrator)(nil)

// NewOrchestrator constructs a new orchestrator with the specified runner strategy.
// The runner determines whether execution is synchronous, asynchronous, distributed, etc.
func NewOrchestrator(store store.TaskStore, runner runners.Runner, lg *logger.Logger) Orchestrator {
	return &orchestrator{
		store:  store,
		runner: runner,
		logger: lg,
	}
}

// SubmitTask creates and executes a new task using the configured runner strategy.
func (o *orchestrator) SubmitTask(taskType string, payload json.RawMessage) (*tasks.Task, error) {
	task := tasks.NewTask(taskType, payload)

	// Persist initial task state
	if err := o.store.Save(task); err != nil {
		o.logger.Task("failed to save task", task.ID, map[string]any{
			"error": err.Error(),
		})
		return task, errors.NewInternalError("failed to save task")
	}

	o.logger.Task(task.ID, "task submitted", map[string]any{
		"task_type":    task.Type,
		"runner_type":  fmt.Sprintf("%T", o.runner),
		"payload_size": len(task.Payload),
	})

	return task, o.executeTask(task)
}

// executeTask handles the execution using the configured runner strategy.
func (o *orchestrator) executeTask(task *tasks.Task) error {
	if err := task.SetStatus(tasks.StatusRunning); err != nil {
		return err
	}

	if err := o.store.Update(task.ID, task.Status, task.Result); err != nil {
		o.logger.Task("failed to update running status", task.ID, map[string]any{
			"error": err.Error(),
		})
		// Continue execution even if store update fails
	}

	// Delegate to runner strategy
	// This is where the behavior changes based on the injected runner
	if err := o.runner.Run(task); err != nil {
		o.logger.Task(task.ID, "execution failed", map[string]any{
			"error": err.Error(),
		})

		if setErr := task.SetStatus(tasks.StatusFailed); setErr != nil {
			o.logger.Error("failed to set task status to failed", map[string]any{
				"task_id": task.ID,
				"error":   setErr.Error(),
			})
		}

		// Set failure result if handler didn't set one
		if task.Result == "" {
			if taskErr, ok := errors.IsTaskError(err); ok {
				task.Result = fmt.Sprintf("task %s: %s", taskErr.Type, taskErr.Message)
			} else {
				task.Result = fmt.Sprintf("execution failed: %s", err.Error())
			}
		}

		// Update store with failure
		if updateErr := o.store.Update(task.ID, task.Status, task.Result); updateErr != nil {
			o.logger.Task("failed to update task failure state", task.ID, map[string]any{
				"update_error":   updateErr.Error(),
				"original_error": err.Error(),
			})
		}

		return err
	}

	// handles success
	if err := task.SetStatus(tasks.StatusDone); err != nil {
		return err
	}

	// Update final state
	if err := o.store.Update(task.ID, task.Status, task.Result); err != nil {
		o.logger.Task("failed to update final task state", task.ID, map[string]any{
			"error":        err.Error(),
			"final_status": task.Status.String(),
		})
		// the task exec was succesful, what failed was the update. We continue.
	}

	o.logger.Task(task.ID, "task completed", map[string]any{
		"status": task.Status.String(),
	})

	return nil
}

// GetTask retrieves a task by ID from the store.
func (o *orchestrator) GetTask(taskID string) (*tasks.Task, error) {
	task, err := o.store.Get(taskID)
	if err != nil {
		return nil, errors.NewNotFoundError(fmt.Sprintf("task %s not found", taskID))
	}
	return task, nil
}

// GetTaskStatus returns just the status of a task for lightweight queries.
// This method is more efficient than GetTask when you only need the status,
// as it avoids copying the entire task payload and result.
func (o *orchestrator) GetTaskStatus(taskID string) (string, error) {
	task, err := o.GetTask(taskID)
	if err != nil {
		return "", err
	}
	return task.Status.String(), nil
}
