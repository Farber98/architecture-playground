package api

import (
	"encoding/json"
	"net/http"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	tasks "task-orchestrator/tasks"
	"task-orchestrator/tasks/runners"

	"github.com/google/uuid"
)

// ErrorResponse defines the JSON structure for error responses
type ErrorResponse struct {
	Error   string         `json:"error"`
	Type    string         `json:"type,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// SubmitRequest defines the expected payload for a Task.
type SubmitRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SubmitResponse defines the JSON response returned after executing a Task.
type SubmitResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Result string `json:"result"`
}

// NewSubmitHandler returns an HTTP handler that processes task submissions.
//
// This handler is synchronous — it calls the Runner immediately and returns the task result.
// It assumes that the runner handles task routing, lifecycle management, and logging.
func NewSubmitHandler(runner runners.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondWithError(w, errors.NewValidationError("method not allowed"))
			return
		}

		var req SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, errors.NewValidationError("invalid JSON payload", map[string]interface{}{
				"error": err.Error(),
			}))
			return
		}

		if req.Type == "" {
			respondWithError(w, errors.NewValidationError("task type is required"))
			return
		}

		task := &tasks.Task{
			ID:      uuid.New().String(),
			Type:    req.Type,
			Payload: req.Payload,
			Status:  "submitted",
			Result:  "",
		}

		logger.Infof("received new task submission: type=%s", req.Type)

		if err := runner.Run(task); err != nil {
			if taskErr, ok := errors.IsTaskError(err); ok {
				respondWithError(w, taskErr)
			} else {
				// Wrap unknown errors as internal errors
				respondWithError(w, errors.NewInternalError(err.Error()))
			}
			return
		}

		resp := SubmitResponse{
			TaskID: task.ID,
			Status: task.Status,
			Result: task.Result,
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			respondWithError(w, errors.NewInternalError("failed to encode response"))
		}
	}
}

// respondWithError sends a structured error response
func respondWithError(w http.ResponseWriter, taskErr *errors.TaskError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(taskErr.Code)

	errorResp := ErrorResponse{
		Error:   taskErr.Message,
		Type:    string(taskErr.Type),
		Details: taskErr.Details,
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// Log the encoding failure
		logger.Errorf("failed to encode error response: %v", err)
		// At this point, we've already written headers and potentially some data,
		// so we can't recover gracefully. The connection may be broken.
	}
}
