package execution

import (
	"errors"
	taskErrors "task-orchestrator/errors"
	"task-orchestrator/tasks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultResultHandler_HandleSuccess(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	task.Result = "existing result"

	execCtx := NewExecutionContext(task)
	handler.HandleSuccess(execCtx)

	assert.True(t, execCtx.IsSuccess())
	assert.Equal(t, "existing result", task.Result)
	assert.False(t, execCtx.Metadata["has_error"].(bool))
}

func TestDefaultResultHandler_HandleFailure_WithExistingResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	task.Result = "custom error result"

	execCtx := NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	handler.HandleFailure(execCtx)

	// Should NOT override existing result
	assert.Equal(t, "custom error result", task.Result)
}

func TestDefaultResultHandler_HandleFailure_WithoutResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	handler.HandleFailure(execCtx)

	// Should set default error result
	assert.Equal(t, "execution failed: execution failed", task.Result)
}

func TestDefaultResultHandler_HandleFailure_WithTaskError(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := NewExecutionContext(task)
	taskErr := taskErrors.NewValidationError("validation error")
	execCtx.SetError(taskErr)

	handler.HandleFailure(execCtx)

	// Should format task error properly
	assert.Contains(t, task.Result, "validation")
	assert.Contains(t, task.Result, "validation error")
}

func TestDefaultResultHandler_HandleFailure_EmptyTaskResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	assert.Equal(t, "", task.Result) // Verify starting state

	execCtx := NewExecutionContext(task)
	execCtx.SetError(errors.New("test error"))

	handler.HandleFailure(execCtx)

	assert.NotEqual(t, "", task.Result)
	assert.Contains(t, task.Result, "test error")
}

func TestDefaultResultHandler_HandleSuccess_SetsMetadata(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := NewExecutionContext(task)
	handler.HandleSuccess(execCtx)

	assert.True(t, execCtx.IsSuccess())
	assert.False(t, execCtx.Metadata["has_error"].(bool))
	assert.False(t, execCtx.EndTime.IsZero())
}
