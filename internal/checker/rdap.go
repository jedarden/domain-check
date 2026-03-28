// Package checker provides domain availability checking via RDAP.
package checker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coding/domain-check/internal/domain"
)

// RDAP errors.
var (
	ErrRateLimited    = errors.New("RDAP rate limited")
	ErrRegistryError  = errors.New("RDAP registry error")
	ErrConnection     = errors.New("RDAP connection error")
	ErrInvalidRDAPURL = errors.New("invalid RDAP URL")
)

// RDAPClient queries RDAP registry servers for domain availability.
type RDAPClient struct {
	httpClient *http.Client
	bootstrap  *BootstrapManager
	ratelimit  *RateLimiter
	allowlist  *AllowList
	userAgent  string
}

// RDAPClientConfig holds configuration for the RDAP client.
type RDAPClientConfig struct {
	HTTPClient *http.Client
	Bootstrap  *BootstrapManager
	RateLimit  *RateLimiter
	AllowList  *AllowList
	UserAgent  string
}

// NewRDAPClient creates a new RDAP client.
func NewRDAPClient(cfg RDAPClientConfig) *RDAPClient {
	return &RDAPClient{
		httpClient: cfg.HTTPClient,
		bootstrap:  cfg.Bootstrap,
		ratelimit:  cfg.RateLimit,
		allowlist:  cfg.AllowList,
		userAgent:  cfg.UserAgent,
	}
}

// Check queries the RDAP server for the given domain and returns the result.
// The domain must be a normalized, validated domain name (lowercase, ASCII).
func (c *RDAPClient) Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	start := time.Now()

	// Sanitize domain input before any network request.
	if err := SanitizeDomain(normalizedDomain); err != nil {
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
	registry := extractRegistryHost(rdapBase)

	var resp *http.Response
	var rdapErr error

	// Execute with rate limiting and retry.
	resp, rdapErr = c.ratelimit.Acquire(ctx, registry, func() (*http.Response, error) {
		return c.doRequest(ctx, rdapURL)
	})

	if rdapErr != nil {
		// Check for rate limit exhaustion.
		if errors.Is(rdapErr, ErrServiceBusy) || strings.Contains(rdapErr.Error(), "429") {
			return &domain.DomainResult{
				Domain:     normalizedDomain,
				TLD:        tld,
				CheckedAt:  time.Now(),
				Source:     domain.SourceRDAP,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      ErrRateLimited.Error(),
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

// doRequest performs the HTTP GET request to the RDAP server.
func (c *RDAPClient) doRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")

	return c.httpClient.Do(req)
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

// extractRegistryHost extracts the hostname from an RDAP base URL for rate limiting.
func extractRegistryHost(baseURL string) string {
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
