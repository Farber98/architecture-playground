package main

import (
	"log"
	"net/http"
	"task-orchestrator/api"
	"task-orchestrator/config"
	"task-orchestrator/logger"
	"task-orchestrator/server"
	"task-orchestrator/tasks"
	taskHandlers "task-orchestrator/tasks/handlers"
	"task-orchestrator/tasks/runners"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Create logger
	cfg.Logger = logger.New(cfg.LogLevel, nil)

	// Log startup using the centralized logger
	cfg.Logger.Info("Starting task orchestrator", map[string]any{
		"version":   cfg.Version,
		"port":      cfg.ServerPort,
		"log_level": cfg.LogLevel,
	})

	// Register Task Handlers
	registry := tasks.NewRegistry()
	registry.Register("print", taskHandlers.NewPrintHandler(cfg.Logger))
	registry.Register("sleep", taskHandlers.NewSleepHandler(cfg.Logger))

	cfg.Logger.Info("Registered task handlers", map[string]any{
		"count": len(registry.GetRegisteredTypes()),
		"types": registry.GetRegisteredTypes(),
	})

	// Set up HTTP Handler
	runner := runners.NewSynchronousRunner(registry, cfg.Logger)

	// Create HTTP mux and register routes
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", api.NewSubmitHandler(runner, cfg.Logger))

	// Add health check endpoint
	mux.HandleFunc("/health", api.NewHealthHandler(cfg, registry))

	// Create and start server with graceful shutdown
	srv := server.New(mux, cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
