package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
