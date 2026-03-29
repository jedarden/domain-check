package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimiter_429Response(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the limit (10 requests)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.50")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Next request should get 429
	req := httptest.NewRequest("GET", "/", nil)
	ctx := contextWithClientIP(req.Context(), "10.0.0.50")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	// Check Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}

	// Check Content-Type is JSON
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Check body contains error message
	body := rec.Body.String()
	if !contains(body, "rate limit exceeded") {
		t.Errorf("expected body to contain 'rate limit exceeded', got %s", body)
	}
}

func TestRateLimiter_BulkRateLimit(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	bulkHandler := rl.BulkRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Bulk limit is 5/min, so 6th request should be blocked
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/bulk", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.200")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		bulkHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("bulk request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// 6th request should be blocked
	req := httptest.NewRequest("POST", "/api/v1/bulk", nil)
	ctx := contextWithClientIP(req.Context(), "10.0.0.200")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	bulkHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("bulk request 6: expected 429, got %d", rec.Code)
	}
}

func TestRateLimiter_RemoteAddrFallback(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests without setting client IP in context
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		req.RemoteAddr = "172.16.0.1:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

func TestRateLimiter_AllLimits(t *testing.T) {
	log := DefaultLogger("text", "error")

	t.Run("web limit is 10/min", func(t *testing.T) {
		rl := NewRateLimiter(log)
		handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// 10 requests should succeed
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			ctx := contextWithClientIP(req.Context(), "192.168.50.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 11th should fail
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), "192.168.50.1")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("request 11: expected 429, got %d", rec.Code)
		}
	})

	t.Run("API limit is 60/min", func(t *testing.T) {
		rl := NewRateLimiter(log)
		handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// 60 requests should succeed
		for i := 0; i < 60; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check", nil)
			ctx := contextWithClientIP(req.Context(), "192.168.50.2")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 61st should fail
		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		ctx := contextWithClientIP(req.Context(), "192.168.50.2")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("request 61: expected 429, got %d", rec.Code)
		}
	})

	t.Run("bulk limit is 5/min", func(t *testing.T) {
		rl := NewRateLimiter(log)
		handler := rl.BulkRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// 5 requests should succeed
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("POST", "/api/v1/bulk", nil)
			ctx := contextWithClientIP(req.Context(), "192.168.50.3")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 6th should fail
		req := httptest.NewRequest("POST", "/api/v1/bulk", nil)
		ctx := contextWithClientIP(req.Context(), "192.168.50.3")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("request 6: expected 429, got %d", rec.Code)
		}
	})
}

// TestRateLimiter_PerIPIsolation verifies that different IPs have separate rate limits.
func TestRateLimiter_PerIPIsolation(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP 1: exhaust its limit
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.1")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP 1 should be rate limited
	req := httptest.NewRequest("GET", "/api/v1/check", nil)
	ctx := contextWithClientIP(req.Context(), "10.0.0.1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("IP 1: expected 429 after limit, got %d", rec.Code)
	}

	// IP 2 should still be able to make requests (isolated limit)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.2")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("IP 2 request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// IP 2 should now be rate limited
	req = httptest.NewRequest("GET", "/api/v1/check", nil)
	ctx = contextWithClientIP(req.Context(), "10.0.0.2")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("IP 2: expected 429 after limit, got %d", rec.Code)
	}

	// IP 3 should still be able to make requests
	req = httptest.NewRequest("GET", "/api/v1/check", nil)
	ctx = contextWithClientIP(req.Context(), "10.0.0.3")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("IP 3: expected 200, got %d", rec.Code)
	}
}

// TestRateLimiter_StaleEntryEviction verifies that cleanup removes inactive entries.
func TestRateLimiter_StaleEntryEviction(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)

	handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create limiters for multiple IPs by making requests
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		ip := "10.0.0." + string(rune('0'+i))
		ctx := contextWithClientIP(req.Context(), ip)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Verify limiters were created
	rl.apiLimit.mu.RLock()
	initialCount := len(rl.apiLimit.limiters)
	rl.apiLimit.mu.RUnlock()

	if initialCount == 0 {
		t.Fatal("expected limiters to be created")
	}

	// Manipulate the limiters: make one have a full bucket (stale)
	rl.apiLimit.mu.Lock()
	for ip, limiter := range rl.apiLimit.limiters {
		// For one IP, don't consume - it will have full bucket
		if ip == "10.0.0.1" {
			// Reset by creating a new limiter with full bucket
			rl.apiLimit.limiters[ip] = newIPLimiter(60, 60).getLimiter(ip)
		} else {
			// Consume some tokens so bucket isn't full
			limiter.Allow()
		}
	}
	rl.apiLimit.mu.Unlock()

	// Run cleanup - should remove IPs with full buckets
	rl.Cleanup()

	rl.apiLimit.mu.RLock()
	afterCount := len(rl.apiLimit.limiters)
	rl.apiLimit.mu.RUnlock()

	if afterCount >= initialCount {
		t.Errorf("expected cleanup to remove stale entries, count before=%d, after=%d", initialCount, afterCount)
	}
}

// TestRateLimiter_CleanupPreservesActiveEntries verifies active entries are not cleaned up.
func TestRateLimiter_CleanupPreservesActiveEntries(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)

	handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make a request from an IP
	req := httptest.NewRequest("GET", "/", nil)
	ctx := contextWithClientIP(req.Context(), "192.168.1.100")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Immediately run cleanup - active entry should NOT be removed
	// because bucket won't be full yet
	rl.Cleanup()

	rl.webLimit.mu.RLock()
	count := len(rl.webLimit.limiters)
	rl.webLimit.mu.RUnlock()

	if count == 0 {
		t.Error("cleanup removed active entry that should have been preserved")
	}
}

// TestRateLimiter_TokenBucketRefill verifies the token bucket refills over time.
func TestRateLimiter_TokenBucketRefill(t *testing.T) {
	// Create a fast-refilling limiter for testing
	ipl := newIPLimiter(60, 2) // 60/min = 1/sec, burst 2

	ip := "10.0.0.1"
	limiter := ipl.getLimiter(ip)

	// Exhaust the burst
	if !limiter.Allow() {
		t.Error("first request should succeed")
	}
	if !limiter.Allow() {
		t.Error("second request should succeed (burst)")
	}
	if limiter.Allow() {
		t.Error("third request should fail (bucket empty)")
	}

	// Wait for tokens to refill (1 second = 1 token at 60/min)
	time.Sleep(1100 * time.Millisecond)

	// Should have 1 token now
	if !limiter.Allow() {
		t.Error("request after refill should succeed")
	}
}

// TestRateLimiter_ConcurrentCleanup verifies cleanup is safe under concurrency.
func TestRateLimiter_ConcurrentCleanup(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)

	handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Start multiple goroutines making requests
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check", nil)
			ctx := contextWithClientIP(req.Context(), "10.0.0.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
		close(done)
	}()

	// Run cleanup concurrently
	for i := 0; i < 10; i++ {
		rl.Cleanup()
	}

	<-done // Wait for requests to complete
}

// TestRateLimiter_MultipleEndpointTypes verifies different endpoint types are isolated.
func TestRateLimiter_MultipleEndpointTypes(t *testing.T) {
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)

	webHandler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	apiHandler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	bulkHandler := rl.BulkRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.0.0.50"

	// Exhaust web limit (10 requests)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), ip)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		webHandler.ServeHTTP(rec, req)
	}

	// Web should be limited
	req := httptest.NewRequest("GET", "/", nil)
	ctx := contextWithClientIP(req.Context(), ip)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	webHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("web: expected 429, got %d", rec.Code)
	}

	// But API should still work (different limit)
	req = httptest.NewRequest("GET", "/api/v1/check", nil)
	ctx = contextWithClientIP(req.Context(), ip)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	apiHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("api: expected 200, got %d", rec.Code)
	}

	// Exhaust bulk limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/bulk", nil)
		ctx := contextWithClientIP(req.Context(), ip)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		bulkHandler.ServeHTTP(rec, req)
	}

	// Bulk should be limited
	req = httptest.NewRequest("POST", "/api/v1/bulk", nil)
	ctx = contextWithClientIP(req.Context(), ip)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	bulkHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("bulk: expected 429, got %d", rec.Code)
	}

	// API should still work
	req = httptest.NewRequest("GET", "/api/v1/check", nil)
	ctx = contextWithClientIP(req.Context(), ip)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	apiHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("api after bulk limit: expected 200, got %d", rec.Code)
	}
}

// TestRateLimiter_ContextWithClientIP tests the helper function.
func TestRateLimiter_ContextWithClientIP(t *testing.T) {
	ctx := context.Background()
	ip := "192.168.1.1"

	ctx = contextWithClientIP(ctx, ip)
	retrieved := GetClientIP(ctx)

	if retrieved != ip {
		t.Errorf("expected IP %s, got %s", ip, retrieved)
	}
}

// TestIPLimiter_GetLimiterConcurrentCreation tests thread-safe limiter creation.
func TestIPLimiter_GetLimiterConcurrentCreation(t *testing.T) {
	ipl := newIPLimiter(60, 10)

	// Concurrently get limiters for the same IP
	done := make(chan *rate.Limiter, 100)
	for i := 0; i < 100; i++ {
		go func() {
			limiter := ipl.getLimiter("10.0.0.1")
			done <- limiter
		}()
	}

	// All should return the same pointer
	var first *rate.Limiter
	for i := 0; i < 100; i++ {
		limiter := <-done
		if first == nil {
			first = limiter
		} else if first != limiter {
			t.Error("concurrent getLimiter calls returned different limiters")
		}
	}

	// Only one entry should exist
	ipl.mu.RLock()
	count := len(ipl.limiters)
	ipl.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 limiter entry, got %d", count)
	}
}

// TestIPLimiter_CleanupRemovesOnlyFullBuckets verifies cleanup only removes entries with full buckets.
func TestIPLimiter_CleanupRemovesOnlyFullBuckets(t *testing.T) {
	ipl := newIPLimiter(60, 10)

	// Create limiters for 3 IPs
	for i := 1; i <= 3; i++ {
		ip := "10.0.0." + string(rune('0'+i))
		limiter := ipl.getLimiter(ip)
		// IP 1: full bucket (no consumption)
		// IP 2: partial bucket
		if i == 2 {
			limiter.Allow()
			limiter.Allow()
		}
		// IP 3: empty bucket
		if i == 3 {
			for j := 0; j < 10; j++ {
				limiter.Allow()
			}
		}
	}

	ipl.mu.RLock()
	initialCount := len(ipl.limiters)
	ipl.mu.RUnlock()

	if initialCount != 3 {
		t.Fatalf("expected 3 limiters, got %d", initialCount)
	}

	// Run cleanup
	ipl.cleanup()

	ipl.mu.RLock()
	afterCount := len(ipl.limiters)
	ipl.mu.RUnlock()

	// Only IP 1 should be removed (full bucket)
	if afterCount != 2 {
		t.Errorf("expected 2 limiters after cleanup, got %d", afterCount)
	}

	// Verify IP 1 is gone
	ipl.mu.RLock()
	_, exists := ipl.limiters["10.0.0.1"]
	ipl.mu.RUnlock()

	if exists {
		t.Error("expected IP 1 to be removed (full bucket)")
	}
}

// TestIPLimiter_ConcurrentAccess tests concurrent access to the IP limiter.
func TestIPLimiter_ConcurrentAccess(t *testing.T) {
	ipl := newIPLimiter(1000, 100) // High limits to avoid blocking

	var wg sync.WaitGroup

	// Concurrent reads and writes
	for i := 0; i < 100; i++ {
		wg.Add(2)

		// Writer goroutine
		go func(idx int) {
			defer wg.Done()
			ip := "10.0.0." + string(rune('0'+idx%10))
			limiter := ipl.getLimiter(ip)
			limiter.Allow()
		}(i)

		// Reader goroutine
		go func() {
			defer wg.Done()
			ipl.mu.RLock()
			_ = len(ipl.limiters)
			ipl.mu.RUnlock()
		}()
	}

	wg.Wait()
}
