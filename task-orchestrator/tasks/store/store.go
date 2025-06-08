package store

import "task-orchestrator/tasks"

// TaskStore defines the contract for task persistance
type TaskStore interface {
	Save(task *tasks.Task) error
	Get(id string) (*tasks.Task, error)
	Update(id string, status string, result string) error
}
