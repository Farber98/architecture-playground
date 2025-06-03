package errors

import (
	"fmt"
	"net/http"
)

// TaskErrorType categorizes different kinds of task failures
type TaskErrorType string

const (
	ValidationError TaskErrorType = "validation"
	ExecutionError  TaskErrorType = "execution"
	NotFoundError   TaskErrorType = "not_found"
	InternalError   TaskErrorType = "internal"
)

// TaskError provides structured error information with HTTP status suggestions
type TaskError struct {
	Type    TaskErrorType  `json:"type"`
	Message string         `json:"message"`
	Code    int            `json:"code"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *TaskError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Constructor functions for common error types
func NewValidationError(message string, details ...map[string]any) *TaskError {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return &TaskError{
		Type:    ValidationError,
		Message: message,
		Code:    http.StatusBadRequest,
		Details: d,
	}
}

func NewExecutionError(message string, details ...map[string]any) *TaskError {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return &TaskError{
		Type:    ExecutionError,
		Message: message,
		Code:    http.StatusUnprocessableEntity,
		Details: d,
	}
}

func NewNotFoundError(message string) *TaskError {
	return &TaskError{
		Type:    NotFoundError,
		Message: message,
		Code:    http.StatusNotFound,
	}
}

func NewInternalError(message string) *TaskError {
	return &TaskError{
		Type:    InternalError,
		Message: message,
		Code:    http.StatusInternalServerError,
	}
}

// IsTaskError checks if an error is a TaskError and returns it
func IsTaskError(err error) (*TaskError, bool) {
	if taskErr, ok := err.(*TaskError); ok {
		return taskErr, true
	}
	return nil, false
}
