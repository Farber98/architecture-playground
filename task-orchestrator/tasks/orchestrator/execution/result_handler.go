package execution

import (
	"fmt"
	"task-orchestrator/errors"
)

// ResultHandler standardizes result formatting across different execution strategies.
// This abstraction enables custom result processing for specific task types
// without coupling execution logic to result formatting details.
type ResultHandler interface {
	HandleSuccess(ctx *ExecutionContext)
	HandleFailure(ctx *ExecutionContext)
}

// DefaultResultHandler provides result formatting with minimal assumptions.
// Respects handler-set results while providing fallbacks for error scenarios.
type DefaultResultHandler struct{}

// NewDefaultResultHandler creates a result handler with conservative formatting.
func NewDefaultResultHandler() *DefaultResultHandler {
	return &DefaultResultHandler{}
}

// HandleSuccess finalizes successful execution without overriding business logic results.
// Task handlers are responsible for setting meaningful results during execution.
func (h *DefaultResultHandler) HandleSuccess(ctx *ExecutionContext) {
	ctx.SetSuccess()
	// Task result should already be set by handler
	// Nothing additional needed for success case
}

// HandleFailure provides informative error messages while respecting existing results.
// Only sets fallback messages when handlers haven't provided specific error details.
func (h *DefaultResultHandler) HandleFailure(ctx *ExecutionContext) {
	// Only set result if handler didn't set one
	// Format domain-specific errors with structured information for debugging
	if ctx.Task.Result == "" {
		if taskErr, ok := errors.IsTaskError(ctx.Error); ok {
			ctx.Task.Result = fmt.Sprintf("task %s: %s", taskErr.Type, taskErr.Message)
		} else {
			// Provide generic fallback for unexpected errors
			ctx.Task.Result = fmt.Sprintf("execution failed: %s", ctx.Error.Error())
		}
	}
}
