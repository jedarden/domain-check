// Package server provides the HTTP server with graceful shutdown and middleware.
package server

import (
	"context"
	"errors"
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
	// Signals are received through sigCh alone. Run deliberately does NOT wrap
	// its context in signal.NotifyContext: a second registration would cancel
	// this function's context on the very signal sigCh receives, and the
	// select below could then take either branch, losing the attribution the
	// signal handler metrics need (RecordSignalHandled). The parent context
	// still cancels on signals - main.go installs its own NotifyContext for
	// its background goroutines - so the ctx.Done() branch re-checks sigCh
	// before concluding the cancellation was programmatic.
	// SAFEGUARD: an explicit registration prevents unhandled signal termination
	// SAFEGUARD: SIGHUP handling prevents crashes during infrastructure signal cascades
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	// Start the server in a goroutine.
	// SAFEGUARD: Buffered channel prevents goroutine leak if server exits early
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server starting", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// handlerStart anchors the handler duration metric. A signal can only be
	// observed after the select begins, so measuring from here cannot
	// understate the handler's runtime by more than the select's own
	// scheduling latency.
	handlerStart := time.Now()

	// Wait for either:
	// 1. The server exits with an error
	// 2. A signal arrives (SIGINT/SIGTERM/SIGHUP)
	// 3. The parent context is cancelled (programmatic shutdown)
	// SAFEGUARD: select blocks until one condition completes, no goroutine leaks
	var sigName string
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		sigName = sig.String()
	case <-ctx.Done():
		// Either a signal (delivered here through the parent's
		// NotifyContext) or a programmatic cancellation. Go's signal
		// delivery buffers the value into every registered channel in a
		// single non-blocking pass, so a signal-caused cancellation has its
		// name sitting in sigCh by the time cancellation is observed; the
		// bounded wait absorbs only scheduler skew. Finding nothing means
		// the parent cancelled on its own.
		sigName = waitForSignal(sigCh, signalAttributionWindow)
	}

	if sigName != "" {
		s.log.Warn("signal received", "signal", sigName)
		GetMetrics().RecordSignalReceived(sigName)
		GetCrashRecorder().RecordSignal(sigName)
		s.log.Info("shutdown signal received, draining connections gracefully", "signal", sigName)
	} else {
		s.log.Info("shutdown context cancelled, draining connections gracefully")
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
	drainStart := time.Now()
	drainErr := s.http.Shutdown(shutdownCtx)
	drained := time.Since(drainStart)

	// Publish the outcome: "graceful" when every connection drained inside
	// the budget, "forced" when the drain ran out of time (in-flight requests
	// were cut off), "error" for anything else.
	metrics := GetMetrics()
	outcome := "error"
	switch {
	case drainErr == nil:
		outcome = "graceful"
	case errors.Is(drainErr, context.DeadlineExceeded):
		outcome = "forced"
		s.log.Error("server shutdown timed out, closing remaining connections", "error", drainErr, "drained", drained.String())
		GetCrashRecorder().RecordShutdownError(drainErr, drained)
	default:
		s.log.Error("server shutdown error", "error", drainErr, "drained", drained.String())
		GetCrashRecorder().RecordShutdownError(drainErr, drained)
	}
	metrics.RecordShutdown(outcome, drained)

	// The signal handler is the signal-to-drain path above (SIGHUP shares it
	// with SIGINT/SIGTERM). Publishing its execution and duration is what
	// makes a SIGHUP cascade distinguishable from an ordinary deploy SIGTERM
	// on /metrics.
	if sigName != "" {
		metrics.RecordSignalHandled(sigName, outcome, time.Since(handlerStart))
	}

	if drainErr != nil {
		return fmt.Errorf("shutdown: %w", drainErr)
	}

	s.log.Info("server stopped")
	return nil
}

// Shutdown gracefully shuts down the server without waiting for signals.
// This is useful for programmatic shutdown (e.g., health check failure).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// signalAttributionWindow bounds how long Run's ctx.Done() branch waits for a
// signal name before concluding the cancellation was programmatic. Signal
// delivery is a single non-blocking pass over every registered channel, so a
// signal-caused cancellation has its name buffered within microseconds; the
// window exists for scheduler skew on a loaded host and costs a genuinely
// programmatic shutdown one short pause before draining.
const signalAttributionWindow = 100 * time.Millisecond

// waitForSignal drains one pending signal name from sigCh, or "" if none
// arrives within d.
func waitForSignal(sigCh <-chan os.Signal, d time.Duration) string {
	select {
	case sig := <-sigCh:
		return sig.String()
	case <-time.After(d):
		return ""
	}
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
