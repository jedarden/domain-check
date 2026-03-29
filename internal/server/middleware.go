package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coding/domain-check/internal/config"
	"github.com/rs/cors"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// RequestIDKey is the context key for the request ID.
	RequestIDKey contextKey = "requestID"
	// ClientIPKey is the context key for the client IP.
	ClientIPKey contextKey = "clientIP"
)

// Chain applies a sequence of middleware to a handler.
// Middleware is applied in order: first middleware is outermost.
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// RequestID middleware adds a unique request ID to each request.
// If X-Request-Id header is present, it uses that; otherwise generates a new ID.
// The ID is added to the response headers and stored in the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = generateRequestID()
		}
		w.Header().Set("X-Request-Id", rid)
		ctx := r.Context()
		ctx = contextWithRequestID(ctx, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID creates a random 16-character hex string.
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID on crypto/rand failure.
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// contextWithRequestID returns a context with the request ID.
func contextWithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, RequestIDKey, rid)
}

// GetRequestID extracts the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if rid, ok := ctx.Value(RequestIDKey).(string); ok {
		return rid
	}
	return ""
}

// Logging middleware logs each request with method, path, status, and duration.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rid := GetRequestID(r.Context())

			// Wrap ResponseWriter to capture status code.
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			clientIP := GetClientIP(r.Context())

			log.Info("request",
				"request_id", rid,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration_ms", duration.Milliseconds(),
				"remote_ip", clientIP,
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before delegating to the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders middleware adds security-related headers to all responses.
// Headers: Content-Security-Policy, X-Content-Type-Options, X-Frame-Options, Referrer-Policy.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy - strict default, allow self for scripts/styles.
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// ClientIP middleware extracts the client IP from the request.
// It respects X-Forwarded-For and X-Real-IP headers when trustProxy is true.
// The IP is stored in the request context.
func ClientIP(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r, trustProxy)
			ctx := contextWithClientIP(r.Context(), ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractClientIP determines the client IP address from the request.
func extractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Try CF-Connecting-IP first (Cloudflare).
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}
		// Try X-Real-IP.
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return ip
		}
		// Try first entry of X-Forwarded-For.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}
	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// contextWithClientIP returns a context with the client IP.
func contextWithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ClientIPKey, ip)
}

// GetClientIP extracts the client IP from the context.
func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(ClientIPKey).(string); ok {
		return ip
	}
	return ""
}

// CORS middleware handles Cross-Origin Resource Sharing.
// Configured via the CorsOrigins config setting.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	origins := strings.Split(cfg.CorsOrigins, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300, // 5 minutes
	})

	return c.Handler
}

// RateLimit middleware implements per-IP rate limiting.
// Web UI: 10 checks/minute, API: 60 checks/minute.
type RateLimiter struct {
	webLimit *ipLimiter
	apiLimit *ipLimiter
	log      *slog.Logger
}

// ipLimiter tracks per-IP rate limits.
type ipLimiter struct {
	ips map[string]*clientState
}

// clientState tracks request counts for an IP.
type clientState struct {
	count     int
	resetAt   time.Time
}

// NewRateLimiter creates a new rate limiter with the specified limits.
func NewRateLimiter(log *slog.Logger) *RateLimiter {
	return &RateLimiter{
		webLimit: &ipLimiter{ips: make(map[string]*clientState)},
		apiLimit: &ipLimiter{ips: make(map[string]*clientState)},
		log:      log,
	}
}

// WebRateLimit middleware enforces the web UI rate limit (10 checks/minute).
func (rl *RateLimiter) WebRateLimit(next http.Handler) http.Handler {
	return rl.rateLimitMiddleware(next, rl.webLimit, 10)
}

// APIRateLimit middleware enforces the API rate limit (60 checks/minute).
func (rl *RateLimiter) APIRateLimit(next http.Handler) http.Handler {
	return rl.rateLimitMiddleware(next, rl.apiLimit, 60)
}

// rateLimitMiddleware enforces a per-IP rate limit.
func (rl *RateLimiter) rateLimitMiddleware(next http.Handler, limiter *ipLimiter, maxReqs int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r.Context())
		if ip == "" {
			ip = r.RemoteAddr
		}

		allowed, retryAfter := limiter.check(ip, maxReqs)
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited",
				fmt.Sprintf("Rate limit exceeded. Try again in %d seconds.", retryAfter), retryAfter)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// check returns false if the IP has exceeded the limit, and the seconds until reset.
func (l *ipLimiter) check(ip string, maxReqs int) (allowed bool, retryAfter int) {
	now := time.Now()

	state, ok := l.ips[ip]
	if !ok || now.After(state.resetAt) {
		// New window.
		l.ips[ip] = &clientState{
			count:   1,
			resetAt: now.Add(time.Minute),
		}
		return true, 0
	}

	if state.count >= maxReqs {
		retryAfter := int(time.Until(state.resetAt).Seconds())
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	state.count++
	return true, 0
}

// Cleanup removes stale entries from the rate limiters.
// Should be called periodically to prevent memory growth.
func (rl *RateLimiter) Cleanup() {
	now := time.Now()
	cleanupLimiter(rl.webLimit, now)
	cleanupLimiter(rl.apiLimit, now)
}

func cleanupLimiter(l *ipLimiter, now time.Time) {
	for ip, state := range l.ips {
		if now.After(state.resetAt) {
			delete(l.ips, ip)
		}
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, errorCode, message string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":"%s","message":"%s","retry_after":%d}`, errorCode, message, retryAfter)
}
