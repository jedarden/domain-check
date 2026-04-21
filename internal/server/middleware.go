// Package server provides HTTP middleware for domain-check.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coding/domain-check/internal/config"
	"golang.org/x/time/rate"
)

// Context key types for type-safe context values.
type contextKey string

const (
	RequestIDKey contextKey = "requestID"
	ClientIPKey  contextKey = "clientIP"
)

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware in order, outer to inner.
// The first middleware is applied first and wraps all subsequent ones.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// GetClientIP retrieves the client IP from the context.
func GetClientIP(ctx context.Context) string {
	if v, ok := ctx.Value(ClientIPKey).(string); ok {
		return v
	}
	return ""
}

// RequestID adds a unique request ID to each request.
// If X-Request-Id header is already present, it uses that value.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = generateRequestID()
		}

		// Set in context and response header
		ctx := context.WithValue(r.Context(), RequestIDKey, rid)
		w.Header().Set("X-Request-Id", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID creates a random 16-character hex string.
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ClientIP extracts the client IP address for rate limiting.
// When trustProxy is true, it checks headers in order:
// CF-Connecting-IP → X-Real-IP → X-Forwarded-For → RemoteAddr
func ClientIP(trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ip string

			if trustProxy {
				// Check headers in priority order
				if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
					ip = cfIP
				} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
					ip = realIP
				} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					// Take the first IP in the chain
					ips := strings.Split(xff, ",")
					if len(ips) > 0 {
						ip = strings.TrimSpace(ips[0])
					}
				}
			}

			// Fall back to RemoteAddr
			if ip == "" {
				ip = extractIPFromRemoteAddr(r.RemoteAddr)
			}

			ctx := context.WithValue(r.Context(), ClientIPKey, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractIPFromRemoteAddr extracts the IP from host:port format.
func extractIPFromRemoteAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr // Return as-is if parsing fails
	}
	return host
}

// SecurityHeaders adds security-related headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'; connect-src 'self'")

		next.ServeHTTP(w, r)
	})
}

// BodyLimit returns a middleware that limits the request body size for
// write methods (POST, PUT, PATCH). Reads beyond the limit return an error.
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds CORS headers for cross-origin requests.
func CORS(cfg *config.Config) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Parse allowed origins
			allowedOrigins := strings.Split(cfg.CorsOrigins, ",")
			allowed := false
			for _, o := range allowedOrigins {
				o = strings.TrimSpace(o)
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Logging logs all requests.
func Logging(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(wrapped, r)

			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration", time.Since(start).Round(time.Microsecond),
				"request_id", GetRequestID(r.Context()),
				"client_ip", GetClientIP(r.Context()),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ipLimiter holds a per-IP rate limiter map with cleanup support.
type ipLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit // Requests per second
	burst    int        // Burst capacity
}

// newIPLimiter creates a new per-IP rate limiter.
func newIPLimiter(requestsPerMinute int, burst int) *ipLimiter {
	rps := float64(requestsPerMinute) / 60.0
	return &ipLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (ipl *ipLimiter) getLimiter(ip string) *rate.Limiter {
	ipl.mu.RLock()
	limiter, exists := ipl.limiters[ip]
	ipl.mu.RUnlock()

	if exists {
		return limiter
	}

	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = ipl.limiters[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(ipl.rate, ipl.burst)
	ipl.limiters[ip] = limiter
	return limiter
}

// cleanup removes entries with full buckets (no recent activity).
func (ipl *ipLimiter) cleanup() {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	for ip, limiter := range ipl.limiters {
		// If the limiter has all tokens available (bucket is full),
		// the IP hasn't made requests recently and can be removed.
		if limiter.Tokens() >= float64(ipl.burst) {
			delete(ipl.limiters, ip)
		}
	}
}

// RateLimiter provides per-IP rate limiting for different endpoint types.
type RateLimiter struct {
	log       *slog.Logger
	webLimit  *ipLimiter // 10 req/min
	apiLimit  *ipLimiter // 60 req/min
	bulkLimit *ipLimiter // 5 req/min
}

// NewRateLimiter creates a new rate limiter with per-IP limits.
// Web: 10 checks/min, API: 60/min, Bulk: 5 requests/min
// Burst values are set equal to the per-minute limit for user-friendly behavior.
func NewRateLimiter(log *slog.Logger) *RateLimiter {
	return &RateLimiter{
		log:       log,
		webLimit:  newIPLimiter(10, 10),  // 10/min, burst 10
		apiLimit:  newIPLimiter(60, 60),  // 60/min, burst 60
		bulkLimit: newIPLimiter(5, 5),    // 5/min, burst 5
	}
}

// WebRateLimit applies rate limiting for web UI endpoints (10 req/min).
func (rl *RateLimiter) WebRateLimit(next http.Handler) http.Handler {
	return rl.rateLimitHandler(rl.webLimit, next)
}

// APIRateLimit applies rate limiting for API endpoints (60 req/min).
func (rl *RateLimiter) APIRateLimit(next http.Handler) http.Handler {
	return rl.rateLimitHandler(rl.apiLimit, next)
}

// BulkRateLimit applies rate limiting for bulk endpoints (5 req/min).
func (rl *RateLimiter) BulkRateLimit(next http.Handler) http.Handler {
	return rl.rateLimitHandler(rl.bulkLimit, next)
}

// rateLimitHandler is the generic rate limiting handler.
func (rl *RateLimiter) rateLimitHandler(ipl *ipLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r.Context())
		if ip == "" {
			ip = extractIPFromRemoteAddr(r.RemoteAddr)
		}

		limiter := ipl.getLimiter(ip)

		if !limiter.Allow() {
			// Calculate retry-after based on the rate limit.
			// With rate R requests/sec, wait time is 1/R seconds.
			// We round up to at least 1 second.
			retryAfter := 1
			if ipl.rate > 0 {
				waitSeconds := 1.0 / float64(ipl.rate)
				if waitSeconds > 1 {
					retryAfter = int(waitSeconds + 0.5) // round to nearest
				}
			}

			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded","retry_after":` + strconv.Itoa(retryAfter) + `}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Cleanup removes stale IP entries from all limiters.
// Should be called periodically (e.g., every 10 minutes).
func (rl *RateLimiter) Cleanup() {
	rl.webLimit.cleanup()
	rl.apiLimit.cleanup()
	rl.bulkLimit.cleanup()
	rl.log.Debug("rate limiter cleanup completed")
}

// contextWithClientIP is a helper for tests to set client IP in context.
func contextWithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ClientIPKey, ip)
}
