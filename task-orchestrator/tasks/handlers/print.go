package handlers

import (
	"encoding/json"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
)

// PrintHandler executes a "print" task by logging the provided message.
type PrintHandler struct{}

type PrintPayload struct {
	Message string `json:"message"`
}

func (h *PrintHandler) Run(task *tasks.Task) error {
	var p PrintPayload
	if err := json.Unmarshal(task.Payload, &p); err != nil {
		return errors.NewValidationError("invalid print payload", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
	}
	logger.Taskf(task.ID, "executing print task: %s", p.Message)
	task.Result = "printed: " + p.Message
	task.Status = "done"
	return nil
}
