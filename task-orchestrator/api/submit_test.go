package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"task-orchestrator/api"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/runners"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestSubmitHandler_Print_Success(t *testing.T) {
	reg := tasks.NewRegistry()
	reg.Register("print", &handlers.PrintHandler{})
	runner := runners.NewSynchronousRunner(reg)
	handler := api.NewSubmitHandler(runner)

	body := []byte(`{"type":"print","payload":{"message":"Hello from HTTP"}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp api.SubmitResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "printed: Hello from HTTP", resp.Result)
	assert.Assert(t, resp.TaskID != "")
}

func TestSubmitHandler_InvalidJSON(t *testing.T) {
	runner := runners.NewSynchronousRunner(tasks.NewRegistry())
	handler := api.NewSubmitHandler(runner)

	body := []byte(`{"type":"print","payload":{`) // malformed
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp api.ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("invalid JSON payload")))
}

func TestSubmitHandler_UnknownTaskType(t *testing.T) {
	runner := runners.NewSynchronousRunner(tasks.NewRegistry())
	handler := api.NewSubmitHandler(runner)

	body := []byte(`{"type":"unknown","payload":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Parse the structured error response
	var errorResp api.ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "not_found", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("no handler registered for task type")))
}

func TestSubmitHandler_MissingTaskType(t *testing.T) {
	runner := runners.NewSynchronousRunner(tasks.NewRegistry())
	handler := api.NewSubmitHandler(runner)

	body := []byte(`{"payload":{"message":"hello"}}`) // missing type field
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp api.ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("task type is required")))
}

func TestSubmitHandler_MethodNotAllowed(t *testing.T) {
	handler := api.NewSubmitHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/submit", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp api.ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("method not allowed")))
}

func TestSubmitHandler_TaskHandlerValidationError(t *testing.T) {
	reg := tasks.NewRegistry()
	reg.Register("sleep", handlers.NewSleepHandler())
	runner := runners.NewSynchronousRunner(reg)
	handler := api.NewSubmitHandler(runner)

	// Send invalid sleep payload (negative seconds)
	body := []byte(`{"type":"sleep","payload":{"seconds":-1}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResp api.ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "validation", errorResp.Type)
	assert.Assert(t, bytes.Contains([]byte(errorResp.Error), []byte("invalid sleep duration")))

	// Check that details are included
	assert.Assert(t, errorResp.Details != nil)
	assert.Assert(t, errorResp.Details["task_id"] != nil)
	assert.Equal(t, -1.0, errorResp.Details["seconds"]) // JSON numbers are float64
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
	reg := tasks.NewRegistry()
	reg.Register("print", &handlers.PrintHandler{})
	runner := runners.NewSynchronousRunner(reg)
	handler := api.NewSubmitHandler(runner)

	body := []byte(`{"type":"print","payload":{"message":"simulate write failure"}}`)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Use our fake writer that forces an Encode() failure
	w := &erroringResponseWriter{}

	// We don't assert on output — we're just ensuring it doesn't panic
	// If we had a pluggable logger, we could assert it was called
	handler.ServeHTTP(w, req)
}
