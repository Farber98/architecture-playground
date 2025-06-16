package runners_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/handlers"
	handlerRegistry "task-orchestrator/tasks/registry"
	"task-orchestrator/tasks/runners"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestSynchronousRunner_Run_WithRegisteredHandler(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))

	runner := runners.NewSynchronousRunner(reg)

	// Use NewTask to create task with proper initial status
	task := tasks.NewTask("print", json.RawMessage(`{"message":"Hello from runner test"}`))

	// Simulate orchestrator setting task to running before calling runner
	require.NoError(t, task.SetStatus(tasks.StatusRunning))

	err := runner.Run(context.Background(), task)

	require.NoError(t, err)
	// Handler should have set the result, but status management is orchestrator's job
	assert.Equal(t, "printed: Hello from runner test", task.Result)

	// Status should still be running - orchestrator will set it to done
	assert.Equal(t, tasks.StatusRunning, task.Status)
}

func TestSynchronousRunner_Run_UnregisteredType(t *testing.T) {
	reg := handlerRegistry.NewRegistry()
	runner := runners.NewSynchronousRunner(reg)

	// Use NewTask to create task with proper initial status
	task := tasks.NewTask("unknown", json.RawMessage(`{}`))

	// Simulate orchestrator setting task to running before calling runner
	require.NoError(t, task.SetStatus(tasks.StatusRunning))

	err := runner.Run(context.Background(), task)

	require.Error(t, err)
	assert.ErrorContains(t, err, "no handler registered")

	// Runner doesn't set status or result - orchestrator will handle it
	assert.Equal(t, tasks.StatusRunning, task.Status) // Still running
	assert.Equal(t, "", task.Result)                  // No result set by runner

	// Error should be a NotFoundError
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.NotFoundError, taskErr.Type)
}

// Updated test handlers to follow orchestrator-managed pattern
// ErroringHandler is a test handler that always returns an error
type ErroringHandler struct {
	ErrorToReturn error
	SetResult     bool   // Whether to set task.Result
	CustomResult  string // Custom result to set
}

func (e *ErroringHandler) Run(ctx context.Context, task *tasks.Task) error {
	// Follow proper state transitions (orchestrator set to running already)
	if e.SetResult {
		task.Result = e.CustomResult
	}
	// Don't set status - let orchestrator handle it
	return e.ErrorToReturn
}

// TaskErrorHandler returns a structured TaskError
type TaskErrorHandler struct {
	TaskError    *errors.TaskError
	SetResult    bool   // Whether to set task.Result
	CustomResult string // Custom result to set
}

func (t *TaskErrorHandler) Run(ctx context.Context, task *tasks.Task) error {
	// Follow proper state transitions (orchestrator set to running already)
	if t.SetResult {
		task.Result = t.CustomResult
	}
	// Don't set status - let orchestrator handle it
	return t.TaskError
}

func TestSynchronousRunner_Run_HandlerReturnsTaskError(t *testing.T) {
	// Test that structured TaskErrors are preserved
	reg := handlerRegistry.NewRegistry()

	expectedError := errors.NewValidationError("invalid payload", map[string]any{
		"field": "missing_value",
	})

	reg.Register("failing", &TaskErrorHandler{
		TaskError:    expectedError,
		SetResult:    true,
		CustomResult: "handler set this error result",
	})
	runner := runners.NewSynchronousRunner(reg)

	// Use NewTask to create task with proper initial status
	task := tasks.NewTask("failing", json.RawMessage(`{}`))

	// Simulate orchestrator setting task to running before calling runner
	require.NoError(t, task.SetStatus(tasks.StatusRunning))

	err := runner.Run(context.Background(), task)

	require.Error(t, err)

	// Verify the original TaskError is preserved
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ValidationError, taskErr.Type)
	assert.Equal(t, "invalid payload", taskErr.Message)
	assert.Equal(t, "missing_value", taskErr.Details["field"])

	// Runner doesn't set status - orchestrator will handle it
	assert.Equal(t, tasks.StatusRunning, task.Status) // Still running

	// Handler set the result
	assert.Equal(t, "handler set this error result", task.Result)
}

func TestSynchronousRunner_Run_HandlerReturnsGenericError(t *testing.T) {
	// Test that generic errors are wrapped as ExecutionErrors
	reg := handlerRegistry.NewRegistry()

	genericError := fmt.Errorf("database connection failed")
	reg.Register("erroring-task", &ErroringHandler{
		ErrorToReturn: genericError,
		SetResult:     false, // Don't set result - let orchestrator handle it
	})
	runner := runners.NewSynchronousRunner(reg)

	// Use NewTask to create task with proper initial status
	task := tasks.NewTask("erroring-task", json.RawMessage(`{"erroring":"task"}`))

	// Simulate orchestrator setting task to running before calling runner
	require.NoError(t, task.SetStatus(tasks.StatusRunning))

	err := runner.Run(context.Background(), task)

	require.Error(t, err)

	// Verify generic error is wrapped as ExecutionError
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ExecutionError, taskErr.Type)
	assert.ErrorContains(t, err, "task execution failed")

	// Verify error details include context
	assert.Equal(t, task.ID, taskErr.Details["task_id"])
	assert.Equal(t, "erroring-task", taskErr.Details["task_type"])
	assert.Equal(t, "database connection failed", taskErr.Details["error"])

	// Runner doesn't set status or result - orchestrator will handle it
	assert.Equal(t, tasks.StatusRunning, task.Status) // Still running
	assert.Equal(t, "", task.Result)                  // No result set
}

// Add test for successful handler that sets result
func TestSynchronousRunner_Run_HandlerSetsResult(t *testing.T) {
	reg := handlerRegistry.NewRegistry()
	reg.Register("success", &ErroringHandler{
		ErrorToReturn: nil, // Success case
		SetResult:     true,
		CustomResult:  "handler completed successfully",
	})
	runner := runners.NewSynchronousRunner(reg)

	// Use NewTask to create task with proper initial status
	task := tasks.NewTask("success", json.RawMessage(`{"test":"data"}`))

	// Simulate orchestrator setting task to running before calling runner
	require.NoError(t, task.SetStatus(tasks.StatusRunning))

	err := runner.Run(context.Background(), task)

	require.NoError(t, err)

	// Handler set the result, status remains running (orchestrator will set to done)
	assert.Equal(t, "handler completed successfully", task.Result)
	assert.Equal(t, tasks.StatusRunning, task.Status)
}
