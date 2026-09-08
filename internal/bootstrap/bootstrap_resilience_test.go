package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/domain-check/internal/resilience"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastRetryConfig returns a retry policy that exercises the retry loop without
// waiting on real backoff. Tests that need a different attempt count set
// MaxRetries on a copy. BaseDelay must stay positive: a non-positive one makes
// Manager.retryConfig fall back to the interactive default.
func fastRetryConfig() resilience.RetryConfig {
	return resilience.RetryConfig{
		MaxRetries: 2,
		BaseDelay:  time.Millisecond,
		MaxDelay:   2 * time.Millisecond,
	}
}

// newCountingBootstrapServer serves status for every request and counts them.
func newCountingBootstrapServer(t *testing.T, status int, requests *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(requests, 1)
		w.WriteHeader(status)
	}))
}

func TestRefreshRetries503ThenSucceeds(t *testing.T) {
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := newManagerWithRetry(context.Background(), srv.URL, fastRetryConfig())
	require.NoError(t, err)
	defer b.Stop()

	// The 503 was retried once and the fetched mapping is live, so the
	// manager is not on fallbacks.
	assert.Equal(t, int32(2), atomic.LoadInt32(&requests))
	assert.Equal(t, 8, b.ServerCount())
	url, err := b.Lookup("io")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.centralnic.com/rdap/", url)
}

func TestRefreshAttemptsExhaustedOnPersistent503(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBootstrap))
	}))
	defer good.Close()

	b, err := newManagerWithRetry(context.Background(), good.URL, fastRetryConfig()) // 3 attempts
	require.NoError(t, err)
	defer b.Stop()

	var downRequests int32
	down := newCountingBootstrapServer(t, http.StatusServiceUnavailable, &downRequests)
	defer down.Close()

	// Point the manager at the registry that is down. The breaker host
	// follows the URL, so the failure count below starts from zero. The
	// refresh loop's first tick is 24h out, so no goroutine contends here.
	b.mu.Lock()
	b.url = down.URL
	b.mu.Unlock()
	serversBefore := b.ServerCount()

	err = b.Refresh(context.Background())
	require.Error(t, err)

	var exhausted *resilience.AttemptsExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.Equal(t, 3, exhausted.Attempts)
	assert.Equal(t, int32(3), atomic.LoadInt32(&downRequests))

	// Graceful degradation: the last known mapping keeps being served.
	assert.Equal(t, serversBefore, b.ServerCount())
	_, lookupErr := b.Lookup("com")
	assert.NoError(t, lookupErr)
}

func TestRefreshFailsFastWhenBreakerOpen(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBootstrap))
	}))
	defer good.Close()

	// Two attempts per Refresh, so two Refreshes = 4 consecutive failures —
	// one short of the default threshold of 5.
	b, err := newManagerWithRetry(context.Background(), good.URL, func() resilience.RetryConfig {
		cfg := fastRetryConfig()
		cfg.MaxRetries = 1
		return cfg
	}())
	require.NoError(t, err)
	defer b.Stop()

	var downRequests int32
	down := newCountingBootstrapServer(t, http.StatusServiceUnavailable, &downRequests)
	defer down.Close()

	b.mu.Lock()
	b.url = down.URL
	b.mu.Unlock()
	host := bootstrapHost(down.URL)

	for i := 0; i < 2; i++ {
		require.Error(t, b.Refresh(context.Background()))
	}
	assert.Equal(t, resilience.StateClosed, b.breakerFor(host).State())

	// The fifth failure opens the circuit mid-refresh; the refresh that
	// tripped it reports the open circuit.
	require.Error(t, b.Refresh(context.Background()))
	assert.Equal(t, resilience.StateOpen, b.breakerFor(host).State())
	assert.Equal(t, int32(5), atomic.LoadInt32(&downRequests))

	// Fail fast: no network I/O while open, and the error says the source is
	// unavailable rather than pretending the file is bad.
	before := atomic.LoadInt32(&downRequests)
	start := time.Now()
	err = b.Refresh(context.Background())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.ErrorIs(t, err, resilience.ErrCircuitOpen)
	assert.Equal(t, before, atomic.LoadInt32(&downRequests))
	assert.Less(t, elapsed, 5*time.Second, "an open circuit must fail fast, not be waited out")

	// The last known mapping is still served while the fetch is refused.
	url, lookupErr := b.Lookup("com")
	assert.NoError(t, lookupErr)
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", url)
}

func TestNewManagerFallsBackWhenBootstrapUnavailable(t *testing.T) {
	var requests int32
	srv := newCountingBootstrapServer(t, http.StatusServiceUnavailable, &requests)
	defer srv.Close()

	// Startup is not blocked by an unavailable IANA: the fetch retries, then
	// the manager comes up on the built-in fallbacks and keeps serving.
	b, err := newManagerWithRetry(context.Background(), srv.URL, fastRetryConfig())
	require.NoError(t, err)
	defer b.Stop()

	assert.Equal(t, 3, b.ServerCount())
	for tld, want := range map[string]string{
		"com": "https://rdap.verisign.com/com/v1/",
		"net": "https://rdap.verisign.com/net/v1/",
		"org": "https://rdap.publicinterestregistry.org/rdap/",
	} {
		url, err := b.Lookup(tld)
		require.NoError(t, err, tld)
		assert.Equal(t, want, url, tld)
	}
}
