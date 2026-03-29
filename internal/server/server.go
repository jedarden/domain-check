// Package server provides the HTTP server with graceful shutdown and middleware.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coding/domain-check/internal/config"
)

// Server wraps an http.Server with graceful shutdown support.
type Server struct {
	http *http.Server
	log  *slog.Logger
}

// New creates a new HTTP server with the given configuration and handler.
func New(cfg *config.Config, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1MB
		},
		log: log,
	}
}

// Run starts the HTTP server and blocks until the server shuts down.
// It sets up signal handling for graceful shutdown on SIGINT and SIGTERM.
func (s *Server) Run(ctx context.Context) error {
	// Create a context that is cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server starting", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for either:
	// 1. The server exits with an error
	// 2. The context is cancelled (signal received)
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.log.Info("shutdown signal received, draining connections")
	}

	// Graceful shutdown with 15s drain timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		s.log.Error("server shutdown error", "error", err)
		return fmt.Errorf("shutdown: %w", err)
	}

	s.log.Info("server stopped")
	return nil
}

// Shutdown gracefully shuts down the server without waiting for signals.
// This is useful for programmatic shutdown (e.g., health check failure).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Addr returns the server's listen address.
func (s *Server) Addr() string {
	return s.http.Addr
}

// DefaultLogger returns a slog.Logger configured for the given format and level.
// If format is "json", uses JSON output; otherwise uses text output.
func DefaultLogger(format, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
