package runners_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/runners"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestSynchronousRunner_Run_WithRegisteredHandler(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := tasks.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))

	runner := runners.NewSynchronousRunner(reg, testLogger)

	task := &tasks.Task{
		ID:      "task-123",
		Type:    "print",
		Payload: json.RawMessage(`{"message":"Hello from runner test"}`),
		Status:  "submitted",
	}

	err := runner.Run(task)

	require.NoError(t, err)
	assert.Equal(t, "done", task.Status)
	assert.Equal(t, "printed: Hello from runner test", task.Result)
}

func TestSynchronousRunner_Run_UnregisteredType(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := tasks.NewRegistry()
	runner := runners.NewSynchronousRunner(reg, testLogger)

	task := &tasks.Task{
		ID:      "unknown-task-id",
		Type:    "unknown",
		Payload: json.RawMessage(`{}`),
		Status:  "submitted",
	}

	err := runner.Run(task)

	require.Error(t, err)
	assert.ErrorContains(t, err, "no handler registered")

	// Task status should be updated to failed
	assert.Equal(t, "failed", task.Status)
	require.Contains(t, task.Result, "no handler registered for task type")
}

// ErroringHandler is a test handler that always returns an error
type ErroringHandler struct {
	ErrorToReturn error
}

func (e *ErroringHandler) Run(task *tasks.Task) error {
	return e.ErrorToReturn
}

// TaskErrorHandler returns a structured TaskError
type TaskErrorHandler struct {
	TaskError *errors.TaskError
}

func (t *TaskErrorHandler) Run(task *tasks.Task) error {
	return t.TaskError
}

func TestSynchronousRunner_Run_HandlerReturnsTaskError(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	// Test that structured TaskErrors are preserved
	reg := tasks.NewRegistry()

	expectedError := errors.NewValidationError("invalid payload", map[string]any{
		"field": "missing_value",
	})

	reg.Register("failing", &TaskErrorHandler{TaskError: expectedError})
	runner := runners.NewSynchronousRunner(reg, testLogger)

	task := &tasks.Task{
		ID:      "failing-task",
		Type:    "failing",
		Payload: json.RawMessage(`{}`),
		Status:  "submitted",
	}

	err := runner.Run(task)

	require.Error(t, err)

	// Verify the original TaskError is preserved
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ValidationError, taskErr.Type)
	assert.Equal(t, "invalid payload", taskErr.Message)
	assert.Equal(t, "missing_value", taskErr.Details["field"])

	// Task status should be updated to failed when handler returns error
	assert.Equal(t, "failed", task.Status)
	require.Contains(t, task.Result, "task execution failed:")
}

func TestSynchronousRunner_Run_HandlerReturnsGenericError(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	// Test that generic errors are wrapped as ExecutionErrors
	reg := tasks.NewRegistry()

	genericError := fmt.Errorf("an error")
	reg.Register("erroring-task", &ErroringHandler{ErrorToReturn: genericError})
	runner := runners.NewSynchronousRunner(reg, testLogger)

	task := &tasks.Task{
		ID:      "erroring-task-123",
		Type:    "erroring-task",
		Payload: json.RawMessage(`{"erroring":"task"}`),
		Status:  "submitted",
	}

	err := runner.Run(task)

	require.Error(t, err)

	// Verify generic error is wrapped as ExecutionError
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ExecutionError, taskErr.Type)
	assert.ErrorContains(t, err, "task execution failed")

	// Verify error details include context
	assert.Equal(t, "erroring-task-123", taskErr.Details["task_id"])
	assert.Equal(t, "erroring-task", taskErr.Details["task_type"])
	assert.Equal(t, "an error", taskErr.Details["error"])

	// Task status should be updated to failed
	assert.Equal(t, "failed", task.Status)
	require.Contains(t, task.Result, "task execution failed")
}
