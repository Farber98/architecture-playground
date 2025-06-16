package handlers

import (
	"context"
	"task-orchestrator/tasks"
)

// TaskHandler defines the interface implemented by any executable task.
//
// This allows task-specific logic (e.g. print, sleep) to be encapsulated
// in modular handlers, decoupled from the task runner or transport layer.
type TaskHandler interface {
	Run(ctx context.Context, task *tasks.Task) error
}
