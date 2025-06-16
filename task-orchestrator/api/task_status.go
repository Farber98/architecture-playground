package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks/orchestrator"
)

// TaskStatusResponse represents a lightweight status response
type TaskStatusResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// NewTaskStatusHandler creates a lightweight handler for task status queries.
func NewTaskStatusHandler(orch orchestrator.Orchestrator, lg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondWithError(w, errors.NewValidationError("method not allowed"), lg)
			return
		}

		// Extract task ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 2 || pathParts[0] != "tasks" {
			respondWithError(w, errors.NewValidationError("invalid URL format"), lg)
			return
		}

		taskID := pathParts[1]
		if taskID == "" {
			respondWithError(w, errors.NewValidationError("task ID is required"), lg)
			return
		}

		lg.Info("task status request", map[string]any{
			"http_method": r.Method,
			"http_path":   r.URL.Path,
			"task_id":     taskID,
		})

		status, err := orch.GetTaskStatus(r.Context(), taskID)
		if err != nil {
			if taskErr, ok := errors.IsTaskError(err); ok {
				respondWithError(w, taskErr, lg)
			} else {
				respondWithError(w, errors.NewInternalError(err.Error()), lg)
			}
			return
		}

		resp := TaskStatusResponse{
			TaskID: taskID,
			Status: status,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			respondWithError(w, errors.NewInternalError("failed to encode response"), lg)
		}
	}
}
