package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-orchestrator/errors"
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

func TestSubmitHandler_Print_Success(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))
	runner := runners.NewSynchronousRunner(reg)
	store := store.NewMemoryTaskStore()
	orchestrator := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orchestrator, testLogger)

	body := []byte(`{"type":"print","payload":{"message":"Hello from HTTP"}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp SubmitResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, tasks.StatusDone.String(), resp.Status)
	assert.Equal(t, "printed: Hello from HTTP", resp.Result)
	assert.Assert(t, resp.TaskID != "")
}

func TestSubmitHandler_InvalidJSON(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	body := []byte(`{"type":"print","payload":{`) // malformed
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("invalid JSON payload")))
}

func TestSubmitHandler_UnknownTaskType(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	body := []byte(`{"type":"unknown","payload":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Parse the structured error response
	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "not_found", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("no handler registered for task type")))
}

func TestSubmitHandler_MissingTaskType(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	body := []byte(`{"payload":{"message":"hello"}}`) // missing type field
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("task type is required")))
}

func TestSubmitHandler_MethodNotAllowed(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	handler := NewSubmitHandler(nil, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/submit", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("method not allowed")))
}

func TestSubmitHandler_TaskHandlerValidationError(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	reg := handlerRegistry.NewRegistry()
	reg.Register("sleep", handlers.NewSleepHandler(testLogger))
	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(reg)
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	// Send invalid sleep payload (negative seconds)
	body := []byte(`{"type":"sleep","payload":{"seconds":-1}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("invalid sleep duration")))

	// Check that details are included
	assert.Assert(t, errorResp.Details != nil)
	assert.Assert(t, errorResp.Details["task_id"] != nil)
	assert.Equal(t, -1.0, errorResp.Details["seconds"]) // JSON numbers are float64
}

func TestSubmitHandler_PayloadTooLarge(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	// Create a payload larger than 100KB (100 * 1024 bytes)
	largePayload := strings.Repeat("a", 101*1024) // 101KB

	// Create a valid JSON payload that's too large
	payloadJSON := fmt.Sprintf(`{"message":"%s"}`, largePayload)
	reqBody := submitRequest{
		Type:    "test",
		Payload: json.RawMessage(payloadJSON),
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err = json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Equal(t, "task payload too large", errorResp.Error)
	assert.Equal(t, float64(100*1024), errorResp.Details["max_size_bytes"])
	assert.Equal(t, float64(len(payloadJSON)), errorResp.Details["actual_size_bytes"])
}

func TestSubmitHandler_TaskTypeTooLong(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	// Create a task type longer than 50 characters
	longTaskType := strings.Repeat("a", 51) // 51 characters

	reqBody := submitRequest{
		Type:    longTaskType,
		Payload: json.RawMessage(`{"message":"test"}`),
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err = json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Equal(t, "task type too long", errorResp.Error)
	assert.Equal(t, float64(50), errorResp.Details["max_length"])
	assert.Equal(t, float64(51), errorResp.Details["actual_length"])
}

func TestSubmitHandler_RequestBodyTooLarge(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(handlerRegistry.NewRegistry())
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	// Create a request body larger than 1MB
	// We'll create a very large payload that makes the entire request > 1MB
	largeMessage := strings.Repeat("x", 1024*1024) // 1MB of 'x' characters

	// This will create a JSON payload that exceeds 1MB when marshaled
	reqBody := fmt.Sprintf(`{"type":"test","payload":{"message":"%s"}}`, largeMessage)

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Equal(t, "request body too large", errorResp.Error)
	assert.Equal(t, float64(1024*1024), errorResp.Details["max_size_bytes"]) // 1MB
}

// erroringResponseWriter simulates a failure when writing to the client.
type erroringResponseWriter struct{}

func (erroringResponseWriter) Header() http.Header {
	return http.Header{}
}

func (erroringResponseWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("simulated write error")
}

func (erroringResponseWriter) WriteHeader(statusCode int) {}

func TestSubmitHandler_ResponseEncodingFailure(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)
	reg := handlerRegistry.NewRegistry()
	reg.Register("print", handlers.NewPrintHandler(testLogger))

	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(reg)
	orch := orchestrator.NewDefaultOrchestrator(store, runner, testLogger)
	handler := NewSubmitHandler(orch, testLogger)

	body := []byte(`{"type":"print","payload":{"message":"simulate write failure"}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Use our fake writer that forces an Encode() failure
	w := &erroringResponseWriter{}

	// We don't assert on output — we're just ensuring it doesn't panic
	// If we had a pluggable logger, we could assert it was called
	handler.ServeHTTP(w, req)
}

func TestSubmitHandler_TaskErrorFromOrchestrator(t *testing.T) {
	// Create test logger
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	// Create a mock orchestrator that returns a TaskError from SubmitTask
	mockOrch := &mockOrchestrator{
		submitTaskErr: errors.NewValidationError("invalid task configuration", map[string]any{
			"task_type": "invalid",
		}),
	}

	handler := NewSubmitHandler(mockOrch, testLogger)

	body := []byte(`{"type":"invalid","payload":{"message":"test"}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp errorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Equal(t, "invalid task configuration", errorResp.Error)
	assert.Equal(t, "invalid", errorResp.Details["task_type"])
}
