package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

// FakeSleeper is a test double for Sleeper.
type fakeSleeper struct {
	CalledWith        time.Duration
	CalledWithContext context.Context
	ShouldReturnError error
}

func (f *fakeSleeper) Sleep(ctx context.Context, d time.Duration) error {
	f.CalledWith = d
	f.CalledWithContext = ctx
	return f.ShouldReturnError
}

func TestSleepHandler_Run(t *testing.T) {
	tests := []struct {
		name              string
		payload           string
		wantErr           bool
		wantResult        string
		wantErrContains   string
		wantSleepCalled   bool
		wantSleepDuration time.Duration
	}{
		{
			name:              "basic sleep - 1 second",
			payload:           `{"seconds":1}`,
			wantErr:           false,
			wantResult:        "slept: 1 seconds",
			wantSleepCalled:   true,
			wantSleepDuration: 1 * time.Second,
		},
		{
			name:              "multiple seconds",
			payload:           `{"seconds":2}`,
			wantErr:           false,
			wantResult:        "slept: 2 seconds",
			wantSleepCalled:   true,
			wantSleepDuration: 2 * time.Second,
		},
		{
			name:            "zero seconds - should error",
			payload:         `{"seconds":0}`,
			wantErr:         true,
			wantErrContains: "invalid sleep duration: must be > 0",
			wantSleepCalled: false,
		},
		{
			name:            "negative seconds - should error",
			payload:         `{"seconds":-1}`,
			wantErr:         true,
			wantErrContains: "invalid sleep duration: must be > 0",
			wantSleepCalled: false,
		},
		{
			name:            "missing seconds field",
			payload:         `{"other_field":"value"}`,
			wantErr:         true,
			wantErrContains: "missing or invalid 'seconds' field",
			wantSleepCalled: false,
		},
		{
			name:            "empty payload",
			payload:         `{}`,
			wantErr:         true,
			wantErrContains: "missing or invalid 'seconds' field",
			wantSleepCalled: false,
		},
		{
			name:            "null seconds",
			payload:         `{"seconds":null}`,
			wantErr:         true,
			wantErrContains: "missing or invalid 'seconds' field",
			wantSleepCalled: false,
		},
		{
			name:            "invalid JSON",
			payload:         `{"seconds":"not a number"`,
			wantErr:         true,
			wantErrContains: "invalid sleep payload",
			wantSleepCalled: false,
		},
		{
			name:            "string seconds",
			payload:         `{"seconds":"5"}`,
			wantErr:         true,
			wantErrContains: "invalid sleep payload",
			wantSleepCalled: false,
		},
		{
			name:            "float seconds",
			payload:         `{"seconds":1.5}`,
			wantErr:         true,
			wantErrContains: "invalid sleep payload",
			wantSleepCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test logger and fake sleeper
			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)
			fakeSleeper := &fakeSleeper{
				ShouldReturnError: nil,
			}
			handler := &SleepHandler{
				sleeper: fakeSleeper,
				logger:  testLogger,
			}

			task := tasks.NewTask("sleep", json.RawMessage(tt.payload))

			err := handler.Run(context.Background(), task)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.ErrorContains(t, err, tt.wantErrContains)
				}
				// Verify sleep was NOT called on error
				assert.Equal(t, time.Duration(0), fakeSleeper.CalledWith)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResult, task.Result)

				// Verify sleep was called with correct duration
				if tt.wantSleepCalled {
					assert.Equal(t, tt.wantSleepDuration, fakeSleeper.CalledWith)
					assert.Equal(t, context.Background(), fakeSleeper.CalledWithContext)
				}

				// Verify logger was called
				logOutput := buf.String()
				assert.Assert(t, len(logOutput) > 0, "Expected log output")
				assert.Assert(t, bytes.Contains(buf.Bytes(), []byte(task.ID)), "Log should contain task ID")
			}
		})
	}
}

func TestSleepHandler_Run_ContextCancellation(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)
	fakeSleeper := &fakeSleeper{
		ShouldReturnError: context.Canceled, // Simulate cancellation
	}
	handler := &SleepHandler{
		sleeper: fakeSleeper,
		logger:  testLogger,
	}

	task := tasks.NewTask("sleep", json.RawMessage(`{"seconds":10}`))

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Run(ctx, task)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Verify sleep was called with cancelled context
	assert.Equal(t, 10*time.Second, fakeSleeper.CalledWith)
	assert.Equal(t, ctx, fakeSleeper.CalledWithContext)

	// Task result should not be set on cancellation
	assert.Equal(t, "", task.Result)

	// Verify cancellation was logged
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("sleep task cancelled")), "Should log cancellation")
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("context canceled")), "Should log cancellation reason")
}

func TestSleepHandler_Run_ContextTimeout(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)
	fakeSleeper := &fakeSleeper{
		ShouldReturnError: context.DeadlineExceeded, // Simulate timeout
	}
	handler := &SleepHandler{
		sleeper: fakeSleeper,
		logger:  testLogger,
	}

	task := tasks.NewTask("sleep", json.RawMessage(`{"seconds":10}`))

	// Create context with timeout (already expired)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(2 * time.Millisecond)

	err := handler.Run(ctx, task)

	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Verify sleep was attempted with timed-out context
	assert.Equal(t, 10*time.Second, fakeSleeper.CalledWith)
	assert.Equal(t, ctx, fakeSleeper.CalledWithContext)

	// Task result should not be set on timeout
	assert.Equal(t, "", task.Result)

	// Verify timeout was logged
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("sleep task cancelled")), "Should log cancellation")
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("context deadline exceeded")), "Should log timeout reason")
}

func TestSleepHandler_Run_SleepError(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)
	customError := errors.New("an error")
	fakeSleeper := &fakeSleeper{
		ShouldReturnError: customError, // Simulate any sleep error
	}
	handler := &SleepHandler{
		sleeper: fakeSleeper,
		logger:  testLogger,
	}

	task := tasks.NewTask("sleep", json.RawMessage(`{"seconds":5}`))

	err := handler.Run(context.Background(), task)

	require.Error(t, err)
	assert.Equal(t, customError, err)

	// Verify sleep was called
	assert.Equal(t, 5*time.Second, fakeSleeper.CalledWith)

	// Task result should not be set on error
	assert.Equal(t, "", task.Result)

	// Verify error was logged
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("sleep task cancelled")), "Should log error")
}

func TestSleepHandler_Run_ContextPropagation(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)
	fakeSleeper := &fakeSleeper{
		ShouldReturnError: nil, // Success
	}
	handler := &SleepHandler{
		sleeper: fakeSleeper,
		logger:  testLogger,
	}

	task := tasks.NewTask("sleep", json.RawMessage(`{"seconds":3}`))

	// Create context with value to test propagation
	// Define custom key type to avoid collisions
	type testContextKey string
	const testKey testContextKey = "test-key"

	ctx := context.WithValue(context.Background(), testKey, "test-value")

	err := handler.Run(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, "slept: 3 seconds", task.Result)

	// Verify the exact context was passed to sleeper
	assert.Equal(t, ctx, fakeSleeper.CalledWithContext)
	assert.Equal(t, "test-value", fakeSleeper.CalledWithContext.Value(testKey))
}

func TestSleepHandler_Run_VariousDurations(t *testing.T) {
	durations := []struct {
		seconds  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{5, 5 * time.Second},
		{10, 10 * time.Second},
		{60, 60 * time.Second},
		{3600, 3600 * time.Second}, // 1 hour
	}

	for _, d := range durations {
		t.Run(fmt.Sprintf("sleep_%d_seconds", d.seconds), func(t *testing.T) {
			var buf bytes.Buffer
			testLogger := logger.New("DEBUG", &buf)
			fakeSleeper := &fakeSleeper{}
			handler := &SleepHandler{
				sleeper: fakeSleeper,
				logger:  testLogger,
			}

			payload := fmt.Sprintf(`{"seconds":%d}`, d.seconds)
			task := tasks.NewTask("sleep", json.RawMessage(payload))

			err := handler.Run(context.Background(), task)

			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("slept: %d seconds", d.seconds), task.Result)
			assert.Equal(t, d.expected, fakeSleeper.CalledWith)
		})
	}
}
