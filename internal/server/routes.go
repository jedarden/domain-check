package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coding/domain-check/internal/config"
)

// Router creates and returns the main HTTP handler with all routes and middleware.
// The middleware chain is applied in order:
// 1. Request ID - add/generate unique request ID
// 2. Client IP - extract client IP for rate limiting
// 3. Logging - log all requests
// 4. Security Headers - CSP, X-Frame-Options, etc.
// 5. Rate Limit - per-IP rate limiting
// 6. CORS - cross-origin support for API
// 7. Handler - the actual route handler
func Router(cfg *config.Config, log *slog.Logger, rateLimiter *RateLimiter) http.Handler {
	mux := http.NewServeMux()

	// Register routes.
	registerRoutes(mux, cfg, log, rateLimiter)

	// Build middleware chain (applied in reverse order).
	// Outer to inner: RequestID -> ClientIP -> Logging -> SecurityHeaders -> RateLimit -> CORS -> Handler
	handler := Chain(mux,
		RequestID,
		ClientIP(cfg.TrustProxy),
		Logging(log),
		SecurityHeaders,
		CORS(cfg),
	)

	return handler
}

// registerRoutes adds all routes to the mux.
func registerRoutes(mux *http.ServeMux, cfg *config.Config, log *slog.Logger, rateLimiter *RateLimiter) {
	// Health check - no rate limiting.
	mux.HandleFunc("GET /health", healthHandler(log))

	// API routes with rate limiting.
	apiHandler := rateLimiter.APIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder - will be replaced with actual API handlers in Phase 2.
		writeJSONResponse(w, http.StatusOK, map[string]string{
			"message": "API endpoint - coming soon",
		})
	}))

	mux.Handle("GET /api/v1/check", apiHandler)
	mux.Handle("GET /api/v1/check/", apiHandler) // With trailing slash for path params
	mux.Handle("POST /api/v1/bulk", apiHandler)
	mux.Handle("GET /api/v1/tlds", apiHandler)

	// Metrics endpoint (if enabled).
	if cfg.Metrics {
		mux.HandleFunc("GET /metrics", metricsHandler(log))
	}

	// Web UI routes with web rate limiting.
	webHandler := rateLimiter.WebRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder - will be replaced with actual web handlers in Phase 3.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Domain Check</title></head>
<body>
<h1>Domain Check</h1>
<p>Web UI coming soon...</p>
</body>
</html>`))
	}))

	mux.Handle("GET /", webHandler)
	mux.Handle("GET /check", webHandler)
}

// healthHandler returns the server health status.
func healthHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "ok",
			"timestamp": r.Context().Value(RequestIDKey),
		}
		writeJSONResponse(w, http.StatusOK, health)
	}
}

// metricsHandler returns Prometheus metrics.
// Placeholder for Phase 2 - returns a simple response for now.
func metricsHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Placeholder metrics output.
		// Will be replaced with actual Prometheus metrics in Phase 2.
		w.Write([]byte(`# HELP domcheck_up Server health status
# TYPE domcheck_up gauge
domcheck_up 1
`))
	}
}

// writeJSONResponse writes a JSON response with the correct content type.
func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
