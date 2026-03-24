// Package checker provides domain availability checking with per-registry rate limiting.
package checker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// ErrServiceBusy is returned when the queue depth limit for a registry is exceeded.
var ErrServiceBusy = errors.New("registry queue depth limit exceeded")

// RegistryConfig defines rate limit and concurrency settings for a registry.
type RegistryConfig struct {
	// Rate is the token bucket rate (requests per second).
	Rate rate.Limit
	// Burst is the token bucket burst size.
	Burst int
	// Concurrency is the maximum number of concurrent requests.
	Concurrency int64
	// BackoffSteps are the exponential backoff durations (seconds) on HTTP 429.
	BackoffSteps []float64
	// MaxRetries is the maximum number of retries on HTTP 429.
	MaxRetries int
}

// Default registry configurations.
var defaultConfigs = map[string]RegistryConfig{
	"rdap.verisign.com": {
		Rate:         10,
		Burst:        10,
		Concurrency:  10,
		BackoffSteps: []float64{1, 2, 4},
		MaxRetries:   3,
	},
	"rdap.publicinterestregistry.org": {
		Rate:         10,
		Burst:        10,
		Concurrency:  10,
		BackoffSteps: []float64{1, 2, 4},
		MaxRetries:   3,
	},
	"pubapi.registry.google": {
		Rate:         1,
		Burst:        2,
		Concurrency:  2,
		BackoffSteps: []float64{5, 10, 20},
		MaxRetries:   3,
	},
}

// defaultConfig is used for unknown registries.
var defaultConfig = RegistryConfig{
	Rate:         2,
	Burst:        3,
	Concurrency:  3,
	BackoffSteps: []float64{2, 4, 8},
	MaxRetries:   3,
}

// queueDepthLimit is the maximum number of requests waiting per registry.
const queueDepthLimit = 100

// registryLimiter holds rate limiting state for a single registry.
type registryLimiter struct {
	limiter    *rate.Limiter
	sem        *semaphore.Weighted
	config     RegistryConfig
	queueDepth int
}

// RateLimiter manages per-registry rate limiting and concurrency control.
type RateLimiter struct {
	mu       sync.Mutex
	registry map[string]*registryLimiter
}

// NewRateLimiter creates a new RateLimiter with default registry configurations.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		registry: make(map[string]*registryLimiter),
	}
}

// getOrCreate returns the limiter for a registry, creating one if needed.
func (rl *RateLimiter) getOrCreate(registry string) *registryLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if existing, ok := rl.registry[registry]; ok {
		return existing
	}

	cfg := defaultConfig
	if c, ok := defaultConfigs[registry]; ok {
		cfg = c
	}

	rl.registry[registry] = &registryLimiter{
		limiter: rate.NewLimiter(cfg.Rate, cfg.Burst),
		sem:     semaphore.NewWeighted(cfg.Concurrency),
		config:  cfg,
	}
	return rl.registry[registry]
}

// Acquire waits for rate limit and concurrency permission before executing fn.
// It returns ErrServiceBusy if the queue depth limit is exceeded.
// On HTTP 429 responses, it retries with exponential backoff up to MaxRetries.
func (rl *RateLimiter) Acquire(ctx context.Context, registry string, fn func() (*http.Response, error)) (*http.Response, error) {
	reg := rl.getOrCreate(registry)

	// Check queue depth.
	rl.mu.Lock()
	if reg.queueDepth >= queueDepthLimit {
		rl.mu.Unlock()
		return nil, ErrServiceBusy
	}
	reg.queueDepth++
	rl.mu.Unlock()

	defer func() {
		rl.mu.Lock()
		reg.queueDepth--
		rl.mu.Unlock()
	}()

	// Wait for rate limiter token.
	if err := reg.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Acquire concurrency semaphore.
	if err := reg.sem.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("concurrency limit: %w", err)
	}
	defer reg.sem.Release(1)

	// Execute with retry on 429.
	var lastErr error
	for attempt := 0; attempt <= reg.config.MaxRetries; attempt++ {
		resp, err := fn()
		if err != nil {
			return resp, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		if resp.Body != nil {
			resp.Body.Close()
		}

		if attempt >= reg.config.MaxRetries {
			lastErr = fmt.Errorf("HTTP 429: max retries (%d) exceeded for %s", reg.config.MaxRetries, registry)
			break
		}

		// Exponential backoff.
		backoffIdx := attempt
		if backoffIdx >= len(reg.config.BackoffSteps) {
			backoffIdx = len(reg.config.BackoffSteps) - 1
		}
		backoffSec := reg.config.BackoffSteps[backoffIdx]

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := sleepContext(ctx, time.Duration(backoffSec*float64(time.Second))); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

// QueueDepth returns the current queue depth for a registry.
func (rl *RateLimiter) QueueDepth(registry string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if reg, ok := rl.registry[registry]; ok {
		return reg.queueDepth
	}
	return 0
}

// Config returns the rate limit config for a registry.
func (rl *RateLimiter) Config(registry string) RegistryConfig {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if reg, ok := rl.registry[registry]; ok {
		return reg.config
	}
	cfg := defaultConfig
	if c, ok := defaultConfigs[registry]; ok {
		cfg = c
	}
	return cfg
}

// sleepContext sleeps for the given duration, respecting context cancellation.
func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
