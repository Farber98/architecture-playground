package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/orchestrator"
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
		return errors.New(r.errorMsg)
	}
	task.Status = "completed"
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

func (s *fakeStore) Update(id string, status string, result string) error {
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
		expectedStatus string
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
			expectedStatus: "completed",
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
			expectedStatus: "completed",
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
			expectedStatus: "completed",
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
		name        string
		taskID      string
		storeSetup  func() store.TaskStore
		expectErr   bool
		errContains string
		expectTask  *tasks.Task
	}{
		{
			name:   "successful get existing task",
			taskID: "existing-task",
			storeSetup: func() store.TaskStore {
				taskStore := store.NewMemoryTaskStore()
				task := &tasks.Task{
					ID:      "existing-task",
					Type:    "print",
					Status:  "completed",
					Result:  "done",
					Payload: json.RawMessage(`{"message":"test"}`),
				}
				require.NoError(t, taskStore.Save(task))
				return taskStore
			},
			expectErr: false,
			expectTask: &tasks.Task{
				ID:      "existing-task",
				Type:    "print",
				Status:  "completed",
				Result:  "done",
				Payload: json.RawMessage(`{"message":"test"}`),
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
			orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

			task, err := orch.GetTask(tc.taskID)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Nil(t, task)
			} else {
				require.NoError(t, err)
				require.NotNil(t, task)
				if tc.expectTask != nil {
					assert.Equal(t, tc.expectTask.ID, task.ID)
					assert.Equal(t, tc.expectTask.Type, task.Type)
					assert.Equal(t, tc.expectTask.Status, task.Status)
					assert.Equal(t, tc.expectTask.Result, task.Result)
					assert.Equal(t, tc.expectTask.Payload, task.Payload)
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
	assert.Contains(t, logOutput, "task submitted")
	assert.Contains(t, logOutput, "running task")
	assert.Contains(t, logOutput, "task completed successfully")
	assert.Contains(t, logOutput, task.ID) // Task ID should appear in logs
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
	assert.Equal(t, "completed", task.Status)
	assert.Equal(t, "executed successfully", task.Result)

	// Verify warning was logged about update failure
	logOutput := buf.String()
	assert.Contains(t, logOutput, "failed to update final task state")
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
	assert.Contains(t, logOutput, "task execution failed")
	assert.Contains(t, logOutput, "runner failed")
}

func TestOrchestrator_GetTaskStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		taskID         string
		storeSetup     func() store.TaskStore
		expectErr      bool
		errContains    string
		expectedStatus string
	}{
		{
			name:   "successful get status for existing task",
			taskID: "status-task-1",
			storeSetup: func() store.TaskStore {
				taskStore := store.NewMemoryTaskStore()
				task := &tasks.Task{
					ID:      "status-task-1",
					Type:    "print",
					Status:  "completed",
					Result:  "task done",
					Payload: json.RawMessage(`{"message":"test"}`),
				}
				require.NoError(t, taskStore.Save(task))
				return taskStore
			},
			expectErr:      false,
			expectedStatus: "completed",
		},
		{
			name:   "get status for running task",
			taskID: "status-task-2",
			storeSetup: func() store.TaskStore {
				taskStore := store.NewMemoryTaskStore()
				task := &tasks.Task{
					ID:     "status-task-2",
					Type:   "sleep",
					Status: "running",
					Result: "",
				}
				require.NoError(t, taskStore.Save(task))
				return taskStore
			},
			expectErr:      false,
			expectedStatus: "running",
		},
		{
			name:   "get status for failed task",
			taskID: "status-task-3",
			storeSetup: func() store.TaskStore {
				taskStore := store.NewMemoryTaskStore()
				task := &tasks.Task{
					ID:     "status-task-3",
					Type:   "print",
					Status: "failed",
					Result: "execution error",
				}
				require.NoError(t, taskStore.Save(task))
				return taskStore
			},
			expectErr:      false,
			expectedStatus: "failed",
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
			orch := orchestrator.NewOrchestrator(taskStore, runner, testLogger)

			status, err := orch.GetTaskStatus(tc.taskID)

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
