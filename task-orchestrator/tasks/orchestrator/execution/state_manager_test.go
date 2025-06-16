package execution

import (
	"bytes"
	"context"
	"errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"testing"

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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)
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
	execCtx := NewExecutionContext(task)
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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)

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
	execCtx := NewExecutionContext(task)

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
