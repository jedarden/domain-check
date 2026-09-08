// Package cli provides command-line interface functionality for domain-check.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jedarden/domain-check/internal/resilience"
)

// Exit codes for the healthcheck subcommand. Unlike check/bulk, which use 2
// for any error, healthcheck distinguishes the failure classes so a script can
// branch (retry later vs. page someone) without parsing output text:
// 3 is reserved for usage and configuration errors, where retrying the probe
// itself cannot help.
const (
	HealthExitOK        = 0 // every target answered as expected (a DEGRADED target still exits 0)
	HealthExitTransient = 1 // at least one target unavailable, transiently
	HealthExitPermanent = 2 // at least one target unavailable, permanently
	HealthExitError     = 3 // usage or configuration error
)

// HealthCheckConfig holds configuration for the healthcheck subcommand.
type HealthCheckConfig struct {
	URLs    []string      // Health endpoints to probe (default: the inference gateway)
	Timeout time.Duration // Per-attempt HTTP timeout (default 5s)
	Retries int           // Retries after the initial attempt (default 5)
}

// HealthCheck probes each configured endpoint and reports one line per target.
// Returns the exit code: the most severe outcome across targets, where a
// permanent failure outranks a transient one and a transient one outranks OK.
func HealthCheck(ctx context.Context, cfg HealthCheckConfig) int {
	targets, err := healthTargets(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return HealthExitError
	}
	return runHealthChecks(ctx, targets, healthRetryConfig(cfg.Retries), os.Stdout)
}

// healthRetryConfig returns the standard retry policy with MaxRetries set from
// the --retries flag. A negative count means no retries.
func healthRetryConfig(retries int) resilience.RetryConfig {
	cfg := resilience.DefaultRetryConfig()
	if retries < 0 {
		retries = 0
	}
	cfg.MaxRetries = retries
	return cfg
}

// healthTargets builds one resilience.Target per configured URL. An empty URL
// list means the documented default: the inference gateway's health endpoint,
// probed with the same self-signed-certificate tolerance the pre-flight shell
// check needs.
func healthTargets(cfg HealthCheckConfig) ([]resilience.Target, error) {
	urls := cfg.URLs
	if len(urls) == 0 {
		urls = []string{resilience.DefaultGatewayURL}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = resilience.DefaultHealthCheckTimeout
	}

	var targets []resilience.Target
	for _, raw := range urls {
		for _, u := range strings.Split(raw, ",") {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			parsed, err := url.Parse(u)
			if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
				return nil, fmt.Errorf("invalid health check URL %q (must be an absolute http(s) URL)", u)
			}
			targets = append(targets, healthTarget(u, timeout))
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no health check URLs given")
	}
	// Several targets at once can share a host (different paths, or health
	// endpoints on different ports of the same box), which would make their
	// output lines indistinguishable. With more than one target, name each by
	// its full URL instead.
	if len(targets) > 1 {
		for i := range targets {
			targets[i].Name = targets[i].URL
		}
	}
	return targets, nil
}

// healthTarget returns the probe target for one URL. Certificates are not
// verified — the default target is the inference gateway, which serves a
// self-signed certificate (the false alarm plain `curl -sf` reports as curl
// 60). The target is named after the URL's host, except for the default URL,
// which keeps the canonical name; healthTargets renames targets when several
// are configured.
func healthTarget(raw string, timeout time.Duration) resilience.Target {
	t := resilience.GatewayTarget(raw)
	t.Timeout = timeout
	if raw != resilience.DefaultGatewayURL {
		if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
			t.Name = parsed.Host
		}
	}
	return t
}

// runHealthChecks probes each target in turn and prints one line per result.
// The returned exit code is the most severe outcome across targets.
func runHealthChecks(ctx context.Context, targets []resilience.Target, retryCfg resilience.RetryConfig, out io.Writer) int {
	exit := HealthExitOK
	for _, target := range targets {
		result := resilience.CheckHealthWithRetry(ctx, target, retryCfg)
		fmt.Fprintln(out, formatHealthResult(result))
		if code := healthExitCode(result); code > exit {
			exit = code
		}
	}
	return exit
}

// healthExitCode maps one result to its exit code. An unknown class on an
// unhealthy target is treated as transient, matching the retry loop's own
// default of retrying unclassified errors.
func healthExitCode(r resilience.HealthResult) int {
	if r.Healthy {
		return HealthExitOK
	}
	if r.Class == resilience.ClassPermanent {
		return HealthExitPermanent
	}
	return HealthExitTransient
}

// formatHealthResult renders one result as a single line that answers, in
// order: is the service up, what did it answer, how long did it take, and —
// when it is down — whether retrying is worth anything. A target that answered
// only after retries is DEGRADED: up now, but flapping.
func formatHealthResult(r resilience.HealthResult) string {
	if r.Healthy {
		if r.Attempts > 1 {
			return fmt.Sprintf("%s: DEGRADED status=%d latency=%dms attempts=%d (recovered after %d retries)",
				r.Name, r.StatusCode, r.LatencyMS, r.Attempts, r.Attempts-1)
		}
		return fmt.Sprintf("%s: OK status=%d latency=%dms attempts=%d",
			r.Name, r.StatusCode, r.LatencyMS, r.Attempts)
	}

	switch r.Class {
	case resilience.ClassTransient:
		retries := max(r.Attempts-1, 0)
		unit := "times"
		if retries == 1 {
			unit = "time"
		}
		return fmt.Sprintf("%s: UNAVAILABLE status=%d latency=%dms attempts=%d (transient — retried %d %s: %s)",
			r.Name, r.StatusCode, r.LatencyMS, r.Attempts, retries, unit, r.Err)
	case resilience.ClassPermanent:
		return fmt.Sprintf("%s: UNAVAILABLE status=%d latency=%dms attempts=%d (permanent failure: %s)",
			r.Name, r.StatusCode, r.LatencyMS, r.Attempts, r.Err)
	default:
		return fmt.Sprintf("%s: UNAVAILABLE status=%d latency=%dms attempts=%d (%s)",
			r.Name, r.StatusCode, r.LatencyMS, r.Attempts, r.Err)
	}
}
