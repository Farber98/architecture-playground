package execution

import (
	"bytes"
	"context"
	"errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore for testing
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Save(ctx context.Context, task *tasks.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockStore) Get(ctx context.Context, id string) (*tasks.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tasks.Task), args.Error(1)
}

func (m *MockStore) Update(ctx context.Context, id string, status tasks.TaskStatus, result string) error {
	args := m.Called(ctx, id, status, result)
	return args.Error(0)
}

func TestNewDefaultStateManager(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	stateManager := NewDefaultStateManager(mockStore, logger)

	assert.NotNil(t, stateManager)
	assert.Equal(t, mockStore, stateManager.store)
	assert.Equal(t, logger, stateManager.logger)
}

func TestStateManager_TransitionToRunning_Success(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	mockStore.On("Update", context.Background(), task.ID, tasks.StatusRunning, "").Return(nil)

	err := stateManager.TransitionToRunning(context.Background(), execCtx)

	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusRunning, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToRunning_SetStatusError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusDone // Invalid transition
	execCtx := taskContext.NewExecutionContext(task)
	err := stateManager.TransitionToRunning(context.Background(), execCtx)

	assert.Error(t, err)
	// Store should not be called if status transition fails
	mockStore.AssertNotCalled(t, "Update")
}

func TestStateManager_TransitionToRunning_StoreUpdateError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)
	storeErr := errors.New("store update failed")
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusRunning, "").Return(storeErr)

	err := stateManager.TransitionToRunning(context.Background(), execCtx)

	// Should not return error even if store update fails
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusRunning, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToFailed_Success(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusRunning
	task.Result = "error result"
	execCtx := taskContext.NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	mockStore.On("Update", context.Background(), task.ID, tasks.StatusFailed, "error result").Return(nil)

	err := stateManager.TransitionToFailed(context.Background(), execCtx)

	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusFailed, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToFailed_StoreUpdateError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusRunning
	task.Result = "error result"
	execCtx := taskContext.NewExecutionContext(task)
	execCtx.SetError(errors.New("execution failed"))

	storeErr := errors.New("store update failed")
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusFailed, "error result").Return(storeErr)

	err := stateManager.TransitionToFailed(context.Background(), execCtx)

	// Should not return error even if store update fails
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusFailed, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToCompleted_Success(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusRunning
	task.Result = "success result"
	execCtx := taskContext.NewExecutionContext(task)

	mockStore.On("Update", context.Background(), task.ID, tasks.StatusDone, "success result").Return(nil)

	err := stateManager.TransitionToCompleted(context.Background(), execCtx)

	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusDone, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToCompleted_SetStatusError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusFailed // Invalid transition
	execCtx := taskContext.NewExecutionContext(task)

	err := stateManager.TransitionToCompleted(context.Background(), execCtx)

	assert.Error(t, err)
	// Store should not be called if status transition fails
	mockStore.AssertNotCalled(t, "Update")
}

func TestStateManager_TransitionToCompleted_StoreUpdateError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusRunning
	task.Result = "success result"
	execCtx := taskContext.NewExecutionContext(task)

	storeErr := errors.New("store update failed")
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusDone, "success result").Return(storeErr)

	err := stateManager.TransitionToCompleted(context.Background(), execCtx)

	// Should not return error even if store update fails
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusDone, task.Status)
	mockStore.AssertExpectations(t)
}

func TestStateManager_AllTransitions_Integration(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	// Test full workflow: Pending -> Running -> Done
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusRunning, "").Return(nil)
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusDone, mock.AnythingOfType("string")).Return(nil)

	// Transition to running
	err := stateManager.TransitionToRunning(context.Background(), execCtx)
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusRunning, task.Status)

	// Transition to completed
	task.Result = "success"
	err = stateManager.TransitionToCompleted(context.Background(), execCtx)
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusDone, task.Status)

	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToQueued_Success(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	mockStore.On("Update", context.Background(), task.ID, tasks.StatusQueued, "").Return(nil)

	err := stateManager.TransitionToQueued(context.Background(), execCtx)

	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusQueued, task.Status)
	assert.NotNil(t, task.QueuedAt)
	assert.False(t, task.QueuedAt.IsZero())
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToQueued_SetStatusError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	task.Status = tasks.StatusDone // Invalid transition
	execCtx := taskContext.NewExecutionContext(task)

	err := stateManager.TransitionToQueued(context.Background(), execCtx)

	assert.Error(t, err)
	assert.Nil(t, task.QueuedAt) // Should not set QueuedAt if status transition fails
	// Store should not be called if status transition fails
	mockStore.AssertNotCalled(t, "Update")
}

func TestStateManager_TransitionToQueued_StoreUpdateError(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	storeErr := errors.New("store update failed")
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusQueued, "").Return(storeErr)

	err := stateManager.TransitionToQueued(context.Background(), execCtx)

	// TransitionToQueued returns store errors (unlike other transition methods)
	assert.Error(t, err)
	assert.Equal(t, storeErr, err)
	assert.Equal(t, tasks.StatusQueued, task.Status) // Status should still be set
	assert.NotNil(t, task.QueuedAt)                  // QueuedAt should still be set
	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToQueued_SetsQueuedAtTimestamp(t *testing.T) {
	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	// Verify QueuedAt starts as nil
	assert.Nil(t, task.QueuedAt)

	beforeTransition := time.Now()

	mockStore.On("Update", context.Background(), task.ID, tasks.StatusQueued, "").Return(nil)

	err := stateManager.TransitionToQueued(context.Background(), execCtx)

	afterTransition := time.Now()

	assert.NoError(t, err)
	assert.NotNil(t, task.QueuedAt)

	// Verify QueuedAt timestamp is reasonable
	assert.True(t, task.QueuedAt.After(beforeTransition) || task.QueuedAt.Equal(beforeTransition))
	assert.True(t, task.QueuedAt.Before(afterTransition) || task.QueuedAt.Equal(afterTransition))

	mockStore.AssertExpectations(t)
}

func TestStateManager_TransitionToQueued_ErrorHandlingInconsistency(t *testing.T) {
	// This test documents the inconsistent error handling behavior
	// TransitionToQueued returns store errors while other methods don't

	mockStore := &MockStore{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	stateManager := NewDefaultStateManager(mockStore, logger)

	task := tasks.NewTask("test", nil)
	execCtx := taskContext.NewExecutionContext(task)

	storeErr := errors.New("store failed")
	mockStore.On("Update", context.Background(), task.ID, tasks.StatusQueued, "").Return(storeErr)

	err := stateManager.TransitionToQueued(context.Background(), execCtx)

	// TransitionToQueued returns store errors (different from other transitions)
	assert.Error(t, err)
	assert.Equal(t, storeErr, err)

	// Compare with TransitionToRunning behavior - let's test that doesn't return store errors
	task2 := tasks.NewTask("test2", nil)
	execCtx2 := taskContext.NewExecutionContext(task2)

	mockStore.On("Update", context.Background(), task2.ID, tasks.StatusRunning, "").Return(storeErr)

	err2 := stateManager.TransitionToRunning(context.Background(), execCtx2)

	// TransitionToRunning does NOT return store errors (logs but continues)
	assert.NoError(t, err2)

	mockStore.AssertExpectations(t)
}
