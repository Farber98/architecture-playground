package queue

import (
	"context"
	"encoding/json"
	"fmt"
	taskContext "task-orchestrator/tasks/context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTaskQueue struct {
	client    *redis.Client
	queueName string
}

var _ TaskQueue = (*RedisTaskQueue)(nil)

func NewRedisTaskQueue(url, queueName string) (*RedisTaskQueue, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisTaskQueue{
		client:    client,
		queueName: queueName,
	}, nil
}

func (q *RedisTaskQueue) Enqueue(ctx context.Context, execCtx *taskContext.ExecutionContext) error {
	data, err := json.Marshal(execCtx)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Add to queue (left push for FIFO with right pop)
	return q.client.LPush(ctx, q.queueName, data).Err()
}

func (q *RedisTaskQueue) Dequeue(ctx context.Context) (*taskContext.ExecutionContext, error) {
	// Blocking right pop with 0 timeout (wait indefinetly)
	result, err := q.client.BRPop(ctx, 0, q.queueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	// BRPop returns [queueName, value]
	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected BRPop result format. Should have %d elements but got %d", 2, len(result))
	}

	var execCtx taskContext.ExecutionContext
	if err := json.Unmarshal([]byte(result[1]), &execCtx); err != nil {
		return nil, fmt.Errorf("failed to unmarshall task: %w", err)
	}

	return &execCtx, nil
}

func (q *RedisTaskQueue) GetQueueDepth(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.queueName).Result()
}

func (q *RedisTaskQueue) Close() error {
	return q.client.Close()
}
