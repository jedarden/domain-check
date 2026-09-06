// safeguards.go holds the crash-prevention middleware: panic recovery and
// per-request timeout guards. Together they keep a single bad request from
// crashing the process or wedging a connection indefinitely.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// DefaultRequestTimeout bounds how long a single request may run before the
// server cancels its context and replies 503.
//
// It must stay at or above the bulk checker's own TotalTimeout (30s) so well
// behaved handlers are never cut short, and strictly below the server's
// WriteTimeout so the timeout response can still be written to the connection.
// TestHTTPTimeouts asserts both invariants.
const DefaultRequestTimeout = 30 * time.Second

// Recover returns middleware that converts a panic in a downstream handler
// into a 500 JSON response instead of an aborted connection.
//
// net/http already prevents a handler panic from killing the process, but its
// recovery only writes to stderr and drops the connection. This middleware
// additionally logs through slog with request context (request ID, client IP,
// stack trace), records a prometheus metric, and gives the client a well
// formed error body.
//
// It must sit *inside* Timeout (see Router): Timeout runs the handler in its
// own goroutine, and a panic in a different goroutine cannot be recovered by
// middleware outside it.
//
// Limitation: a panic in a goroutine spawned by a handler is not recovered
// here — handlers must not launch unprotected goroutines.
func Recover(log *slog.Logger, metrics *Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{ResponseWriter: w}

			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// ErrAbortHandler is the net/http sentinel for "abort this
				// connection quietly" (used by TimeoutHandler and by this
				// middleware below). Propagate it without logging or metrics.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				log.Error("panic recovered in request handler",
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", GetRequestID(r.Context()),
					"client_ip", GetClientIP(r.Context()),
				)
				if metrics != nil {
					metrics.RecordPanicRecovered()
				}

				if rw.status == 0 {
					// Nothing written yet: the client gets a clean 500.
					writeAPIError(rw, http.StatusInternalServerError,
						"internal_server_error", "An internal error occurred.")
					return
				}
				// The response head is already on the wire; appending a second
				// response would corrupt it. Abort the connection instead —
				// net/http swallows ErrAbortHandler.
				panic(http.ErrAbortHandler)
			}()

			next.ServeHTTP(rw, r)
		})
	}
}

// Timeout returns middleware that bounds how long a request may run.
//
// The downstream handler runs in its own goroutine with a context that
// carries a deadline of d. Context-aware handlers (the RDAP/WHOIS checkers)
// observe cancellation and return promptly. If the handler has not finished
// when the deadline expires, the client receives a 503 JSON error and the
// handler's remaining output is discarded.
//
// The deadline is watched through a timer rather than ctx.Done() so a client
// that hangs up early is not logged — or counted — as a timeout. The handler
// sees the cancellation either way and returns on its own.
//
// A handler that ignores its context keeps running in the background until it
// returns on its own — the timeout bounds the *client-visible* duration, not
// the handler's lifetime. That matches the behaviour of net/http's own
// TimeoutHandler.
func Timeout(d time.Duration, log *slog.Logger, metrics *Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			r = r.WithContext(ctx)

			tw := newTimeoutWriter(w)
			done := make(chan struct{})
			go func() {
				defer close(done)
				next.ServeHTTP(tw, r)
			}()

			timer := time.NewTimer(d)
			defer timer.Stop()

			select {
			case <-done:
				// Handler finished inside the budget.
			case <-timer.C:
				log.Warn("request timed out",
					"timeout", d,
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", GetRequestID(ctx),
					"client_ip", GetClientIP(ctx),
				)
				if metrics != nil {
					metrics.RecordRequestTimeout()
				}
				// Take over the response before writing so a straggler
				// handler cannot interleave body bytes with the 503.
				if uw, ok := tw.takeOver(); ok {
					writeAPIError(uw, http.StatusServiceUnavailable,
						"request_timeout",
						fmt.Sprintf("The request did not complete within %s.", d))
				}
			}
		})
	}
}

// timeoutWriter is a mutex-guarded ResponseWriter that stands between the
// Timeout middleware and the handler. It owns a private header map that is
// replayed to the real writer on the first write, and it drops all handler
// output once the middleware has taken the response over.
type timeoutWriter struct {
	mu     sync.Mutex
	w      http.ResponseWriter
	h      http.Header
	wrote  bool // response head has been written to the real writer
	closed bool // middleware owns the response; handler writes are dropped
}

func newTimeoutWriter(w http.ResponseWriter) *timeoutWriter {
	return &timeoutWriter{w: w, h: make(http.Header)}
}

// Header returns the handler's private header map. It is replayed to the real
// writer when the head is written.
func (tw *timeoutWriter) Header() http.Header { return tw.h }

// takeOver hands the underlying writer to the Timeout middleware once the
// deadline has fired. It reports whether the response was still untouched; if
// the handler had already written, the connection is only marked closed.
func (tw *timeoutWriter) takeOver() (http.ResponseWriter, bool) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.closed = true
	if tw.wrote {
		return nil, false
	}
	tw.wrote = true
	return tw.w, true
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.closed || tw.wrote {
		return
	}
	tw.wrote = true
	tw.flushHeader()
	tw.w.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.closed {
		// Swallow late output rather than corrupting the timeout response.
		return len(b), nil
	}
	if !tw.wrote {
		tw.wrote = true
		tw.flushHeader()
		tw.w.WriteHeader(http.StatusOK)
	}
	return tw.w.Write(b)
}

// Flush forwards to the underlying writer when it supports flushing and the
// response is still the handler's to write.
func (tw *timeoutWriter) Flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.closed {
		return
	}
	if f, ok := tw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// flushHeader copies the handler's buffered headers onto the real writer.
// Callers must hold tw.mu.
func (tw *timeoutWriter) flushHeader() {
	dst := tw.w.Header()
	for k, vv := range tw.h {
		dst[k] = vv
	}
}
