// Package cli provides tests for the healthcheck subcommand functionality.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/domain-check/internal/resilience"
)

// instantRetryConfig returns the standard policy with the backoff waits
// removed, so retry cycles in tests run without sleeping.
func instantRetryConfig(maxRetries int) resilience.RetryConfig {
	cfg := healthRetryConfig(maxRetries)
	cfg.Sleep = func(context.Context, time.Duration) error { return nil }
	return cfg
}

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

// flakyServer returns a server that answers with failCode for the first
// failures requests and 200 afterwards.
func flakyServer(t *testing.T, failCode int, failures int32) *httptest.Server {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&count, 1) <= failures {
			w.WriteHeader(failCode)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFormatHealthResult(t *testing.T) {
	tests := []struct {
		name     string
		result   resilience.HealthResult
		contains []string
		want     string
	}{
		{
			name: "healthy on the first attempt",
			result: resilience.HealthResult{
				Name: "inference-gateway", Healthy: true, StatusCode: 200, LatencyMS: 12, Attempts: 1,
			},
			want: "inference-gateway: OK status=200 latency=12ms attempts=1",
		},
		{
			name: "healthy only after retries is degraded",
			result: resilience.HealthResult{
				Name: "inference-gateway", Healthy: true, StatusCode: 200, LatencyMS: 340, Attempts: 3,
			},
			want:     "inference-gateway: DEGRADED status=200 latency=340ms attempts=3 (recovered after 2 retries)",
			contains: []string{"DEGRADED"},
		},
		{
			name: "transient failure names the retries",
			result: resilience.HealthResult{
				Name: "inference-gateway", StatusCode: 503, LatencyMS: 88, Attempts: 6,
				Class: resilience.ClassTransient,
				Err:   "HTTP 503 (service unavailable)",
			},
			want: "inference-gateway: UNAVAILABLE status=503 latency=88ms attempts=6 (transient — retried 5 times: HTTP 503 (service unavailable))",
		},
		{
			name: "a single retry is grammatical",
			result: resilience.HealthResult{
				Name: "inference-gateway", StatusCode: 503, LatencyMS: 88, Attempts: 2,
				Class: resilience.ClassTransient,
				Err:   "HTTP 503 (service unavailable)",
			},
			contains: []string{"(transient — retried 1 time: ", "attempts=2"},
		},
		{
			name: "permanent failure is not described as retried",
			result: resilience.HealthResult{
				Name: "rdap.example", StatusCode: 404, LatencyMS: 5, Attempts: 1,
				Class: resilience.ClassPermanent,
				Err:   "HTTP 404",
			},
			want: "rdap.example: UNAVAILABLE status=404 latency=5ms attempts=1 (permanent failure: HTTP 404)",
		},
		{
			name: "transport error with no response carries status 0",
			result: resilience.HealthResult{
				Name: "inference-gateway", StatusCode: 0, LatencyMS: 5000, Attempts: 2,
				Class: resilience.ClassTransient,
				Err:   "connection refused",
			},
			contains: []string{"UNAVAILABLE", "status=0", "transient"},
		},
		{
			name: "unknown class falls back to the bare error",
			result: resilience.HealthResult{
				Name: "mystery", StatusCode: 418, LatencyMS: 1, Attempts: 1,
				Class: resilience.ClassUnknown,
				Err:   "HTTP 418",
			},
			want: "mystery: UNAVAILABLE status=418 latency=1ms attempts=1 (HTTP 418)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHealthResult(tt.result)
			if tt.want != "" && got != tt.want {
				t.Errorf("formatHealthResult() = %q, want %q", got, tt.want)
			}
			for _, substr := range tt.contains {
				if !strings.Contains(got, substr) {
					t.Errorf("formatHealthResult() = %q, want it to contain %q", got, substr)
				}
			}
		})
	}
}

func TestHealthExitCode(t *testing.T) {
	tests := []struct {
		name   string
		result resilience.HealthResult
		want   int
	}{
		{
			name:   "healthy",
			result: resilience.HealthResult{Healthy: true},
			want:   HealthExitOK,
		},
		{
			name:   "degraded is still healthy",
			result: resilience.HealthResult{Healthy: true, Attempts: 4},
			want:   HealthExitOK,
		},
		{
			name:   "transient unavailable",
			result: resilience.HealthResult{Class: resilience.ClassTransient, Err: "HTTP 503"},
			want:   HealthExitTransient,
		},
		{
			name:   "permanent unavailable",
			result: resilience.HealthResult{Class: resilience.ClassPermanent, Err: "HTTP 404"},
			want:   HealthExitPermanent,
		},
		{
			name:   "unknown class defaults to transient",
			result: resilience.HealthResult{Class: resilience.ClassUnknown, Err: "mystery"},
			want:   HealthExitTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthExitCode(tt.result); got != tt.want {
				t.Errorf("healthExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRunHealthChecks_ExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		servers []int // status code per target
		retries int
		want    int
	}{
		{
			name:    "single healthy target",
			servers: []int{http.StatusOK},
			retries: 2,
			want:    HealthExitOK,
		},
		{
			name:    "transient failure after exhausting retries",
			servers: []int{http.StatusServiceUnavailable},
			retries: 2,
			want:    HealthExitTransient,
		},
		{
			name:    "permanent failure is not retried",
			servers: []int{http.StatusNotFound},
			retries: 2,
			want:    HealthExitPermanent,
		},
		{
			name:    "permanent failure outranks a healthy sibling",
			servers: []int{http.StatusOK, http.StatusNotFound},
			retries: 1,
			want:    HealthExitPermanent,
		},
		{
			name:    "permanent failure outranks a transient sibling",
			servers: []int{http.StatusServiceUnavailable, http.StatusNotFound},
			retries: 1,
			want:    HealthExitPermanent,
		},
		{
			name:    "transient failure outranks a healthy sibling",
			servers: []int{http.StatusOK, http.StatusBadGateway},
			retries: 1,
			want:    HealthExitTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var targets []resilience.Target
			for i, code := range tt.servers {
				srv := statusServer(t, code)
				target := resilience.GatewayTarget(srv.URL)
				target.Name = fmt.Sprintf("target-%d", i)
				target.Client = srv.Client()
				targets = append(targets, target)
			}

			var out bytes.Buffer
			got := runHealthChecks(context.Background(), targets, instantRetryConfig(tt.retries), &out)

			if got != tt.want {
				t.Errorf("runHealthChecks() exit = %d, want %d\noutput:\n%s", got, tt.want, out.String())
			}
			if lines := strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1; lines != len(tt.servers) {
				t.Errorf("runHealthChecks() printed %d lines, want %d\noutput:\n%s", lines, len(tt.servers), out.String())
			}
		})
	}
}

func TestRunHealthChecks_RecoveredAfterRetries(t *testing.T) {
	srv := flakyServer(t, http.StatusServiceUnavailable, 2)
	target := resilience.GatewayTarget(srv.URL)
	target.Name = "flaky"
	target.Client = srv.Client()

	var out bytes.Buffer
	got := runHealthChecks(context.Background(), []resilience.Target{target}, instantRetryConfig(5), &out)

	if got != HealthExitOK {
		t.Fatalf("runHealthChecks() exit = %d, want %d\noutput:\n%s", got, HealthExitOK, out.String())
	}
	want := "flaky: DEGRADED status=200 latency="
	if !strings.HasPrefix(out.String(), want) {
		t.Errorf("output = %q, want it to start with %q", out.String(), want)
	}
	for _, substr := range []string{"attempts=3", "recovered after 2 retries"} {
		if !strings.Contains(out.String(), substr) {
			t.Errorf("output %q does not contain %q", out.String(), substr)
		}
	}
}

func TestRunHealthChecks_TransientNamesRetries(t *testing.T) {
	srv := statusServer(t, http.StatusServiceUnavailable)
	target := resilience.GatewayTarget(srv.URL)
	target.Name = "down"
	target.Client = srv.Client()

	var out bytes.Buffer
	got := runHealthChecks(context.Background(), []resilience.Target{target}, instantRetryConfig(3), &out)

	if got != HealthExitTransient {
		t.Fatalf("runHealthChecks() exit = %d, want %d\noutput:\n%s", got, HealthExitTransient, out.String())
	}
	for _, substr := range []string{"UNAVAILABLE", "transient — retried 3 times", "attempts=4", "HTTP 503"} {
		if !strings.Contains(out.String(), substr) {
			t.Errorf("output %q does not contain %q", out.String(), substr)
		}
	}
}

func TestRunHealthChecks_PermanentStopsAfterOneAttempt(t *testing.T) {
	srv := statusServer(t, http.StatusInternalServerError)
	target := resilience.GatewayTarget(srv.URL)
	target.Name = "strict"
	target.Client = srv.Client()

	var out bytes.Buffer
	got := runHealthChecks(context.Background(), []resilience.Target{target}, instantRetryConfig(5), &out)

	if got != HealthExitPermanent {
		t.Fatalf("runHealthChecks() exit = %d, want %d\noutput:\n%s", got, HealthExitPermanent, out.String())
	}
	if !strings.Contains(out.String(), "attempts=1") {
		t.Errorf("output %q should record a single attempt (permanent failures are not retried)", out.String())
	}
}

func TestHealthTargets(t *testing.T) {
	t.Run("empty URLs default to the inference gateway", func(t *testing.T) {
		targets, err := healthTargets(HealthCheckConfig{})
		if err != nil {
			t.Fatalf("healthTargets() error = %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("healthTargets() returned %d targets, want 1", len(targets))
		}
		if targets[0].URL != resilience.DefaultGatewayURL {
			t.Errorf("target URL = %q, want the default gateway URL", targets[0].URL)
		}
		if targets[0].Name != "inference-gateway" {
			t.Errorf("target name = %q, want %q", targets[0].Name, "inference-gateway")
		}
		if !targets[0].InsecureSkipVerify {
			t.Error("the gateway target must tolerate its self-signed certificate")
		}
		if targets[0].Timeout != resilience.DefaultHealthCheckTimeout {
			t.Errorf("target timeout = %v, want the default %v", targets[0].Timeout, resilience.DefaultHealthCheckTimeout)
		}
	})

	t.Run("a single custom URL is named by host", func(t *testing.T) {
		targets, err := healthTargets(HealthCheckConfig{
			URLs: []string{"https://rdap.example.com:8443/health"},
		})
		if err != nil {
			t.Fatalf("healthTargets() error = %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("healthTargets() returned %d targets, want 1", len(targets))
		}
		if targets[0].Name != "rdap.example.com:8443" {
			t.Errorf("target name = %q, want %q", targets[0].Name, "rdap.example.com:8443")
		}
	})

	t.Run("multiple targets are named by full URL", func(t *testing.T) {
		// Same host for both — host-derived names would be indistinguishable.
		first, second := "http://one.example:9000/health", "http://one.example:9000/deep"
		targets, err := healthTargets(HealthCheckConfig{URLs: []string{first + "," + second}})
		if err != nil {
			t.Fatalf("healthTargets() error = %v", err)
		}
		if len(targets) != 2 {
			t.Fatalf("healthTargets() returned %d targets, want 2", len(targets))
		}
		if targets[0].Name != first || targets[1].Name != second {
			t.Errorf("target names = %q, %q; want the full URLs", targets[0].Name, targets[1].Name)
		}
	})

	t.Run("comma-separated and repeated URLs all become targets", func(t *testing.T) {
		targets, err := healthTargets(HealthCheckConfig{
			URLs: []string{"http://a.example/health, https://b.example/health", "http://c.example/health"},
		})
		if err != nil {
			t.Fatalf("healthTargets() error = %v", err)
		}
		if len(targets) != 3 {
			t.Fatalf("healthTargets() returned %d targets, want 3", len(targets))
		}
	})

	t.Run("configured timeout overrides the default", func(t *testing.T) {
		targets, err := healthTargets(HealthCheckConfig{Timeout: 1500 * time.Millisecond})
		if err != nil {
			t.Fatalf("healthTargets() error = %v", err)
		}
		if targets[0].Timeout != 1500*time.Millisecond {
			t.Errorf("target timeout = %v, want 1.5s", targets[0].Timeout)
		}
	})

	t.Run("invalid URLs are rejected", func(t *testing.T) {
		for _, bad := range []string{"not-a-url", "ftp://x.example/health", "http://"} {
			if _, err := healthTargets(HealthCheckConfig{URLs: []string{bad}}); err == nil {
				t.Errorf("healthTargets(%q) expected an error", bad)
			}
		}
	})

	t.Run("comma-separated list of only separators is an error", func(t *testing.T) {
		if _, err := healthTargets(HealthCheckConfig{URLs: []string{",,,"}}); err == nil {
			t.Error("healthTargets(\",,,\") expected an error, got nil")
		}
	})
}

func TestHealthRetryConfig(t *testing.T) {
	if got := healthRetryConfig(2).MaxRetries; got != 2 {
		t.Errorf("healthRetryConfig(2).MaxRetries = %d, want 2", got)
	}
	if got := healthRetryConfig(-1).MaxRetries; got != 0 {
		t.Errorf("healthRetryConfig(-1).MaxRetries = %d, want 0 (no retries)", got)
	}
	def := healthRetryConfig(resilience.DefaultMaxRetries)
	if def.MaxRetries != resilience.DefaultMaxRetries || def.BaseDelay != resilience.DefaultBaseDelay {
		t.Errorf("healthRetryConfig(default) = %+v, want the standard policy", def)
	}
}

func TestHealthCheck_UsageError(t *testing.T) {
	if got := HealthCheck(context.Background(), HealthCheckConfig{URLs: []string{"not-a-url"}}); got != HealthExitError {
		t.Errorf("HealthCheck(invalid URL) exit = %d, want %d", got, HealthExitError)
	}
}
