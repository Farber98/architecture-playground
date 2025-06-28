package execution

import (
	"errors"
	taskErrors "task-orchestrator/errors"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultResultHandler_HandleSuccess(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	task.Result = "existing result"

	execCtx := taskContext.NewExecutionContext(task)
	handler.HandleSuccess(execCtx)

	assert.Nil(t, execCtx.Error)
	assert.False(t, execCtx.EndTime.IsZero())
	assert.Equal(t, "existing result", task.Result)
}

func TestDefaultResultHandler_HandleFailure_WithExistingResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	task.Result = "custom error result"

	execCtx := taskContext.NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	handler.HandleFailure(execCtx)

	// Should NOT override existing result
	assert.Equal(t, "custom error result", task.Result)
	assert.NotNil(t, execCtx.Error)
}

func TestDefaultResultHandler_HandleFailure_WithoutResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := taskContext.NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	handler.HandleFailure(execCtx)

	// Should set default error result
	assert.Equal(t, "execution failed: execution failed", task.Result)
	assert.NotNil(t, execCtx.Error)
}

func TestDefaultResultHandler_HandleFailure_WithTaskError(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := taskContext.NewExecutionContext(task)
	taskErr := taskErrors.NewValidationError("validation error", nil)
	execCtx.SetError(taskErr)

	handler.HandleFailure(execCtx)

	// Should format task error properly
	assert.Contains(t, task.Result, "validation")
	assert.Contains(t, task.Result, "validation error")
	assert.NotNil(t, execCtx.Error)
}

func TestDefaultResultHandler_HandleFailure_EmptyTaskResult(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)
	assert.Equal(t, "", task.Result) // Verify starting state

	execCtx := taskContext.NewExecutionContext(task)
	execCtx.SetError(errors.New("test error"))

	handler.HandleFailure(execCtx)

	assert.NotEqual(t, "", task.Result)
	assert.Contains(t, task.Result, "test error")
	assert.NotNil(t, execCtx.Error)
}

func TestDefaultResultHandler_HandleSuccess_SetsMetadata(t *testing.T) {
	handler := NewDefaultResultHandler()
	task := tasks.NewTask("test", nil)

	execCtx := taskContext.NewExecutionContext(task)

	assert.True(t, execCtx.EndTime.IsZero())
	handler.HandleSuccess(execCtx)
	assert.Nil(t, execCtx.Error)
	assert.False(t, execCtx.EndTime.IsZero())
}
