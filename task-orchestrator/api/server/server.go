package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task-orchestrator/api"
	"task-orchestrator/api/middleware"
	"task-orchestrator/config"
	"task-orchestrator/logger"
	"task-orchestrator/tasks/orchestrator"
	handlerRegistry "task-orchestrator/tasks/registry"
	"time"
)

// Server wraps http.Server with graceful shutdown capabilities
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     *logger.Logger
}

// dependencies contains all the dependencies needed to create a server
type dependencies struct {
	orchestrator orchestrator.Orchestrator
	registry     *handlerRegistry.HandlerRegistry
	config       *config.Config
	logger       *logger.Logger
}

// New creates a new server with all HTTP configuration
func New(orch orchestrator.Orchestrator, registry *handlerRegistry.HandlerRegistry, cfg *config.Config, lg *logger.Logger) *Server {
	deps := &dependencies{
		orchestrator: orch,
		registry:     registry,
		config:       cfg,
		logger:       lg,
	}

	// Create router with all routes and middleware
	handler := newRouter(deps)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Address(),
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		config: cfg,
		logger: lg,
	}
}

// NewRouter creates and configures the HTTP router with all routes and middleware
func newRouter(deps *dependencies) http.Handler {
	// Create HTTP mux and register routes
	mux := http.NewServeMux()

	// Register API routes
	mux.HandleFunc("/submit", api.NewSubmitHandler(deps.orchestrator, deps.logger))
	mux.HandleFunc("/health", api.NewHealthHandler(deps.config, deps.registry, deps.logger))
	mux.HandleFunc("/tasks/", api.NewTaskStatusHandler(deps.orchestrator, deps.logger))

	// Encapsulate middleware configuration
	return applyMiddleware(mux, deps.logger)
}

// applyMiddleware wraps the handler with all necessary middleware
func applyMiddleware(handler http.Handler, lg *logger.Logger) http.Handler {
	// Apply middleware in reverse order (last applied = first executed)
	wrapped := handler

	// Request logging middleware
	wrapped = middleware.LoggingMiddleware(lg)(wrapped)

	return wrapped
}

// Start starts the server and blocks until shutdown
func (s *Server) Start() error {
	// Create a channel to receive OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		s.logger.Info("Server starting", map[string]any{
			"address": s.config.Address(),
		})

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server failed to start", map[string]any{
				"error": err.Error(),
			})
		}
	}()

	// Wait for interrupt signal
	<-stop
	s.logger.Info("Shutting down server")

	return s.shutdown()
}

// shutdown gracefully shuts down the server
func (s *Server) shutdown() error {
	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("Server forced to shutdown", map[string]any{
			"error": err.Error(),
		})

		return err
	}

	s.logger.Info("Server shutdown complete")
	return nil
}
