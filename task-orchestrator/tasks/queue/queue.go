package queue

import (
	"context"
	taskContext "task-orchestrator/tasks/context"
)

// TaskQueue interface for background task processing
type TaskQueue interface {
	// Enqueue adds a task with its execution context to the queue
	Enqueue(ctx context.Context, execCtx *taskContext.ExecutionContext) error

	// Dequeue removes and returns a task with execution context from the queue
	Dequeue(ctx context.Context) (*taskContext.ExecutionContext, error)

	// GetQueueDepth returns the number of tasks waiting in queue
	GetQueueDepth(ctx context.Context) (int64, error)

	// Close cleanly shuts down the queue connection
	Close() error
}
