package rdap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/domain-check/internal/resilience"
)

// fastRetryConfig returns a retry policy that exercises the retry loop without
// waiting on real backoff: three attempts (two retries) separated by
// millisecond-scale delays. Tests that need a different attempt count set
// MaxRetries on a copy.
func fastRetryConfig() resilience.RetryConfig {
	return resilience.RetryConfig{
		MaxRetries: 2,
		BaseDelay:  time.Millisecond,
		MaxDelay:   2 * time.Millisecond,
	}
}

// newCountingServer serves status for every request and counts them.
func newCountingServer(t *testing.T, status int, requests *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(requests, 1)
		w.WriteHeader(status)
	}))
}

// newRetryClient returns a client wired to server's transport with the fast
// retry policy. Only the fields doRequest touches are needed.
func newRetryClient(t *testing.T, server *httptest.Server, cfg resilience.RetryConfig) *RDAPClient {
	t.Helper()
	client := NewRDAPClient(RDAPClientConfig{HTTPClient: server.Client()})
	client.retryCfg = cfg
	return client
}

func TestDoRequestRetriesTransientThenSucceeds(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{"objectClassName":"domain","ldhName":"example.com"}`))
	}))
	defer server.Close()

	client := newRetryClient(t, server, fastRetryConfig())

	resp, err := client.doRequest(context.Background(), server.URL+"/domain/example.com", ExtractRegistryHost(server.URL))
	if err != nil {
		t.Fatalf("doRequest with 503-then-200 failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("requests: got %d, want 2 (first 503 retried once)", got)
	}
}

func TestDoRequestAttemptsExhaustedOnPersistent503(t *testing.T) {
	var requests int32
	server := newCountingServer(t, http.StatusServiceUnavailable, &requests)
	defer server.Close()

	client := newRetryClient(t, server, fastRetryConfig()) // 3 attempts

	resp, err := client.doRequest(context.Background(), server.URL+"/domain/example.com", ExtractRegistryHost(server.URL))
	if err == nil {
		defer resp.Body.Close()
		t.Fatal("expected an error after every attempt returned 503")
	}

	var exhausted *resilience.AttemptsExhaustedError
	if !errors.As(err, &exhausted) {
		t.Fatalf("error: got %v (%T), want *resilience.AttemptsExhaustedError", err, err)
	}
	if exhausted.Attempts != 3 {
		t.Errorf("attempts: got %d, want 3", exhausted.Attempts)
	}
	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Errorf("requests: got %d, want 3", got)
	}
}

func TestDoRequestDoesNotRetryPermanentStatus(t *testing.T) {
	var requests int32
	server := newCountingServer(t, http.StatusBadRequest, &requests)
	defer server.Close()

	cfg := fastRetryConfig()
	cfg.MaxRetries = 5 // a retry bug would surface as 6 requests
	client := newRetryClient(t, server, cfg)

	resp, err := client.doRequest(context.Background(), server.URL+"/domain/example.com", ExtractRegistryHost(server.URL))
	if err != nil {
		t.Fatalf("doRequest with a definitive 400 failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("requests: got %d, want 1 (400 is a definitive answer)", got)
	}
}

func TestDoRequestBreakerTripsAndFailsFast(t *testing.T) {
	var requests int32
	server := newCountingServer(t, http.StatusServiceUnavailable, &requests)
	defer server.Close()

	// Two attempts per call, so two calls = 4 consecutive failures — one
	// short of the default threshold of 5.
	client := newRetryClient(t, server, func() resilience.RetryConfig {
		cfg := fastRetryConfig()
		cfg.MaxRetries = 1
		return cfg
	}())
	registry := ExtractRegistryHost(server.URL)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := client.doRequest(ctx, server.URL+"/domain/example.com", registry); err == nil {
			t.Fatalf("call %d: expected a 503 error", i+1)
		}
	}
	if state := client.breakerFor(registry).State(); state != resilience.StateClosed {
		t.Fatalf("breaker state after 4 failures: got %s, want closed", state)
	}

	// The fifth failure (first attempt of this call) opens the circuit; the
	// second attempt is then refused before any I/O.
	_, err := client.doRequest(ctx, server.URL+"/domain/example.com", registry)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("error on the call that tripped the breaker: got %v, want one wrapping ErrCircuitOpen", err)
	}
	if state := client.breakerFor(registry).State(); state != resilience.StateOpen {
		t.Fatalf("breaker state: got %s, want open", state)
	}

	// Fail fast: no network I/O, and the error names the registry and reads
	// as an outage of the dependency rather than a registry answer.
	before := atomic.LoadInt32(&requests)
	start := time.Now()
	_, err = client.doRequest(ctx, server.URL+"/domain/example.com", registry)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a fail-fast error while the circuit is open")
	}
	if atomic.LoadInt32(&requests) != before {
		t.Errorf("requests while open: got %d more, want 0 (fail fast, no network I/O)",
			atomic.LoadInt32(&requests)-before)
	}
	if elapsed > 5*time.Second {
		t.Errorf("fail-fast call took %s; an open circuit must not be waited out", elapsed)
	}
	if !strings.Contains(err.Error(), registry) {
		t.Errorf("error %q does not name the registry %q", err, registry)
	}
	if !resilience.IsPermanent(err) {
		t.Errorf("error %q should be classified permanent: retrying an open circuit is futile", err)
	}

	// One breaker per registry hostname: a second registry is unaffected by
	// the first one's outage.
	var okRequests int32
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&okRequests, 1)
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{"objectClassName":"domain","ldhName":"example.org"}`))
	}))
	defer okServer.Close()

	resp, err := newRetryClient(t, okServer, fastRetryConfig()).
		doRequest(ctx, okServer.URL+"/domain/example.org", ExtractRegistryHost(okServer.URL))
	if err != nil {
		t.Fatalf("doRequest against a healthy registry failed while another registry's breaker was open: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func TestCheckGracefulDegradationWhenBreakerOpen(t *testing.T) {
	var requests int32
	server := newCountingServer(t, http.StatusServiceUnavailable, &requests)
	defer server.Close()

	// Two Check calls (2 attempts each = 4 failures) leave the breaker one
	// failure short of open; the third call's first attempt trips it.
	client := newTestRDAPClient(server)
	client.retryCfg = func() resilience.RetryConfig {
		cfg := fastRetryConfig()
		cfg.MaxRetries = 1
		return cfg
	}()

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		result, err := client.Check(ctx, "flaky503.com")
		if err != nil {
			t.Fatalf("Check %d: got error %v, want a degraded result", i+1, err)
		}
		if result.Error == "" {
			t.Fatalf("Check %d: expected the result to carry the registry error", i+1)
		}
		if !strings.Contains(result.Error, ErrRegistryUnavailable.Error()) {
			t.Errorf("Check %d: error %q does not report the registry as unavailable", i+1, result.Error)
		}
	}

	// With the circuit now open the same call fails fast with no network
	// I/O: the server keeps answering, and answers quickly.
	before := atomic.LoadInt32(&requests)
	start := time.Now()
	result, err := client.Check(ctx, "flaky503.com")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Check with the breaker open returned an error: %v", err)
	}
	if atomic.LoadInt32(&requests) != before {
		t.Errorf("requests while open: got %d more, want 0", atomic.LoadInt32(&requests)-before)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Check with the breaker open took %s; it must not hang", elapsed)
	}
	for _, want := range []string{ErrRegistryUnavailable.Error(), "circuit open"} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("result error %q does not mention %q", result.Error, want)
		}
	}
	if result.Domain != "flaky503.com" {
		t.Errorf("result domain: got %q, want flaky503.com", result.Domain)
	}
}

func TestCheck429SurfacesAsRateLimitedNotUnavailable(t *testing.T) {
	var requests int32
	server := newCountingServer(t, http.StatusTooManyRequests, &requests)
	defer server.Close()

	client := newTestRDAPClient(server)
	client.retryCfg = fastRetryConfig()

	result, err := client.Check(context.Background(), "ratelimited429.com")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	// Exhausted retries on 429 read as rate limiting — the registry is up
	// and answering — not as an outage. The match is on the "HTTP 429"
	// message format, so a registry host whose port merely contains "429"
	// cannot be misread as a rate limit.
	if result.Error != ErrRateLimited.Error() {
		t.Errorf("result error: got %q, want %q", result.Error, ErrRateLimited.Error())
	}
	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Errorf("requests: got %d, want 3 (429 is transient and retried)", got)
	}
}
