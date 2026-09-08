package resilience

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// DefaultHealthCheckTimeout bounds a single health-check attempt.
const DefaultHealthCheckTimeout = 5 * time.Second

// DefaultGatewayURL is the lab inference gateway's health endpoint, as used by
// the documented pre-flight check. The gateway serves a self-signed
// certificate, which is why GatewayTarget sets InsecureSkipVerify.
//
// Override it with the healthcheck command's --url flag (or Target.URL) for
// any other deployment.
const DefaultGatewayURL = "https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"

// Target describes an HTTP endpoint to probe for availability.
type Target struct {
	// Name identifies the dependency in results and error messages.
	Name string

	// URL is the health endpoint to GET.
	URL string

	// Timeout bounds a single attempt. DefaultHealthCheckTimeout when zero.
	Timeout time.Duration

	// InsecureSkipVerify accepts self-signed TLS certificates. Required for
	// the inference gateway, which serves one; leave false for anything with
	// a real certificate chain.
	InsecureSkipVerify bool

	// ExpectedStatus is the status code that counts as healthy. Zero means
	// any 2xx response is healthy.
	ExpectedStatus int

	// Client overrides the constructed HTTP client. Intended for tests.
	Client *http.Client
}

// GatewayTarget returns a Target for the inference gateway (or another
// self-signed endpoint) at url, with the timeout and certificate handling the
// documented pre-flight check uses.
func GatewayTarget(url string) Target {
	return Target{
		Name:               "inference-gateway",
		URL:                url,
		Timeout:            DefaultHealthCheckTimeout,
		InsecureSkipVerify: true,
	}
}

// HealthResult is the outcome of one health check (or, with
// CheckHealthWithRetry, of the whole retry cycle).
type HealthResult struct {
	// Name echoes Target.Name.
	Name string `json:"name"`
	// URL echoes Target.URL.
	URL string `json:"url"`
	// Healthy is true when the endpoint answered as expected.
	Healthy bool `json:"healthy"`
	// StatusCode is the HTTP status of the final attempt, 0 if the request
	// never got a response.
	StatusCode int `json:"status_code,omitempty"`
	// LatencyMS is the final attempt's round-trip time in milliseconds.
	LatencyMS int64 `json:"latency_ms,omitempty"`
	// Err describes why the check failed. Empty when healthy.
	Err string `json:"error,omitempty"`
	// Class says whether the failure looks transient (worth retrying) or
	// permanent (worth investigating). ClassUnknown when healthy.
	Class Class `json:"class"`
	// Attempts is the number of attempts made: 1 for a bare CheckHealth, or
	// the retry cycle's total for CheckHealthWithRetry.
	Attempts int `json:"attempts"`
	// CheckedAt is when the final attempt completed.
	CheckedAt time.Time `json:"checked_at"`
}

// String renders the result as a single log-style line.
func (r HealthResult) String() string {
	status := "HEALTHY"
	if !r.Healthy {
		status = "UNAVAILABLE"
	}
	if r.Err != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Name, status, r.Err)
	}
	return fmt.Sprintf("%s: %s status=%d latency=%dms", r.Name, status, r.StatusCode, r.LatencyMS)
}

// client returns the HTTP client for this target, building one with the
// configured timeout and certificate handling when Target.Client is unset.
func (t Target) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = DefaultHealthCheckTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: timeout}).DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			// Deliberate: the inference gateway serves a self-signed
			// certificate, so certificate verification cannot succeed. Only
			// targets that opt into it via InsecureSkipVerify reach here.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: t.InsecureSkipVerify},
		},
	}
}

func (t Target) timeout() time.Duration {
	if t.Timeout <= 0 {
		return DefaultHealthCheckTimeout
	}
	return t.Timeout
}

// CheckHealth probes the target once and classifies the outcome. It never
// retries; wrap it in Do, or use CheckHealthWithRetry.
//
// Classification:
//   - expected status (or any 2xx) → healthy
//   - a status in TransientHTTPStatuses (502, 503, …) → unhealthy, transient
//   - any other status → unhealthy, permanent
//   - transport error (connection refused, TLS, DNS) → unhealthy, transient
//   - context cancelled or deadline exceeded → unhealthy, permanent
func CheckHealth(ctx context.Context, target Target) HealthResult {
	timeout := target.timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := HealthResult{
		Name:      target.Name,
		URL:       target.URL,
		Attempts:  1,
		CheckedAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		result.Err = fmt.Sprintf("invalid health check URL: %v", err)
		result.Class = ClassPermanent
		return result
	}

	start := time.Now()
	resp, err := target.client().Do(req)
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.CheckedAt = time.Now()
		result.Err = err.Error()
		// A context that the caller cancelled (or whose deadline the attempt
		// exhausted) is not the service's fault; retrying it adds nothing.
		if ctx.Err() != nil {
			result.Err = fmt.Sprintf("health check did not complete within %s: %v", timeout, err)
			result.Class = ClassPermanent
			return result
		}
		result.Class = ClassTransient
		return result
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // reuse the connection
		_ = resp.Body.Close()
	}()

	result.StatusCode = resp.StatusCode
	result.CheckedAt = time.Now()

	switch {
	case target.ExpectedStatus != 0 && resp.StatusCode == target.ExpectedStatus,
		target.ExpectedStatus == 0 && resp.StatusCode >= 200 && resp.StatusCode < 300:
		result.Healthy = true
		return result
	case IsTransientStatus(resp.StatusCode):
		reason := StatusReason(resp.StatusCode)
		if reason == "" {
			reason = "transient"
		}
		result.Err = fmt.Sprintf("HTTP %d (%s)", resp.StatusCode, reason)
		result.Class = ClassTransient
		return result
	default:
		result.Err = fmt.Sprintf("HTTP %d", resp.StatusCode)
		result.Class = ClassPermanent
		return result
	}
}

// CheckHealthWithRetry probes the target with the given retry policy and
// reports the final outcome, including how many attempts it took.
//
// A transient verdict is retried with exponential backoff; a permanent one is
// returned after the first attempt, and an open circuit-breaker verdict would
// be too. The returned result carries the last attempt's error, so callers
// can log one line that says both what failed and how many times it was tried.
func CheckHealthWithRetry(ctx context.Context, target Target, cfg RetryConfig) HealthResult {
	var result HealthResult
	attempts := 0

	err := Do(ctx, cfg, func(attempt int) error {
		attempts = attempt
		result = CheckHealth(ctx, target)
		if result.Healthy {
			return nil
		}
		if result.Class == ClassPermanent {
			return WrapPermanent(fmt.Errorf("%s", result.Err))
		}
		return WrapTransient(fmt.Errorf("%s", result.Err))
	})

	if err != nil && !result.Healthy {
		// Keep the last attempt's status/latency, but report the retry loop's
		// view of the error so exhausted retries say so explicitly.
		result.Attempts = attempts
		if _, exhausted := err.(*AttemptsExhaustedError); exhausted {
			result.Err = err.Error()
		}
		return result
	}

	if result.Healthy {
		result.Attempts = attempts
	}
	return result
}
