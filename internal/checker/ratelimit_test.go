package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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
	var secondDone atomic.Bool

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

	// Wait for the first request to start and acquire the semaphore.
	time.Sleep(20 * time.Millisecond)

	// Second request should block until first releases (concurrency limit = 1).
	// Run in goroutine to avoid deadlock.
	go func() {
		rl.Acquire(context.Background(), "conc.registry", func() (*http.Response, error) {
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
		secondDone.Store(true)
	}()

	// Give second request time to queue up (but it can't proceed yet).
	time.Sleep(20 * time.Millisecond)

	// At this point, maxConcurrent should still be 1 (second request is blocked).
	assert.Equal(t, int32(1), maxConcurrent.Load())

	// Release the first request.
	close(block)

	// Wait for both requests to complete.
	for !firstDone.Load() || !secondDone.Load() {
		time.Sleep(time.Millisecond)
	}

	// Max concurrent should never have exceeded 1.
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

func TestQueueDepth_ActiveEntries(t *testing.T) {
	rl := NewRateLimiter()

	// Configure a slow registry that will have queued requests.
	rl.mu.Lock()
	rl.registry["slow.queue"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(1), 1),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(1),
			Burst:        1,
			Concurrency:  1,
			BackoffSteps: []float64{1},
			MaxRetries:   0,
		},
	}
	rl.mu.Unlock()

	// Start a blocking request.
	block := make(chan struct{})
	var firstStarted atomic.Bool
	go func() {
		rl.Acquire(context.Background(), "slow.queue", func() (*http.Response, error) {
			firstStarted.Store(true)
			<-block
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
	}()

	// Wait for first request to start.
	for !firstStarted.Load() {
		time.Sleep(time.Millisecond)
	}

	// Queue additional requests.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Acquire(context.Background(), "slow.queue", func() (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			})
		}()
	}

	// Give time for requests to queue.
	time.Sleep(50 * time.Millisecond)

	// Queue depth should be > 0.
	depth := rl.QueueDepth("slow.queue")
	assert.Greater(t, depth, 0, "expected queued requests")

	// Release the blocking request.
	close(block)
	wg.Wait()

	// After completion, queue depth should be 0.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, rl.QueueDepth("slow.queue"))
}

func TestGetOrCreate_ConcurrentCreation(t *testing.T) {
	rl := NewRateLimiter()

	var wg sync.WaitGroup
	created := make(chan *registryLimiter, 100)

	// Concurrently get or create the same registry.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter := rl.getOrCreate("concurrent.test")
			created <- limiter
		}()
	}
	wg.Wait()
	close(created)

	// All should return the same pointer.
	var first *registryLimiter
	for limiter := range created {
		if first == nil {
			first = limiter
		} else {
			assert.Same(t, first, limiter, "all concurrent calls should get same limiter")
		}
	}

	// Verify only one entry exists.
	rl.mu.Lock()
	count := len(rl.registry)
	rl.mu.Unlock()
	assert.Equal(t, 1, count)
}

func TestAcquire_BackoffBeyondSteps(t *testing.T) {
	rl := NewRateLimiter()

	// Configure with only 1 backoff step but 5 max retries.
	rl.mu.Lock()
	rl.registry["backoff.test"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{0.01}, // Only 1 step
			MaxRetries:   5,                // More retries than steps
		},
	}
	rl.mu.Unlock()

	var calls atomic.Int32

	_, err := rl.Acquire(context.Background(), "backoff.test", func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusTooManyRequests}, nil
	})

	require.Error(t, err)
	// Initial + 5 retries = 6 calls.
	assert.Equal(t, int32(6), calls.Load())
}

func TestAcquire_MultipleRegistries_Isolated(t *testing.T) {
	rl := NewRateLimiter()

	// Create two registries with different configs.
	rl.mu.Lock()
	rl.registry["registry-a"] = &registryLimiter{
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
	rl.registry["registry-b"] = &registryLimiter{
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

	var aCalls, bCalls atomic.Int32

	// Make concurrent requests to different registries.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			rl.Acquire(context.Background(), "registry-a", func() (*http.Response, error) {
				aCalls.Add(1)
				return &http.Response{StatusCode: http.StatusOK}, nil
			})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			rl.Acquire(context.Background(), "registry-b", func() (*http.Response, error) {
				bCalls.Add(1)
				return &http.Response{StatusCode: http.StatusOK}, nil
			})
		}
	}()

	wg.Wait()

	// Both registries should have processed all their requests.
	assert.Equal(t, int32(5), aCalls.Load())
	assert.Equal(t, int32(5), bCalls.Load())
}

func TestAcquire_ContextCanceledDuringSemaphoreWait(t *testing.T) {
	rl := NewRateLimiter()

	// Concurrency of 1.
	rl.mu.Lock()
	rl.registry["sem.wait"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(1),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  1,
			BackoffSteps: []float64{1},
			MaxRetries:   0,
		},
	}
	rl.mu.Unlock()

	// Block the semaphore.
	block := make(chan struct{})
	var firstStarted atomic.Bool
	go func() {
		rl.Acquire(context.Background(), "sem.wait", func() (*http.Response, error) {
			firstStarted.Store(true)
			<-block
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
	}()

	for !firstStarted.Load() {
		time.Sleep(time.Millisecond)
	}

	// Second request with cancelable context.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := rl.Acquire(ctx, "sem.wait", func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "concurrency limit")
	assert.Less(t, elapsed, 100*time.Millisecond, "should return quickly on context cancel")

	close(block)
}

func TestConfig_AfterGetOrCreate(t *testing.T) {
	rl := NewRateLimiter()

	// Config for non-existent registry returns default.
	cfg := rl.Config("nonexistent.registry")
	assert.Equal(t, defaultConfig.Rate, cfg.Rate)

	// After getOrCreate, Config returns the same default.
	rl.getOrCreate("nonexistent.registry")
	cfg = rl.Config("nonexistent.registry")
	assert.Equal(t, defaultConfig.Rate, cfg.Rate)
}

func TestAcquire_ConcurrencyLimitMultiple(t *testing.T) {
	rl := NewRateLimiter()

	// Concurrency of 3.
	rl.mu.Lock()
	rl.registry["multi.conc"] = &registryLimiter{
		limiter: rate.NewLimiter(rate.Limit(100), 100),
		sem:     semaphore.NewWeighted(3),
		config: RegistryConfig{
			Rate:         rate.Limit(100),
			Burst:        100,
			Concurrency:  3,
			BackoffSteps: []float64{0.01},
			MaxRetries:   0,
		},
	}
	rl.mu.Unlock()

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	block := make(chan struct{})

	// Start 10 requests, max 3 should run concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Acquire(context.Background(), "multi.conc", func() (*http.Response, error) {
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
		}()
	}

	// Wait for requests to queue up.
	time.Sleep(50 * time.Millisecond)

	// Max concurrent should be exactly 3.
	assert.Equal(t, int32(3), maxConcurrent.Load())

	close(block)
	wg.Wait()
}

func TestAcquire_QueueDepthDecrementsOnError(t *testing.T) {
	rl := NewRateLimiter()

	rl.mu.Lock()
	rl.registry["error.queue"] = &registryLimiter{
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

	// Make a request that errors.
	_, err := rl.Acquire(context.Background(), "error.queue", func() (*http.Response, error) {
		return nil, assert.AnError
	})

	require.Error(t, err)

	// Queue depth should be 0 after error.
	assert.Equal(t, 0, rl.QueueDepth("error.queue"))
}
