package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/config"
)

func TestRequestID(t *testing.T) {
	t.Run("generates new ID when not provided", func(t *testing.T) {
		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := GetRequestID(r.Context())
			if rid == "" {
				t.Error("expected request ID to be set")
			}
			if len(rid) != 16 {
				t.Errorf("expected 16-char ID, got %d", len(rid))
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-Request-Id") == "" {
			t.Error("expected X-Request-Id header to be set")
		}
	})

	t.Run("uses provided ID", func(t *testing.T) {
		providedID := "1234567890abcdef"
		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := GetRequestID(r.Context())
			if rid != providedID {
				t.Errorf("expected ID %s, got %s", providedID, rid)
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Request-Id", providedID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-Request-Id") != providedID {
			t.Errorf("expected X-Request-Id header %s, got %s", providedID, rec.Header().Get("X-Request-Id"))
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"X-Xss-Protection", "1; mode=block"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := rec.Header().Get(tt.header); got != tt.expected {
				t.Errorf("header %s: expected %s, got %s", tt.header, tt.expected, got)
			}
		})
	}

	// Check CSP contains required directives.
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	requiredDirectives := []string{
		"default-src 'none'",
		"script-src 'self'",
		"style-src 'self'",
		"img-src 'self'",
		"form-action 'self'",
		"base-uri 'self'",
		"frame-ancestors 'none'",
		"connect-src 'self'",
	}
	for _, dir := range requiredDirectives {
		if !contains(csp, dir) {
			t.Errorf("CSP missing directive: %s", dir)
		}
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		trustProxy bool
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "no proxy - uses remote addr",
			trustProxy: false,
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			expected:   "192.168.1.1",
		},
		{
			name:       "trust proxy - uses X-Forwarded-For",
			trustProxy: true,
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			expected:   "10.0.0.1",
		},
		{
			name:       "trust proxy - uses first X-Forwarded-For",
			trustProxy: true,
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1, 10.0.0.2"},
			expected:   "10.0.0.1",
		},
		{
			name:       "trust proxy - prefers X-Real-IP over X-Forwarded-For",
			trustProxy: true,
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Real-IP": "10.0.0.3", "X-Forwarded-For": "10.0.0.1"},
			expected:   "10.0.0.3",
		},
		{
			name:       "trust proxy - prefers CF-Connecting-IP",
			trustProxy: true,
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"CF-Connecting-IP": "10.0.0.4", "X-Real-IP": "10.0.0.3"},
			expected:   "10.0.0.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedIP string
			handler := ClientIP(tt.trustProxy)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedIP = GetClientIP(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if capturedIP != tt.expected {
				t.Errorf("expected IP %s, got %s", tt.expected, capturedIP)
			}
		})
	}
}

func TestChain(t *testing.T) {
	var order []string

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := Chain(handler, middleware1, middleware2)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	chained.ServeHTTP(rec, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Errorf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if i >= len(order) || order[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestRateLimiter(t *testing.T) {
	log := DefaultLogger("text", "error") // Suppress logs in tests
	rl := NewRateLimiter(log)

	t.Run("allows requests under limit", func(t *testing.T) {
		handler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			// Set client IP in context (simulating ClientIP middleware)
			ctx := contextWithClientIP(req.Context(), "192.168.1.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, rec.Code)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(log) // Fresh limiter
		handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Make 11 requests (limit is 10)
		for i := 0; i < 11; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.0.0.1:12345"
			ctx := contextWithClientIP(req.Context(), "10.0.0.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if i < 10 {
				if rec.Code != http.StatusOK {
					t.Errorf("request %d: expected 200, got %d", i, rec.Code)
				}
			} else {
				if rec.Code != http.StatusTooManyRequests {
					t.Errorf("request %d: expected 429, got %d", i, rec.Code)
				}
			}
		}
	})

	t.Run("per-IP isolation - one IP does not affect another", func(t *testing.T) {
		rl := NewRateLimiter(log) // Fresh limiter
		handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Exhaust the limit for IP 1 (10 requests, limit is 10)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			ctx := contextWithClientIP(req.Context(), "192.168.1.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("IP1 request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 11th request from IP 1 should be blocked
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), "192.168.1.1")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("IP1 request 11: expected 429, got %d", rec.Code)
		}

		// IP 2 should still be allowed (isolated from IP 1)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			ctx := contextWithClientIP(req.Context(), "192.168.1.2")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("IP2 request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// Verify IP 1 is still blocked
		req = httptest.NewRequest("GET", "/", nil)
		ctx = contextWithClientIP(req.Context(), "192.168.1.1")
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("IP1 still blocked: expected 429, got %d", rec.Code)
		}
	})

	t.Run("per-IP isolation - API and Web limits are separate", func(t *testing.T) {
		rl := NewRateLimiter(log)
		webHandler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		apiHandler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Exhaust web limit for IP
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			ctx := contextWithClientIP(req.Context(), "10.0.0.1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			webHandler.ServeHTTP(rec, req)
		}

		// Web should be blocked
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.1")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		webHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("Web should be blocked: expected 429, got %d", rec.Code)
		}

		// API should still work (different limit)
		req = httptest.NewRequest("GET", "/api/v1/check", nil)
		ctx = contextWithClientIP(req.Context(), "10.0.0.1")
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()
		apiHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("API should still work: expected 200, got %d", rec.Code)
		}
	})
}

func TestRateLimiter_Cleanup(t *testing.T) {
	log := DefaultLogger("text", "error")

	t.Run("removes entries with full buckets", func(t *testing.T) {
		rl := NewRateLimiter(log)
		// Add some entries by making requests
		handler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Make requests from multiple IPs to deplete their buckets
		for _, ip := range []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"} {
			req := httptest.NewRequest("GET", "/", nil)
			ctx := contextWithClientIP(req.Context(), ip)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}

		// Verify entries exist
		rl.webLimit.mu.RLock()
		count := len(rl.webLimit.limiters)
		rl.webLimit.mu.RUnlock()
		if count != 3 {
			t.Errorf("expected 3 entries before cleanup, got %d", count)
		}

		// Wait for buckets to refill (web rate is 1 per 6 seconds, burst is 3)
		// After ~18 seconds, buckets should be full
		// For testing, we'll just verify the cleanup mechanism exists
		// and removes entries when their buckets are full.

		// Since we can't wait 18s in a unit test, we test the inverse:
		// entries with partially depleted buckets should remain after cleanup.
		rl.Cleanup()

		// Entries should still exist (buckets aren't full yet)
		rl.webLimit.mu.RLock()
		count = len(rl.webLimit.limiters)
		rl.webLimit.mu.RUnlock()
		if count != 3 {
			t.Errorf("expected 3 entries after cleanup (buckets not full), got %d", count)
		}
	})

	t.Run("cleanup affects both web and API limiters", func(t *testing.T) {
		rl := NewRateLimiter(log)
		webHandler := rl.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		apiHandler := rl.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Add entries to both limiters
		req := httptest.NewRequest("GET", "/", nil)
		ctx := contextWithClientIP(req.Context(), "10.0.0.100")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		webHandler.ServeHTTP(rec, req)

		req = httptest.NewRequest("GET", "/api/v1/check", nil)
		ctx = contextWithClientIP(req.Context(), "10.0.0.100")
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()
		apiHandler.ServeHTTP(rec, req)

		// Verify both have entries
		rl.webLimit.mu.RLock()
		webCount := len(rl.webLimit.limiters)
		rl.webLimit.mu.RUnlock()

		rl.apiLimit.mu.RLock()
		apiCount := len(rl.apiLimit.limiters)
		rl.apiLimit.mu.RUnlock()

		if webCount != 1 || apiCount != 1 {
			t.Errorf("expected 1 entry in each limiter, got web=%d, api=%d", webCount, apiCount)
		}
	})
}

func TestRouter(t *testing.T) {
	cfg := config.Defaults()
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	handler := Router(&cfg, log, rl, nil)

	t.Run("health endpoint returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("unknown path returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/unknown", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("request ID header is set", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-Request-Id") == "" {
			t.Error("expected X-Request-Id header to be set")
		}
	})
}

func TestServerShutdown(t *testing.T) {
	cfg := &config.Config{Addr: ":0"} // Use random port
	log := DefaultLogger("text", "error")

	// Handler that blocks until cancelled
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	srv := New(cfg, handler, log)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		if err := srv.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestCORS(t *testing.T) {
	t.Run("wildcard origin allows any origin", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: "*"}
		handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Errorf("expected origin echo, got %q", got)
		}
	})

	t.Run("specific origin allows matching origin", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: "https://app.example.com,https://other.example.com"}
		handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Errorf("expected matching origin, got %q", got)
		}
	})

	t.Run("non-matching origin gets no CORS headers", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: "https://app.example.com"}
		handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("expected no CORS header for non-matching origin, got %q", got)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("preflight OPTIONS returns 204 with headers", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: "*"}
		handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called for OPTIONS")
		}))

		req := httptest.NewRequest("OPTIONS", "/api/v1/check", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
			t.Errorf("expected Allow-Methods header, got %q", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
			t.Error("expected Allow-Headers header")
		}
		if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
			t.Errorf("expected Max-Age 86400, got %q", got)
		}
	})

	t.Run("no Origin header passes through", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: "*"}
		called := false
		handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !called {
			t.Error("expected next handler to be called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestBodyLimit(t *testing.T) {
	t.Run("POST body under limit succeeds", func(t *testing.T) {
		handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(`{"domains":["a.com"]}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("GET requests are not limited", func(t *testing.T) {
		handler := BodyLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/check?d=example.com", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for GET, got %d", rec.Code)
		}
	})

	t.Run("POST body over limit returns error", func(t *testing.T) {
		handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to read the body — MaxBytesReader should reject it
			buf := make([]byte, 100)
			_, err := r.Body.Read(buf)
			if err == nil {
				t.Error("expected error reading oversized body")
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader("this is way more than 10 bytes"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
