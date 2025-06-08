package orchestrator

import (
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"

	"github.com/google/uuid"
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
	task := &tasks.Task{
		ID:      uuid.New().String(),
		Type:    taskType,
		Payload: payload,
		Status:  "submitted",
		Result:  "",
	}

	// Persist initial task state
	if err := o.store.Save(task); err != nil {
		o.logger.Task("failed to save task", task.ID, map[string]any{
			"error": err.Error(),
		})
		return task, errors.NewInternalError("failed to save task")
	}

	o.logger.Task("task submitted", task.ID, map[string]any{
		"task_type":    task.Type,
		"runner_type":  fmt.Sprintf("%T", o.runner),
		"payload_size": len(task.Payload),
	})

	return task, o.executeTask(task)
}

// executeTask handles the execution using the configured runner strategy.
func (o *orchestrator) executeTask(task *tasks.Task) error {
	o.logger.Task("running task", task.ID, map[string]any{
		"runner_type": fmt.Sprintf("%T", o.runner),
		"status":      task.Status,
	})

	// Delegate to runner strategy
	// This is where the behavior changes based on the injected runner
	if err := o.runner.Run(task); err != nil {
		o.logger.Task("task execution failed", task.ID, map[string]any{
			"error":       err.Error(),
			"runner_type": fmt.Sprintf("%T", o.runner),
		})

		if updateErr := o.store.Update(task.ID, "failed", fmt.Sprintf("execution failed: %s", err.Error())); updateErr != nil {
			o.logger.Task("failed to update task failure state", task.ID, map[string]any{
				"update_error":   updateErr.Error(),
				"original_error": err.Error(),
			})
		}

		return err
	}

	// Update final state
	if err := o.store.Update(task.ID, task.Status, task.Result); err != nil {
		o.logger.Task("failed to update final task state", task.ID, map[string]any{
			"error":        err.Error(),
			"final_status": task.Status,
		})
		// the task exec was succesful, what failed was the update. We continue.
	}

	o.logger.Task("task completed successfully", task.ID, map[string]any{
		"final_status": task.Status,
		"runner_type":  fmt.Sprintf("%T", o.runner),
		"result_size":  len(task.Result),
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
	return task.Status, nil
}
