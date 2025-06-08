package main

import (
	"log"
	"net/http"
	"task-orchestrator/api"
	"task-orchestrator/api/middleware"
	"task-orchestrator/api/server"
	"task-orchestrator/config"
	"task-orchestrator/logger"
	taskHandlers "task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/orchestrator"
	handlerRegistry "task-orchestrator/tasks/registry"
	"task-orchestrator/tasks/runners"
	"task-orchestrator/tasks/store"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Create logger
	lg := logger.New(cfg.LogLevel, nil)

	// Log startup using the centralized logger
	lg.Info("Starting task orchestrator", map[string]any{
		"version":   cfg.Version,
		"port":      cfg.ServerPort,
		"log_level": cfg.LogLevel,
	})

	// Register Task Handlers
	registry := handlerRegistry.NewRegistry()
	registry.Register("print", taskHandlers.NewPrintHandler(lg))
	registry.Register("sleep", taskHandlers.NewSleepHandler(lg))

	lg.Info("Registered task handlers", map[string]any{
		"count": len(registry.GetRegisteredTypes()),
		"types": registry.GetRegisteredTypes(),
	})

	// Create task store, runner and wire up orchestrator
	store := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(registry, lg)
	orchestrator := orchestrator.NewOrchestrator(store, runner, lg)

	// Create HTTP mux and register routes
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", api.NewSubmitHandler(orchestrator, lg))
	mux.HandleFunc("/health", api.NewHealthHandler(cfg, registry, lg))
	mux.HandleFunc("/tasks/", api.NewTaskStatusHandler(orchestrator, lg))

	// Wrap mux with middlewares
	loggingMw := middleware.LoggingMiddleware(lg)
	finalMux := loggingMw(mux)

	// Create and start server with graceful shutdown
	// Wrap mux with middlewares
	srv := server.New(finalMux, cfg, lg)
	if err := srv.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
