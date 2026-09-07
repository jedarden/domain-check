package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jedarden/domain-check/internal/watch"
)

// WatchHandler handles watch API requests.
type WatchHandler struct {
	manager *watch.Manager
	logger  *slog.Logger
}

// NewWatchHandler creates a new watch handler.
func NewWatchHandler(manager *watch.Manager, logger *slog.Logger) *WatchHandler {
	return &WatchHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRequest represents a request to register a new watch.
type RegisterRequest struct {
	Domain     string `json:"domain"`
	WebhookURL string `json:"webhook_url"`
}

// RegisterResponse represents the response from registering a watch.
type RegisterResponse struct {
	ID           string    `json:"id"`
	Domain       string    `json:"domain"`
	WebhookURL   string    `json:"webhook_url"`
	Secret       string    `json:"secret"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RegisterWatch handles POST /api/v1/watch
func (h *WatchHandler) RegisterWatch(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)

	// Parse request
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body")
		return
	}

	// Validate inputs
	if req.Domain == "" {
		h.writeError(w, http.StatusBadRequest, "missing_parameter", "domain is required")
		return
	}
	if req.WebhookURL == "" {
		h.writeError(w, http.StatusBadRequest, "missing_parameter", "webhook_url is required")
		return
	}

	// Register watch
	watchData, err := h.manager.Register(req.Domain, req.WebhookURL, clientIP)
	if err != nil {
		if err.Error() == "maximum watches per IP reached: 10 per 24 hours" {
			h.writeError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
			return
		}
		h.writeError(w, http.StatusBadRequest, "registration_failed", err.Error())
		return
	}

	// Return response
	resp := RegisterResponse{
		ID:         watchData.ID,
		Domain:     watchData.Domain,
		WebhookURL: watchData.WebhookURL,
		Secret:     watchData.Secret,
		CreatedAt:  watchData.CreatedAt,
		ExpiresAt:  watchData.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
	h.logger.Info("watch registered via API", "domain", req.Domain, "watch_id", watchData.ID, "client_ip", clientIP)
}

// CancelWatch handles DELETE /api/v1/watch/{id}
func (h *WatchHandler) CancelWatch(w http.ResponseWriter, r *http.Request) {
	// Extract watch ID from URL path
	id := r.URL.Path[len("/api/v1/watch/"):]
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "missing_parameter", "watch ID is required")
		return
	}

	// Get secret from query parameter or header
	secret := r.URL.Query().Get("secret")
	if secret == "" {
		secret = r.Header.Get("X-Watch-Secret")
	}
	if secret == "" {
		h.writeError(w, http.StatusUnauthorized, "missing_secret", "secret is required via ?secret= or X-Watch-Secret header")
		return
	}

	// Cancel watch
	if err := h.manager.Cancel(id, secret); err != nil {
		if err.Error() == "watch not found" {
			h.writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "invalid secret" {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "cancel_failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Info("watch canceled via API", "watch_id", id)
}

// writeError writes an error response.
func (h *WatchHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Message: message,
	})
}

// getClientIP extracts the client IP from the request, considering proxy headers.
func getClientIP(r *http.Request) string {
	// Check CF-Connecting-IP first (Cloudflare)
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	// Check X-Real-IP
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Check X-Forwarded-For (first IP)
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if len(ip) > 0 {
			// Simple extraction - for production you'd want more robust parsing
			for i, c := range ip {
				if c == ',' {
					return ip[:i]
				}
			}
			return ip
		}
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
