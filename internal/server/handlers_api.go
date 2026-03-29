// Package server provides HTTP handlers for the API endpoints.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/coding/domain-check/internal/checker"
	"github.com/coding/domain-check/internal/domain"
)

// DomainChecker is the interface for checking domain availability.
// It allows for mocking in tests.
type DomainChecker interface {
	Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error)
}

// APIHandlers provides HTTP handlers for the API endpoints.
type APIHandlers struct {
	checker DomainChecker
	log     *slog.Logger
}

// NewAPIHandlers creates a new APIHandlers instance.
func NewAPIHandlers(ch DomainChecker, log *slog.Logger) *APIHandlers {
	return &APIHandlers{
		checker: ch,
		log:     log,
	}
}

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CheckHandler handles GET /api/v1/check?d=example.com
// It performs a domain availability check and returns the result as JSON.
func (h *APIHandlers) CheckHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests.
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	// Extract domain from query parameter.
	domainParam := r.URL.Query().Get("d")
	if domainParam == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_parameter", "Domain parameter 'd' is required")
		return
	}

	// Parse and validate the domain.
	parsed, err := domain.Parse(domainParam)
	if err != nil {
		var parseErr *domain.ParseError
		if errors.As(err, &parseErr) {
			writeAPIError(w, http.StatusBadRequest, "invalid_domain", parseErr.Err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_domain", err.Error())
		return
	}

	// Perform the domain check.
	result, err := h.checker.Check(r.Context(), parsed.Domain)
	if err != nil {
		// Check for unsupported TLD error.
		if errors.Is(err, checker.ErrTLDNotFound) {
			writeAPIError(w, http.StatusBadRequest, "unsupported_tld", "No RDAP or WHOIS support for TLD: "+parsed.TLD)
			return
		}
		// Other errors.
		h.log.Error("domain check failed", "domain", parsed.Domain, "error", err)
		writeAPIError(w, http.StatusInternalServerError, "check_failed", "Domain availability check failed")
		return
	}

	// Return the result as JSON.
	writeJSONResponse(w, http.StatusOK, result)
}

// writeAPIError writes a JSON error response with the given status code and error details.
func writeAPIError(w http.ResponseWriter, status int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error:   errorCode,
		Message: message,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

