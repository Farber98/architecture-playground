//go:build integration

package queue

import (
	"context"
	"encoding/json"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// Helper function to create test execution context
func createTestExecutionContext(taskType string, payload interface{}) *taskContext.ExecutionContext {
	payloadBytes, _ := json.Marshal(payload)
	task := tasks.NewTask(taskType, json.RawMessage(payloadBytes))
	return taskContext.NewExecutionContext(task)
}

// Test helper for common queue operations
func testQueueBasicOperations(t *testing.T, queue TaskQueue) {
	ctx := context.Background()

	// Test enqueue and dequeue
	originalExecCtx := createTestExecutionContext("test", map[string]string{
		"message": "hello world",
	})

	// Enqueue
	err := queue.Enqueue(ctx, originalExecCtx)
	assert.NilError(t, err, "Failed to enqueue task")

	// Check queue depth
	depth, err := queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get queue depth")
	assert.Equal(t, int64(1), depth, "Queue depth should be 1 after enqueue")

	// Dequeue
	dequeuedExecCtx, err := queue.Dequeue(ctx)
	assert.NilError(t, err, "Failed to dequeue task")
	assert.Assert(t, dequeuedExecCtx != nil, "Dequeued execution context should not be nil")

	// Verify task content
	assert.Equal(t, originalExecCtx.Task.Type, dequeuedExecCtx.Task.Type)
	assert.Equal(t, string(originalExecCtx.Task.Payload), string(dequeuedExecCtx.Task.Payload))
	assert.Equal(t, originalExecCtx.Task.ID, dequeuedExecCtx.Task.ID)

	// Check queue is empty
	depth, err = queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get queue depth after dequeue")
	assert.Equal(t, int64(0), depth, "Queue should be empty after dequeue")
}

func testQueueFIFOOrdering(t *testing.T, queue TaskQueue) {
	ctx := context.Background()

	// Enqueue multiple tasks
	tasks := []*taskContext.ExecutionContext{
		createTestExecutionContext("task1", map[string]string{"order": "first"}),
		createTestExecutionContext("task2", map[string]string{"order": "second"}),
		createTestExecutionContext("task3", map[string]string{"order": "third"}),
	}

	for _, task := range tasks {
		err := queue.Enqueue(ctx, task)
		assert.NilError(t, err, "Failed to enqueue task")
	}

	// Check queue depth
	depth, err := queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get queue depth")
	assert.Equal(t, int64(3), depth, "Queue depth should be 3")

	// Dequeue and verify FIFO order
	for i, expectedTask := range tasks {
		dequeuedTask, err := queue.Dequeue(ctx)
		assert.NilError(t, err, "Failed to dequeue task %d", i)
		assert.Equal(t, expectedTask.Task.Type, dequeuedTask.Task.Type, "Task %d type mismatch", i)
		assert.Equal(t, string(expectedTask.Task.Payload), string(dequeuedTask.Task.Payload), "Task %d payload mismatch", i)
	}

	// Queue should be empty
	depth, err = queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get final queue depth")
	assert.Equal(t, int64(0), depth, "Queue should be empty")
}

func testQueueConcurrency(t *testing.T, queue TaskQueue) {
	ctx := context.Background()
	numTasks := 10

	// Enqueue tasks concurrently
	enqueueDone := make(chan struct{})
	go func() {
		defer close(enqueueDone)
		for i := 0; i < numTasks; i++ {
			execCtx := createTestExecutionContext("concurrent", map[string]int{"id": i})
			err := queue.Enqueue(ctx, execCtx)
			assert.NilError(t, err, "Failed to enqueue concurrent task %d", i)
		}
	}()

	// Wait for enqueuing to complete
	<-enqueueDone

	// Verify all tasks were enqueued
	depth, err := queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get queue depth")
	assert.Equal(t, int64(numTasks), depth, "All tasks should be enqueued")

	// Dequeue all tasks concurrently
	results := make(chan *taskContext.ExecutionContext, numTasks)
	errors := make(chan error, numTasks)

	for i := 0; i < numTasks; i++ {
		go func() {
			execCtx, err := queue.Dequeue(ctx)
			if err != nil {
				errors <- err
			} else {
				results <- execCtx
			}
		}()
	}

	// Collect results
	var dequeuedTasks []*taskContext.ExecutionContext
	for i := 0; i < numTasks; i++ {
		select {
		case execCtx := <-results:
			dequeuedTasks = append(dequeuedTasks, execCtx)
		case err := <-errors:
			t.Fatalf("Error during concurrent dequeue: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for concurrent dequeue %d", i)
		}
	}

	// Verify we got all tasks
	assert.Equal(t, numTasks, len(dequeuedTasks), "Should have dequeued all tasks")

	// Queue should be empty
	depth, err = queue.GetQueueDepth(ctx)
	assert.NilError(t, err, "Failed to get final queue depth")
	assert.Equal(t, int64(0), depth, "Queue should be empty after concurrent operations")
}
