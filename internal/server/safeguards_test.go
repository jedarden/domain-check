// safeguards_test.go verifies the crash-prevention middleware: panic
// recovery, request timeout guards, and the context-owned cleanup loop.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/domain-check/internal/config"
	"github.com/jedarden/domain-check/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// quietLogger returns a logger that discards output, so tests stay quiet.
// (memory_test.go already owns the name testLogger.)
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// panicChecker is a DomainChecker whose Check always panics, so the recovery
// middleware can be exercised end to end through Router.
type panicChecker struct{}

func (p *panicChecker) Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	panic("checker exploded")
}

// counterValue reads a prometheus counter's current value.
func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("reading counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestRecoverReturns500AndKeepsServing(t *testing.T) {
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	srv := Recover(quietLogger(), nil)(boom)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/check?d=x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the JSON error envelope: %v (%q)", err, rec.Body.String())
	}
	if body.Error != "internal_server_error" {
		t.Errorf("error code = %q, want internal_server_error", body.Error)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// The safeguard only matters if the process keeps serving afterwards.
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/check?d=y", nil))
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("second request status = %d, want %d (server must keep serving)", rec2.Code, http.StatusInternalServerError)
	}
}

func TestRecoverRecordsMetric(t *testing.T) {
	m := GetMetrics()
	before := counterValue(t, m.panicsRecovered)

	srv := Recover(quietLogger(), m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	// Delta, not absolute: the metrics singleton is shared across the package.
	if got := counterValue(t, m.panicsRecovered) - before; got != 1 {
		t.Errorf("domcheck_panics_recovered_total increased by %v, want 1", got)
	}
}

func TestRecoverPassesThroughCleanHandlers(t *testing.T) {
	srv := Recover(quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("tea"))
	}))

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (recover must not touch healthy responses)", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "tea" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "tea")
	}
}

func TestRecoverRepanicsErrAbortHandler(t *testing.T) {
	// ErrAbortHandler is the net/http sentinel for quietly aborting a
	// connection; Recover must propagate it, not swallow it.
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected ErrAbortHandler to be re-panicked, got none")
		}
		if !errors.Is(asErr(rec), http.ErrAbortHandler) {
			t.Errorf("recovered %v, want http.ErrAbortHandler", rec)
		}
	}()

	srv := Recover(quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRecoverAbortsWhenResponseAlreadyStarted(t *testing.T) {
	// A panic after the response head is on the wire cannot be replaced by a
	// 500 — Recover must abort instead of writing a second response.
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected http.ErrAbortHandler after a started response, got none")
		}
		if !errors.Is(asErr(rec), http.ErrAbortHandler) {
			t.Errorf("recovered %v, want http.ErrAbortHandler", rec)
		}
	}()

	srv := Recover(quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("late failure")
	}))

	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRecoverAbortsAfterImplicitWrite(t *testing.T) {
	// Same as above, but the handler never calls WriteHeader: net/http sends
	// the 200 head on the first body write. responseWriter must record that,
	// or Recover would read the request as "nothing written yet" and append a
	// JSON 500 to the body the client is already receiving.
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected http.ErrAbortHandler after an implicit 200, got none")
		}
		if !errors.Is(asErr(rec), http.ErrAbortHandler) {
			t.Errorf("recovered %v, want http.ErrAbortHandler", rec)
		}
	}()

	rec := httptest.NewRecorder()
	srv := Recover(quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial")) // implicit WriteHeader(200)
		panic("late failure")
	}))

	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if code := rec.Code; code != http.StatusOK {
		t.Errorf("status = %d, want %d", code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "partial" {
		t.Errorf("body = %q, want %q (error body must not be appended to a started response)", body, "partial")
	}
}

func TestResponseWriterRecordsImplicitStatus(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	if rw.status != 0 {
		t.Fatalf("fresh wrapper status = %d, want 0", rw.status)
	}

	_, _ = rw.Write([]byte("hi"))

	if rw.status != http.StatusOK {
		t.Errorf("status after a bare Write = %d, want %d (net/http's implicit 200)", rw.status, http.StatusOK)
	}
}

// overrunHandler ignores its request context entirely — the handler the
// timeout guard exists for. Using a plain sleep rather than <-r.Context().Done()
// keeps these tests deterministic: a context-aware handler returns at the very
// moment the middleware's select wakes, and Go's select picks at random among
// ready cases, so which side "wins" would otherwise be a coin flip.
func overrunHandler(d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(d)
	})
}

func TestTimeoutReturns503WhenHandlerOverruns(t *testing.T) {
	srv := Timeout(50*time.Millisecond, quietLogger(), nil)(overrunHandler(5 * time.Second))

	start := time.Now()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the JSON error envelope: %v (%q)", err, rec.Body.String())
	}
	if body.Error != "request_timeout" {
		t.Errorf("error code = %q, want request_timeout", body.Error)
	}
	if elapsed > 2*time.Second {
		t.Errorf("request took %v, want ~timeout (50ms), not the handler's 5s", elapsed)
	}
}

func TestTimeoutRecordsMetric(t *testing.T) {
	m := GetMetrics()
	before := counterValue(t, m.requestTimeouts)

	srv := Timeout(20*time.Millisecond, quietLogger(), m)(overrunHandler(500 * time.Millisecond))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if got := counterValue(t, m.requestTimeouts) - before; got != 1 {
		t.Errorf("domcheck_request_timeouts_total increased by %v, want 1", got)
	}
}

func TestTimeoutDoesNotAffectFastHandlers(t *testing.T) {
	srv := Timeout(50*time.Millisecond, quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "kept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fast"))
	}))

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "fast" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "fast")
	}
	if rec.Header().Get("X-Custom") != "kept" {
		t.Errorf("handler-set header lost: X-Custom = %q", rec.Header().Get("X-Custom"))
	}
}

func TestTimeoutPropagatesDeadlineToHandler(t *testing.T) {
	// The guard only works if context-aware handlers (the RDAP/WHOIS checkers)
	// can see the deadline they are being held to.
	deadlineOK := make(chan bool, 1)
	srv := Timeout(50*time.Millisecond, quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		deadlineOK <- ok && time.Until(deadline) <= 50*time.Millisecond
	}))

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	select {
	case ok := <-deadlineOK:
		if !ok {
			t.Error("handler context has no deadline derived from the timeout budget")
		}
	default:
		t.Error("handler never ran")
	}
}

func TestTimeoutDropsLateHandlerWrites(t *testing.T) {
	// A handler that ignores its context and keeps writing after the deadline
	// must not be able to interleave output with the 503.
	srv := Timeout(30*time.Millisecond, quietLogger(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late"))
	}))

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if body := rec.Body.String(); strings.Contains(body, "late") {
		t.Errorf("late handler output leaked into the timeout response: %q", body)
	}
}

func TestTimeoutClientDisconnectIsNotATimeout(t *testing.T) {
	// A client hanging up early cancels the request context, and a
	// context-aware handler returns because of it. That is not a timeout: it
	// must not be answered with a 503 or counted in domcheck_request_timeouts_total.
	m := GetMetrics()
	before := counterValue(t, m.requestTimeouts)

	ctx, cancel := context.WithCancel(context.Background())
	srv := Timeout(5*time.Second, quietLogger(), m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // respond to the disconnect the way the RDAP client would
	}))

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))

	if rec.Code == http.StatusServiceUnavailable {
		t.Error("client disconnect was answered with the timeout 503")
	}
	if got := counterValue(t, m.requestTimeouts) - before; got != 0 {
		t.Errorf("domcheck_request_timeouts_total increased by %v on a client disconnect, want 0", got)
	}
}

func TestRouterChainsRecoverAndTimeout(t *testing.T) {
	// End-to-end through Router: a panicking route must come back as a 500,
	// not crash the server.
	cfg := &config.Config{Addr: "127.0.0.1:0"}
	log := quietLogger()
	srv := New(cfg, Router(cfg, log, NewRateLimiter(log), &panicChecker{}, nil, NewServiceMonitor(), nil, nil), log)

	ts := httptest.NewServer(srv.http.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/check?d=example.com")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (Router must recover route panics)", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestRateLimiterRunCleanupStopsOnContextCancel(t *testing.T) {
	rl := NewRateLimiter(quietLogger())

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		rl.RunCleanup(ctx, time.Hour) // long interval: only ctx can end it
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunCleanup did not return after context cancellation (goroutine leak)")
	}
}

// asErr adapts a recovered panic value to an error for errors.Is.
func asErr(v interface{}) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return errors.New(e)
	default:
		return errors.New("non-error panic value")
	}
}
