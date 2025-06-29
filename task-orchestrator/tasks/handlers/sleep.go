package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"task-orchestrator/errors"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"time"
)

// Sleeper abstracts time.Sleep to allow injection of real vs fake implementations.
// This makes the handler testable without incurring real wait time,
// and ensures task behavior can be validated deterministically in unit tests.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

// RealSleeper is the production implementation of Sleeper.
// It delegates directly to time.Sleep.
type realSleeper struct{}

func (s *realSleeper) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var _ TaskHandler = (*SleepHandler)(nil)

// SleepHandler pauses execution for the specified number of seconds.
type SleepHandler struct {
	sleeper Sleeper
	logger  *logger.Logger
}

// NewSleepHandler returns a production-ready SleepHandler using RealSleeper.
func NewSleepHandler(lg *logger.Logger) *SleepHandler {
	return &SleepHandler{sleeper: &realSleeper{}, logger: lg}
}

type sleepPayload struct {
	// Seconds must be a required, positive integer field.
	// Pointer type is used to distinguish missing vs zero values.
	Seconds *int `json:"seconds"`
}

func (h *SleepHandler) Run(ctx context.Context, task *tasks.Task) error {
	var p sleepPayload
	if err := json.Unmarshal(task.Payload, &p); err != nil {
		return errors.NewValidationError("invalid sleep payload", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
	}
	if p.Seconds == nil {
		return errors.NewValidationError("missing or invalid 'seconds' field", map[string]any{
			"task_id": task.ID,
		})
	}
	if *p.Seconds <= 0 {
		return errors.NewValidationError("invalid sleep duration: must be > 0", map[string]any{
			"task_id": task.ID,
			"seconds": *p.Seconds,
		})
	}
	h.logger.Task(task.ID, "executing sleep task", map[string]any{
		"seconds": *p.Seconds,
	})

	if err := h.sleeper.Sleep(ctx, time.Duration(*p.Seconds)*time.Second); err != nil {
		h.logger.Task(task.ID, "sleep task cancelled", map[string]any{
			"seconds": *p.Seconds,
			"error":   err.Error(),
		})
		return err
	}

	task.Result = fmt.Sprintf("slept: %d seconds", *p.Seconds)
	return nil
}
