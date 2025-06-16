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

// MockRunner for testing
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(ctx context.Context, task *tasks.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

// MockStateManager for testing
type MockStateManager struct {
	mock.Mock
}

func (m *MockStateManager) TransitionToRunning(ctx context.Context, execCtx *ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockStateManager) TransitionToFailed(ctx context.Context, execCtx *ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

func (m *MockStateManager) TransitionToCompleted(ctx context.Context, execCtx *ExecutionContext) error {
	args := m.Called(ctx, execCtx)
	return args.Error(0)
}

// MockResultHandler for testing
type MockResultHandler struct {
	mock.Mock
}

func (m *MockResultHandler) HandleSuccess(execCtx *ExecutionContext) {
	m.Called(execCtx)
}

func (m *MockResultHandler) HandleFailure(execCtx *ExecutionContext) {
	m.Called(execCtx)
}

func TestNewDefaultExecutionWorkflow(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	assert.NotNil(t, workflow)
	assert.Equal(t, runner, workflow.runner)
	assert.Equal(t, stateManager, workflow.stateManager)
	assert.Equal(t, resultHandler, workflow.resultHandler)
	assert.Equal(t, logger, workflow.logger)
}

func TestWorkflow_Execute_Success(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)

	// Setup expectations
	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(nil)
	runner.On("Run", context.Background(), task).Return(nil)
	resultHandler.On("HandleSuccess", mock.AnythingOfType("*execution.ExecutionContext"))
	stateManager.On("TransitionToCompleted", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(nil)

	err := workflow.Execute(context.Background(), task)

	assert.NoError(t, err)

	// Verify all components were called
	runner.AssertExpectations(t)
	stateManager.AssertExpectations(t)
	resultHandler.AssertExpectations(t)
}

func TestWorkflow_Execute_TransitionToRunningError(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)
	transitionErr := errors.New("transition failed")

	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(transitionErr)

	err := workflow.Execute(context.Background(), task)

	assert.Error(t, err)
	assert.Equal(t, transitionErr, err)

	// Runner should not be called if transition fails
	runner.AssertNotCalled(t, "Run")
	resultHandler.AssertNotCalled(t, "HandleSuccess")
	resultHandler.AssertNotCalled(t, "HandleFailure")
	stateManager.AssertNotCalled(t, "TransitionToCompleted")
	stateManager.AssertNotCalled(t, "TransitionToFailed")
}

func TestWorkflow_Execute_RunnerError(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)
	runnerErr := errors.New("runner failed")

	// Setup expectations
	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(nil)
	runner.On("Run", context.Background(), task).Return(runnerErr)
	resultHandler.On("HandleFailure", mock.MatchedBy(func(execCtx *ExecutionContext) bool {
		return execCtx.Error == runnerErr
	}))
	stateManager.On("TransitionToFailed", context.Background(), mock.MatchedBy(func(execCtx *ExecutionContext) bool {
		return execCtx.Error == runnerErr
	})).Return(nil)

	err := workflow.Execute(context.Background(), task)

	assert.Error(t, err)
	assert.Equal(t, runnerErr, err)

	// Verify error handling path was taken
	runner.AssertExpectations(t)
	stateManager.AssertExpectations(t)
	resultHandler.AssertExpectations(t)

	// Success path should not be called
	resultHandler.AssertNotCalled(t, "HandleSuccess")
	stateManager.AssertNotCalled(t, "TransitionToCompleted")
}

func TestWorkflow_Execute_SuccessPathExecution(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)

	// Track execution order
	var executionOrder []string

	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "transition_running")
	}).Return(nil)

	runner.On("Run", context.Background(), task).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "runner_execute")
	}).Return(nil)

	resultHandler.On("HandleSuccess", mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "handle_success")
	})

	stateManager.On("TransitionToCompleted", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "transition_completed")
	}).Return(nil)

	err := workflow.Execute(context.Background(), task)

	assert.NoError(t, err)

	// Verify execution order
	expectedOrder := []string{
		"transition_running",
		"runner_execute",
		"handle_success",
		"transition_completed",
	}
	assert.Equal(t, expectedOrder, executionOrder)
}

func TestWorkflow_Execute_FailurePathExecution(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)
	runnerErr := errors.New("execution failed")

	// Track execution order
	var executionOrder []string

	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "transition_running")
	}).Return(nil)

	runner.On("Run", context.Background(), task).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "runner_execute")
	}).Return(runnerErr)

	resultHandler.On("HandleFailure", mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "handle_failure")
	})

	stateManager.On("TransitionToFailed", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		executionOrder = append(executionOrder, "transition_failed")
	}).Return(nil)

	err := workflow.Execute(context.Background(), task)

	assert.Error(t, err)
	assert.Equal(t, runnerErr, err)

	// Verify execution order
	expectedOrder := []string{
		"transition_running",
		"runner_execute",
		"handle_failure",
		"transition_failed",
	}
	assert.Equal(t, expectedOrder, executionOrder)
}

func TestWorkflow_Execute_ContextPropagation(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)

	// Verify that the same context is passed through the pipeline
	var capturedContexts []*ExecutionContext

	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		execCtx := args[1].(*ExecutionContext)
		capturedContexts = append(capturedContexts, execCtx)
	}).Return(nil)

	runner.On("Run", context.Background(), task).Return(nil)

	resultHandler.On("HandleSuccess", mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		execCtx := args[0].(*ExecutionContext)
		capturedContexts = append(capturedContexts, execCtx)
	})

	stateManager.On("TransitionToCompleted", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Run(func(args mock.Arguments) {
		execCtx := args[1].(*ExecutionContext)
		capturedContexts = append(capturedContexts, execCtx)
	}).Return(nil)

	err := workflow.Execute(context.Background(), task)

	assert.NoError(t, err)
	assert.Len(t, capturedContexts, 3)

	// All contexts should be the same instance
	for i := 1; i < len(capturedContexts); i++ {
		assert.Same(t, capturedContexts[0], capturedContexts[i])
	}

	// Context should contain the task
	assert.Equal(t, task, capturedContexts[0].Task)
}

func TestWorkflow_Execute_TransitionToFailedError_LogsButReturnsOriginalError(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)
	runnerErr := errors.New("runner execution failed")
	transitionErr := errors.New("transition to failed state failed")

	// Setup expectations
	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(nil)
	runner.On("Run", context.Background(), task).Return(runnerErr)
	resultHandler.On("HandleFailure", mock.MatchedBy(func(execCtx *ExecutionContext) bool {
		return execCtx.Error == runnerErr
	}))
	stateManager.On("TransitionToFailed", context.Background(), mock.MatchedBy(func(execCtx *ExecutionContext) bool {
		return execCtx.Error == runnerErr
	})).Return(transitionErr)

	err := workflow.Execute(context.Background(), task)

	// Should return the original runner error, not the transition error
	assert.Error(t, err)
	assert.Equal(t, runnerErr, err)
	assert.NotEqual(t, transitionErr, err)

	// All components should have been called
	runner.AssertExpectations(t)
	stateManager.AssertExpectations(t)
	resultHandler.AssertExpectations(t)

	// Verify error was logged (check log buffer contains transition error)
	logOutput := buf.String()
	assert.Contains(t, logOutput, "failed to transition task to failed state")
	assert.Contains(t, logOutput, "transition to failed state failed")
	assert.Contains(t, logOutput, "runner execution failed")
}

func TestWorkflow_Execute_TransitionToCompletedError_LogsButContinues(t *testing.T) {
	runner := &MockRunner{}
	stateManager := &MockStateManager{}
	resultHandler := &MockResultHandler{}
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)

	workflow := NewDefaultExecutionWorkflow(runner, stateManager, resultHandler, logger)

	task := tasks.NewTask("test", nil)
	transitionErr := errors.New("transition to completed state failed")

	// Setup expectations
	stateManager.On("TransitionToRunning", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(nil)
	runner.On("Run", context.Background(), task).Return(nil)
	resultHandler.On("HandleSuccess", mock.AnythingOfType("*execution.ExecutionContext"))
	stateManager.On("TransitionToCompleted", context.Background(), mock.AnythingOfType("*execution.ExecutionContext")).Return(transitionErr)

	err := workflow.Execute(context.Background(), task)

	// Should NOT return error even though state transition failed
	// because the actual execution was successful
	assert.NoError(t, err)

	// All components should have been called
	runner.AssertExpectations(t)
	stateManager.AssertExpectations(t)
	resultHandler.AssertExpectations(t)

	// Verify error was logged (check log buffer contains transition error)
	logOutput := buf.String()
	assert.Contains(t, logOutput, "failed to transition task to completed state after successful execution")
	assert.Contains(t, logOutput, "transition to completed state failed")
}
