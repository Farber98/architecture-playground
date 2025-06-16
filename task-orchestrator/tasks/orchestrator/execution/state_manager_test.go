package execution

import (
	"bytes"
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

func (m *MockStore) Save(task *tasks.Task) error {
	args := m.Called(task)
	return args.Error(0)
}

func (m *MockStore) Get(id string) (*tasks.Task, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tasks.Task), args.Error(1)
}

func (m *MockStore) Update(id string, status tasks.TaskStatus, result string) error {
	args := m.Called(id, status, result)
	return args.Error(0)
}

func (m *MockStore) GetAll() ([]*tasks.Task, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*tasks.Task), args.Error(1)
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
	ctx := NewExecutionContext(task)

	mockStore.On("Update", task.ID, tasks.StatusRunning, "").Return(nil)

	err := stateManager.TransitionToRunning(ctx)

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
	ctx := NewExecutionContext(task)

	err := stateManager.TransitionToRunning(ctx)

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
	ctx := NewExecutionContext(task)

	storeErr := errors.New("store update failed")
	mockStore.On("Update", task.ID, tasks.StatusRunning, "").Return(storeErr)

	err := stateManager.TransitionToRunning(ctx)

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
	ctx := NewExecutionContext(task)
	ctx.SetError(errors.New("execution failed"))

	mockStore.On("Update", task.ID, tasks.StatusFailed, "error result").Return(nil)

	err := stateManager.TransitionToFailed(ctx)

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
	ctx := NewExecutionContext(task)
	ctx.SetError(errors.New("execution failed"))

	storeErr := errors.New("store update failed")
	mockStore.On("Update", task.ID, tasks.StatusFailed, "error result").Return(storeErr)

	err := stateManager.TransitionToFailed(ctx)

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
	ctx := NewExecutionContext(task)

	mockStore.On("Update", task.ID, tasks.StatusDone, "success result").Return(nil)

	err := stateManager.TransitionToCompleted(ctx)

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
	ctx := NewExecutionContext(task)

	err := stateManager.TransitionToCompleted(ctx)

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
	ctx := NewExecutionContext(task)

	storeErr := errors.New("store update failed")
	mockStore.On("Update", task.ID, tasks.StatusDone, "success result").Return(storeErr)

	err := stateManager.TransitionToCompleted(ctx)

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
	ctx := NewExecutionContext(task)

	// Test full workflow: Pending -> Running -> Done
	mockStore.On("Update", task.ID, tasks.StatusRunning, "").Return(nil)
	mockStore.On("Update", task.ID, tasks.StatusDone, mock.AnythingOfType("string")).Return(nil)

	// Transition to running
	err := stateManager.TransitionToRunning(ctx)
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusRunning, task.Status)

	// Transition to completed
	task.Result = "success"
	err = stateManager.TransitionToCompleted(ctx)
	assert.NoError(t, err)
	assert.Equal(t, tasks.StatusDone, task.Status)

	mockStore.AssertExpectations(t)
}
