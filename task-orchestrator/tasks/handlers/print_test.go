package handlers

import (
	"bytes"
	"encoding/json"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestPrintHandler_Run(t *testing.T) {
	// Create a test logger that captures output
	var buf bytes.Buffer
	testLogger := logger.New("DEBUG", &buf)

	handler := &PrintHandler{logger: testLogger}

	tests := []struct {
		name            string
		payload         string
		wantErr         bool
		wantStatus      string
		wantResult      string
		wantErrContains string
	}{
		{
			name:       "basic message",
			payload:    `{"message":"hello"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: hello",
		},
		{
			name:       "empty message",
			payload:    `{"message":""}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: ",
		},
		{
			name:       "special characters",
			payload:    `{"message":"hello\nworld\t!"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: hello\nworld\t!",
		},
		{
			name:       "unicode message",
			payload:    `{"message":"Hello 世界 🌍"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: Hello 世界 🌍",
		},
		{
			name:            "invalid JSON",
			payload:         `{"message":"unclosed string`,
			wantErr:         true,
			wantErrContains: "invalid print payload",
		},
		{
			name:       "missing message field",
			payload:    `{"other_field":"value"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: ",
		},
		{
			name:       "empty payload",
			payload:    `{}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: ",
		},
		{
			name:       "long message",
			payload:    `{"message":"this is a very long message with lots of text to test how the handler deals with longer content"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: this is a very long message with lots of text to test how the handler deals with longer content",
		},
		{
			name:       "message with quotes",
			payload:    `{"message":"hello \"world\" with 'quotes'"}`,
			wantErr:    false,
			wantStatus: "done",
			wantResult: "printed: hello \"world\" with 'quotes'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset buffer for each test
			buf.Reset()

			task := &tasks.Task{
				ID:      "test-id",
				Type:    "print",
				Payload: json.RawMessage(tt.payload),
			}

			err := handler.Run(task)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.ErrorContains(t, err, tt.wantErrContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStatus, task.Status)
				assert.Equal(t, tt.wantResult, task.Result)

				// Verify logger was called (optional verification)
				logOutput := buf.String()
				assert.Assert(t, len(logOutput) > 0, "Expected log output")
				assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("test-id")), "Log should contain task ID")
			}
		})
	}
}
