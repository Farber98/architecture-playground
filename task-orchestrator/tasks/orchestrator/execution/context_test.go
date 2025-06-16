package execution

import (
	"errors"
	"task-orchestrator/tasks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewExecutionContext(t *testing.T) {
	task := tasks.NewTask("test", nil)

	ctx := NewExecutionContext(task)

	assert.Equal(t, task, ctx.Task)
	assert.Nil(t, ctx.Error)
	assert.False(t, ctx.StartTime.IsZero())
	assert.True(t, ctx.EndTime.IsZero())
	assert.NotNil(t, ctx.Metadata)
	assert.Equal(t, 0, len(ctx.Metadata))
}

func TestExecutionContext_SetError(t *testing.T) {
	task := tasks.NewTask("test", nil)
	ctx := NewExecutionContext(task)
	testErr := errors.New("test error")

	ctx.SetError(testErr)

	assert.Equal(t, testErr, ctx.Error)
	assert.False(t, ctx.EndTime.IsZero())
	assert.True(t, ctx.Metadata["has_error"].(bool))
	assert.Equal(t, "*errors.errorString", ctx.Metadata["error_type"])
	assert.False(t, ctx.IsSuccess())
}

func TestExecutionContext_SetSuccess(t *testing.T) {
	task := tasks.NewTask("test", nil)
	ctx := NewExecutionContext(task)

	ctx.SetSuccess()

	assert.Nil(t, ctx.Error)
	assert.False(t, ctx.EndTime.IsZero())
	assert.False(t, ctx.Metadata["has_error"].(bool))
	assert.True(t, ctx.IsSuccess())
}

func TestExecutionContext_IsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func(*ExecutionContext)
		expected bool
	}{
		{
			name:     "new context is success",
			setupCtx: func(ctx *ExecutionContext) {},
			expected: true,
		},
		{
			name: "context with error is not success",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SetError(errors.New("test error"))
			},
			expected: false,
		},
		{
			name: "context explicitly set to success",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SetSuccess()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tasks.NewTask("test", nil)
			ctx := NewExecutionContext(task)
			tt.setupCtx(ctx)

			assert.Equal(t, tt.expected, ctx.IsSuccess())
		})
	}
}

func TestExecutionContext_Duration(t *testing.T) {
	t.Run("duration while running", func(t *testing.T) {
		task := tasks.NewTask("test", nil)
		ctx := NewExecutionContext(task)

		time.Sleep(10 * time.Millisecond)
		duration := ctx.Duration()

		assert.True(t, duration >= 10*time.Millisecond)
		assert.True(t, duration < 100*time.Millisecond) // reasonable upper bound
	})

	t.Run("duration after completion", func(t *testing.T) {
		task := tasks.NewTask("test", nil)
		ctx := NewExecutionContext(task)

		time.Sleep(10 * time.Millisecond)
		ctx.SetSuccess()
		time.Sleep(10 * time.Millisecond) // Additional time after completion

		duration := ctx.Duration()

		// Duration should be fixed at completion time, not continue growing
		assert.True(t, duration >= 10*time.Millisecond)
		assert.True(t, duration < 50*time.Millisecond)
	})

	t.Run("duration after error", func(t *testing.T) {
		task := tasks.NewTask("test", nil)
		ctx := NewExecutionContext(task)

		time.Sleep(10 * time.Millisecond)
		ctx.SetError(errors.New("test error"))
		time.Sleep(10 * time.Millisecond) // Additional time after error

		duration := ctx.Duration()

		// Duration should be fixed at error time
		assert.True(t, duration >= 10*time.Millisecond)
		assert.True(t, duration < 50*time.Millisecond)
	})
}

func TestExecutionContext_Metadata(t *testing.T) {
	task := tasks.NewTask("test", nil)
	ctx := NewExecutionContext(task)

	// Test custom metadata
	ctx.Metadata["custom_key"] = "custom_value"
	assert.Equal(t, "custom_value", ctx.Metadata["custom_key"])

	// Test error metadata
	ctx.SetError(errors.New("test error"))
	assert.True(t, ctx.Metadata["has_error"].(bool))
	assert.Contains(t, ctx.Metadata["error_type"], "errorString")
	assert.Equal(t, "custom_value", ctx.Metadata["custom_key"]) // Should preserve existing metadata

	// Test success metadata
	ctx.SetSuccess()
	assert.False(t, ctx.Metadata["has_error"].(bool))
	assert.Equal(t, "custom_value", ctx.Metadata["custom_key"]) // Should preserve existing metadata
}
