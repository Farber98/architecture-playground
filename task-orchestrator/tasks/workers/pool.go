package workers

import (
	"context"
	"sync"
	"task-orchestrator/logger"
	"task-orchestrator/tasks/orchestrator/execution"
	"task-orchestrator/tasks/queue"
	"task-orchestrator/tasks/registry"
	"time"
)

// WorkerPool manages a collection of workers and their lifecycle
type WorkerPool struct {
	workers         []*Worker
	logger          *logger.Logger
	wg              sync.WaitGroup
	cancelFn        context.CancelFunc
	shutdownTimeout time.Duration
	mu              sync.RWMutex // protects cancelFn and state
}

func NewWorkerPool(
	workerCount int,
	queue queue.TaskQueue,
	stateManager execution.StateManager,
	registry *registry.HandlerRegistry,
	logger *logger.Logger,
) *WorkerPool {
	workers := make([]*Worker, workerCount)
	for i := range workerCount {
		workers[i] = NewWorker(i+1, queue, stateManager, registry, logger)
	}

	return &WorkerPool{
		workers:         workers,
		logger:          logger,
		shutdownTimeout: 30 * time.Second, // Default for now
	}
}

// Start begins all workers in the pool
func (p *WorkerPool) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// create cancellable context for workers
	workerCtx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	p.logger.Info("starting worker pool", map[string]any{
		"worker_count": len(p.workers),
	})

	// start each worker in its own goroutine
	for _, worker := range p.workers {
		p.wg.Add(1)
		go func(w *Worker) {
			defer p.wg.Done()
			w.Start(workerCtx)
		}(worker)
	}

	p.logger.Info("worker pool started succesfully", map[string]any{
		"active_workers": len(p.workers),
	})
}

// Stop gracefully shuts down all workers
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	cancelFn := p.cancelFn
	p.mu.Unlock()

	p.logger.Info("stopping worker pool", map[string]any{
		"worker_count": len(p.workers),
		"timeout":      p.shutdownTimeout,
	})

	// Cancel context to signal workers to stop
	if cancelFn != nil {
		cancelFn()
	}

	// Stop all workers explicitly too
	for _, worker := range p.workers {
		worker.Stop()
	}

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("worker pool stopped gracefully", map[string]any{
			"shutdown_time": "within_timeout",
		})
	case <-time.After(p.shutdownTimeout):
		p.logger.Warn("worker pool shutdown timed out", map[string]any{
			"timeout":         p.shutdownTimeout,
			"forced_shutdown": true,
		})
	}

	// Clear cancel function
	p.mu.Lock()
	p.cancelFn = nil
	p.mu.Unlock()
}

// GetWorkerCount returns the number of workers in the pool
func (p *WorkerPool) GetWorkerCount() int {
	return len(p.workers)
}

// SetShutdownTimeout configures how long to wait for graceful shutdown
func (p *WorkerPool) SetShutdownTimeout(timeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shutdownTimeout = timeout
}
