package main

import (
	"log"
	"net/http"
	"task-orchestrator/api"
	"task-orchestrator/api/middleware"
	"task-orchestrator/api/server"
	"task-orchestrator/config"
	"task-orchestrator/logger"
	"task-orchestrator/tasks"
	taskHandlers "task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/runners"
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
	registry := tasks.NewRegistry()
	registry.Register("print", taskHandlers.NewPrintHandler(lg))
	registry.Register("sleep", taskHandlers.NewSleepHandler(lg))

	lg.Info("Registered task handlers", map[string]any{
		"count": len(registry.GetRegisteredTypes()),
		"types": registry.GetRegisteredTypes(),
	})

	// Set up HTTP Handler
	runner := runners.NewSynchronousRunner(registry, lg)

	// Create HTTP mux and register routes
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", api.NewSubmitHandler(runner, lg))
	mux.HandleFunc("/health", api.NewHealthHandler(cfg, registry, lg))

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
