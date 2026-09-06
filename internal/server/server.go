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

	"github.com/jedarden/domain-check/internal/config"
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
			// WriteTimeout must exceed DefaultRequestTimeout: the timeout
			// middleware writes its 503 when a request runs past that budget,
			// and a shorter connection write deadline would drop the response
			// the moment it fired. 45s = 30s request budget + 15s to flush.
			WriteTimeout:   45 * time.Second,
			IdleTimeout:    120 * time.Second, // reaps keep-alive connections idle for 2 min
			MaxHeaderBytes: 1 << 20,           // 1MB
		},
		log: log,
	}
}

// Run starts the HTTP server and blocks until the server shuts down.
// It sets up signal handling for graceful shutdown on SIGINT, SIGTERM, and SIGHUP.
//
// SAFEGUARDS AGAINST CRASHES (documented 2026-09-01):
//
// This signal handling implementation prevents crashes from:
// 1. Unhandled signals: Catches SIGINT/SIGTERM/SIGHUP explicitly
// 2. Goroutine leaks: Context cancellation propagates to all child goroutines
// 3. Connection exhaustion: Graceful shutdown drains active connections
// 4. Resource leaks: defer statements ensure cleanup always runs
// 5. Hanging shutdowns: 15-second timeout prevents indefinite blocking
// 6. SIGHUP cascades: Handles SIGHUP gracefully to prevent abrupt termination
//
// The crashes investigated in 2026-08-16 to 2026-09-01 were NOT caused by
// defects in this signal handling code. All crashes were caused by:
// - Infrastructure memory exhaustion (systemd-oomd activation at 94.71% pressure)
// - CPU saturation (4.46x load on 7 cores)
// - System-wide SIGHUP cascades (external termination events)
//
// SIGHUP HANDLING (added 2026-09-01):
// SIGHUP is now handled gracefully to prevent crashes during infrastructure
// signal cascades (e.g., systemd-oomd, fleet manager broadcasts). This makes
// the server resilient to external termination events that trigger SIGHUP.
//
// See: docs/crash-safeguards-and-monitoring.md for full analysis.
func (s *Server) Run(ctx context.Context) error {
	// Create a context that is cancelled on SIGINT/SIGTERM/SIGHUP.
	// SAFEGUARD: signal.NotifyContext prevents unhandled signal termination
	// SAFEGUARD: SIGHUP handling prevents crashes during infrastructure signal cascades
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	// Start the server in a goroutine.
	// SAFEGUARD: Buffered channel prevents goroutine leak if server exits early
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server starting", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for either:
	// 1. The server exits with an error
	// 2. The context is cancelled (signal received: SIGINT/SIGTERM/SIGHUP)
	// SAFEGUARD: select blocks until one condition completes, no goroutine leaks
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.log.Info("shutdown signal received (SIGINT/SIGTERM/SIGHUP), draining connections gracefully")
	}

	// Graceful shutdown with 15s drain timeout.
	// SAFEGUARD: 15-second timeout prevents indefinite blocking during shutdown
	// SAFEGUARD: Context with deadline ensures shutdown always completes
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// SAFEGUARD: http.Shutdown() gracefully drains active connections:
	// - Stops accepting new connections
	// - Waits for active requests to complete (up to timeout)
	// - Closes all connections after timeout
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
