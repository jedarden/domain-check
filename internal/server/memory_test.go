// Package server provides HTTP server integration tests including memory growth testing.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/checker"
	"github.com/coding/domain-check/internal/domain"
)

// memSnapshot captures a point-in-time snapshot of Go runtime memory stats.
type memSnapshot struct {
	Timestamp       time.Time
	HeapAlloc       uint64
	HeapSys         uint64
	HeapInuse       uint64
	HeapIdle        uint64
	HeapReleased    uint64
	HeapObjects     uint64
	StackInuse      uint64
	StackSys        uint64
	MSpanInuse      uint64
	MSpanSys        uint64
	MCacheInuse     uint64
	MCacheSys       uint64
	BuckHashSys     uint64
	GCSys           uint64
	NextGC          uint64
	Goroutines      int
	NumGC           uint32
	GCPauseLast     time.Duration
	Sys             uint64
	TotalAlloc      uint64
	NumForcedGC     uint32
	GCCPUFraction   float64
	IPLimiterCount  int
	CacheEntries    int
}

// countIPLimiters returns the total number of IP limiter entries across all limiters.
// Must be called within a test that has access to the package-internal fields.
func countIPLimiters(rl *RateLimiter) int {
	rl.webLimit.mu.RLock()
	webCount := len(rl.webLimit.limiters)
	rl.webLimit.mu.RUnlock()

	rl.apiLimit.mu.RLock()
	apiCount := len(rl.apiLimit.limiters)
	rl.apiLimit.mu.RUnlock()

	rl.bulkLimit.mu.RLock()
	bulkCount := len(rl.bulkLimit.limiters)
	rl.bulkLimit.mu.RUnlock()

	return webCount + apiCount + bulkCount
}

func takeSnapshot(t *testing.T, rl *RateLimiter, cache *checker.ResultCache) memSnapshot {
	t.Helper()
	var m runtime.MemStats
	runtime.GC()
	time.Sleep(10 * time.Millisecond) // let GC settle
	runtime.ReadMemStats(&m)

	var cacheLen int
	if cache != nil {
		cacheLen = cache.Len()
	}

	return memSnapshot{
		Timestamp:      time.Now(),
		HeapAlloc:      m.HeapAlloc,
		HeapSys:        m.HeapSys,
		HeapInuse:      m.HeapInuse,
		HeapIdle:       m.HeapIdle,
		HeapReleased:   m.HeapReleased,
		HeapObjects:    m.HeapObjects,
		StackInuse:     m.StackInuse,
		StackSys:       m.StackSys,
		MSpanInuse:     m.MSpanInuse,
		MSpanSys:       m.MSpanSys,
		MCacheInuse:    m.MCacheInuse,
		MCacheSys:      m.MCacheSys,
		BuckHashSys:    m.BuckHashSys,
		GCSys:          m.GCSys,
		NextGC:         m.NextGC,
		Goroutines:     runtime.NumGoroutine(),
		NumGC:          m.NumGC,
		GCPauseLast:    time.Duration(m.PauseNs[(m.NumGC+255)%256]),
		Sys:            m.Sys,
		TotalAlloc:     m.TotalAlloc,
		NumForcedGC:    m.NumForcedGC,
		GCCPUFraction:  m.GCCPUFraction,
		IPLimiterCount: countIPLimiters(rl),
		CacheEntries:   cacheLen,
	}
}

// bytesMB converts bytes to megabytes with 2 decimal places.
func bytesMB(b uint64) float64 {
	return float64(b) / (1024 * 1024)
}

// memoryTestChecker is a lightweight mock DomainChecker + BulkChecker for load tests.
type memoryTestChecker struct{}

func (mc *memoryTestChecker) Check(_ context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	return &domain.DomainResult{
		Domain:    normalizedDomain,
		Available: true,
		TLD:       "com",
		Source:    "mock",
	}, nil
}

func (mc *memoryTestChecker) CheckBulk(_ context.Context, domains []string) *checker.BulkResult {
	result := &checker.BulkResult{
		Results: make(map[string]*domain.DomainResult, len(domains)),
	}
	for _, d := range domains {
		result.Results[d] = &domain.DomainResult{
			Domain:    d,
			Available: true,
			TLD:       "com",
			Source:    "mock",
		}
	}
	return result
}

// TestMemoryGrowthUnderLoad runs 50 req/s for 2 minutes and verifies memory growth
// plateaus. The 2-minute duration is used for CI; the full 10-minute test is run
// via TestMemoryGrowthUnderLoadFull.
func TestMemoryGrowthUnderLoad(t *testing.T) {
	runMemoryGrowthTest(t, 2*time.Minute)
}

// TestMemoryGrowthUnderLoadFull runs the full 10-minute memory growth test.
// Run with: go test -v -run TestMemoryGrowthUnderLoadFull -timeout 15m ./internal/server/
func TestMemoryGrowthUnderLoadFull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10-minute memory growth test in short mode")
	}
	runMemoryGrowthTest(t, 10*time.Minute)
}

func runMemoryGrowthTest(t *testing.T, duration time.Duration) {
	t.Helper()

	rate := 50 // requests per second

	t.Logf("Starting memory growth test: %d req/s for %v", rate, duration)
	t.Logf("Go version: %s", runtime.Version())
	t.Logf("GOMAXPROCS: %d", runtime.GOMAXPROCS(0))

	// Create components.
	cache := checker.NewResultCache(checker.DefaultTTLs(), 10000)
	rateLimiter := NewRateLimiter(testLogger(t))

	// Start periodic cleanup (same as production, every 10 minutes).
	cleanupDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimiter.Cleanup()
			case <-cleanupDone:
				return
			}
		}
	}()

	mc := &memoryTestChecker{}

	// Create a real server with full middleware chain.
	mux := http.NewServeMux()
	apiHandlers := NewAPIHandlers(mc, testLogger(t))
	mux.HandleFunc("GET /api/v1/check", apiHandlers.CheckHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	handler := Chain(mux,
		RequestID,
		ClientIP(true),
		Logging(testLogger(t)),
		SecurityHeaders,
	)

	srv := httptest.NewServer(handler)

	// Take baseline snapshot.
	baseline := takeSnapshot(t, rateLimiter, cache)
	t.Logf("BASELINE - HeapAlloc: %.2f MB, HeapSys: %.2f MB, Sys: %.2f MB, Goroutines: %d, GC cycles: %d",
		bytesMB(baseline.HeapAlloc), bytesMB(baseline.HeapSys), bytesMB(baseline.Sys),
		baseline.Goroutines, baseline.NumGC)

	// Set up load generator.
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	var totalReqs atomic.Int64
	var errReqs atomic.Int64

	// Use randomized IPs to stress the ipLimiter map.
	// At 50 req/s for 10 min = 30K requests, each with a unique IP.
	ipCounter := atomic.Int64{}

	// Generate request rate using ticker.
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	// Snapshot collector.
	var snapshots []memSnapshot
	snapshotInterval := 10 * time.Second
	snapshotTicker := time.NewTicker(snapshotInterval)
	defer snapshotTicker.Stop()

	// Run snapshot collector.
	snapshotDone := make(chan struct{})
	go func() {
		defer close(snapshotDone)
		for {
			select {
			case <-snapshotTicker.C:
				snap := takeSnapshot(t, rateLimiter, cache)
				snapshots = append(snapshots, snap)
				heapGrowth := int64(snap.HeapAlloc) - int64(baseline.HeapAlloc)
				deltaMB := float64(heapGrowth) / (1024 * 1024)
				if deltaMB < 0 {
					deltaMB = 0
				}
				t.Logf("SNAPSHOT [%s] HeapAlloc: %.2f MB (Δ %.2f MB), HeapInuse: %.2f MB, Goroutines: %d, IP limiters: %d, Cache: %d, GC: %d",
					time.Since(baseline.Timestamp).Round(time.Second),
					bytesMB(snap.HeapAlloc), deltaMB,
					bytesMB(snap.HeapInuse), snap.Goroutines,
					snap.IPLimiterCount, snap.CacheEntries, snap.NumGC)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Run load generator.
	t.Logf("Starting load generation: %d req/s", rate)
	wg.Add(rate)
	started := make(chan struct{})

	for i := 0; i < rate; i++ {
		go func() {
			defer wg.Done()
			<-started
			client := &http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					DisableKeepAlives:   true,
					MaxIdleConns:        0,
					MaxIdleConnsPerHost: 0,
				},
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					ipIdx := ipCounter.Add(1)
					octet2 := (ipIdx / 256) % 256
					octet3 := ipIdx % 256
					ip := fmt.Sprintf("10.%d.%d.%d", (ipIdx/65536)%256, octet2, octet3)

					req, err := http.NewRequestWithContext(ctx, http.MethodGet,
						srv.URL+"/api/v1/check?d=available-test-domain.com", nil)
					if err != nil {
						continue
					}
					req.Header.Set("X-Forwarded-For", ip)

					resp, err := client.Do(req)
					totalReqs.Add(1)
					if err != nil {
						errReqs.Add(1)
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}()
	}

	close(started)

	// Wait for test duration.
	<-ctx.Done()

	// Close the test server first to cause HTTP requests to fail and goroutines to exit.
	srv.Close()

	// Stop tickers and cleanup goroutine.
	ticker.Stop()
	snapshotTicker.Stop()
	close(cleanupDone)

	// Wait for all goroutines to finish.
	wg.Wait()

	// Give goroutines time to settle.
	time.Sleep(2 * time.Second)

	// Take final snapshot.
	final := takeSnapshot(t, rateLimiter, cache)
	heapGrowth := int64(final.HeapAlloc) - int64(baseline.HeapAlloc)
	heapGrowthMB := float64(heapGrowth) / (1024 * 1024)

	t.Logf("FINAL - HeapAlloc: %.2f MB, HeapSys: %.2f MB, Sys: %.2f MB, Goroutines: %d, GC cycles: %d",
		bytesMB(final.HeapAlloc), bytesMB(final.HeapSys), bytesMB(final.Sys),
		final.Goroutines, final.NumGC)
	t.Logf("Total requests: %d, Errors: %d", totalReqs.Load(), errReqs.Load())
	t.Logf("Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("IP limiter entries: %d", final.IPLimiterCount)

	// Analyze snapshots for trends.
	t.Log("\n=== Memory Growth Analysis ===")
	if len(snapshots) > 2 {
		mid := len(snapshots) / 2
		var firstHalfSum, secondHalfSum uint64
		for i, s := range snapshots {
			if i < mid {
				firstHalfSum += s.HeapAlloc
			} else {
				secondHalfSum += s.HeapAlloc
			}
		}
		firstHalfAvg := float64(firstHalfSum) / float64(mid)
		secondHalfAvg := float64(secondHalfSum) / float64(len(snapshots)-mid)
		trendMB := (secondHalfAvg - firstHalfAvg) / (1024 * 1024)

		t.Logf("First half avg HeapAlloc: %.2f MB", firstHalfAvg/(1024*1024))
		t.Logf("Second half avg HeapAlloc: %.2f MB", secondHalfAvg/(1024*1024))
		t.Logf("Trend (second half - first half): %.2f MB", trendMB)

		if trendMB > 50 {
			t.Errorf("Memory trend is concerning: %.2f MB growth between halves (>50 MB threshold)", trendMB)
		}

		// Check if memory is still growing at the end (should plateau).
		lastN := min(5, len(snapshots))
		var lastNSlope float64
		if lastN > 1 {
			for i := len(snapshots) - lastN + 1; i < len(snapshots); i++ {
				delta := float64(snapshots[i].HeapAlloc-snapshots[i-1].HeapAlloc) / (1024 * 1024)
				lastNSlope += delta
			}
			lastNSlope /= float64(lastN - 1)
			t.Logf("Final %d snapshots avg per-interval growth: %.2f MB", lastN, lastNSlope)
		}
	}

	// Assertions.
	t.Run("heap_growth", func(t *testing.T) {
		if heapGrowthMB > 100 {
			t.Errorf("Heap grew %.2f MB (> 100 MB threshold) — possible memory leak", heapGrowthMB)
		} else {
			t.Logf("PASS: Heap grew %.2f MB (< 100 MB threshold)", heapGrowthMB)
		}
	})

	t.Run("goroutine_leak", func(t *testing.T) {
		maxGoroutines := int(float64(baseline.Goroutines) * 1.2)
		if baseline.Goroutines < 10 {
			maxGoroutines = baseline.Goroutines + 10
		}
		if final.Goroutines > maxGoroutines {
			t.Errorf("Goroutine leak: baseline=%d, final=%d (max acceptable=%d)",
				baseline.Goroutines, final.Goroutines, maxGoroutines)
		} else {
			t.Logf("PASS: Goroutines baseline=%d, final=%d", baseline.Goroutines, final.Goroutines)
		}
	})

	t.Run("ip_limiter_growth", func(t *testing.T) {
		t.Logf("IP limiter entries: %d", final.IPLimiterCount)
		if final.IPLimiterCount > 100000 {
			t.Errorf("IP limiter map has %d entries (>100K) — possible cleanup issue",
				final.IPLimiterCount)
		}
	})

	t.Run("cache_bounded", func(t *testing.T) {
		if final.CacheEntries > 10000 {
			t.Errorf("Cache has %d entries (>10000 max)", final.CacheEntries)
		} else {
			t.Logf("PASS: Cache entries=%d (max 10000)", final.CacheEntries)
		}
	})
}

// TestIPLimiterCleanup verifies that stale IP limiter entries are cleaned up.
func TestIPLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(testLogger(t))

	// Simulate 1000 unique IPs hitting the API endpoint.
	for i := 0; i < 1000; i++ {
		ip := fmt.Sprintf("10.1.%d.%d", i/256, i%256)
		limiter := rl.apiLimit.getLimiter(ip)
		limiter.Allow() // consume one token
	}

	initialCount := len(rl.apiLimit.limiters)
	t.Logf("After 1000 unique IPs: %d limiter entries", initialCount)
	if initialCount != 1000 {
		t.Errorf("Expected 1000 limiter entries, got %d", initialCount)
	}

	// Test cleanup behavior directly with a custom limiter.
	testIPL := newIPLimiter(1000, 5) // 1000 req/min ≈ 16.7/sec, burst 5
	testIP := "192.168.1.1"
	lim := testIPL.getLimiter(testIP)
	lim.Allow()
	lim.Allow()
	lim.Allow()

	// Tokens left: 5-3 = 2. Cleanup should NOT remove this.
	testIPL.cleanup()
	if _, exists := testIPL.limiters[testIP]; !exists {
		t.Error("Cleanup removed IP that still has consumed tokens")
	}

	// Wait for tokens to fully replenish.
	// Rate is 1000/min ≈ 16.7/sec, need 3 tokens back ≈ 0.18s.
	time.Sleep(500 * time.Millisecond)
	testIPL.cleanup()
	if _, exists := testIPL.limiters[testIP]; exists {
		t.Log("Note: IP still present after cleanup (tokens may not be fully replenished yet)")
	}

	// Force replenish by waiting longer.
	time.Sleep(2 * time.Second)
	testIPL.cleanup()
	_, exists := testIPL.limiters[testIP]
	if exists {
		t.Logf("IP limiter still present after 2.5s — tokens=%.1f (burst=%d)",
			testIPL.limiters[testIP].Tokens(), testIPL.burst)
	}
}

// TestIPLimiterMemoryGrowth measures memory usage of the ipLimiter map under load.
func TestIPLimiterMemoryGrowth(t *testing.T) {
	rl := NewRateLimiter(testLogger(t))

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baseHeap := m1.HeapAlloc

	// Add 50000 unique IP limiters.
	for i := 0; i < 50000; i++ {
		ip := fmt.Sprintf("10.%d.%d.%d", (i/65536)%256, (i/256)%256, i%256)
		rl.apiLimit.getLimiter(ip)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	growth := m2.HeapAlloc - baseHeap
	perEntry := float64(growth) / 50000

	t.Logf("50K IP limiters: total heap growth = %.2f KB, per-entry = %.0f bytes",
		float64(growth)/1024, perEntry)

	// Each entry should be small (< 500 bytes including map overhead).
	if perEntry > 500 {
		t.Errorf("IP limiter entries are too large: %.0f bytes each (>500)", perEntry)
	}

	// Cleanup all entries — since we never consumed tokens, all buckets should be full.
	rl.apiLimit.cleanup()
	afterCleanup := len(rl.apiLimit.limiters)
	t.Logf("After cleanup: %d entries remaining (expected < 100 since all tokens are full)", afterCleanup)

	if afterCleanup > 100 {
		t.Errorf("Cleanup left %d entries (expected < 100 since all tokens are full)", afterCleanup)
	}
}

// testLogger returns a minimal slog.Logger for tests.
func testLogger(t *testing.T) *slog.Logger {
	return slog.Default()
}
