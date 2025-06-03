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

	// Configure logger
	logger.SetLevel(cfg.LogLevel)
	logger.Infof("starting task orchestrator with config: port=%d, log_level=%s",
		cfg.ServerPort, cfg.LogLevel)

	// Register Task Handlers
	registry := tasks.NewRegistry()
	registry.Register("print", &taskHandlers.PrintHandler{})
	registry.Register("sleep", taskHandlers.NewSleepHandler())
	logger.Infof("registered task handlers: %v", registry.GetRegisteredTypes())

	// Set up HTTP Handler
	runner := runners.NewSynchronousRunner(registry)

	// Create HTTP mux and register routes
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", api.NewSubmitHandler(runner))

	// Add health check endpoint
	mux.HandleFunc("/health", api.NewHealthHandler(cfg, registry))

	// Create and start server with graceful shutdown
	srv := server.New(mux, cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
