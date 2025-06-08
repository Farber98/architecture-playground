package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/orchestrator"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner simulates different runner behaviors for testing
type fakeRunner struct {
	shouldFail bool
	errorMsg   string
}

func (r *fakeRunner) Run(task *tasks.Task) error {
	if r.shouldFail {
		// Just set error result and return error - let orchestrator handle status
		task.Result = r.errorMsg
		return errors.New(r.errorMsg)
	}

	// Just set success result - let orchestrator handle status
	task.Result = "executed successfully"
	return nil
}

// fakeStore simulates store failures for testing error conditions
type fakeStore struct {
	store.TaskStore
	shouldFailSave   bool
	shouldFailGet    bool
	shouldFailUpdate bool
}

func (s *fakeStore) Save(task *tasks.Task) error {
	if s.shouldFailSave {
		return errors.New("store save failed")
	}
	return s.TaskStore.Save(task)
}

func (s *fakeStore) Get(id string) (*tasks.Task, error) {
	if s.shouldFailGet {
		return nil, errors.New("store get failed")
	}
	return s.TaskStore.Get(id)
}

func (s *fakeStore) Update(id string, status tasks.TaskStatus, result string) error {
	if s.shouldFailUpdate {
		return errors.New("store update failed")
	}
	return s.TaskStore.Update(id, status, result)
}

func TestOrchestrator_SubmitTask(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		taskType       string
		payload        json.RawMessage
		runnerSetup    func() *fakeRunner
		storeSetup     func() store.TaskStore
		expectErr      bool
		errContains    string
		expectedStatus tasks.TaskStatus
		expectedResult string
	}{
		{
			name:     "successful task submission",
			taskType: "print",
			payload:  json.RawMessage(`{"message":"hello world"}`),
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: false}
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusDone,
			expectedResult: "executed successfully",
		},
		{
			name:     "runner execution failure",
			taskType: "print",
			payload:  json.RawMessage(`{"message":"hello world"}`),
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: true, errorMsg: "runner execution failed"}
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:   true,
			errContains: "runner execution failed",
		},
		{
			name:     "store save failure",
			taskType: "print",
			payload:  json.RawMessage(`{"message":"hello world"}`),
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: false}
			},
			storeSetup: func() store.TaskStore {
				return &fakeStore{
					TaskStore:      store.NewMemoryTaskStore(),
					shouldFailSave: true,
				}
			},
			expectErr:   true,
			errContains: "failed to save task",
		},
		{
			name:     "runner failure AND store update failure",
			taskType: "print",
			payload:  json.RawMessage(`{"message":"double failure"}`),
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: true, errorMsg: "runner execution failed"}
			},
			storeSetup: func() store.TaskStore {
				return &fakeStore{
					TaskStore:        store.NewMemoryTaskStore(),
					shouldFailUpdate: true,
				}
			},
			expectErr:   true,
			errContains: "runner execution failed", // Should still return the original runner error
		},
		{
			name:     "empty task type",
			taskType: "",
			payload:  json.RawMessage(`{"message":"test"}`),
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: false}
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false, // Orchestrator doesn't validate task type
			expectedStatus: tasks.StatusDone,
			expectedResult: "executed successfully",
		},
		{
			name:     "nil payload",
			taskType: "print",
			payload:  nil,
			runnerSetup: func() *fakeRunner {
				return &fakeRunner{shouldFail: false}
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusDone,
			expectedResult: "executed successfully",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create test logger
			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)

			runner := tc.runnerSetup()
			taskStore := tc.storeSetup()
			orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

			task, err := orch.SubmitTask(tc.taskType, tc.payload)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, task)

				// Verify task was created properly
				assert.NotEmpty(t, task.ID)
				assert.Equal(t, tc.taskType, task.Type)
				assert.Equal(t, tc.payload, task.Payload)
				assert.Equal(t, tc.expectedStatus, task.Status)
				assert.Equal(t, tc.expectedResult, task.Result)

				// Verify task was persisted correctly
				savedTask, getErr := taskStore.Get(task.ID)
				require.NoError(t, getErr)
				assert.Equal(t, task.ID, savedTask.ID)
				assert.Equal(t, tc.expectedStatus, savedTask.Status)
				assert.Equal(t, tc.expectedResult, savedTask.Result)
			}
		})
	}
}

func TestOrchestrator_GetTask(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		setupTask    func(store store.TaskStore) string // Returns task ID
		taskID       string                             // For cases where we don't create a task
		storeSetup   func() store.TaskStore
		expectErr    bool
		errContains  string
		validateTask func(t *testing.T, task *tasks.Task) // Custom validation
	}{
		{
			name: "successful get existing task",
			setupTask: func(taskStore store.TaskStore) string {
				// Create task using NewTask (which auto-generates ID)
				task := tasks.NewTask("print", json.RawMessage(`{"message":"test"}`))
				require.NoError(t, task.SetStatus(tasks.StatusRunning))
				require.NoError(t, task.SetStatus(tasks.StatusDone))
				task.Result = "done"
				require.NoError(t, taskStore.Save(task))
				return task.ID
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr: false,
			validateTask: func(t *testing.T, task *tasks.Task) {
				assert.NotEmpty(t, task.ID)
				assert.Equal(t, "print", task.Type)
				assert.Equal(t, tasks.StatusDone, task.Status)
				assert.Equal(t, "done", task.Result)
				assert.Equal(t, json.RawMessage(`{"message":"test"}`), task.Payload)
			},
		},
		{
			name:   "get non-existing task",
			taskID: "non-existing",
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:   "store get failure",
			taskID: "store-fail-task",
			storeSetup: func() store.TaskStore {
				return &fakeStore{
					TaskStore:     store.NewMemoryTaskStore(),
					shouldFailGet: true,
				}
			},
			expectErr:   true,
			errContains: "not found", // Orchestrator wraps store errors as not found
		},
		{
			name:   "empty task ID",
			taskID: "",
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:   true,
			errContains: "not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create test logger
			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)

			runner := &fakeRunner{}
			taskStore := tc.storeSetup()

			// Setup task if needed
			var taskID string
			if tc.setupTask != nil {
				taskID = tc.setupTask(taskStore)
			} else {
				taskID = tc.taskID
			}

			orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

			task, err := orch.GetTask(taskID)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Nil(t, task)
			} else {
				require.NoError(t, err)
				require.NotNil(t, task)
				if tc.validateTask != nil {
					tc.validateTask(t, task)
				}
			}
		})
	}
}

func TestOrchestrator_LoggingIntegration(t *testing.T) {
	t.Parallel()

	// Create test logger with buffer to capture logs
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	runner := &fakeRunner{shouldFail: false}
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

	// Submit a task
	task, err := orch.SubmitTask("print", json.RawMessage(`{"message":"test logging"}`))
	require.NoError(t, err)
	require.NotNil(t, task)

	// Verify that logs were written
	logOutput := buf.String()

	// Check all expected log messages from successful execution
	assert.Contains(t, logOutput, "task submitted")
	assert.Contains(t, logOutput, "task completed")
	assert.Contains(t, logOutput, task.ID)                         // Task ID should appear in logs
	assert.Contains(t, logOutput, "print")                         // Task type should appear
	assert.Contains(t, logOutput, "*orchestrator_test.fakeRunner") // Runner type
	assert.Contains(t, logOutput, "done")                          // Final status

	// Verify specific log structure
	assert.Contains(t, logOutput, "payload_size")
	assert.Contains(t, logOutput, "status")

	// Should NOT contain error logs
	assert.NotContains(t, logOutput, "execution failed")
	assert.NotContains(t, logOutput, "failed to update")
}

func TestOrchestrator_StoreUpdateFailureDoesNotFailExecution(t *testing.T) {
	t.Parallel()

	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	runner := &fakeRunner{shouldFail: false}
	// Store that fails on update but succeeds on save
	taskStore := &fakeStore{
		TaskStore:        store.NewMemoryTaskStore(),
		shouldFailUpdate: true,
	}
	orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

	// Submit task - should succeed even though store update fails
	task, err := orch.SubmitTask("print", json.RawMessage(`{"message":"test"}`))
	require.NoError(t, err) // Should not fail even though store update failed
	require.NotNil(t, task)

	// Verify task was executed (runner set status/result)
	assert.Equal(t, tasks.StatusDone, task.Status)
	assert.Equal(t, "executed successfully", task.Result)

	// Verify warning was logged about update failure
	logOutput := buf.String()
	assert.Contains(t, logOutput, "task submitted")
	assert.Contains(t, logOutput, "failed to update running status")   // First update failure
	assert.Contains(t, logOutput, "failed to update final task state") // Final update failure
	assert.Contains(t, logOutput, "store update failed")               // Error details

	// Should still contain success indicators despite store failures
	assert.Contains(t, logOutput, "done") // Final status should be logged
}

func TestOrchestrator_TaskIDGeneration(t *testing.T) {
	t.Parallel()

	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	runner := &fakeRunner{shouldFail: false}
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

	// Submit multiple tasks
	task1, err1 := orch.SubmitTask("print", json.RawMessage(`{"message":"task1"}`))
	require.NoError(t, err1)

	task2, err2 := orch.SubmitTask("print", json.RawMessage(`{"message":"task2"}`))
	require.NoError(t, err2)

	// Verify unique IDs were generated
	assert.NotEmpty(t, task1.ID)
	assert.NotEmpty(t, task2.ID)
	assert.NotEqual(t, task1.ID, task2.ID)

	// Verify both tasks can be retrieved
	retrieved1, err := orch.GetTask(task1.ID)
	require.NoError(t, err)
	assert.Equal(t, task1.ID, retrieved1.ID)

	retrieved2, err := orch.GetTask(task2.ID)
	require.NoError(t, err)
	assert.Equal(t, task2.ID, retrieved2.ID)
}

func TestOrchestrator_RunnerFailureUpdatesStore(t *testing.T) {
	t.Parallel()

	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	runner := &fakeRunner{shouldFail: true, errorMsg: "runner failed"}
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

	// Submit task that will fail
	_, err := orch.SubmitTask("print", json.RawMessage(`{"message":"fail me"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner failed")

	// Verify logs show the failure
	logOutput := buf.String()
	assert.Contains(t, logOutput, "task submitted")   // Initial submission
	assert.Contains(t, logOutput, "execution failed") // Runner failure
	assert.Contains(t, logOutput, "runner failed")    // Specific error message

	// Should NOT contain success logs
	assert.NotContains(t, logOutput, "task completed")
}

func TestOrchestrator_GetTaskStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		setupTask      func(store store.TaskStore) string // Returns task ID
		taskID         string                             // For cases where we don't create a task
		storeSetup     func() store.TaskStore
		expectErr      bool
		errContains    string
		expectedStatus string // GetTaskStatus returns string
	}{
		{
			name: "successful get status for existing task",
			setupTask: func(taskStore store.TaskStore) string {
				task := tasks.NewTask("print", json.RawMessage(`{"message":"test"}`))
				require.NoError(t, task.SetStatus(tasks.StatusRunning))
				require.NoError(t, task.SetStatus(tasks.StatusDone))
				task.Result = "task done"
				require.NoError(t, taskStore.Save(task))
				return task.ID
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusDone.String(),
		},
		{
			name: "get status for running task",
			setupTask: func(taskStore store.TaskStore) string {
				task := tasks.NewTask("sleep", json.RawMessage(`{}`))
				require.NoError(t, task.SetStatus(tasks.StatusRunning))
				task.Result = ""
				require.NoError(t, taskStore.Save(task))
				return task.ID
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusRunning.String(),
		},
		{
			name: "get status for failed task",
			setupTask: func(taskStore store.TaskStore) string {
				task := tasks.NewTask("print", json.RawMessage(`{}`))
				require.NoError(t, task.SetStatus(tasks.StatusRunning))
				require.NoError(t, task.SetStatus(tasks.StatusFailed))
				task.Result = "execution error"
				require.NoError(t, taskStore.Save(task))
				return task.ID
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusFailed.String(),
		},
		{
			name: "get status for submitted task",
			setupTask: func(taskStore store.TaskStore) string {
				task := tasks.NewTask("print", json.RawMessage(`{}`))
				// Status is already StatusSubmitted from NewTask
				require.NoError(t, taskStore.Save(task))
				return task.ID
			},
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:      false,
			expectedStatus: tasks.StatusSubmitted.String(),
		},
		{
			name:   "get status for non-existing task",
			taskID: "non-existing-task",
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:   "empty task ID",
			taskID: "",
			storeSetup: func() store.TaskStore {
				return store.NewMemoryTaskStore()
			},
			expectErr:   true,
			errContains: "not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create test logger
			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)

			runner := &fakeRunner{}
			taskStore := tc.storeSetup()

			// Setup task if needed
			var taskID string
			if tc.setupTask != nil {
				taskID = tc.setupTask(taskStore)
			} else {
				taskID = tc.taskID
			}

			orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

			status, err := orch.GetTaskStatus(taskID)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Empty(t, status)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedStatus, status)
			}
		})
	}
}

func TestOrchestrator_ResultHandling_Comprehensive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		runnerSetup    func() runners.Runner
		expectedResult string
		expectErr      bool
	}{
		{
			name: "handler fails without setting result - orchestrator sets generic error result",
			runnerSetup: func() runners.Runner {
				return &fakeRunnerNoResult{
					shouldFail: true,
					errorMsg:   "database timeout",
				}
			},
			expectedResult: "execution failed: database timeout",
			expectErr:      true,
		},
		{
			name: "handler fails with custom result - orchestrator preserves it",
			runnerSetup: func() runners.Runner {
				return &fakeRunner{
					shouldFail: true,
					errorMsg:   "runner failed", // This gets set as result
				}
			},
			expectedResult: "runner failed", // Preserved from handler
			expectErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)

			taskStore := store.NewMemoryTaskStore()
			orch := orchestrator.NewOrchestrator(taskStore, tc.runnerSetup(), testLogger)

			task, err := orch.SubmitTask("test", json.RawMessage(`{"test":"data"}`))

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.NotNil(t, task)
			assert.Equal(t, tc.expectedResult, task.Result)

			// Verify persistence
			savedTask, getErr := taskStore.Get(task.ID)
			require.NoError(t, getErr)
			assert.Equal(t, tc.expectedResult, savedTask.Result)
		})
	}
}

// fakeRunnerNoResult simulates a runner that fails without setting task.Result
type fakeRunnerNoResult struct {
	shouldFail bool
	errorMsg   string
}

func (r *fakeRunnerNoResult) Run(task *tasks.Task) error {
	if r.shouldFail {
		// Don't set task.Result - let orchestrator handle it
		return errors.New(r.errorMsg)
	}

	task.Result = "executed successfully"
	return nil
}
