package tasks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStatus_String(t *testing.T) {
	testCases := []struct {
		status   TaskStatus
		expected string
	}{
		{StatusSubmitted, "submitted"},
		{StatusRunning, "running"},
		{StatusDone, "done"},
		{StatusFailed, "failed"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.String())
		})
	}
}

func TestTaskStatus_IsFinal(t *testing.T) {
	testCases := []struct {
		status   TaskStatus
		expected bool
	}{
		{StatusSubmitted, false},
		{StatusRunning, false},
		{StatusDone, true},
		{StatusFailed, true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.IsFinal())
		})
	}
}

func TestTaskStatus_IsActive(t *testing.T) {
	testCases := []struct {
		status   TaskStatus
		expected bool
	}{
		{StatusSubmitted, false},
		{StatusRunning, true},
		{StatusDone, false},
		{StatusFailed, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.IsActive())
		})
	}
}

func TestTaskStatus_CanTransitionTo(t *testing.T) {
	testCases := []struct {
		name        string
		from        TaskStatus
		to          TaskStatus
		shouldError bool
	}{
		// Valid transitions from submitted
		{"submitted to running", StatusSubmitted, StatusRunning, false},
		{"submitted to failed", StatusSubmitted, StatusFailed, false},

		// Valid transitions from running
		{"running to done", StatusRunning, StatusDone, false},
		{"running to failed", StatusRunning, StatusFailed, false},

		// Invalid transitions from submitted
		{"submitted to done", StatusSubmitted, StatusDone, true},

		// Invalid transitions from running
		{"running to submitted", StatusRunning, StatusSubmitted, true},

		// Invalid transitions from terminal states
		{"done to running", StatusDone, StatusRunning, true},
		{"done to failed", StatusDone, StatusFailed, true},
		{"done to submitted", StatusDone, StatusSubmitted, true},
		{"failed to running", StatusFailed, StatusRunning, true},
		{"failed to done", StatusFailed, StatusDone, true},
		{"failed to submitted", StatusFailed, StatusSubmitted, true},

		// Self-transitions (should fail)
		{"submitted to submitted", StatusSubmitted, StatusSubmitted, true},
		{"running to running", StatusRunning, StatusRunning, true},
		{"done to done", StatusDone, StatusDone, true},
		{"failed to failed", StatusFailed, StatusFailed, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.canTransitionTo(tc.to)
			if tc.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid transition")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskStatus_CanTransitionTo_InvalidStatus(t *testing.T) {
	invalidStatus := TaskStatus("invalid")
	err := invalidStatus.canTransitionTo(StatusRunning)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown current status")
}

func TestNewTask(t *testing.T) {
	taskType := "print"
	payload := json.RawMessage(`{"message":"test"}`)

	task := NewTask(taskType, payload)

	require.NotNil(t, task)
	assert.Equal(t, taskType, task.Type)
	assert.Equal(t, payload, task.Payload)
	assert.Equal(t, StatusSubmitted, task.Status)
	assert.Empty(t, task.Result)
	assert.NotEmpty(t, task.ID)

	// Verify ID is a valid UUID format (36 characters with dashes)
	assert.Len(t, task.ID, 36)
	assert.Contains(t, task.ID, "-")
}

func TestNewTask_GeneratesUniqueIDs(t *testing.T) {
	// Test that each call generates a unique ID
	task1 := NewTask("print", json.RawMessage(`{"message":"test1"}`))
	task2 := NewTask("print", json.RawMessage(`{"message":"test2"}`))

	assert.NotEqual(t, task1.ID, task2.ID, "Each task should have a unique ID")
}

func TestTask_SetStatus(t *testing.T) {
	task := NewTask("print", json.RawMessage(`{}`))

	// Test valid transition: submitted -> running
	err := task.SetStatus(StatusRunning)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, task.Status)

	// Test invalid transition: running -> submitted
	err = task.SetStatus(StatusSubmitted)
	require.Error(t, err)
	assert.Contains(t, err.Error(), task.ID) // Use the auto-generated ID
	assert.Contains(t, err.Error(), "invalid transition")
	assert.Equal(t, StatusRunning, task.Status) // Status unchanged
}

func TestTask_SetStatus_ValidTransitions(t *testing.T) {
	t.Run("submitted to running", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		err := task.SetStatus(StatusRunning)
		require.NoError(t, err)
		assert.Equal(t, StatusRunning, task.Status)
	})

	t.Run("submitted to failed", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		err := task.SetStatus(StatusFailed)
		require.NoError(t, err)
		assert.Equal(t, StatusFailed, task.Status)
	})

	t.Run("running to done", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		// First transition to running
		require.NoError(t, task.SetStatus(StatusRunning))

		// Then transition to done
		err := task.SetStatus(StatusDone)
		require.NoError(t, err)
		assert.Equal(t, StatusDone, task.Status)
	})

	t.Run("running to failed", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		// First transition to running
		require.NoError(t, task.SetStatus(StatusRunning))

		// Then transition to failed
		err := task.SetStatus(StatusFailed)
		require.NoError(t, err)
		assert.Equal(t, StatusFailed, task.Status)
	})
}

func TestTask_SetStatus_InvalidTransitions(t *testing.T) {
	t.Run("submitted to done", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		err := task.SetStatus(StatusDone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transition")
		assert.Equal(t, StatusSubmitted, task.Status) // Status unchanged
	})

	t.Run("done to running", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		// First complete the task
		require.NoError(t, task.SetStatus(StatusRunning))
		require.NoError(t, task.SetStatus(StatusDone))

		// Try to restart it
		err := task.SetStatus(StatusRunning)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transition")
		assert.Equal(t, StatusDone, task.Status) // Status unchanged
	})

	t.Run("failed to running", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		// First fail the task
		require.NoError(t, task.SetStatus(StatusFailed))

		// Try to restart it
		err := task.SetStatus(StatusRunning)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transition")
		assert.Equal(t, StatusFailed, task.Status) // Status unchanged
	})
}

func TestTask_WorkflowScenarios(t *testing.T) {
	t.Run("happy path: submitted -> running -> done", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{"message":"test"}`))

		// Start running
		require.NoError(t, task.SetStatus(StatusRunning))
		assert.Equal(t, StatusRunning, task.Status)
		assert.False(t, task.Status.IsFinal())

		// Complete successfully
		task.Result = "task completed successfully"
		require.NoError(t, task.SetStatus(StatusDone))
		assert.Equal(t, StatusDone, task.Status)
		assert.Equal(t, "task completed successfully", task.Result)
		assert.True(t, task.Status.IsFinal())
	})

	t.Run("setup failure: submitted -> failed", func(t *testing.T) {
		task := NewTask("invalid", json.RawMessage(`{}`))

		// Fail during setup
		task.Result = "no handler found"
		require.NoError(t, task.SetStatus(StatusFailed))
		assert.Equal(t, StatusFailed, task.Status)
		assert.Equal(t, "no handler found", task.Result)
		assert.True(t, task.Status.IsFinal())
	})

	t.Run("execution failure: submitted -> running -> failed", func(t *testing.T) {
		task := NewTask("print", json.RawMessage(`{}`))

		// Start running
		require.NoError(t, task.SetStatus(StatusRunning))

		// Fail during execution
		task.Result = "handler crashed"
		require.NoError(t, task.SetStatus(StatusFailed))
		assert.Equal(t, StatusFailed, task.Status)
		assert.Equal(t, "handler crashed", task.Result)
		assert.True(t, task.Status.IsFinal())
	})
}

func TestTask_ResultAndStatusTogether(t *testing.T) {
	task := NewTask("print", json.RawMessage(`{"message":"test"}`))

	// Start running
	require.NoError(t, task.SetStatus(StatusRunning))

	// Complete with result
	task.Result = "operation completed successfully"
	require.NoError(t, task.SetStatus(StatusDone))

	assert.Equal(t, StatusDone, task.Status)
	assert.Equal(t, "operation completed successfully", task.Result)
}

func TestTask_JSONSerialization(t *testing.T) {
	original := &Task{
		ID:      "test-id",
		Type:    "print",
		Status:  StatusRunning,
		Result:  "in progress",
		Payload: json.RawMessage(`{"message":"test"}`),
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var restored Task
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify all fields preserved
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Status, restored.Status)
	assert.Equal(t, original.Result, restored.Result)
	assert.Equal(t, original.Payload, restored.Payload)
}

func TestTask_StatusHelperMethods(t *testing.T) {
	task := NewTask("print", json.RawMessage(`{}`))

	// Test submitted state
	assert.False(t, task.Status.IsActive())
	assert.False(t, task.Status.IsFinal())

	// Test running state
	require.NoError(t, task.SetStatus(StatusRunning))
	assert.True(t, task.Status.IsActive())
	assert.False(t, task.Status.IsFinal())

	// Test done state
	require.NoError(t, task.SetStatus(StatusDone))
	assert.False(t, task.Status.IsActive())
	assert.True(t, task.Status.IsFinal())
}

func TestTask_StatusHelperMethods_Failed(t *testing.T) {
	task := NewTask("print", json.RawMessage(`{}`))

	// Test failed state
	require.NoError(t, task.SetStatus(StatusFailed))
	assert.False(t, task.Status.IsActive())
	assert.True(t, task.Status.IsFinal())
}
