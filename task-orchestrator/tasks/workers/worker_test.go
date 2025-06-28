package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/orchestrator/execution"
	"task-orchestrator/tasks/queue"
	"task-orchestrator/tasks/registry"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
)

// Mock implementations for testing

type MockTaskQueue struct {
	mock.Mock
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockTaskQueue) Dequeue(ctx context.Context) (*taskContext.ExecutionContext, error) {
	args := m.Called(ctx)

	// Handle function return type
	if len(args) == 1 && args.Get(0) != nil {
		// Check if it's a function
		if fn, ok := args.Get(0).(func(context.Context) (*taskContext.ExecutionContext, error)); ok {
			return fn(ctx)
		}
	}

	// Handle normal return values
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taskContext.ExecutionContext), args.Error(1)
}

func (m *MockTaskQueue) GetQueueDepth(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockStateManager struct {
	mock.Mock
}

func (m *MockStateManager) TransitionToRunning(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockStateManager) TransitionToQueued(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockStateManager) TransitionToFailed(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockStateManager) TransitionToCompleted(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) Run(ctx context.Context, task *tasks.Task) error {
	args := m.Called(ctx, task)
	// Simulate setting result for successful execution
	if args.Error(0) == nil {
		task.Result = "Mock handler completed successfully"
	}
	return args.Error(0)
}

// Helper function to create a test worker
func createTestWorker(queue queue.TaskQueue, stateManager execution.StateManager, registry *registry.HandlerRegistry) (*Worker, *logger.Logger) {
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	worker := NewWorker(1, queue, stateManager, registry, logger)
	return worker, logger
}

func TestWorker_NewWorker(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}
	registry := registry.NewRegistry()

	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	assert.Assert(t, worker != nil)
	assert.Equal(t, 1, worker.id)
	assert.Assert(t, worker.queue != nil)
	assert.Assert(t, worker.stateManager != nil)
	assert.Assert(t, worker.registry != nil)
	assert.Assert(t, worker.logger != nil)
	assert.Assert(t, worker.stopCh != nil)
}

func TestWorker_ProcessTask_Success(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Create registry with mock handler
	registry := registry.NewRegistry()
	mockHandler := &MockHandler{}
	registry.Register("test", mockHandler)

	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Create test task and execution context
	task := tasks.NewTask("test", json.RawMessage(`{"message":"test"}`))
	task.Status = tasks.StatusQueued
	execCtx := taskContext.NewExecutionContext(task)

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(nil)
	mockHandler.On("Run", mock.Anything, task).Return(nil) // Successful execution
	mockStateManager.On("TransitionToCompleted", mock.Anything, execCtx).Return(nil)

	// Process one task
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Verify all expectations met
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
	mockHandler.AssertExpectations(t)

	// Verify task result was set
	assert.Equal(t, "Mock handler completed successfully", task.Result)
}

func TestWorker_ProcessTask_HandlerNotFound(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Create registry WITHOUT registering the handler
	registry := registry.NewRegistry()

	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Create test task for unknown handler
	task := tasks.NewTask("unknown_handler", json.RawMessage(`{}`))
	task.Status = tasks.StatusQueued
	execCtx := taskContext.NewExecutionContext(task)

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(nil)
	mockStateManager.On("TransitionToFailed", mock.Anything, execCtx).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Verify failure handling
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)

	// Verify error was set in execution context
	assert.Assert(t, execCtx.Error != nil)
	assert.Assert(t, execCtx.Task.Result != "")
	assert.Assert(t, execCtx.Task.Result == "no handler registered for task type: unknown_handler")
}

func TestWorker_ProcessTask_HandlerExecutionFails(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Create registry with failing handler
	registry := registry.NewRegistry()
	mockHandler := &MockHandler{}
	registry.Register("failing_task", mockHandler)

	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	task := tasks.NewTask("failing_task", json.RawMessage(`{}`))
	task.Status = tasks.StatusQueued
	execCtx := taskContext.NewExecutionContext(task)

	handlerErr := errors.New("handler execution failed")

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(nil)
	mockHandler.On("Run", mock.Anything, task).Return(handlerErr) // Handler fails
	mockStateManager.On("TransitionToFailed", mock.Anything, execCtx).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Verify failure handling
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
	mockHandler.AssertExpectations(t)

	// Verify error details
	assert.Assert(t, execCtx.Error != nil)
	assert.Equal(t, handlerErr.Error(), execCtx.Task.Result)
}

func TestWorker_ProcessTask_DequeueError(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Setup dequeue to fail
	dequeueErr := errors.New("Redis connection failed")
	mockQueue.On("Dequeue", mock.Anything).Return(nil, dequeueErr).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Verify queue was called but state manager was not
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertNotCalled(t, "TransitionToRunning")
	mockStateManager.AssertNotCalled(t, "TransitionToCompleted")
	mockStateManager.AssertNotCalled(t, "TransitionToFailed")
}

func TestWorker_ProcessTask_DequeueContextCancelled(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Setup context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Setup dequeue to return context cancelled error
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Once()

	worker.processNextTask(ctx)

	// Should exit gracefully without additional logging
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertNotCalled(t, "TransitionToRunning")
}

func TestWorker_ProcessTask_TransitionToRunningFails(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	task := tasks.NewTask("test", json.RawMessage(`{}`))
	execCtx := taskContext.NewExecutionContext(task)

	transitionErr := errors.New("database connection failed")

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(transitionErr)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Should not proceed to execution if state transition fails
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
	mockStateManager.AssertNotCalled(t, "TransitionToCompleted")
	mockStateManager.AssertNotCalled(t, "TransitionToFailed")
}

func TestWorker_ProcessTask_TransitionToCompletedFails(t *testing.T) {
	// Setup mocks
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	mockHandler := &MockHandler{}
	registry.Register("test", mockHandler)

	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	task := tasks.NewTask("test", json.RawMessage(`{}`))
	execCtx := taskContext.NewExecutionContext(task)

	transitionErr := errors.New("database update failed")

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(nil)
	mockHandler.On("Run", mock.Anything, task).Return(nil)                                     // Success
	mockStateManager.On("TransitionToCompleted", mock.Anything, execCtx).Return(transitionErr) // But completion fails

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Task execution succeeded, but final state update failed (should be logged)
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
	mockHandler.AssertExpectations(t)
}

func TestWorker_Start_ContextCancellation(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Use a context with a very short timeout instead of manual cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Setup mock to return context error when called
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.DeadlineExceeded)

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	// Worker should stop when context times out
	select {
	case <-done:
		// Success - worker stopped
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Worker did not stop within timeout")
	}

	mockQueue.AssertExpectations(t)
}

func TestWorker_Start_Stop(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	ctx := context.Background()

	// Use a channel to control when dequeue returns
	blockDequeue := make(chan struct{})

	mockQueue.On("Dequeue", mock.Anything).Return(func(ctx context.Context) (*taskContext.ExecutionContext, error) {
		// Block until test closes the channel or worker is stopped
		<-blockDequeue
		return nil, errors.New("stopped")
	})

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	// Give worker time to start
	time.Sleep(10 * time.Millisecond)

	// Stop worker (this should close stopCh)
	worker.Stop()

	// Unblock dequeue
	close(blockDequeue)

	// Worker should stop gracefully
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Worker did not stop within timeout")
	}

	mockQueue.AssertExpectations(t)
}

func TestWorker_Stop_AlreadyStopped(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	// Stop worker multiple times - should not panic
	worker.Stop()
	worker.Stop()
	worker.Stop()

	// Should not panic or cause issues
}

func TestWorker_Integration_WithRealHandler(t *testing.T) {
	// Test with real print handler for integration testing
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	// Use real registry with real print handler
	registry := registry.NewRegistry()
	registry.Register("print", handlers.NewPrintHandler(logger))

	worker := NewWorker(1, mockQueue, mockStateManager, registry, logger)

	// Create real print task
	task := tasks.NewTask("print", json.RawMessage(`{"message":"integration test"}`))
	execCtx := taskContext.NewExecutionContext(task)

	// Setup expectations
	mockQueue.On("Dequeue", mock.Anything).Return(execCtx, nil).Once()
	mockStateManager.On("TransitionToRunning", mock.Anything, execCtx).Return(nil)
	mockStateManager.On("TransitionToCompleted", mock.Anything, execCtx).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.processNextTask(ctx)

	// Verify expectations
	mockQueue.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)

	// Verify print handler executed
	assert.Assert(t, task.Result != "")
	logOutput := buf.String()
	assert.Assert(t, logOutput != "")
}

func TestWorker_HandleTaskSuccess_StateTransitionFails(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	task := tasks.NewTask("test", json.RawMessage(`{}`))
	execCtx := taskContext.NewExecutionContext(task)

	transitionErr := errors.New("state transition failed")
	mockStateManager.On("TransitionToCompleted", mock.Anything, execCtx).Return(transitionErr)

	ctx := context.Background()

	// Should not panic even if state transition fails
	worker.handleTaskSuccess(ctx, execCtx)

	mockStateManager.AssertExpectations(t)
}

func TestWorker_HandleTaskFailure_StateTransitionFails(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	registry := registry.NewRegistry()
	worker, _ := createTestWorker(mockQueue, mockStateManager, registry)

	task := tasks.NewTask("test", json.RawMessage(`{}`))
	execCtx := taskContext.NewExecutionContext(task)

	originalErr := errors.New("original task error")
	transitionErr := errors.New("state transition failed")

	mockStateManager.On("TransitionToFailed", mock.Anything, execCtx).Return(transitionErr)

	ctx := context.Background()

	// Should not panic even if state transition fails
	worker.handleTaskFailure(ctx, execCtx, originalErr)

	mockStateManager.AssertExpectations(t)

	// Original error should still be set
	assert.Assert(t, execCtx != nil)
	assert.Equal(t, originalErr.Error(), execCtx.Task.Result)
}
