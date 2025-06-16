package orchestrator

import (
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	execution "task-orchestrator/tasks/orchestrator/execution"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"
)

// Orchestrator provides a high-level API for task management.
// It coordinates task lifecycle without handling execution complexity.
type Orchestrator interface {
	// SubmitTask accepts user requests and ensures they are processed immediately.
	// Returns the task even on execution failure to allow status inspection.
	SubmitTask(taskType string, payload json.RawMessage) (*tasks.Task, error)

	// GetTask retrieves a task by ID from the underlying storage.
	GetTask(taskID string) (*tasks.Task, error)

	// GetTaskStatus returns just the status of a task for lightweight queries.
	// This is more efficient than GetTask when you only need the status.
	GetTaskStatus(taskID string) (string, error)
}

// orchestrator separates API concerns from execution complexity.
// This enables different execution strategies without changing the public interface.
type orchestrator struct {
	store    store.TaskStore
	workflow execution.ExecutionWorkflow
	logger   *logger.Logger
}

// Compile-time interface compliance check
var _ Orchestrator = (*orchestrator)(nil)

// NewDefaultOrchestrator creates an orchestrator with defaults.
// Uses dependency injection to enable testing and future configuration flexibility.
func NewDefaultOrchestrator(
	store store.TaskStore,
	runner runners.Runner,
	logger *logger.Logger,
) Orchestrator {
	// Compose the execution workflow with default components
	stateManager := execution.NewDefaultStateManager(store, logger)
	resultHandler := execution.NewDefaultResultHandler()
	workflow := execution.NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	return &orchestrator{
		store:    store,
		workflow: workflow,
		logger:   logger,
	}
}

// SubmitTask creates and persists a task. It delegates execution to execution module.
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
		"payload_size": len(task.Payload),
	})

	// Delegate execution
	if err := o.executeTask(task); err != nil {
		// Task execution failed, but we still return the task with its current state
		return task, err
	}

	return task, nil
}

// executeTask handles the execution using the configured workflow.
func (o *orchestrator) executeTask(task *tasks.Task) error {
	return o.workflow.Execute(task)
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
