package store

import (
	"context"
	"fmt"
	"sync"
	"task-orchestrator/tasks"
)

// Compile-time check to ensure MemoryTaskStore implements TaskStore interface
var _ TaskStore = (*MemoryTaskStore)(nil)

// MemoryTaskStore provides an in-memory implementation of a task persistence layer.
type MemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*tasks.Task
}

// NewMemoryTaskStore creates and initializes a new MemoryTaskStore.
func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{
		tasks: make(map[string]*tasks.Task),
	}
}

// Save adds a new task to the store.
// It ensures task ID uniqueness to prevent accidental overwrites or state corruption.
func (s *MemoryTaskStore) Save(ctx context.Context, task *tasks.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

// Get retrieves a task by its ID.
// It returns a copy of the task to prevent external callers from unintentionally
// modifying the state of the task stored within the map.
func (s *MemoryTaskStore) Get(_ context.Context, id string) (*tasks.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task with ID %s not found", id)
	}

	copied := *task
	return &copied, nil
}

// Update modifies the status and result of an existing task.
// This method allows for changing mutable fields of a task after it has been saved,
func (s *MemoryTaskStore) Update(_ context.Context, id string, status tasks.TaskStatus, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task with ID %s not found", id)
	}

	// Don't validate transitions here - trust the orchestrator
	// The orchestrator has already managed the status transition
	task.Status = status
	task.Result = result

	return nil
}
