package handlers

import (
	"encoding/json"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
)

var _ TaskHandler = (*PrintHandler)(nil)

// PrintHandler executes a "print" task by logging the provided message.
type PrintHandler struct {
	logger *logger.Logger
}

type printPayload struct {
	Message string `json:"message"`
}

func NewPrintHandler(lg *logger.Logger) *PrintHandler {
	return &PrintHandler{logger: lg}
}

func (h *PrintHandler) Run(task *tasks.Task) error {
	var p printPayload
	if err := json.Unmarshal(task.Payload, &p); err != nil {
		return errors.NewValidationError("invalid print payload", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
	}

	h.logger.Task(task.ID, "executing print task", map[string]any{
		"message": p.Message,
	})

	task.Result = "printed: " + p.Message
	return nil
}
