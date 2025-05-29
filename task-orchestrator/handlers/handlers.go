package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	tasks "task-orchestrator/tasks"

	"github.com/google/uuid"
)

func NewSubmitHandler(runner tasks.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var incoming struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		task := &tasks.Task{
			ID:      uuid.New().String(),
			Type:    incoming.Type,
			Payload: incoming.Payload,
			Status:  "submitted",
			Result:  "",
		}

		log.Printf("task received: %+v\n", task)

		if err := runner.Run(task); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]string{
			"task_id": task.ID,
			"status":  task.Status,
			"result":  task.Result,
		})
		if err != nil {
			log.Printf("failed to encode response: %v", err)
		}
	}
}
