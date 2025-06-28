package runners_test

import (
	"context"
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/handlers"
	handlerRegistry "task-orchestrator/tasks/registry"
	"task-orchestrator/tasks/runners"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

// MockTaskQueue for testing without Redis
type MockTaskQueue struct {
	enqueuedTasks []*taskContext.ExecutionContext
	shouldError   bool
	errorToReturn error
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	if m.shouldError {
		return m.errorToReturn
	}
	m.enqueuedTasks = append(m.enqueuedTasks, execCtx)
	return nil
}

func (m *MockTaskQueue) Dequeue(ctx context.Context) (*taskContext.ExecutionContext, error) {
	return nil, nil // Not needed for async runner tests
}

func (m *MockTaskQueue) GetQueueDepth(ctx context.Context) (int64, error) {
	return int64(len(m.enqueuedTasks)), nil
}

func (m *MockTaskQueue) Close() error {
	return nil
}

func TestAsynchronousRunner_Run_WithRegisteredHandler(t *testing.T) {
	// Setup
	mockQueue := &MockTaskQueue{}
	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(nil)) // Handler doesn't matter for async

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create execution context
	task := tasks.NewTask("print", json.RawMessage(`{"message":"Hello async"}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// Run
	err := runner.Run(context.Background(), execCtx)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, len(mockQueue.enqueuedTasks))
	assert.Equal(t, "print", mockQueue.enqueuedTasks[0].Task.Type)
	assert.Equal(t, task.ID, mockQueue.enqueuedTasks[0].Task.ID)
}

func TestAsynchronousRunner_Run_UnregisteredHandler(t *testing.T) {
	// Setup
	mockQueue := &MockTaskQueue{}
	reg := handlerRegistry.NewRegistry()
	// Don't register any handlers

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create execution context
	task := tasks.NewTask("unknown", json.RawMessage(`{"test":"data"}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// Run
	err := runner.Run(context.Background(), execCtx)

	// Verify
	require.Error(t, err)
	assert.ErrorContains(t, err, "no handler registered for task type: unknown")

	// Should be NotFoundError
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.NotFoundError, taskErr.Type)

	// Should not have enqueued anything
	assert.Equal(t, 0, len(mockQueue.enqueuedTasks))
}

func TestAsynchronousRunner_Run_QueueEnqueueError(t *testing.T) {
	// Setup - queue that returns error
	queueError := fmt.Errorf("queue connection failed")
	mockQueue := &MockTaskQueue{
		shouldError:   true,
		errorToReturn: queueError,
	}
	reg := handlerRegistry.NewRegistry()
	reg.Register("test", handlers.NewPrintHandler(nil))

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create execution context
	task := tasks.NewTask("test", json.RawMessage(`{"data":"test"}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// Run
	err := runner.Run(context.Background(), execCtx)

	// Verify
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to enqueue task")

	// Should be ExecutionError wrapping the queue error
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ExecutionError, taskErr.Type)

	// Verify error details
	assert.Equal(t, task.ID, taskErr.Details["task_id"])
	assert.Equal(t, "test", taskErr.Details["task_type"])
	assert.Equal(t, queueError.Error(), taskErr.Details["error"])

	// Should not have enqueued anything
	assert.Equal(t, 0, len(mockQueue.enqueuedTasks))
}

func TestAsynchronousRunner_Run_QueueReturnsTaskError(t *testing.T) {
	// Setup - queue that returns TaskError
	expectedErr := errors.NewValidationError("queue validation failed", map[string]any{
		"reason": "payload too large",
	})
	mockQueue := &MockTaskQueue{
		shouldError:   true,
		errorToReturn: expectedErr,
	}
	reg := handlerRegistry.NewRegistry()
	reg.Register("test", handlers.NewPrintHandler(nil))

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create execution context
	task := tasks.NewTask("test", json.RawMessage(`{"large":"payload"}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// Run
	err := runner.Run(context.Background(), execCtx)

	// Verify
	require.Error(t, err)

	// Should preserve the original TaskError
	taskErr, ok := errors.IsTaskError(err)
	require.True(t, ok, "expected TaskError")
	assert.Equal(t, errors.ValidationError, taskErr.Type)
	assert.Equal(t, "queue validation failed", taskErr.Message)
	assert.Equal(t, "payload too large", taskErr.Details["reason"])

	// Should not have enqueued anything
	assert.Equal(t, 0, len(mockQueue.enqueuedTasks))
}

func TestAsynchronousRunner_Run_MultipleTasksEnqueued(t *testing.T) {
	// Setup
	mockQueue := &MockTaskQueue{}
	reg := handlerRegistry.NewRegistry()
	reg.Register("type1", handlers.NewPrintHandler(nil))
	reg.Register("type2", handlers.NewPrintHandler(nil))

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create multiple tasks
	testTasks := []*taskContext.ExecutionContext{
		taskContext.NewExecutionContext(tasks.NewTask("type1", json.RawMessage(`{"msg":"first"}`))),
		taskContext.NewExecutionContext(tasks.NewTask("type2", json.RawMessage(`{"msg":"second"}`))),
		taskContext.NewExecutionContext(tasks.NewTask("type1", json.RawMessage(`{"msg":"third"}`))),
	}

	// Enqueue all tasks
	for _, execCtx := range testTasks {
		require.NoError(t, execCtx.Task.SetStatus(tasks.StatusRunning))
		err := runner.Run(context.Background(), execCtx)
		require.NoError(t, err)
	}

	// Verify all were enqueued
	assert.Equal(t, 3, len(mockQueue.enqueuedTasks))

	// Verify order and types
	assert.Equal(t, "type1", mockQueue.enqueuedTasks[0].Task.Type)
	assert.Equal(t, "type2", mockQueue.enqueuedTasks[1].Task.Type)
	assert.Equal(t, "type1", mockQueue.enqueuedTasks[2].Task.Type)
}

func TestAsynchronousRunner_Run_ContextCancellation(t *testing.T) {
	// Setup
	mockQueue := &MockTaskQueue{}
	reg := handlerRegistry.NewRegistry()
	reg.Register("test", handlers.NewPrintHandler(nil))

	runner := runners.NewAsynchronousRunner(mockQueue, reg)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create execution context
	task := tasks.NewTask("test", json.RawMessage(`{"test":"data"}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// Run with cancelled context
	err := runner.Run(ctx, execCtx)

	// This should still work since async runner doesn't block on context
	// It just validates and enqueues
	require.NoError(t, err)
	assert.Equal(t, 1, len(mockQueue.enqueuedTasks))
}

func TestAsynchronousRunner_Run_ImplementsRunnerInterface(t *testing.T) {
	// Verify AsynchronousRunner implements Runner interface
	mockQueue := &MockTaskQueue{}
	reg := handlerRegistry.NewRegistry()

	var runner runners.Runner = runners.NewAsynchronousRunner(mockQueue, reg)

	// Should be able to call Run
	task := tasks.NewTask("test", json.RawMessage(`{}`))
	require.NoError(t, task.SetStatus(tasks.StatusRunning))
	execCtx := taskContext.NewExecutionContext(task)

	// This will fail (no handler), but proves interface is implemented
	err := runner.Run(context.Background(), execCtx)
	require.Error(t, err) // Expected - no handler registered
}
