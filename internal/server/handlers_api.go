// Package server provides HTTP handlers for the API endpoints.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jedarden/domain-check/internal/bootstrap"
	"github.com/jedarden/domain-check/internal/checker"
	"github.com/jedarden/domain-check/internal/domain"
)

// DomainChecker is the interface for checking domain availability.
// It allows for mocking in tests.
type DomainChecker interface {
	Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error)
}

// BulkChecker is the interface for bulk domain availability checks.
type BulkChecker interface {
	CheckBulk(ctx context.Context, domains []string) *checker.BulkResult
}

// CombinedChecker combines single and bulk check capabilities.
type CombinedChecker interface {
	DomainChecker
	BulkChecker
}

// BootstrapProvider is the interface for accessing bootstrap data.
// It provides the TLD list and bootstrap update timestamp.
type BootstrapProvider interface {
	// ServerCount returns the number of TLDs currently mapped.
	ServerCount() int
	// Updated returns the time of the last successful bootstrap refresh.
	Updated() time.Time
	// TLDs returns all TLDs currently mapped.
	TLDs() []string
}

// APIHandlers provides HTTP handlers for the API endpoints.
type APIHandlers struct {
	checker   DomainChecker
	log       *slog.Logger
	bootstrap BootstrapProvider
	metrics   *Metrics
	monitor   *ServiceMonitor
}

// NewAPIHandlers creates a new APIHandlers instance.
// The bootstrap parameter is optional; pass nil if bootstrap data is not available.
// The metrics parameter is optional; pass nil to disable metrics recording.
// The monitor parameter is optional; pass nil to disable service monitoring.
func NewAPIHandlers(ch DomainChecker, log *slog.Logger, bootstrap BootstrapProvider, metrics *Metrics, monitor *ServiceMonitor) *APIHandlers {
	return &APIHandlers{
		checker:   ch,
		log:       log,
		bootstrap: bootstrap,
		metrics:   metrics,
		monitor:   monitor,
	}
}

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CheckHandler handles GET /api/v1/check?d=example.com
// It performs a domain availability check and returns the result as JSON.
// When the "tlds" query parameter is present (e.g. ?d=example&tlds=com,org,dev),
// it checks the same domain name across multiple TLDs in parallel.
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

	// If tlds parameter is present, delegate to multi-TLD handler.
	if tldsParam := r.URL.Query().Get("tlds"); tldsParam != "" {
		h.MultiTLDHandler(w, r)
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
		if errors.Is(err, bootstrap.ErrTLDNotFound) {
			writeAPIError(w, http.StatusBadRequest, "unsupported_tld", "No RDAP or WHOIS support for TLD: "+parsed.TLD)
			return
		}
		// Transient timeout — the upstream registry was too slow to respond.
		// Check both errors.Is (for properly wrapped errors) and string matching
		// as a fallback for errors wrapped in types without Unwrap().
		// This must be checked before context.Canceled since canceled contexts
		// can also have deadline exceeded messages.
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			h.log.Warn("domain check timed out", "domain", parsed.Domain, "error", err)
			w.Header().Set("Retry-After", "30")
			writeAPIError(w, http.StatusGatewayTimeout, "upstream_timeout", "Registry did not respond in time, please retry")
			return
		}
		// Context canceled (client disconnected).
		if errors.Is(err, context.Canceled) {
			return
		}
		// Other errors (e.g., SSRF violations, invalid input). These are genuine
		// failures, not transient network issues, so log at ERROR level.
		h.log.Error("domain check failed", "domain", parsed.Domain, "error", err)
		writeAPIError(w, http.StatusInternalServerError, "check_failed", "Domain availability check failed")
		return
	}

	// Record bulk check size metric (single check = size 1).
	if h.metrics != nil {
		h.metrics.RecordBulkCheck(1)
		h.metrics.AddChecksServed(1)
	}

	// Increment the service monitor counter (count all served requests, including errors).
	if h.monitor != nil {
		h.monitor.IncrementCheck()
	}

	// Return the result as JSON.
	writeJSONResponse(w, http.StatusOK, result)
}

// MultiTLDResult represents the result for a single TLD in a multi-TLD check.
type MultiTLDResult struct {
	Domain string                `json:"domain"`
	TLD    string                `json:"tld"`
	Result *domain.DomainResult  `json:"result,omitempty"`
	Error  string                `json:"error,omitempty"`
}

// MultiTLDResponse represents the response for a multi-TLD check.
type MultiTLDResponse struct {
	Name      string            `json:"name"`
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Duration  time.Duration     `json:"duration"`
	Results   []MultiTLDResult  `json:"results"`
}

// MultiTLDHandler handles GET /api/v1/check?d=example&tlds=com,org,dev,net,io
// It checks the same domain name across multiple TLDs in parallel.
func (h *APIHandlers) MultiTLDHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests.
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	// Extract domain name (without TLD) and TLD list from query parameters.
	nameParam := r.URL.Query().Get("d")
	if nameParam == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_parameter", "Domain name parameter 'd' is required")
		return
	}

	tldsParam := r.URL.Query().Get("tlds")
	if tldsParam == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_parameter", "TLD list parameter 'tlds' is required")
		return
	}

	// Parse TLD list.
	tlds := parseTLDList(tldsParam)
	if len(tlds) == 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_parameter", "At least one TLD is required")
		return
	}

	// Validate and normalize the domain name.
	// We normalize the name but don't require a TLD since we're adding it.
	name := normalizeDomainName(nameParam)
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_domain", "Domain name cannot be empty")
		return
	}
	if err := validateDomainName(name); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_domain", err.Error())
		return
	}

	// Construct full domain names.
	var domains []string
	for _, tld := range tlds {
		domains = append(domains, name+"."+tld)
	}

	// Check if we have a bulk checker available.
	bulkChecker, ok := h.checker.(BulkChecker)
	if !ok {
		// Fallback: check each domain sequentially (shouldn't happen in production).
		h.checkMultiTLDSequential(r.Context(), w, name, domains, tlds)
		return
	}

	// Perform bulk check with parallel execution.
	start := time.Now()
	bulkResult := bulkChecker.CheckBulk(r.Context(), domains)
	duration := time.Since(start)

	// Increment the service monitor counter (count all served requests).
	if h.monitor != nil {
		h.monitor.AddChecks(len(tlds))
	}

	// Build response.
	response := MultiTLDResponse{
		Name:     name,
		Total:    len(tlds),
		Duration: duration,
		Results:  make([]MultiTLDResult, 0, len(tlds)),
	}

	for i, fullDomain := range domains {
		result := MultiTLDResult{
			Domain: fullDomain,
			TLD:    tlds[i],
		}

		if errStr, hasErr := bulkResult.Errors[fullDomain]; hasErr {
			result.Error = errStr
			response.Failed++
		} else if res := bulkResult.Results[fullDomain]; res != nil {
			result.Result = res
			response.Succeeded++
		} else {
			result.Error = "no result returned"
			response.Failed++
		}

		response.Results = append(response.Results, result)
	}

	// Record bulk check size metric.
	if h.metrics != nil {
		h.metrics.RecordBulkCheck(len(tlds))
		h.metrics.AddChecksServed(response.Succeeded)
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// checkMultiTLDSequential handles multi-TLD checking when bulk checker is not available.
func (h *APIHandlers) checkMultiTLDSequential(ctx context.Context, w http.ResponseWriter, name string, domains, tlds []string) {
	start := time.Now()

	response := MultiTLDResponse{
		Name:    name,
		Total:   len(tlds),
		Results: make([]MultiTLDResult, 0, len(tlds)),
	}

	for i, fullDomain := range domains {
		result := MultiTLDResult{
			Domain: fullDomain,
			TLD:    tlds[i],
		}

		checkResult, err := h.checker.Check(ctx, fullDomain)
		if err != nil {
			result.Error = err.Error()
			response.Failed++
		} else {
			result.Result = checkResult
			response.Succeeded++
		}

		response.Results = append(response.Results, result)
	}

	response.Duration = time.Since(start)

	// Increment the service monitor counter (count all served requests).
	if h.monitor != nil {
		h.monitor.AddChecks(len(tlds))
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// parseTLDList parses a comma-separated TLD list and returns normalized TLDs.
func parseTLDList(tldsParam string) []string {
	parts := strings.Split(tldsParam, ",")
	var tlds []string
	for _, part := range parts {
		tld := strings.TrimSpace(part)
		tld = strings.TrimPrefix(tld, ".")
		tld = strings.ToLower(tld)
		if tld != "" {
			tlds = append(tlds, tld)
		}
	}
	return tlds
}

// normalizeDomainName normalizes a domain name (without TLD).
func normalizeDomainName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.TrimRight(name, ".")
	return name
}

// validateDomainName validates a domain name label (without TLD).
func validateDomainName(name string) error {
	if len(name) < 1 {
		return errors.New("domain name cannot be empty")
	}
	if len(name) > 63 {
		return errors.New("domain name exceeds 63 characters")
	}
	if name[0] == '-' {
		return errors.New("domain name cannot start with a hyphen")
	}
	if name[len(name)-1] == '-' {
		return errors.New("domain name cannot end with a hyphen")
	}
	for i, c := range name {
		if !isLDH(c) {
			return errors.New("domain name contains invalid character at position " + string(rune('0'+i)))
		}
	}
	return nil
}

// isLDH reports whether c is a valid LDH character: a-z, 0-9, or hyphen.
func isLDH(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

// isTimeoutError reports whether err represents a timeout from an upstream registry.
// It complements errors.Is(err, context.DeadlineExceeded) by string-matching common
// timeout messages that appear in errors wrapped in types without Unwrap() support
// (e.g. net/url.Error, custom RDAP client wrappers). It also walks the error chain
// via Unwrap() so that wrapped errors are still detected by their inner message.
//
// This function also checks for temporary network errors that should be treated as
// timeouts for the purposes of domain availability checking, such as connection
// timeouts and temporary DNS failures.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// First check if this is a context.DeadlineExceeded error type.
	// This handles cases where the error is context.DeadlineExceeded itself
	// or wraps it, ensuring consistent timeout detection.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	for e := err; e != nil; {
		msg := strings.ToLower(e.Error())
		for _, substr := range timeoutStrings {
			if strings.Contains(msg, substr) {
				return true
			}
		}
		// Check for temporary network errors (connection timeout, etc.)
		if netErr, ok := e.(interface{ Temporary() bool }); ok {
			if netErr.Temporary() {
				return true
			}
		}
		// Check for timeout errors specifically
		if netErr, ok := e.(interface{ Timeout() bool }); ok {
			if netErr.Timeout() {
				return true
			}
		}
		e = errors.Unwrap(e)
	}
	return false
}

// timeoutStrings is the set of lowercase substrings that indicate a timeout error.
var timeoutStrings = []string{
	"context deadline exceeded",
	"i/o timeout",
	"timeout",
	"client connection timed out",
	"tls handshake timeout",
}

// Bulk check constants.
const (
	// MaxBulkDomains is the maximum number of domains allowed in a single bulk request.
	MaxBulkDomains = 50
	// MaxBulkBodySize is the maximum size of the request body (64 KB).
	MaxBulkBodySize = 64 * 1024
)

// BulkRequest represents a bulk check request.
type BulkRequest struct {
	Domains []string `json:"domains"`
}

// BulkCheckResult represents the result for a single domain in a bulk check.
type BulkCheckResult struct {
	Domain string               `json:"domain"`
	Result *domain.DomainResult `json:"result,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// BulkCheckResponse represents the response for a bulk check.
type BulkCheckResponse struct {
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Duration  time.Duration     `json:"duration"`
	Results   []BulkCheckResult `json:"results"`
}

// BulkHandler handles POST /api/v1/bulk
// It performs parallel domain availability checks for multiple domains.
func (h *APIHandlers) BulkHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests.
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is supported")
		return
	}

	// Limit request body size to 64 KB.
	r.Body = http.MaxBytesReader(w, r.Body, MaxBulkBodySize)

	// Decode the request body.
	var req BulkRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		// Check for body too large error.
		if err.Error() == "http: request body too large" {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body exceeds 64 KB limit")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON in request body")
		return
	}

	// Validate domains array is not empty.
	if len(req.Domains) == 0 {
		writeAPIError(w, http.StatusBadRequest, "empty_array", "Domains array cannot be empty")
		return
	}

	// Validate max domains limit.
	if len(req.Domains) > MaxBulkDomains {
		writeAPIError(w, http.StatusBadRequest, "too_many_domains", "Maximum 50 domains allowed per request")
		return
	}

	// Record bulk check size metric.
	if h.metrics != nil {
		h.metrics.RecordBulkCheck(len(req.Domains))
	}

	// Validate and normalize all domains first.
	normalizedDomains := make([]string, 0, len(req.Domains))
	domainErrors := make(map[string]string)

	for _, d := range req.Domains {
		parsed, err := domain.Parse(d)
		if err != nil {
			var parseErr *domain.ParseError
			if errors.As(err, &parseErr) {
				domainErrors[d] = parseErr.Err
			} else {
				domainErrors[d] = err.Error()
			}
			continue
		}
		normalizedDomains = append(normalizedDomains, parsed.Domain)
	}

	// Check if we have a bulk checker available.
	bulkChecker, ok := h.checker.(BulkChecker)
	if !ok {
		// Fallback: check each domain sequentially (shouldn't happen in production).
		h.checkBulkSequential(r.Context(), w, normalizedDomains, domainErrors)
		return
	}

	// Perform bulk check with parallel execution.
	start := time.Now()
	bulkResult := bulkChecker.CheckBulk(r.Context(), normalizedDomains)
	duration := time.Since(start)

	// Build response.
	response := BulkCheckResponse{
		Total:    len(req.Domains),
		Duration: duration,
		Results:  make([]BulkCheckResult, 0, len(req.Domains)),
	}

	// Process results for each original domain.
	for _, originalDomain := range req.Domains {
		result := BulkCheckResult{Domain: originalDomain}

		// Check if we had a validation error for this domain.
		if errStr, hasErr := domainErrors[originalDomain]; hasErr {
			result.Error = errStr
			response.Failed++
			response.Results = append(response.Results, result)
			continue
		}

		// Get normalized domain for lookup.
		parsed, _ := domain.Parse(originalDomain)
		normalized := parsed.Domain

		// Check for errors from bulk check.
		if errStr, hasErr := bulkResult.Errors[normalized]; hasErr {
			result.Error = errStr
			response.Failed++
		} else if res := bulkResult.Results[normalized]; res != nil {
			result.Result = res
			response.Succeeded++
		} else {
			result.Error = "no result returned"
			response.Failed++
		}

		response.Results = append(response.Results, result)
	}

	// Record checks served metric.
	if h.metrics != nil {
		h.metrics.AddChecksServed(response.Succeeded)
	}

	// Increment the service monitor counter (count all served requests).
	if h.monitor != nil {
		h.monitor.AddChecks(len(req.Domains))
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// checkBulkSequential handles bulk checking when bulk checker is not available.
func (h *APIHandlers) checkBulkSequential(ctx context.Context, w http.ResponseWriter, domains []string, domainErrors map[string]string) {
	start := time.Now()

	response := BulkCheckResponse{
		Total:   len(domains) + len(domainErrors),
		Results: make([]BulkCheckResult, 0, len(domains)+len(domainErrors)),
	}

	// Process validation errors first.
	for domain, errStr := range domainErrors {
		response.Results = append(response.Results, BulkCheckResult{
			Domain: domain,
			Error:  errStr,
		})
		response.Failed++
	}

	// Process each valid domain.
	for _, normalizedDomain := range domains {
		result := BulkCheckResult{Domain: normalizedDomain}

		checkResult, err := h.checker.Check(ctx, normalizedDomain)
		if err != nil {
			result.Error = err.Error()
			response.Failed++
		} else {
			result.Result = checkResult
			response.Succeeded++
		}

		response.Results = append(response.Results, result)
	}

	response.Duration = time.Since(start)

	// Increment the service monitor counter (count all served requests).
	if h.monitor != nil {
		h.monitor.AddChecks(len(domains) + len(domainErrors))
	}

	writeJSONResponse(w, http.StatusOK, response)
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

// TLDsResponse represents the response for the TLD list endpoint.
type TLDsResponse struct {
	Count            int      `json:"count"`
	TLDs             []string `json:"tlds"`
	BootstrapUpdated string   `json:"bootstrap_updated"`
}

// TLDsHandler handles GET /api/v1/tlds
// It returns the list of supported TLDs with count and bootstrap update timestamp.
func (h *APIHandlers) TLDsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	if h.bootstrap == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "bootstrap_unavailable", "Bootstrap data is not available")
		return
	}

	count := h.bootstrap.ServerCount()
	updated := h.bootstrap.Updated()
	tlds := h.bootstrap.TLDs()

	sort.Strings(tlds)

	response := TLDsResponse{
		Count:            count,
		TLDs:             tlds,
		BootstrapUpdated: updated.UTC().Format(time.RFC3339),
	}

	writeJSONResponse(w, http.StatusOK, response)
}

