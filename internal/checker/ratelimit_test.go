package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	assert.NotNil(t, rl)
}

func TestConfig_KnownRegistry(t *testing.T) {
	rl := NewRateLimiter()

	cfg := rl.Config("rdap.verisign.com")
	assert.Equal(t, rate.Limit(10), cfg.Rate)
	assert.Equal(t, 10, cfg.Burst)
	assert.Equal(t, int64(10), cfg.Concurrency)
	assert.Equal(t, []float64{1, 2, 4}, cfg.BackoffSteps)
	assert.Equal(t, 3, cfg.MaxRetries)
}

func TestConfig_PIR(t *testing.T) {
	rl := NewRateLimiter()

	cfg := rl.Config("rdap.publicinterestregistry.org")
	assert.Equal(t, rate.Limit(10), cfg.Rate)
	assert.Equal(t, 10, cfg.Burst)
	assert.Equal(t, int64(10), cfg.Concurrency)
}

func TestConfig_Google(t *testing.T) {
	rl := NewRateLimiter()

	cfg := rl.Config("pubapi.registry.google")
	assert.Equal(t, rate.Limit(1), cfg.Rate)
	assert.Equal(t, 2, cfg.Burst)
	assert.Equal(t, int64(2), cfg.Concurrency)
	assert.Equal(t, []float64{5, 10, 20}, cfg.BackoffSteps)
}

func TestConfig_UnknownRegistry(t *testing.T) {
	rl := NewRateLimiter()

	cfg := rl.Config("unknown.registry.example")
	assert.Equal(t, rate.Limit(2), cfg.Rate)
	assert.Equal(t, 3, cfg.Burst)
	assert.Equal(t, int64(3), cfg.Concurrency)
	assert.Equal(t, []float64{2, 4, 8}, cfg.BackoffSteps)
}

func TestAcquire_Success(t *testing.T) {
	rl := NewRateLimiter()

	var called atomic.Int32
	resp := &http.Response{StatusCode: http.StatusOK}

	result, err := rl.Acquire(context.Background(), "test.registry", func() (*http.Response, error) {
		called.Add(1)
		return resp, nil
	})

	require.NoError(t, err)
	assert.Equal(t, resp, result)
	assert.Equal(t, int32(1), called.Load())
}

func TestAcquire_RateLimiting(t *testing.T) {
	rl := NewRateLimiter()

	// Use a very slow limiter to verify rate limiting works.
	rl.mu.Lock()
	rl.registry["slow.registry"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(1), 1),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(1),
			Burst:        1,
			Concurrency:  1,
			BackoffSteps: []float64{0.01},
			MaxRetries:   0,
		},
	}
	rl.mu.Unlock()

	var called atomic.Int32
	start := time.Now()

	// Two rapid calls should be rate limited.
	done := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			rl.Acquire(context.Background(), "slow.registry", func() (*http.Response, error) {
				called.Add(1)
				return &http.Response{StatusCode: http.StatusOK}, nil
			})
			done <- struct{}{}
		}()
	}
	<-done
	<-done

	elapsed := time.Since(start)
	assert.Equal(t, int32(2), called.Load())
	// With rate limit of 1/sec, two calls should take at least ~1 second.
	assert.GreaterOrEqual(t, elapsed, 800*time.Millisecond)
}

func TestAcquire_RetryOn429(t *testing.T) {
	rl := NewRateLimiter()

	// Configure with fast backoff for testing.
	rl.mu.Lock()
	rl.registry["retry.registry"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{0.01, 0.01, 0.01},
			MaxRetries:   3,
		},
	}
	rl.mu.Unlock()

	var calls atomic.Int32
	start := time.Now()

	_, err := rl.Acquire(context.Background(), "retry.registry", func() (*http.Response, error) {
		n := calls.Add(1)
		if n <= 2 {
			// First two calls return 429.
			return &http.Response{StatusCode: http.StatusTooManyRequests}, nil
		}
		// Third call succeeds.
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
	// Should have waited for at least 2 backoff periods.
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 15*time.Millisecond)
}

func TestAcquire_RetryExhausted(t *testing.T) {
	rl := NewRateLimiter()

	rl.mu.Lock()
	rl.registry["exhaust.registry"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{0.01},
			MaxRetries:   1,
		},
	}
	rl.mu.Unlock()

	var calls atomic.Int32

	_, err := rl.Acquire(context.Background(), "exhaust.registry", func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusTooManyRequests}, nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
	// Initial call + 1 retry = 2 calls.
	assert.Equal(t, int32(2), calls.Load())
}

func TestAcquire_ContextCanceled(t *testing.T) {
	rl := NewRateLimiter()

	rl.mu.Lock()
	rl.registry["cancel.registry"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{10},
			MaxRetries:   3,
		},
	}
	rl.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	// First call triggers 429, then we cancel context during backoff.
	callCount := 0

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	_, err := rl.Acquire(ctx, "cancel.registry", func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{StatusCode: http.StatusTooManyRequests}, nil
		}
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestAcquire_QueueDepthLimit(t *testing.T) {
	rl := NewRateLimiter()

	rl.mu.Lock()
	rl.registry["busy.registry"] = &registryLimiter{
		limiter:    rate.NewLimiter(rate.Limit(100), 100),
		sem:        semaphore.NewWeighted(1),
		config:     RegistryConfig{Rate: rate.Limit(100), Burst: 100, Concurrency: 1, BackoffSteps: []float64{1}, MaxRetries: 0},
		queueDepth: queueDepthLimit,
	}
	rl.mu.Unlock()

	_, err := rl.Acquire(context.Background(), "busy.registry", func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	assert.ErrorIs(t, err, ErrServiceBusy)
}

func TestAcquire_ConcurrencyLimit(t *testing.T) {
	rl := NewRateLimiter()

	// Concurrency limit of 1, but we need 2 concurrent.
	rl.mu.Lock()
	rl.registry["conc.registry"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{0.01},
			MaxRetries:   0,
		},
	}
	rl.mu.Unlock()

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	// Block the semaphore with one long-running request.
	block := make(chan struct{})
	var firstDone atomic.Bool

	go func() {
		rl.Acquire(context.Background(), "conc.registry", func() (*http.Response, error) {
			c := concurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if c <= old || maxConcurrent.CompareAndSwap(old, c) {
					break
				}
			}
			<-block
			concurrent.Add(-1)
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
		firstDone.Store(true)
	}()

	// Wait for the first request to start.
	time.Sleep(10 * time.Millisecond)

	// Second request should also succeed but after first releases.
	_, err := rl.Acquire(context.Background(), "conc.registry", func() (*http.Response, error) {
		c := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if c <= old || maxConcurrent.CompareAndSwap(old, c) {
				break
			}
		}
		concurrent.Add(-1)
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	require.NoError(t, err)
	close(block)

	// Wait for first goroutine to finish.
	for !firstDone.Load() {
		time.Sleep(time.Millisecond)
	}

	assert.LessOrEqual(t, maxConcurrent.Load(), int32(1))
}

func TestQueueDepth(t *testing.T) {
	rl := NewRateLimiter()

	// Unknown registry has depth 0.
	assert.Equal(t, 0, rl.QueueDepth("unknown"))
}

func TestAcquire_FunctionError(t *testing.T) {
	rl := NewRateLimiter()

	_, err := rl.Acquire(context.Background(), "test.registry", func() (*http.Response, error) {
		return nil, assert.AnError
	})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestAcquire_Non429Response(t *testing.T) {
	rl := NewRateLimiter()

	statusCodes := []int{200, 404, 500, 503}
	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			resp, err := rl.Acquire(context.Background(), "test.registry", func() (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: code}, nil
			})

			require.NoError(t, err)
			assert.Equal(t, code, resp.StatusCode)
			assert.Equal(t, int32(1), calls.Load(), "should not retry non-429 status")
		})
	}
}

func TestAcquire_IntegrationWithTestServer(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rl := NewRateLimiter()

	rl.mu.Lock()
	rl.registry[srv.URL] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{0.01, 0.01, 0.01},
			MaxRetries:   3,
		},
	}
	rl.mu.Unlock()

	client := srv.Client()
	resp, err := rl.Acquire(context.Background(), srv.URL, func() (*http.Response, error) {
		return client.Get(srv.URL)
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), requestCount.Load())
}
