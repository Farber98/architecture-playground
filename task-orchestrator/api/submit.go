package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks/orchestrator"
)

const (
	maxBodySize    = 1024 * 1024 // 1 MB
	maxPayloadSize = 1024 * 100  // 100 KB
	maxTypeLen     = 50
)

// ErrorResponse defines the JSON structure for error responses
type errorResponse struct {
	Error   string         `json:"error"`
	Type    string         `json:"type,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// SubmitRequest defines the expected payload for a Task.
type submitRequest struct {
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
func NewSubmitHandler(orch orchestrator.Orchestrator, lg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondWithError(w, errors.NewValidationError("method not allowed"), lg)
			return
		}

		// Limit request body size - this will cause Decode to fail if exceeded
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		var req submitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Check if the error is due to request size limit
			if strings.Contains(err.Error(), "http: request body too large") {
				respondWithError(w, errors.NewValidationError("request body too large", map[string]any{
					"max_size_bytes": maxBodySize,
				}), lg)
				return
			}

			// Other JSON parsing errors
			respondWithError(w, errors.NewValidationError("invalid JSON payload", map[string]any{
				"error": err.Error(),
			}), lg)
			return
		}

		if req.Type == "" {
			respondWithError(w, errors.NewValidationError("task type is required"), lg)
			return
		}

		if len(req.Type) > maxTypeLen {
			respondWithError(w, errors.NewValidationError("task type too long", map[string]any{
				"max_length":    maxTypeLen,
				"actual_length": len(req.Type),
			}), lg)
			return
		}

		if len(req.Payload) > maxPayloadSize {
			respondWithError(w, errors.NewValidationError("task payload too large", map[string]any{
				"max_size_bytes":    maxPayloadSize,
				"actual_size_bytes": len(req.Payload),
			}), lg)
			return
		}

		task, err := orch.SubmitTask(req.Type, req.Payload)
		if err != nil {
			if taskErr, ok := errors.IsTaskError(err); ok {
				respondWithError(w, taskErr, lg)
			} else {
				respondWithError(w, errors.NewInternalError(err.Error()), lg)
			}
			return
		}

		resp := SubmitResponse{
			TaskID: task.ID,
			Status: task.Status,
			Result: task.Result,
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			respondWithError(w, errors.NewInternalError("failed to encode response"), lg)
		}
	}
}

// respondWithError sends a structured error response
func respondWithError(w http.ResponseWriter, taskErr *errors.TaskError, lg *logger.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(taskErr.Code)

	errorResp := errorResponse{
		Error:   taskErr.Message,
		Type:    string(taskErr.Type),
		Details: taskErr.Details,
	}

	lg.Error("HTTP error response", map[string]any{
		"error_type":    string(taskErr.Type),
		"error_message": taskErr.Message,
		"status_code":   taskErr.Code,
		"error_details": taskErr.Details,
	})

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// Log the encoding failure
		lg.Error("HTTP error response", map[string]any{
			"error_type":    string(taskErr.Type),
			"error_message": taskErr.Message,
			"status_code":   taskErr.Code,
			"error_details": taskErr.Details,
		}) // At this point, we've already written headers and potentially some data,
		// so we can't recover gracefully. The connection may be broken.
	}
}
