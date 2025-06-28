package workers

import (
	"bytes"
	"context"
	"task-orchestrator/logger"
	taskContext "task-orchestrator/tasks/context"
	"task-orchestrator/tasks/orchestrator/execution"
	"task-orchestrator/tasks/queue"
	"task-orchestrator/tasks/registry"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
)

func createTestPool(workerCount int, queue queue.TaskQueue, stateManager execution.StateManager) (*WorkerPool, *logger.Logger) {
	var buf bytes.Buffer
	logger := logger.New("DEBUG", &buf)
	registry := registry.NewRegistry()
	pool := NewWorkerPool(workerCount, queue, stateManager, registry, logger)
	return pool, logger
}

func TestWorkerPool_NewWorkerPool(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Test with different worker counts
	testCases := []struct {
		name        string
		workerCount int
	}{
		{"single worker", 1},
		{"multiple workers", 3},
		{"many workers", 10},
		{"zero workers", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pool, _ := createTestPool(tc.workerCount, mockQueue, mockStateManager)

			assert.Assert(t, pool != nil)
			assert.Equal(t, tc.workerCount, pool.GetWorkerCount())
			assert.Equal(t, 30*time.Second, pool.shutdownTimeout) // Default timeout
			assert.Assert(t, pool.workers != nil)
			assert.Assert(t, pool.logger != nil)
		})
	}
}

func TestWorkerPool_GetWorkerCount(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	pool, _ := createTestPool(5, mockQueue, mockStateManager)

	assert.Equal(t, 5, pool.GetWorkerCount())
}

func TestWorkerPool_SetShutdownTimeout(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	// Test setting custom timeout
	customTimeout := 10 * time.Second
	pool.SetShutdownTimeout(customTimeout)

	// Verify timeout was set (access via getter if available, or check behavior)
	assert.Equal(t, customTimeout, pool.shutdownTimeout)
}

func TestWorkerPool_StartStop_WithZeroWorkers(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	pool, _ := createTestPool(0, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start pool with zero workers
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop pool
	pool.Stop()

	// Should complete quickly since no workers to wait for
	select {
	case <-done:
		// Success - empty pool handled correctly
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Empty pool should start/stop quickly")
	}
}

func TestWorkerPool_StartStop_SingleWorker(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock to not panic when dequeue is called
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start pool
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Stop pool
	pool.Stop()

	// Should stop gracefully
	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Single worker pool did not stop within timeout")
	}
}

func TestWorkerPool_StartStop_MultipleWorkers(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock to not panic when dequeue is called
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(3, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start pool
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Stop pool
	pool.Stop()

	// Should stop gracefully
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Multi-worker pool did not stop within timeout")
	}
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock to not panic
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(2, mockQueue, mockStateManager)

	ctx, cancel := context.WithCancel(context.Background())

	// Start pool
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context (instead of calling Stop)
	cancel()

	// Should stop via context cancellation
	select {
	case <-done:
		// Success
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Pool did not stop on context cancellation")
	}
}

func TestWorkerPool_ShutdownTimeout(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock to simulate hanging workers
	mockQueue.On("Dequeue", mock.Anything).Return(func(ctx context.Context) (*taskContext.ExecutionContext, error) {
		// Simulate a worker that takes a long time to stop
		select {
		case <-ctx.Done():
			// Simulate slow shutdown
			time.Sleep(200 * time.Millisecond)
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return nil, context.DeadlineExceeded
		}
	}).Maybe()

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	// Set very short timeout to trigger timeout scenario
	pool.SetShutdownTimeout(50 * time.Millisecond)

	ctx := t.Context()

	// Start pool
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	// Give worker time to start
	time.Sleep(20 * time.Millisecond)

	// Stop pool - should timeout waiting for worker
	stopStart := time.Now()
	pool.Stop()
	stopDuration := time.Since(stopStart)

	// Should have timed out quickly (around 50ms, not 200ms)
	assert.Assert(t, stopDuration < 150*time.Millisecond, "Stop should have timed out quickly")
	assert.Assert(t, stopDuration >= 40*time.Millisecond, "Stop should have waited for timeout period")
}

func TestWorkerPool_DoubleStart(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock expectations for Dequeue calls
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start pool first time
	done1 := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done1)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Try to start again - should handle gracefully
	done2 := make(chan struct{})
	go func() {
		pool.Start(ctx) // This might create new workers or do nothing
		close(done2)
	}()

	time.Sleep(50 * time.Millisecond)

	// Stop pool
	pool.Stop()

	// Both should complete
	select {
	case <-done1:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("First start did not complete")
	}

	select {
	case <-done2:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Second start did not complete")
	}

	mockQueue.AssertExpectations(t)
}

func TestWorkerPool_StopWithoutStart(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	pool, _ := createTestPool(2, mockQueue, mockStateManager)

	// Stop pool without starting - should not panic
	pool.Stop()

	// Should complete quickly
	// If this test hangs, there's a bug in Stop()
}

func TestWorkerPool_MultipleStops(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}

	// Setup mock
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start pool
	done := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)

	// Stop multiple times - should not panic
	pool.Stop()
	pool.Stop()
	pool.Stop()

	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Pool did not stop after multiple Stop calls")
	}
}

func TestWorkerPool_ConcurrentStartStop(t *testing.T) {
	mockQueue := &MockTaskQueue{}
	mockStateManager := &MockStateManager{}
	mockQueue.On("Dequeue", mock.Anything).Return(nil, context.Canceled).Maybe()

	pool, _ := createTestPool(1, mockQueue, mockStateManager)

	ctx := t.Context()

	// Start and stop concurrently multiple times
	for range 5 {
		go func() {
			pool.Start(ctx)
		}()

		go func() {
			time.Sleep(1 * time.Millisecond)
			pool.Stop()
		}()
	}

	// Give concurrent operations time to complete
	time.Sleep(100 * time.Millisecond)

	// Final stop to clean up
	pool.Stop()

	// If we get here without panic, the pool handled concurrency correctly
}
