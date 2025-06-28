//go:build integration

package queue

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRedisTestcontainer(t *testing.T) (*RedisTaskQueue, func()) {
	ctx := context.Background()

	uniqueQueueName := fmt.Sprintf("test_queue_%s_%d", t.Name(), time.Now().UnixNano())

	// Start Redis container with explicit port configuration
	redisContainer, err := redis.Run(ctx, "redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp").WithStartupTimeout(30*time.Second),
			wait.ForLog("Ready to accept connections").WithOccurrence(1).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Skipf("Failed to start Redis testcontainer: %v", err)
	}

	// Robust connection string generation with retries
	var redisURL string
	var connectionErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Try ConnectionString method first (most reliable)
		if connStr, err := redisContainer.ConnectionString(ctx); err == nil {
			redisURL = connStr + "/1" // Add database selection
			break
		}

		// Fallback to manual construction
		host, err := redisContainer.Host(ctx)
		if err != nil {
			connectionErr = fmt.Errorf("failed to get host (attempt %d): %w", attempt+1, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Try multiple port formats
		var port string
		if mappedPort, err := redisContainer.MappedPort(ctx, "6379/tcp"); err == nil {
			port = mappedPort.Port()
		} else if mappedPort, err := redisContainer.MappedPort(ctx, "6379"); err == nil {
			port = mappedPort.Port()
		} else {
			connectionErr = fmt.Errorf("failed to get port (attempt %d): %w", attempt+1, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		redisURL = fmt.Sprintf("redis://%s:%s/1", host, port)
		break
	}

	if redisURL == "" {
		redisContainer.Terminate(ctx)
		t.Fatalf("Failed to get Redis connection details after 3 attempts: %v", connectionErr)
	}

	t.Logf("Redis container started at: %s (queue: %s)", redisURL, uniqueQueueName)

	// Create queue with connection verification
	var queue *RedisTaskQueue
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		queue, err = NewRedisTaskQueue(redisURL, uniqueQueueName)
		if err == nil {
			// Verify connection works
			testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if pingErr := queue.client.Ping(testCtx).Err(); pingErr == nil {
				cancel()
				break
			}
			cancel()
			queue.Close()
			queue = nil
		}

		if i < maxRetries-1 {
			t.Logf("Failed to connect to Redis, retrying... (%d/%d): %v", i+1, maxRetries, err)
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}

	if queue == nil {
		redisContainer.Terminate(ctx)
		t.Fatalf("Failed to create working Redis queue after %d retries: %v", maxRetries, err)
	}

	cleanup := func() {
		ctx := context.Background()
		if queue != nil {
			// Clean up test data before closing
			queue.client.Del(ctx, uniqueQueueName)
			queue.Close()
		}
		if terminateErr := redisContainer.Terminate(ctx); terminateErr != nil {
			t.Logf("Failed to terminate container: %v", terminateErr)
		}
	}

	// Clear any existing test data
	queue.client.Del(ctx, uniqueQueueName)

	return queue, cleanup
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
