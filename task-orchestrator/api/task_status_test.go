package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/orchestrator"
	handlerRegistry "task-orchestrator/tasks/registry"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestTaskStatusHandler_Success(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))
	runner := runners.NewSynchronousRunner(reg)
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)

	// First, submit a task to have something to query
	task, err := orch.SubmitTask(context.Background(), "print", []byte(`{"message":"test task"}`))
	require.NoError(t, err)

	handler := NewTaskStatusHandler(orch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID+"/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp TaskStatusResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, task.ID, resp.TaskID)
	assert.Equal(t, tasks.StatusDone.String(), resp.Status) // Print handler sets status to "done"
}

func TestTaskStatusHandler_NonExistentTask(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	taskStore := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)
	handler := NewTaskStatusHandler(orch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/non-existent-id/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "not_found", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("not found")))
}

func TestTaskStatusHandler_MethodNotAllowed(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	handler := NewTaskStatusHandler(nil, testLogger)

	req := httptest.NewRequest(http.MethodPost, "/tasks/some-id/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("method not allowed")))
}

func TestTaskStatusHandler_InvalidURLFormat(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	handler := NewTaskStatusHandler(nil, testLogger)

	testCases := []struct {
		name string
		path string
	}{
		{
			name: "missing task ID",
			path: "/tasks/",
		},
		{
			name: "wrong path structure",
			path: "/wrong/path",
		},
		{
			name: "incomplete path",
			path: "/tasks",
		},
		{
			name: "empty task ID",
			path: "/tasks//status",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)

			var errorResp errorResponse
			err := json.NewDecoder(rr.Body).Decode(&errorResp)
			require.NoError(t, err)

			assert.Equal(t, "validation", errorResp.Type)
			// Should be either "invalid URL format" or "task ID is required"
			assert.Assert(t,
				bytes.Contains([]byte(errorResp.Error), []byte("invalid URL format")) ||
					bytes.Contains([]byte(errorResp.Error), []byte("task ID is required")))
		})
	}
}

func TestTaskStatusHandler_DifferentTaskStatuses(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	taskStore := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)
	handler := NewTaskStatusHandler(orch, testLogger)

	testCases := []struct {
		name           string
		taskID         string
		taskStatus     tasks.TaskStatus
		taskResult     string
		expectedStatus string
	}{
		{
			name:           "completed task",
			taskID:         "test-task-completed",
			taskStatus:     tasks.StatusDone,
			taskResult:     "task completed successfully",
			expectedStatus: "done",
		},
		{
			name:           "failed task",
			taskID:         "test-task-failed",
			taskStatus:     tasks.StatusFailed,
			taskResult:     "task execution failed",
			expectedStatus: "failed",
		},
		{
			name:           "running task",
			taskID:         "test-task-running",
			taskStatus:     tasks.StatusRunning,
			taskResult:     "",
			expectedStatus: "running",
		},
		{
			name:           "submitted task",
			taskID:         "test-task-submitted",
			taskStatus:     tasks.StatusSubmitted,
			taskResult:     "",
			expectedStatus: "submitted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := tasks.NewTask("print", []byte(`{"message":"test"}`))
			task.ID = tc.taskID         // Override the auto-generated ID for this test
			task.Status = tc.taskStatus // Now using TaskStatus type
			task.Result = tc.taskResult
			require.NoError(t, taskStore.Save(context.Background(), task))

			req := httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID+"/status", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)

			var resp TaskStatusResponse
			err := json.NewDecoder(rr.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, task.ID, resp.TaskID)
			assert.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}

func TestTaskStatusHandler_ResponseEncodingFailure(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))
	runner := runners.NewSynchronousRunner(reg)
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)

	// Submit a task to have something to query
	task, err := orch.SubmitTask(context.Background(), "print", []byte(`{"message":"test"}`))
	require.NoError(t, err)

	handler := NewTaskStatusHandler(orch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID+"/status", nil)

	// Use the erroringResponseWriter from submit_test.go
	w := &erroringResponseWriter{}

	// We don't assert on output — we're just ensuring it doesn't panic
	handler.ServeHTTP(w, req)
}

func TestTaskStatusHandler_LoggingIntegration(t *testing.T) {
	// Create test logger with buffer to capture logs
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))
	runner := runners.NewSynchronousRunner(reg)
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)

	// Submit a task
	task, err := orch.SubmitTask(context.Background(), "print", []byte(`{"message":"test logging"}`))
	require.NoError(t, err)

	handler := NewTaskStatusHandler(orch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID+"/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	// Verify that logs were written
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("task status request")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte(task.ID)))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("GET")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("/tasks/")))
}

func TestTaskStatusHandler_URLPathParsing(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	taskStore := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)
	handler := NewTaskStatusHandler(orch, testLogger)

	task := tasks.NewTask("print", []byte(`{"message":"test"}`))
	task.ID = "test-task-with-special-chars"
	task.Status = tasks.StatusDone
	task.Result = "completed"
	require.NoError(t, taskStore.Save(context.Background(), task))

	testCases := []struct {
		name           string
		path           string
		expectedTaskID string
		shouldSucceed  bool
	}{
		{
			name:           "normal task ID",
			path:           "/tasks/test-task-with-special-chars/status",
			expectedTaskID: "test-task-with-special-chars",
			shouldSucceed:  true,
		},
		{
			name:           "task ID with numbers",
			path:           "/tasks/task-123/status",
			expectedTaskID: "task-123",
			shouldSucceed:  false, // task doesn't exist
		},
		{
			name:           "extra path segments ignored",
			path:           "/tasks/test-task-with-special-chars/status/extra",
			expectedTaskID: "test-task-with-special-chars",
			shouldSucceed:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tc.shouldSucceed {
				require.Equal(t, http.StatusOK, rr.Code)

				var resp TaskStatusResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedTaskID, resp.TaskID)
			} else {
				assert.Equal(t, http.StatusNotFound, rr.Code)
			}
		})
	}
}

func TestTaskStatusHandler_ContentType(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))
	runner := runners.NewSynchronousRunner(reg)
	taskStore := store.NewMemoryTaskStore()
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, testLogger)

	// Submit a task
	task, err := orch.SubmitTask(context.Background(), "print", []byte(`{"message":"test"}`))
	require.NoError(t, err)

	handler := NewTaskStatusHandler(orch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID+"/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

type mockOrchestrator struct {
	submitTaskResult    *tasks.Task
	submitTaskErr       error
	getTaskResult       *tasks.Task
	getTaskErr          error
	getTaskStatusResult string
	getTaskStatusErr    error
}

func (m *mockOrchestrator) SubmitTask(ctx context.Context, taskType string, payload json.RawMessage) (*tasks.Task, error) {
	return m.submitTaskResult, m.submitTaskErr
}

func (m *mockOrchestrator) GetTask(ctx context.Context, taskID string) (*tasks.Task, error) {
	return m.getTaskResult, m.getTaskErr
}

func (m *mockOrchestrator) GetTaskStatus(ctx context.Context, taskID string) (string, error) {
	return m.getTaskStatusResult, m.getTaskStatusErr
}

func TestTaskStatusHandler_OrchestratorInternalError(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	// Create a mock orchestrator that returns a non-TaskError
	mockOrch := &mockOrchestrator{
		getTaskStatusErr: errors.New("internal system failure"), // This is NOT a TaskError
	}

	handler := NewTaskStatusHandler(mockOrch, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/tasks/some-task-id/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should return 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "internal", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("internal system failure")))
}
