package store

import (
	"context"
	"task-orchestrator/tasks"
)

// TaskStore defines the contract for task persistance
type TaskStore interface {
	Save(ctx context.Context, task *tasks.Task) error
	Get(ctx context.Context, id string) (*tasks.Task, error)
	Update(ctx context.Context, id string, status tasks.TaskStatus, result string) error
}
