package registry

import (
	"sync"
	"task-orchestrator/tasks/handlers"
)

// HandlerRegistry maintains a mapping between task types and their associated handlers.
// This registry enables runtime resolution of task logic based on the task's declared type,
// supporting a plugin-style architecture where new behaviors can be registered independently.
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]handlers.TaskHandler
}

// NewRegistry constructs a new handler registry.
func NewRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]handlers.TaskHandler),
	}
}

// Register binds a TaskHandler to a specific task type.
// This should be called during application initialization.
func (r *HandlerRegistry) Register(taskType string, handler handlers.TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[taskType] = handler
}

// Get returns the handler registered for the given task type.
// If no handler is registered, ok will be false.
func (r *HandlerRegistry) Get(taskType string) (handlers.TaskHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[taskType]
	return h, ok
}

// GetRegisteredTypes returns a slice of all registered task types.
// This is useful for health checks, debugging, and API documentation.
func (r *HandlerRegistry) GetRegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for taskType := range r.handlers {
		types = append(types, taskType)
	}
	return types
}
