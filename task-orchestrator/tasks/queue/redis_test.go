//go:build integration

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"task-orchestrator/tasks"
	taskContext "task-orchestrator/tasks/context"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

const testQueueName = "test_queue"

func TestRedisTaskQueue_NewRedisTaskQueue(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	assert.Assert(t, queue != nil)
	assert.Assert(t, len(queue.queueName) > 0)
	assert.Assert(t, queue.client != nil)
}

func TestRedisTaskQueue_BasicOperations(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	testQueueBasicOperations(t, queue)
}

func TestRedisTaskQueue_FIFOOrdering(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	testQueueFIFOOrdering(t, queue)
}

func TestRedisTaskQueue_Concurrency(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	testQueueConcurrency(t, queue)
}

func TestRedisTaskQueue_ConnectionErrors(t *testing.T) {
	// Test invalid Redis URL
	_, err := NewRedisTaskQueue("invalid://url", "test")
	assert.ErrorContains(t, err, "invalid Redis URL")

	// Test unreachable Redis
	_, err = NewRedisTaskQueue("redis://localhost:99999/1", "test")
	assert.ErrorContains(t, err, "failed to connect to Redis")
}

func TestRedisTaskQueue_LargePayload(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	ctx := context.Background()

	// Create 1MB payload
	largeData := make(map[string]string)
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = strings.Repeat("x", 1000)
	}

	execCtx := createTestExecutionContext("large_test", largeData)

	err := queue.Enqueue(ctx, execCtx)
	assert.NilError(t, err)

	dequeuedCtx, err := queue.Dequeue(ctx)
	assert.NilError(t, err)
	assert.Equal(t, string(execCtx.Task.Payload), string(dequeuedCtx.Task.Payload))
}

func TestRedisTaskQueue_InvalidData(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	ctx := context.Background()

	// Manually insert invalid JSON
	err := queue.client.LPush(ctx, queue.queueName, "invalid-json").Err()
	assert.NilError(t, err)

	_, err = queue.Dequeue(ctx)
	assert.ErrorContains(t, err, "failed to unmarshall task")
}

func TestRedisTaskQueue_Close(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	err := queue.Close()
	assert.NilError(t, err)

	// Operations after close should fail
	ctx := context.Background()
	execCtx := createTestExecutionContext("test", map[string]string{})

	err = queue.Enqueue(ctx, execCtx)
	assert.ErrorContains(t, err, "client is closed")
}

func TestRedisTaskQueue_HighThroughput(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	ctx := context.Background()
	numTasks := 1000

	start := time.Now()

	// Enqueue many tasks
	for i := 0; i < numTasks; i++ {
		execCtx := createTestExecutionContext("perf_test", map[string]int{"id": i})
		err := queue.Enqueue(ctx, execCtx)
		assert.NilError(t, err)
	}

	// Dequeue all
	for i := 0; i < numTasks; i++ {
		_, err := queue.Dequeue(ctx)
		assert.NilError(t, err)
	}

	duration := time.Since(start)
	t.Logf("Processed %d tasks in %v (%.2f tasks/sec)",
		numTasks, duration, float64(numTasks)/duration.Seconds())
}

func TestRedisTaskQueue_MarshalError(t *testing.T) {
	queue, cleanup := setupRedisTestcontainer(t)
	defer cleanup()

	ctx := context.Background()

	// Create task with invalid JSON payload
	task := &tasks.Task{
		ID:      "test-id",
		Type:    "marshal_test",
		Payload: json.RawMessage(string([]byte{0xff, 0xfe, 0xfd})), // Invalid UTF-8
	}
	execCtx := &taskContext.ExecutionContext{Task: task}

	err := queue.Enqueue(ctx, execCtx)
	assert.ErrorContains(t, err, "failed to marshal task")
}
