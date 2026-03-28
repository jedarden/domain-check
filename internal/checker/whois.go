// Package checker provides domain availability checking via RDAP and WHOIS.
package checker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coding/domain-check/internal/domain"
	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"golang.org/x/time/rate"
)

// WHOIS errors.
var (
	ErrWHOISRateLimited  = errors.New("WHOIS rate limited")
	ErrWHOISConnection   = errors.New("WHOIS connection error")
	ErrWHOISParseFailure = errors.New("WHOIS parse failure")
	ErrWHOISTimeout      = errors.New("WHOIS query timeout")
)

// WHOISRateConfig defines rate limit settings for WHOIS servers.
type WHOISRateConfig struct {
	// Rate is the token bucket rate (requests per second).
	Rate rate.Limit
	// Burst is the token bucket burst size.
	Burst int
	// MinInterval is the minimum time between requests (for very aggressive registries).
	MinInterval time.Duration
}

// WHOIS-specific registry configurations.
// These ccTLDs are known to have aggressive rate limiting.
var whoisRegistryConfigs = map[string]WHOISRateConfig{
	// DENIC (.de) - very aggressive, ~1 request per 5 seconds
	"whois.denic.de": {
		Rate:        rate.Every(5 * time.Second),
		Burst:       1,
		MinInterval: 5 * time.Second,
	},
	// .co (Colombia) - moderate rate limiting
	"whois.nic.co": {
		Rate:        rate.Every(2 * time.Second),
		Burst:       2,
		MinInterval: 2 * time.Second,
	},
	// JPRS (.jp) - moderate rate limiting
	"whois.jprs.jp": {
		Rate:        rate.Every(time.Second),
		Burst:       2,
		MinInterval: time.Second,
	},
	// KRNIC (.kr) - moderate rate limiting
	"whois.krnic.or.kr": {
		Rate:        rate.Every(time.Second),
		Burst:       2,
		MinInterval: time.Second,
	},
	// CNNIC (.cn) - moderate rate limiting
	"whois.cnnic.cn": {
		Rate:        rate.Every(2 * time.Second),
		Burst:       2,
		MinInterval: 2 * time.Second,
	},
	// TCI (.ru) - moderate rate limiting
	"whois.tcin.ru": {
		Rate:        rate.Every(time.Second),
		Burst:       3,
		MinInterval: time.Second,
	},
	// IIS (.se) - moderate rate limiting
	"whois.iis.se": {
		Rate:        rate.Every(500 * time.Millisecond),
		Burst:       3,
		MinInterval: 500 * time.Millisecond,
	},
	// SWITCH (.ch) - moderate rate limiting
	"whois.nic.ch": {
		Rate:        rate.Every(time.Second),
		Burst:       3,
		MinInterval: time.Second,
	},
	// NIC.AT (.at) - moderate rate limiting
	"whois.nic.at": {
		Rate:        rate.Every(time.Second),
		Burst:       3,
		MinInterval: time.Second,
	},
	// DNS Belgium (.be) - moderate rate limiting
	"whois.dns.be": {
		Rate:        rate.Every(time.Second),
		Burst:       3,
		MinInterval: time.Second,
	},
	// .nz - moderate rate limiting
	"whois.srs.net.nz": {
		Rate:        rate.Every(time.Second),
		Burst:       3,
		MinInterval: time.Second,
	},
	// .gg (Guernsey) - light rate limiting
	"whois.gg": {
		Rate:        rate.Every(500 * time.Millisecond),
		Burst:       5,
		MinInterval: 500 * time.Millisecond,
	},
}

// defaultWHOISConfig is used for unknown WHOIS servers.
var defaultWHOISConfig = WHOISRateConfig{
	Rate:        rate.Every(500 * time.Millisecond),
	Burst:       3,
	MinInterval: 500 * time.Millisecond,
}

// whoisLimiter holds rate limiting state for a single WHOIS server.
type whoisLimiter struct {
	limiter    *rate.Limiter
	config     WHOISRateConfig
	lastQuery  time.Time
	mu         sync.Mutex
}

// WHOISRateLimiter manages per-registry rate limiting for WHOIS queries.
type WHOISRateLimiter struct {
	mu       sync.Mutex
	servers  map[string]*whoisLimiter
}

// NewWHOISRateLimiter creates a new WHOIS rate limiter.
func NewWHOISRateLimiter() *WHOISRateLimiter {
	return &WHOISRateLimiter{
		servers: make(map[string]*whoisLimiter),
	}
}

// getOrCreate returns the limiter for a WHOIS server, creating one if needed.
func (rl *WHOISRateLimiter) getOrCreate(server string) *whoisLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if existing, ok := rl.servers[server]; ok {
		return existing
	}

	cfg := defaultWHOISConfig
	if c, ok := whoisRegistryConfigs[server]; ok {
		cfg = c
	}

	rl.servers[server] = &whoisLimiter{
		limiter: rate.NewLimiter(cfg.Rate, cfg.Burst),
		config:  cfg,
	}
	return rl.servers[server]
}

// Wait blocks until a request to the given server is allowed.
func (rl *WHOISRateLimiter) Wait(ctx context.Context, server string) error {
	lim := rl.getOrCreate(server)

	// Enforce minimum interval.
	lim.mu.Lock()
	if lim.config.MinInterval > 0 && !lim.lastQuery.IsZero() {
		elapsed := time.Since(lim.lastQuery)
		if elapsed < lim.config.MinInterval {
			waitTime := lim.config.MinInterval - elapsed
			select {
			case <-ctx.Done():
				lim.mu.Unlock()
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}
	}
	lim.mu.Unlock()

	// Wait for rate limiter token.
	if err := lim.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	// Update last query time.
	lim.mu.Lock()
	lim.lastQuery = time.Now()
	lim.mu.Unlock()

	return nil
}

// WHOISClient queries WHOIS servers for domain availability.
type WHOISClient struct {
	ratelimit *WHOISRateLimiter
	userAgent string
	timeout   time.Duration
}

// WHOISClientConfig holds configuration for the WHOIS client.
type WHOISClientConfig struct {
	RateLimit *WHOISRateLimiter
	UserAgent string
	Timeout   time.Duration
}

// NewWHOISClient creates a new WHOIS client.
func NewWHOISClient(cfg WHOISClientConfig) *WHOISClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	rl := cfg.RateLimit
	if rl == nil {
		rl = NewWHOISRateLimiter()
	}
	return &WHOISClient{
		ratelimit: rl,
		userAgent: cfg.UserAgent,
		timeout:   timeout,
	}
}

// Check queries the WHOIS server for the given domain and returns the result.
// The domain must be a normalized, validated domain name (lowercase, ASCII).
func (c *WHOISClient) Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	start := time.Now()

	// Sanitize domain input before any network request.
	if err := SanitizeDomain(normalizedDomain); err != nil {
		return nil, err
	}

	// Extract TLD.
	parts := strings.Split(normalizedDomain, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid domain format: %s", normalizedDomain)
	}
	tld := parts[len(parts)-1]

	// Get WHOIS server for this TLD.
	whoisServer := whoisServerForTLD(tld)

	// Wait for rate limiter.
	if err := c.ratelimit.Wait(ctx, whoisServer); err != nil {
		return &domain.DomainResult{
			Domain:     normalizedDomain,
			TLD:        tld,
			CheckedAt:  time.Now(),
			Source:     domain.SourceWHOIS,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      ErrWHOISRateLimited.Error(),
		}, nil
	}

	// Execute WHOIS query with timeout.
	queryCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	rawWhois, err := c.doQuery(queryCtx, normalizedDomain, whoisServer)
	if err != nil {
		// Handle specific error types.
		if errors.Is(err, context.DeadlineExceeded) {
			return &domain.DomainResult{
				Domain:     normalizedDomain,
				TLD:        tld,
				CheckedAt:  time.Now(),
				Source:     domain.SourceWHOIS,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      ErrWHOISTimeout.Error(),
			}, nil
		}
		return &domain.DomainResult{
			Domain:     normalizedDomain,
			TLD:        tld,
			CheckedAt:  time.Now(),
			Source:     domain.SourceWHOIS,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}, nil
	}

	// Parse WHOIS response.
	result := c.parseResponse(rawWhois, normalizedDomain, tld, start, whoisServer)

	return result, nil
}

// doQuery performs the WHOIS query using the likexian/whois library.
func (c *WHOISClient) doQuery(ctx context.Context, domain, server string) (string, error) {
	// The likexian/whois library auto-discovers the WHOIS server if not specified.
	// We specify the server for better control and rate limiting.
	client := whois.NewClient()

	// Set timeout on the client.
	client.SetTimeout(c.timeout)

	// Use goroutine to support context cancellation.
	type result struct {
		raw string
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		raw, err := client.Whois(domain, server)
		resultCh <- result{raw: raw, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return "", fmt.Errorf("%w: %v", ErrWHOISConnection, res.err)
		}
		return res.raw, nil
	}
}

// parseResponse interprets the WHOIS response.
func (c *WHOISClient) parseResponse(rawWhois, domainName, tld string, start time.Time, server string) *domain.DomainResult {
	result := &domain.DomainResult{
		Domain:     domainName,
		TLD:        tld,
		CheckedAt:  time.Now(),
		Source:     domain.SourceWHOIS,
		DurationMs: time.Since(start).Milliseconds(),
	}

	// Parse the WHOIS response.
	parsed, err := whoisparser.Parse(rawWhois)
	if err != nil {
		// Parser failed - try to determine availability from raw response.
		result.Available = isAvailableFromRaw(rawWhois, tld)
		if !result.Available {
			result.Error = fmt.Sprintf("%s: parse error: %v", ErrWHOISParseFailure.Error(), err)
		}
		return result
	}

	// Check domain status.
	if parsed.Domain == nil {
		// No domain info - likely available.
		result.Available = isAvailableFromRaw(rawWhois, tld)
		return result
	}

	// Check domain status.
	if len(parsed.Domain.Status) > 0 {
		// Domain has status entries - it's registered.
		result.Available = false
	} else {
		// Check if the domain is in the response.
		if parsed.Domain.Name != "" {
			result.Available = false
		} else {
			result.Available = isAvailableFromRaw(rawWhois, tld)
		}
	}

	// Extract registration details for registered domains.
	if !result.Available {
		reg := &domain.Registration{}

		// Extract registrar.
		if parsed.Registrar != nil {
			reg.Registrar = parsed.Registrar.Name
		}

		// Extract dates.
		if parsed.Domain.CreatedDate != "" {
			reg.Created = parsed.Domain.CreatedDate
		}
		if parsed.Domain.ExpirationDate != "" {
			reg.Expires = parsed.Domain.ExpirationDate
		}

		// Extract nameservers.
		if len(parsed.Domain.NameServers) > 0 {
			reg.Nameservers = make([]string, len(parsed.Domain.NameServers))
			for i, ns := range parsed.Domain.NameServers {
				reg.Nameservers[i] = strings.ToLower(ns)
			}
		}

		// Extract status.
		if len(parsed.Domain.Status) > 0 {
			reg.Status = parsed.Domain.Status
		}

		result.Registration = reg
	}

	return result
}

// isAvailableFromRaw checks the raw WHOIS response for availability indicators.
// Different registries use different patterns to indicate availability.
func isAvailableFromRaw(raw string, tld string) bool {
	rawLower := strings.ToLower(raw)

	// Common "not found" patterns.
	notFoundPatterns := []string{
		"no match",
		"not found",
		"no entries found",
		"domain not found",
		"object does not exist",
		"no data found",
		"status: free",
		"status: available",
		"not registered",
		"not available for registration",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(rawLower, pattern) {
			return true
		}
	}

	// TLD-specific patterns.
	switch tld {
	case "de":
		// DENIC uses "Status: free" for available domains.
		if strings.Contains(rawLower, "status: free") {
			return true
		}
	case "jp":
		// JPRS uses "[ No Matching Data ]" for available domains.
		if strings.Contains(raw, "[ No Matching Data ]") {
			return true
		}
	case "ru", "su", "rf":
		// TCI uses "No entries found" for available domains.
		if strings.Contains(rawLower, "no entries found") {
			return true
		}
	}

	// If we see registrar information, it's taken.
	takenPatterns := []string{
		"registrar:",
		"created:",
		"creation date:",
		"registered on:",
		"registration time:",
		"domain name:",
		"registrant:",
	}

	for _, pattern := range takenPatterns {
		if strings.Contains(rawLower, pattern) {
			return false
		}
	}

	// Default to unknown - conservatively assume taken.
	return false
}

// whoisServerForTLD returns the WHOIS server for a given TLD.
// This provides explicit server addresses for rate limiting control.
func whoisServerForTLD(tld string) string {
	servers := map[string]string{
		"de":  "whois.denic.de",
		"co":  "whois.nic.co",
		"jp":  "whois.jprs.jp",
		"kr":  "whois.krnic.or.kr",
		"cn":  "whois.cnnic.cn",
		"ru":  "whois.tcin.ru",
		"su":  "whois.tcin.ru",
		"rf":  "whois.tcin.ru",
		"se":  "whois.iis.se",
		"ch":  "whois.nic.ch",
		"li":  "whois.nic.li",
		"at":  "whois.nic.at",
		"be":  "whois.dns.be",
		"nz":  "whois.srs.net.nz",
		"gg":  "whois.gg",
		"je":  "whois.je",
		"as":  "whois.nic.as",
		"ac":  "whois.nic.ac",
		"io":  "whois.nic.io",
		"tv":  "whois.nic.tv",
		"cc":  "whois.nic.cc",
		"me":  "whois.nic.me",
		"fm":  "whois.nic.fm",
		"ws":  "whois.nic.ws",
		"tk":  "whois.dot.tk",
		"ml":  "whois.dot.ml",
		"ga":  "whois.dot.ga",
		"cf":  "whois.dot.cf",
		"gq":  "whois.dot.gq",
	}

	if server, ok := servers[tld]; ok {
		return server
	}

	// Fallback to IANA whois for unknown TLDs.
	return "whois.iana.org"
}

// ccTLDsNeedingWHOIS returns the list of ccTLDs that require WHOIS fallback.
func ccTLDsNeedingWHOIS() []string {
	return []string{
		"de", "co", "jp", "kr", "cn", "ru", "se", "ch", "at", "be", "nz", "gg",
	}
}

// NeedsWHOIS returns true if a TLD requires WHOIS fallback (no RDAP support).
func NeedsWHOIS(tld string) bool {
	for _, ccTLD := range ccTLDsNeedingWHOIS() {
		if tld == ccTLD {
			return true
		}
	}
	return false
}
