package handlers

import (
	"bytes"
	"encoding/json"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

// FakeSleeper is a test double for Sleeper.
type fakeSleeper struct {
	CalledWith time.Duration
}

func (f *fakeSleeper) Sleep(d time.Duration) {
	f.CalledWith = d
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
			fakeSleeper := &fakeSleeper{}
			handler := &SleepHandler{
				sleeper: fakeSleeper,
				logger:  testLogger,
			}

			task := tasks.NewTask("sleep", json.RawMessage(tt.payload))

			err := handler.Run(task)

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
				}

				// Verify logger was called
				logOutput := buf.String()
				assert.Assert(t, len(logOutput) > 0, "Expected log output")
				assert.Assert(t, bytes.Contains(buf.Bytes(), []byte(task.ID)), "Log should contain task ID")
			}
		})
	}
}
