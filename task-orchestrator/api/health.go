package api

import (
	"encoding/json"
	"net/http"
	"task-orchestrator/config"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	tasks "task-orchestrator/tasks"
	"time"
)

var startTime = time.Now()

// HealthResponse provides detailed health information
// HealthResponse provides detailed health information
type HealthResponse struct {
	Status          string   `json:"status"`
	Timestamp       string   `json:"timestamp"`
	Uptime          string   `json:"uptime"`
	RegisteredTasks []string `json:"registered_tasks"`
	Version         string   `json:"version,omitempty"`
}

// NewHealthHandler returns a health check handler
func NewHealthHandler(cfg *config.Config, registry *tasks.HandlerRegistry, lg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		uptime := time.Since(startTime)
		response := HealthResponse{
			Status:          "healthy",
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
			RegisteredTasks: registry.GetRegisteredTypes(),
			Uptime:          uptime.String(),
			Version:         cfg.Version,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			respondWithError(w, errors.NewInternalError("failed to encode response"), lg)
		}
	}
}
