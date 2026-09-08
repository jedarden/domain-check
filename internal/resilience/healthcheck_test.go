package resilience

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusServer returns an httptest server that answers every request with the
// given status code.
func statusServer(t *testing.T, code int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckHealthHealthy(t *testing.T) {
	srv := statusServer(t, http.StatusOK)

	result := CheckHealth(context.Background(), Target{Name: "registry", URL: srv.URL})

	assert.True(t, result.Healthy)
	assert.Empty(t, result.Err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, "registry", result.Name)
	assert.Equal(t, srv.URL, result.URL)
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, ClassUnknown, result.Class, "a healthy check carries no failure class")
	assert.GreaterOrEqual(t, result.LatencyMS, int64(0))
	assert.False(t, result.CheckedAt.IsZero())
}

func TestCheckHealthAny2xxIsHealthy(t *testing.T) {
	// Without an ExpectedStatus, any 2xx counts — 204 is a common health
	// endpoint answer.
	srv := statusServer(t, http.StatusNoContent)

	result := CheckHealth(context.Background(), Target{Name: "gateway", URL: srv.URL})

	assert.True(t, result.Healthy)
	assert.Equal(t, http.StatusNoContent, result.StatusCode)
}

func TestCheckHealthExpectedStatus(t *testing.T) {
	t.Run("matching expected status is healthy", func(t *testing.T) {
		srv := statusServer(t, http.StatusNoContent)
		result := CheckHealth(context.Background(), Target{
			Name: "strict", URL: srv.URL, ExpectedStatus: http.StatusNoContent,
		})
		assert.True(t, result.Healthy)
	})

	t.Run("a 2xx outside ExpectedStatus is permanent", func(t *testing.T) {
		srv := statusServer(t, http.StatusNoContent)
		result := CheckHealth(context.Background(), Target{
			Name: "strict", URL: srv.URL, ExpectedStatus: http.StatusOK,
		})
		assert.False(t, result.Healthy)
		assert.Equal(t, ClassPermanent, result.Class)
		assert.Contains(t, result.Err, "204")
	})
}

func TestCheckHealth503IsTransient(t *testing.T) {
	srv := statusServer(t, http.StatusServiceUnavailable)

	result := CheckHealth(context.Background(), Target{Name: "gateway", URL: srv.URL})

	assert.False(t, result.Healthy)
	assert.Equal(t, http.StatusServiceUnavailable, result.StatusCode)
	assert.Equal(t, ClassTransient, result.Class, "a 503 is worth retrying")
	assert.Contains(t, result.Err, "503")
	assert.Contains(t, result.Err, "service unavailable")
}

func TestCheckHealth500IsPermanent(t *testing.T) {
	srv := statusServer(t, http.StatusInternalServerError)

	result := CheckHealth(context.Background(), Target{Name: "gateway", URL: srv.URL})

	assert.False(t, result.Healthy)
	assert.Equal(t, ClassPermanent, result.Class, "a 500 will not improve on retry")
	assert.Contains(t, result.Err, "500")
}

func TestCheckHealthConnectionRefusedIsTransient(t *testing.T) {
	// A server that has stopped listening: the endpoint is unreachable, which
	// is the transient "service down" case the retry loop exists for.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	result := CheckHealth(context.Background(), Target{Name: "down", URL: url})

	assert.False(t, result.Healthy)
	assert.Equal(t, 0, result.StatusCode, "no response ever arrived")
	assert.Equal(t, ClassTransient, result.Class)
	assert.NotEmpty(t, result.Err)
}

func TestCheckHealthCancelledContextIsPermanent(t *testing.T) {
	srv := statusServer(t, http.StatusOK)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := CheckHealth(ctx, Target{Name: "gateway", URL: srv.URL})

	assert.False(t, result.Healthy)
	assert.Equal(t, ClassPermanent, result.Class,
		"a cancelled caller is not the service's fault, so it is not retried")
	assert.Contains(t, result.Err, "did not complete")
}

func TestCheckHealthTimeoutIsPermanent(t *testing.T) {
	// The endpoint accepts the connection and never answers; the attempt runs
	// out of time, which CheckHealth reports as permanent so the retry loop
	// does not multiply the caller's own timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until the client gives up
	}))
	t.Cleanup(srv.Close)

	result := CheckHealth(context.Background(), Target{Name: "hung", URL: srv.URL, Timeout: 50 * time.Millisecond})

	assert.False(t, result.Healthy)
	assert.Equal(t, ClassPermanent, result.Class)
	assert.Contains(t, result.Err, "did not complete within 50ms")
}

func TestCheckHealthTLSSelfSigned(t *testing.T) {
	// Started unstarted so the deliberate bad-certificate handshake below is
	// not logged as a server error — it is the test's expected outcome.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	t.Run("InsecureSkipVerify accepts the self-signed certificate", func(t *testing.T) {
		result := CheckHealth(context.Background(), Target{
			Name: "gateway", URL: srv.URL, InsecureSkipVerify: true,
		})
		assert.True(t, result.Healthy,
			"the documented gateway check needs this: it serves a self-signed cert")
		assert.Equal(t, http.StatusOK, result.StatusCode)
	})

	t.Run("verification left on rejects it", func(t *testing.T) {
		result := CheckHealth(context.Background(), Target{Name: "gateway", URL: srv.URL})
		assert.False(t, result.Healthy)
		assert.Equal(t, ClassTransient, result.Class)
		assert.NotEmpty(t, result.Err)
	})
}

func TestCheckHealthInvalidURLIsPermanent(t *testing.T) {
	result := CheckHealth(context.Background(), Target{Name: "bad", URL: "http://exa\x7fmple.invalid"})

	assert.False(t, result.Healthy)
	assert.Equal(t, 0, result.StatusCode)
	assert.Equal(t, ClassPermanent, result.Class)
	assert.Contains(t, result.Err, "invalid health check URL")
}

func TestCheckHealthTargetClientOverride(t *testing.T) {
	// Target.Client bypasses the constructed transport — the seam tests use,
	// and the way a caller can share a tuned client.
	srv := statusServer(t, http.StatusOK)
	result := CheckHealth(context.Background(), Target{
		Name:   "custom",
		URL:    srv.URL,
		Client: srv.Client(),
	})
	assert.True(t, result.Healthy)
}

func TestCheckHealthZeroTimeoutUsesDefault(t *testing.T) {
	// timeout() must fall back to DefaultHealthCheckTimeout, not hand the
	// client a zero timeout (no timeout at all).
	srv := statusServer(t, http.StatusOK)
	target := Target{Name: "gateway", URL: srv.URL}

	assert.Equal(t, DefaultHealthCheckTimeout, target.timeout())

	result := CheckHealth(context.Background(), target)
	assert.True(t, result.Healthy)
}

func TestHealthResultString(t *testing.T) {
	healthy := HealthResult{Name: "gateway", Healthy: true, StatusCode: 200, LatencyMS: 42}
	assert.Equal(t, "gateway: HEALTHY status=200 latency=42ms", healthy.String())

	unavailable := HealthResult{Name: "gateway", Healthy: false, Err: "HTTP 503 (service unavailable)"}
	assert.Equal(t, "gateway: UNAVAILABLE (HTTP 503 (service unavailable))", unavailable.String())
}

func TestGatewayTarget(t *testing.T) {
	target := GatewayTarget("https://gateway.example/health")

	assert.Equal(t, "inference-gateway", target.Name)
	assert.Equal(t, "https://gateway.example/health", target.URL)
	assert.Equal(t, DefaultHealthCheckTimeout, target.Timeout)
	assert.True(t, target.InsecureSkipVerify,
		"the gateway serves a self-signed certificate")
}

func TestDefaultGatewayURLIsHTTPS(t *testing.T) {
	require.True(t, len(DefaultGatewayURL) > 8)
	assert.Equal(t, "https://", DefaultGatewayURL[:8],
		"the default target is the gateway's TLS health endpoint")
}

func TestCheckHealthWithRetrySucceedsAfterRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultRetryConfig()
	cfg.Sleep = func(context.Context, time.Duration) error { return nil } // observe, don't wait

	result := CheckHealthWithRetry(context.Background(), Target{Name: "gateway", URL: srv.URL}, cfg)

	assert.True(t, result.Healthy)
	assert.Equal(t, int32(3), calls)
	assert.Equal(t, 3, result.Attempts, "the result names the attempt that succeeded")
	assert.Equal(t, http.StatusOK, result.StatusCode)
}

func TestCheckHealthWithRetryDoesNotRetryPermanent(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultRetryConfig()
	cfg.Sleep = func(context.Context, time.Duration) error { return nil }

	result := CheckHealthWithRetry(context.Background(), Target{Name: "gateway", URL: srv.URL}, cfg)

	assert.False(t, result.Healthy)
	assert.Equal(t, int32(1), calls, "a permanent verdict is returned after one attempt")
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, ClassPermanent, result.Class)
	assert.Contains(t, result.Err, "HTTP 500")
}

func TestCheckHealthWithRetryExhaustsTransient(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	cfg := RetryConfig{MaxRetries: 2, BaseDelay: time.Second, MaxDelay: 4 * time.Second}
	cfg.Sleep = func(context.Context, time.Duration) error { return nil }

	result := CheckHealthWithRetry(context.Background(), Target{Name: "gateway", URL: srv.URL}, cfg)

	assert.False(t, result.Healthy)
	assert.Equal(t, int32(3), calls, "initial attempt + 2 retries")
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, ClassTransient, result.Class)
	assert.Contains(t, result.Err, "transient failure persisted after 3 attempts",
		"exhaustion is reported explicitly, not as a bare 503")
}

func TestCheckHealthWithRetryHealthyOnFirstAttempt(t *testing.T) {
	srv := statusServer(t, http.StatusOK)

	result := CheckHealthWithRetry(context.Background(),
		Target{Name: "gateway", URL: srv.URL}, DefaultRetryConfig())

	assert.True(t, result.Healthy)
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, ClassUnknown, result.Class)
}
