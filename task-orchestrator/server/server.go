package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task-orchestrator/config"
	"task-orchestrator/logger"
	"time"
)

// Server wraps http.Server with graceful shutdown capabilities
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     *logger.Logger
}

// New creates a new server with the given handler and configuration
func New(handler http.Handler, cfg *config.Config, lg *logger.Logger) *Server {
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
