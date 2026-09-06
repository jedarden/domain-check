package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jedarden/domain-check/internal/config"
	"github.com/jedarden/domain-check/web"
)

// WatchHandlerInterface defines the interface for watch handlers (to avoid circular dependency).
type WatchHandlerInterface interface {
	RegisterWatch(http.ResponseWriter, *http.Request)
	CancelWatch(http.ResponseWriter, *http.Request)
}

// Router creates and returns the main HTTP handler with all routes and middleware.
// The middleware chain is applied in order:
// 1. Request ID - add/generate unique request ID
// 2. Client IP - extract client IP for rate limiting
// 3. Logging - log all requests
// 4. Security Headers - CSP, X-Frame-Options, etc.
// 5. Body Limit - cap request body size
// 6. Timeout - cancel long-running requests and reply 503
// 7. Recover - convert handler panics into 500 responses
// 8. CORS - cross-origin support for API
// 9. Handler - the actual route handler
//
// Recover sits inside Timeout because Timeout runs the handler in its own
// goroutine, and a panic in another goroutine can only be recovered by
// middleware that is itself inside that goroutine. BodyLimit sits outside
// Timeout for the mirror reason: an oversized body is rejected before the
// timeout middleware has spent a goroutine and a deadline on it.
func Router(cfg *config.Config, log *slog.Logger, rateLimiter *RateLimiter, ch DomainChecker, bootstrap BootstrapProvider, monitor *ServiceMonitor, metrics *Metrics, watchHandler WatchHandlerInterface) http.Handler {
	mux := http.NewServeMux()

	// Register routes.
	registerRoutes(mux, cfg, log, rateLimiter, ch, bootstrap, monitor, metrics, watchHandler)

	// Build middleware chain (applied in reverse order).
	// Outer to inner: RequestID -> ClientIP -> Logging -> Metrics -> SecurityHeaders -> BodyLimit -> Timeout -> Recover -> CORS -> Handler
	handler := Chain(mux,
		RequestID,
		ClientIP(cfg.TrustProxy()),
		Logging(log),
		MetricsMiddleware(metrics),
		SecurityHeaders,
		BodyLimit(64*1024), // 64 KB max body for POST endpoints
		Timeout(DefaultRequestTimeout, log, metrics),
		Recover(log, metrics),
		CORS(cfg),
	)

	return handler
}

// registerRoutes adds all routes to the mux.
func registerRoutes(mux *http.ServeMux, cfg *config.Config, log *slog.Logger, rateLimiter *RateLimiter, ch DomainChecker, bootstrap BootstrapProvider, monitor *ServiceMonitor, metrics *Metrics, watchHandler WatchHandlerInterface) {
	// Create handlers
	apiHandlers := NewAPIHandlers(ch, log, bootstrap, metrics, monitor)
	webHandlers := NewWebHandlers(ch, log)

	// Health check - no rate limiting.
	mux.HandleFunc("GET /health", healthHandler(log, bootstrap, monitor, metrics))

	// Static assets - no rate limiting, cached by browsers.
	mux.Handle("GET /static/", http.StripPrefix("/static/", StaticHandler()))

	// robots.txt at root (served from embedded static).
	mux.Handle("GET /robots.txt", robotsTxtHandler())

	// API routes with rate limiting
	mux.Handle("GET /api/v1/check", rateLimiter.APIRateLimit(http.HandlerFunc(apiHandlers.CheckHandler)))
	mux.Handle("GET /api/v1/check/", rateLimiter.APIRateLimit(http.HandlerFunc(apiHandlers.CheckHandler)))
	// Multi-TLD check endpoint
	mux.Handle("GET /api/v1/check/multi", rateLimiter.APIRateLimit(http.HandlerFunc(apiHandlers.MultiTLDHandler)))
	// Bulk check endpoint
	mux.Handle("POST /api/v1/bulk", rateLimiter.APIRateLimit(http.HandlerFunc(apiHandlers.BulkHandler)))
	// TLD list endpoint
	mux.Handle("GET /api/v1/tlds", rateLimiter.APIRateLimit(http.HandlerFunc(apiHandlers.TLDsHandler)))

	// Watch endpoints (if enabled)
	if cfg.EnableWatch() && watchHandler != nil {
		mux.Handle("POST /api/v1/watch", rateLimiter.APIRateLimit(http.HandlerFunc(watchHandler.RegisterWatch)))
		mux.Handle("DELETE /api/v1/watch/", rateLimiter.APIRateLimit(http.HandlerFunc(watchHandler.CancelWatch)))
	}

	// Metrics endpoint (if enabled).
	if cfg.Metrics() && metrics != nil {
		mux.Handle("GET /metrics", metrics.Handler())
	}

	// Web UI routes with web rate limiting.
	mux.Handle("GET /{$}", rateLimiter.WebRateLimit(http.HandlerFunc(webHandlers.IndexHandler())))
	mux.Handle("GET /check", rateLimiter.WebRateLimit(http.HandlerFunc(webHandlers.CheckHandler())))
}

const (
	healthyThreshold   = 48 * time.Hour     // Bootstrap age for "degraded" status
	unhealthyThreshold = 7 * 24 * time.Hour // Bootstrap age for "unhealthy" status (7 days)
)

// healthHandler returns the server health status with bootstrap age and uptime.
func healthHandler(log *slog.Logger, bootstrap BootstrapProvider, monitor *ServiceMonitor, metrics *Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		statusCode := http.StatusOK
		bootstrapAge := time.Duration(0)

		// Determine health status based on bootstrap age
		if bootstrap != nil {
			bootstrapAge = time.Since(bootstrap.Updated())

			// Update metrics with bootstrap age
			if metrics != nil {
				metrics.SetBootstrapAge(bootstrapAge.Seconds())
			}

			if bootstrapAge > unhealthyThreshold {
				status = "unhealthy"
				statusCode = http.StatusServiceUnavailable
			} else if bootstrapAge > healthyThreshold {
				status = "degraded"
			}
		}

		uptime := time.Duration(0)
		checksServed := int64(0)
		if monitor != nil {
			uptime = monitor.Uptime()
			checksServed = monitor.ChecksServed()
		}

		health := map[string]interface{}{
			"status":        status,
			"bootstrap_age": formatDuration(bootstrapAge),
			"uptime":        formatDuration(uptime),
			"checks_served": checksServed,
		}

		writeJSONResponse(w, statusCode, health)
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Hour).String()
}

// robotsTxtHandler serves the embedded robots.txt.
func robotsTxtHandler() http.HandlerFunc {
	data, _ := web.StaticFS()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		f, err := data.Open("robots.txt")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, "robots.txt", time.Time{}, f.(io.ReadSeeker))
	}
}

// writeJSONResponse writes a JSON response with the correct content type.
func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
