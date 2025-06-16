package main

import (
	"log"
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
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Create logger
	lg := logger.New(cfg.LogLevel, nil)

	// Log startup using the centralized logger
	lg.Info("Starting task orchestrator", map[string]any{
		"version":   cfg.Version,
		"port":      cfg.ServerPort,
		"log_level": cfg.LogLevel,
	})

	// Wire up business logic dependencies
	registry := createHandlerRegistry(lg)
	taskStore := store.NewMemoryTaskStore()
	runner := runners.NewSynchronousRunner(registry)
	orch := orchestrator.NewDefaultOrchestrator(taskStore, runner, lg)

	//	Create and start server
	srv := server.New(orch, registry, cfg, lg)
	if err := srv.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// createHandlerRegistry sets up all task handlers
func createHandlerRegistry(lg *logger.Logger) *handlerRegistry.HandlerRegistry {
	registry := handlerRegistry.NewRegistry()
	registry.Register("print", taskHandlers.NewPrintHandler(lg))
	registry.Register("sleep", taskHandlers.NewSleepHandler(lg))

	lg.Info("Registered task handlers", map[string]any{
		"count": len(registry.GetRegisteredTypes()),
		"types": registry.GetRegisteredTypes(),
	})

	return registry
}
