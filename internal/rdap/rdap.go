// Package rdap provides RDAP client functionality for domain availability checking.
package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jedarden/domain-check/internal/bootstrap"
	"github.com/jedarden/domain-check/internal/domain"
	"github.com/jedarden/domain-check/internal/httpclient"
	"github.com/jedarden/domain-check/internal/ratelimit"
	"github.com/jedarden/domain-check/internal/resilience"
)

// RDAP errors.
var (
	ErrRateLimited    = errors.New("RDAP rate limited")
	ErrRegistryError  = errors.New("RDAP registry error")
	ErrConnection     = errors.New("RDAP connection error")
	ErrInvalidRDAPURL = errors.New("invalid RDAP URL")
	// ErrRegistryUnavailable reports that the RDAP registry could not be
	// reached: transient failures persisted through the retry budget, or the
	// registry's circuit breaker is open. This is an outage of the
	// dependency — the answer may change once it recovers — and is reported
	// differently from a permanent registry answer such as 404 (available).
	ErrRegistryUnavailable = errors.New("RDAP registry temporarily unavailable")
)

// RDAPClient queries RDAP registry servers for domain availability.
type RDAPClient struct {
	httpClient *http.Client
	bootstrap  *bootstrap.Manager
	ratelimit  *ratelimit.RateLimiter
	allowlist  AllowList
	userAgent  string
	metrics    RDAPMetrics
	breakers   *resilience.BreakerSet // one circuit breaker per registry host
	retryCfg   resilience.RetryConfig // policy for the outbound registry call
}

// RDAPClientConfig holds configuration for the RDAP client.
type RDAPClientConfig struct {
	HTTPClient *http.Client
	Bootstrap  *bootstrap.Manager
	RateLimit  *ratelimit.RateLimiter
	AllowList  AllowList
	UserAgent  string
	Metrics    RDAPMetrics
}

// RDAPMetrics records metrics for RDAP requests.
type RDAPMetrics interface {
	RecordRDAPRequest(registry, status string, durationSeconds float64)
}

// AllowList manages a set of allowed RDAP base URLs.
type AllowList interface {
	Allowed(url string) bool
}

// NewRDAPClient creates a new RDAP client.
func NewRDAPClient(cfg RDAPClientConfig) *RDAPClient {
	return &RDAPClient{
		httpClient: cfg.HTTPClient,
		bootstrap:  cfg.Bootstrap,
		ratelimit:  cfg.RateLimit,
		allowlist:  cfg.AllowList,
		userAgent:  cfg.UserAgent,
		metrics:    cfg.Metrics,
		breakers:   resilience.NewBreakerSet(resilience.BreakerConfig{}),
		retryCfg:   resilience.DefaultInteractiveRetryConfig(),
	}
}

// retryConfig returns the client's retry policy, falling back to the
// interactive default when unset (a client not built by NewRDAPClient).
func (c *RDAPClient) retryConfig() resilience.RetryConfig {
	if c.retryCfg.BaseDelay <= 0 {
		return resilience.DefaultInteractiveRetryConfig()
	}
	return c.retryCfg
}

// breakerFor returns the circuit breaker for a registry host, creating the
// set on first use so any client — including a zero-value one — is gated.
func (c *RDAPClient) breakerFor(registry string) *resilience.CircuitBreaker {
	if c.breakers == nil {
		c.breakers = resilience.NewBreakerSet(resilience.BreakerConfig{})
	}
	return c.breakers.Get(registry)
}

// Check queries the RDAP server for the given domain and returns the result.
// The domain must be a normalized, validated domain name (lowercase, ASCII).
func (c *RDAPClient) Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	start := time.Now()

	// Sanitize domain input before any network request.
	if err := httpclient.SanitizeDomain(normalizedDomain); err != nil {
		return nil, err
	}

	// Extract TLD to lookup RDAP server.
	parts := strings.Split(normalizedDomain, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid domain format: %s", normalizedDomain)
	}
	tld := parts[len(parts)-1]

	// Get RDAP server URL from bootstrap.
	rdapBase, err := c.bootstrap.Lookup(tld)
	if err != nil {
		return nil, err
	}

	// Build RDAP query URL.
	rdapURL := buildRDAPURL(rdapBase, normalizedDomain)

	// Validate URL is in allowlist.
	if c.allowlist != nil && !c.allowlist.Allowed(rdapURL) {
		return nil, fmt.Errorf("%w: %s", ErrNotInAllowlist, rdapURL)
	}

	// Extract registry host for rate limiting.
	registry := ExtractRegistryHost(rdapBase)

	var resp *http.Response
	var rdapErr error

	// Execute with rate limiting and retry.
	resp, rdapErr = c.ratelimit.Acquire(ctx, registry, func() (*http.Response, error) {
		return c.doRequest(ctx, rdapURL, registry)
	})

	// Record RDAP request metrics if metrics is available
	if c.metrics != nil {
		duration := time.Since(start).Seconds()
		status := "success"
		if rdapErr != nil {
			switch {
			case errors.Is(rdapErr, context.DeadlineExceeded):
				status = "timeout"
			case errors.Is(rdapErr, context.Canceled):
				status = "canceled"
			case errors.Is(rdapErr, ratelimit.ErrServiceBusy) || strings.Contains(rdapErr.Error(), "HTTP 429"):
				status = "rate_limited"
			case errors.Is(rdapErr, resilience.ErrCircuitOpen) || resilience.IsTransient(rdapErr):
				status = "registry_unavailable"
			default:
				status = "error"
			}
		} else if resp != nil {
			switch resp.StatusCode {
			case http.StatusOK:
				status = "success"
			case http.StatusNotFound:
				status = "not_found"
			case http.StatusTooManyRequests:
				status = "rate_limited"
			case http.StatusBadRequest:
				status = "bad_request"
			default:
				status = fmt.Sprintf("http_%d", resp.StatusCode)
			}
		}
		c.metrics.RecordRDAPRequest(registry, status, duration)
	}

	if rdapErr != nil {
		// Propagate context errors (timeout, cancellation) so the caller
		// can return appropriate HTTP status codes (504, no response).
		// These are transport-level failures, not domain-availability results.
		if errors.Is(rdapErr, context.DeadlineExceeded) || errors.Is(rdapErr, context.Canceled) {
			return nil, rdapErr
		}
		// Check for rate limit exhaustion. Matched on the "HTTP 429" message
		// format the ratelimit and retry layers emit — a bare "429" would
		// also match a registry host whose port happens to contain it.
		if errors.Is(rdapErr, ratelimit.ErrServiceBusy) || strings.Contains(rdapErr.Error(), "HTTP 429") {
			return &domain.DomainResult{
				Domain:     normalizedDomain,
				TLD:        tld,
				CheckedAt:  time.Now(),
				Source:     domain.SourceRDAP,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      ErrRateLimited.Error(),
			}, nil
		}
		// Registry outage: the retry budget ran out or the circuit breaker is
		// open. Surface it as an availability answer that says the registry —
		// not the domain — is the problem, so the server keeps answering
		// requests while a registry is down.
		if errors.Is(rdapErr, resilience.ErrCircuitOpen) || resilience.IsTransient(rdapErr) {
			return &domain.DomainResult{
				Domain:     normalizedDomain,
				TLD:        tld,
				CheckedAt:  time.Now(),
				Source:     domain.SourceRDAP,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%s: %v", ErrRegistryUnavailable, rdapErr),
			}, nil
		}
		// Connection or other errors.
		return &domain.DomainResult{
			Domain:     normalizedDomain,
			TLD:        tld,
			CheckedAt:  time.Now(),
			Source:     domain.SourceRDAP,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      rdapErr.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Parse response based on status code.
	result := c.parseResponse(resp, normalizedDomain, tld, start)

	return result, nil
}

// doRequest performs the HTTP GET request to the RDAP server, retrying
// transient registry failures with exponential backoff and gating the
// registry behind its circuit breaker. Only transient conditions are retried —
// transport errors and 408/429/502/503/504 answers; a definitive registry
// answer (200 registered, 404 available, 400 malformed) is returned on the
// first attempt.
//
// The retry policy is the interactive one, sized to finish inside the server's
// 30s request timeout. Once a registry's breaker is open the call fails fast
// with an error wrapping ErrCircuitOpen — no network I/O — so an outage turns
// into a clear, immediate "registry unavailable" answer instead of a storm of
// doomed requests.
func (c *RDAPClient) doRequest(ctx context.Context, url, registry string) (*http.Response, error) {
	breaker := c.breakerFor(registry)

	var resp *http.Response
	err := resilience.Do(ctx, c.retryConfig(), func(int) error {
		// An open circuit fails before any network I/O. Its message already
		// names the registry (the breaker is keyed by host); classified
		// permanent so the retry loop does not burn its budget on a
		// dependency we have deliberately stopped asking.
		if err := breaker.Allow(); err != nil {
			return resilience.WrapPermanent(err)
		}

		r, err := c.send(ctx, url)
		if err != nil {
			breaker.Record(err)
			if resilience.IsPermanent(err) {
				return err
			}
			// Unclassified transport errors are treated as transient: a
			// dropped connection or a refused dial is usually a blip.
			return resilience.WrapTransient(fmt.Errorf("registry %s: %w", registry, err))
		}
		if resilience.IsTransientStatus(r.StatusCode) {
			// The registry answered but with a retry-worthy status. Drain and
			// close so the connection returns to the pool, then retry.
			drainAndClose(r)
			err := fmt.Errorf("registry %s: HTTP %d (%s)", registry, r.StatusCode, resilience.StatusReason(r.StatusCode))
			breaker.Record(err)
			return resilience.WrapTransient(err)
		}
		// A definitive answer — including 404 (available) and 400. The
		// dependency is healthy, so the breaker must not count it as a
		// failure.
		breaker.Record(nil)
		resp = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// send issues a single RDAP HTTP GET — one attempt of doRequest's retry loop.
func (c *RDAPClient) send(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		// A URL that cannot be parsed will not parse any better on retry.
		return nil, resilience.WrapPermanent(fmt.Errorf("create request: %w", err))
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")

	return c.httpClient.Do(req)
}

// drainAndClose discards a response body and releases its connection.
func drainAndClose(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
	}
}

// parseResponse interprets the RDAP HTTP response.
func (c *RDAPClient) parseResponse(resp *http.Response, domainName, tld string, start time.Time) *domain.DomainResult {
	result := &domain.DomainResult{
		Domain:     domainName,
		TLD:        tld,
		CheckedAt:  time.Now(),
		Source:     domain.SourceRDAP,
		DurationMs: time.Since(start).Milliseconds(),
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// Domain is registered. Parse the response body.
		result.Available = false
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
		if err != nil {
			result.Error = fmt.Sprintf("read response body: %v", err)
			return result
		}
		reg := parseRDAPBody(body)
		result.Registration = reg

	case http.StatusNotFound:
		// Domain is available.
		result.Available = true

	case http.StatusTooManyRequests:
		// Rate limited by registry.
		result.Error = ErrRateLimited.Error()

	case http.StatusBadRequest:
		// Some registries return 400 for invalid format, treat as unavailable
		// but we can't be certain. Log as registry error.
		result.Available = true // Conservative: assume available
		result.Error = ErrRegistryError.Error()

	default:
		// Other status codes (5xx, etc.) are errors.
		result.Error = fmt.Sprintf("%s: HTTP %d", ErrRegistryError, resp.StatusCode)
	}

	return result
}

// buildRDAPURL constructs the RDAP query URL for a domain.
func buildRDAPURL(baseURL, domain string) string {
	base := strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/domain/%s", base, domain)
}

// ErrNotInAllowlist is returned when a URL is not in the RDAP allowlist.
var ErrNotInAllowlist = errors.New("URL not in RDAP allowlist")

// ExtractRegistryHost extracts the hostname from an RDAP base URL for rate limiting.
func ExtractRegistryHost(baseURL string) string {
	// Simple extraction: get the host from the URL.
	// Assumes baseURL is like "https://rdap.verisign.com/com/v1/"
	if idx := strings.Index(baseURL, "://"); idx != -1 {
		rest := baseURL[idx+3:]
		if end := strings.Index(rest, "/"); end != -1 {
			return rest[:end]
		}
		return rest
	}
	return baseURL
}

// --- RDAP Response Parsing ---

// rdapResponse represents the RDAP response structure.
// Fields are optional as different registries include different data.
type rdapResponse struct {
	Handle        string            `json:"handle,omitempty"`
	LDHName       string            `json:"ldhName,omitempty"`
	UnicodeName   string            `json:"unicodeName,omitempty"`
	Status        []string          `json:"status,omitempty"`
	Entities      []rdapEntity      `json:"entities,omitempty"`
	Nameservers   []rdapNameserver  `json:"nameservers,omitempty"`
	Events        []rdapEvent       `json:"events,omitempty"`
	SecureDNS     *rdapSecureDNS    `json:"secureDNS,omitempty"`
	Links         []rdapLink        `json:"links,omitempty"`
	Port43        string            `json:"port43,omitempty"`
	PublicIDs     []rdapPublicID    `json:"publicIds,omitempty"`
	Remarks       []rdapRemark      `json:"remarks,omitempty"`
	Redacted      []rdapRedacted    `json:"redacted,omitempty"`
	Conformance   []string          `json:"rdapConformance,omitempty"`
	Notices       []rdapNotice      `json:"notices,omitempty"`
	Lang          string            `json:"lang,omitempty"`
	ObjectClass   string            `json:"objectClassName,omitempty"`
}

// rdapEntity represents an entity (registrar, registrant, etc.) in RDAP.
type rdapEntity struct {
	Handle    string        `json:"handle,omitempty"`
	LDHName   string        `json:"ldhName,omitempty"`
	Roles     []string      `json:"roles,omitempty"`
	Events    []rdapEvent   `json:"events,omitempty"`
	Status    []string      `json:"status,omitempty"`
	Entities  []rdapEntity  `json:"entities,omitempty"`
	PublicIDs []rdapPublicID `json:"publicIds,omitempty"`
	VCard     interface{}   `json:"vcardArray,omitempty"`
}

// rdapNameserver represents a nameserver in RDAP.
type rdapNameserver struct {
	Handle  string       `json:"handle,omitempty"`
	LDHName string       `json:"ldhName,omitempty"`
	Status  []string     `json:"status,omitempty"`
	Events  []rdapEvent  `json:"events,omitempty"`
	Links   []rdapLink   `json:"links,omitempty"`
}

// rdapEvent represents an event (registration, expiration, etc.) in RDAP.
type rdapEvent struct {
	Action     string `json:"eventAction,omitempty"`
	Date       string `json:"eventDate,omitempty"`
	Actor      string `json:"eventActor,omitempty"`
}

// rdapLink represents a link in RDAP.
type rdapLink struct {
	Rel      string `json:"rel,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Value    string `json:"value,omitempty"`
}

// rdapSecureDNS represents DNSSEC information in RDAP.
type rdapSecureDNS struct {
	ZoneSigned    *bool   `json:"zoneSigned,omitempty"`
	DelegationSigned *bool `json:"delegationSigned,omitempty"`
	MaxSigLife    *int    `json:"maxSigLife,omitempty"`
	DSData        []interface{} `json:"dsData,omitempty"`
	KeyData       []interface{} `json:"keyData,omitempty"`
}

// rdapPublicID represents a public identifier in RDAP.
type rdapPublicID struct {
	Type       string `json:"type,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

// rdapRemark represents a remark in RDAP.
type rdapRemark struct {
	Title        string   `json:"title,omitempty"`
	Description  []string `json:"description,omitempty"`
	Links        []rdapLink `json:"links,omitempty"`
}

// rdapNotice represents a notice in RDAP.
type rdapNotice struct {
	Title       string      `json:"title,omitempty"`
	Description []string    `json:"description,omitempty"`
	Links       []rdapLink  `json:"links,omitempty"`
}

// rdapRedacted represents a redaction notice in RDAP (RFC 9537).
type rdapRedacted struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	PrePath     string `json:"prePath,omitempty"`
	PostPath    string `json:"postPath,omitempty"`
}

// parseRDAPBody parses the RDAP JSON response and extracts registration details.
func parseRDAPBody(data []byte) *domain.Registration {
	reg := &domain.Registration{}

	// Check for empty body (Verisign 404, but we shouldn't get here on 200).
	if len(data) == 0 {
		return reg
	}

	var resp rdapResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		// Invalid JSON, return empty registration.
		return reg
	}

	// Check if this is an error response (some registries return error objects).
	if resp.ObjectClass == "error" {
		return reg
	}

	// Extract registrar from entities with "registrar" role.
	reg.Registrar = extractRegistrar(resp.Entities)

	// Extract dates from events.
	reg.Created, reg.Expires = extractDates(resp.Events)

	// Extract nameservers.
	reg.Nameservers = extractNameservers(resp.Nameservers)

	// Extract status.
	reg.Status = resp.Status

	return reg
}

// extractRegistrar finds the registrar name from entities.
func extractRegistrar(entities []rdapEntity) string {
	for _, ent := range entities {
		for _, role := range ent.Roles {
			if role == "registrar" {
				// Prefer LDHName, fallback to Handle.
				if ent.LDHName != "" {
					return ent.LDHName
				}
				return ent.Handle
			}
		}
		// Check nested entities (some registries nest registrar under registrant).
		if nested := extractRegistrar(ent.Entities); nested != "" {
			return nested
		}
	}
	return ""
}

// extractDates extracts creation and expiration dates from events.
// Known event actions: "registration", "last changed", "expiration", "transfer",
// "reregistration" (Google), "delegation check" (ignored).
func extractDates(events []rdapEvent) (created, expires string) {
	for _, ev := range events {
		switch strings.ToLower(ev.Action) {
		case "registration":
			created = parseRDAPDate(ev.Date)
		case "expiration":
			expires = parseRDAPDate(ev.Date)
		case "last changed":
			// We don't expose this, but could be useful.
		case "reregistration":
			// Google uses this for some domains. Treat as registration date.
			if created == "" {
				created = parseRDAPDate(ev.Date)
			}
		// Unknown event types are ignored (e.g., "delegation check").
		default:
			// Ignore unknown event types.
		}
	}
	return created, expires
}

// extractNameservers extracts and normalizes nameserver names.
// Strips trailing dots (DENIC, Nominet append them).
func extractNameservers(nss []rdapNameserver) []string {
	if len(nss) == 0 {
		return nil
	}
	names := make([]string, 0, len(nss))
	for _, ns := range nss {
		name := strings.ToLower(ns.LDHName)
		name = strings.TrimSuffix(name, ".")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// parseRDAPDate parses RDAP date strings with various formats:
// - RFC 3339: 2006-01-02T15:04:05Z
// - With fractional seconds (0-6 digits): 2006-01-02T15:04:05.123Z
// - With timezone offset: 2006-01-02T15:04:05+01:00
// - With fractional + offset: 2006-01-02T15:04:05.123+01:00
func parseRDAPDate(s string) string {
	if s == "" {
		return ""
	}

	// Try parsing with various layouts.
	layouts := []string{
		"2006-01-02T15:04:05.999999Z07:00", // Up to 6 fractional digits with offset
		"2006-01-02T15:04:05.999999Z",      // Up to 6 fractional digits with Z
		"2006-01-02T15:04:05Z07:00",        // No fractional seconds with offset
		"2006-01-02T15:04:05Z",             // No fractional seconds with Z
		time.RFC3339,                       // Standard RFC3339
		time.RFC3339Nano,                   // Standard RFC3339 with nanoseconds
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			// Return in a consistent format (RFC3339 without fractional seconds).
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}

	// If parsing fails, return the original string.
	return s
}

// SanitizeDomain rejects domains containing characters that indicate URL
// fragments, credentials, or path segments rather than plain domain names.
// This must be called before any network request.
//
// Rejected characters: / : @
func SanitizeDomain(domain string) error {
	if strings.ContainsAny(domain, "/:@") {
		return ErrInvalidDomainInput
	}
	return nil
}

// ErrInvalidDomainInput is returned when domain input contains forbidden characters.
var ErrInvalidDomainInput = errors.New("domain contains forbidden characters")
