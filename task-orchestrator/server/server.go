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
}

// New creates a new server with the given handler and configuration
func New(handler http.Handler, cfg *config.Config) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Address(),
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		config: cfg,
	}
}

// Start starts the server and blocks until shutdown
func (s *Server) Start() error {
	// Create a channel to receive OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Infof("server starting on %s", s.config.Address())
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	logger.Infof("shutting down server...")

	return s.shutdown()
}

// shutdown gracefully shuts down the server
func (s *Server) shutdown() error {
	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Errorf("server forced to shutdown: %v", err)
		return err
	}

	logger.Infof("server shutdown complete")
	return nil
}
