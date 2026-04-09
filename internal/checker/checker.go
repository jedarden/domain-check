// Package checker provides the domain check engine that orchestrates RDAP, WHOIS, and DNS lookups.
package checker

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/coding/domain-check/internal/domain"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// BulkCheckConfig holds configuration for bulk check operations.
type BulkCheckConfig struct {
	// GlobalConcurrency is the maximum number of concurrent domain checks globally.
	GlobalConcurrency int
	// TotalTimeout is the maximum duration for a bulk operation.
	TotalTimeout time.Duration
}

// DefaultBulkCheckConfig returns the default bulk check configuration.
func DefaultBulkCheckConfig() BulkCheckConfig {
	return BulkCheckConfig{
		GlobalConcurrency: 50,
		TotalTimeout:      30 * time.Second,
	}
}

// Per-registry concurrency limits for bulk operations.
// These limits ensure we don't overwhelm individual registries.
const (
	// VerisignConcurrency is the max concurrent requests to Verisign (.com, .net).
	VerisignConcurrency int64 = 10
	// PIRConcurrency is the max concurrent requests to Public Interest Registry (.org).
	PIRConcurrency int64 = 10
	// GoogleConcurrency is the max concurrent requests to Google Registry (.app, .dev, etc).
	GoogleConcurrency int64 = 2
	// DefaultRegistryConcurrency is the default for unknown registries.
	DefaultRegistryConcurrency int64 = 3
)

// registryConcurrencyConfig maps registry hosts to their concurrency limits.
var registryConcurrencyConfig = map[string]int64{
	"rdap.verisign.com":              VerisignConcurrency,
	"rdap.publicinterestregistry.org": PIRConcurrency,
	"pubapi.registry.google":         GoogleConcurrency,
}

// Checker orchestrates domain availability checks using RDAP, WHOIS, and DNS.
type Checker struct {
	rdap       *RDAPClient
	whois      *WHOISClient
	dns        *DNSPreFilter
	cache      *ResultCache
	bootstrap  *BootstrapManager
	useDNSPrefilter bool
	bulkConfig BulkCheckConfig

	// Per-registry semaphores for bulk operation rate limiting.
	registrySemMu   sync.Mutex
	registrySems    map[string]*semaphore.Weighted
}

// CheckerConfig holds configuration for creating a Checker.
type CheckerConfig struct {
	RDAPClient      *RDAPClient
	WHOISClient     *WHOISClient
	DNSPreFilter    *DNSPreFilter
	Cache           *ResultCache
	Bootstrap       *BootstrapManager
	UseDNSPrefilter bool
	BulkConfig      BulkCheckConfig
}

// NewChecker creates a new domain check engine.
func NewChecker(cfg CheckerConfig) *Checker {
	bulkConfig := cfg.BulkConfig
	if bulkConfig.GlobalConcurrency <= 0 {
		bulkConfig = DefaultBulkCheckConfig()
	}

	return &Checker{
		rdap:       cfg.RDAPClient,
		whois:      cfg.WHOISClient,
		dns:        cfg.DNSPreFilter,
		cache:      cfg.Cache,
		bootstrap:  cfg.Bootstrap,
		useDNSPrefilter: cfg.UseDNSPrefilter,
		bulkConfig: bulkConfig,
		registrySems: make(map[string]*semaphore.Weighted),
	}
}

// getRegistrySem returns the semaphore for a registry, creating one if needed.
func (c *Checker) getRegistrySem(registry string) *semaphore.Weighted {
	c.registrySemMu.Lock()
	defer c.registrySemMu.Unlock()

	if sem, ok := c.registrySems[registry]; ok {
		return sem
	}

	limit := DefaultRegistryConcurrency
	if l, ok := registryConcurrencyConfig[registry]; ok {
		limit = l
	}

	sem := semaphore.NewWeighted(limit)
	c.registrySems[registry] = sem
	return sem
}

// Check performs a single domain availability check.
// It first checks the cache, then optionally uses DNS pre-filtering,
// and finally queries RDAP or WHOIS as needed.
func (c *Checker) Check(ctx context.Context, normalizedDomain string) (*domain.DomainResult, error) {
	start := time.Now()

	// Sanitize domain input.
	if err := SanitizeDomain(normalizedDomain); err != nil {
		return nil, err
	}

	// Extract TLD.
	parts := strings.Split(normalizedDomain, ".")
	if len(parts) < 2 {
		return nil, domainError("invalid domain format", normalizedDomain)
	}
	tld := parts[len(parts)-1]

	// Check cache first.
	if c.cache != nil {
		if cached := c.cache.Get(normalizedDomain); cached != nil {
			return cached, nil
		}
	}

	// Optional: DNS pre-filter for fast path.
	if c.useDNSPrefilter && c.dns != nil {
		if result := c.dns.CheckDomain(ctx, normalizedDomain); result != nil {
			// DNS found NS records - domain is definitely registered.
			result.TLD = tld
			if c.cache != nil {
				c.cache.Set(normalizedDomain, *result)
			}
			return result, nil
		}
		// No NS records or error - continue to RDAP/WHOIS.
	}

	// Determine if we need WHOIS fallback.
	var result *domain.DomainResult
	var err error

	if NeedsWHOIS(tld) && c.whois != nil {
		result, err = c.whois.Check(ctx, normalizedDomain)
	} else if c.rdap != nil {
		result, err = c.rdap.Check(ctx, normalizedDomain)
	} else {
		return nil, domainError("no check method available", normalizedDomain)
	}

	if err != nil {
		return nil, err
	}

	// Cache the result.
	if c.cache != nil && result != nil {
		c.cache.Set(normalizedDomain, *result)
	}

	// Set duration if not already set.
	if result != nil && result.DurationMs == 0 {
		result.DurationMs = time.Since(start).Milliseconds()
	}

	return result, nil
}

// BulkResult contains the results of a bulk domain check.
type BulkResult struct {
	// Results is a map of domain to DomainResult.
	Results map[string]*domain.DomainResult
	// Duration is the total time taken for the bulk check.
	Duration time.Duration
	// TotalChecked is the number of domains that were checked.
	TotalChecked int
	// TotalCached is the number of domains served from cache.
	TotalCached int
	// Errors is a map of domain to error message for failed checks.
	Errors map[string]string
}

// CheckBulk performs parallel domain availability checks for multiple domains.
//
// Features:
//   - Global concurrency cap (default 50 concurrent checks)
//   - Per-registry semaphores for rate limit utilization
//   - Domains grouped by registry for efficient execution
//   - 30s total timeout for bulk operations
//   - Partial results returned even if some checks fail
func (c *Checker) CheckBulk(ctx context.Context, domains []string) *BulkResult {
	start := time.Now()

	result := &BulkResult{
		Results: make(map[string]*domain.DomainResult),
		Errors:  make(map[string]string),
	}

	if len(domains) == 0 {
		return result
	}

	// Apply total timeout.
	ctx, cancel := context.WithTimeout(ctx, c.bulkConfig.TotalTimeout)
	defer cancel()

	// Separate domains into cached and uncached.
	var uncached []string
	for _, d := range domains {
		// Normalize domain for cache lookup.
		normalized := strings.ToLower(strings.TrimSpace(d))
		normalized = strings.TrimRight(normalized, ".")

		if c.cache != nil {
			if cached := c.cache.Get(normalized); cached != nil {
				result.Results[normalized] = cached
				result.TotalCached++
				continue
			}
		}
		uncached = append(uncached, normalized)
	}

	result.TotalChecked = len(uncached)

	if len(uncached) == 0 {
		result.Duration = time.Since(start)
		return result
	}

	// Group domains by registry for efficient rate limit utilization.
	registryGroups := c.groupByRegistry(uncached)

	// Use errgroup with global concurrency limit.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.bulkConfig.GlobalConcurrency)

	// Mutex for thread-safe result writing.
	var mu sync.Mutex

	// Process each domain.
	for _, normalizedDomain := range uncached {
		domain := normalizedDomain // capture for goroutine

		g.Go(func() error {
			// Check context before starting.
			select {
			case <-ctx.Done():
				mu.Lock()
				result.Errors[domain] = ctx.Err().Error()
				mu.Unlock()
				return nil // Don't fail the group, just record error.
			default:
			}

			// Perform the check.
			checkResult, err := c.checkWithContext(ctx, domain, registryGroups[domain])

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Errors[domain] = err.Error()
				return nil // Don't fail the group, just record error.
			}

			if checkResult != nil {
				result.Results[domain] = checkResult
				// Cache the result.
				if c.cache != nil {
					c.cache.Set(domain, *checkResult)
				}
			}

			return nil
		})
	}

	// Wait for all goroutines to complete.
	_ = g.Wait() // Errors are recorded per-domain, not returned.

	result.Duration = time.Since(start)
	return result
}

// checkWithContext performs a single domain check with context support.
// The registry parameter is used to apply per-registry rate limiting via semaphores.
func (c *Checker) checkWithContext(ctx context.Context, normalizedDomain, registry string) (*domain.DomainResult, error) {
	start := time.Now()

	// Acquire per-registry semaphore to limit concurrent requests to this registry.
	// This prevents overwhelming a single registry while allowing full concurrency
	// across different registries.
	sem := c.getRegistrySem(registry)
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, domainError("registry concurrency limit: "+err.Error(), normalizedDomain)
	}
	defer sem.Release(1)

	// Sanitize domain input.
	if err := SanitizeDomain(normalizedDomain); err != nil {
		return nil, err
	}

	// Extract TLD.
	parts := strings.Split(normalizedDomain, ".")
	if len(parts) < 2 {
		return nil, domainError("invalid domain format", normalizedDomain)
	}
	tld := parts[len(parts)-1]

	// Optional: DNS pre-filter for fast path.
	if c.useDNSPrefilter && c.dns != nil {
		if result := c.dns.CheckDomain(ctx, normalizedDomain); result != nil {
			result.TLD = tld
			if result.DurationMs == 0 {
				result.DurationMs = time.Since(start).Milliseconds()
			}
			return result, nil
		}
	}

	// Determine if we need WHOIS fallback.
	var result *domain.DomainResult
	var err error

	if NeedsWHOIS(tld) && c.whois != nil {
		result, err = c.whois.Check(ctx, normalizedDomain)
	} else if c.rdap != nil {
		result, err = c.rdap.Check(ctx, normalizedDomain)
	} else {
		return nil, domainError("no check method available", normalizedDomain)
	}

	if err != nil {
		return nil, err
	}

	// Check if context was canceled during the check.
	// This can happen if the context timeout is shorter than the HTTP timeout.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Set duration if not already set.
	if result != nil && result.DurationMs == 0 {
		result.DurationMs = time.Since(start).Milliseconds()
	}

	return result, nil
}

// groupByRegistry groups domains by their RDAP registry for efficient rate limiting.
// This allows the bulk checker to respect per-registry concurrency limits.
func (c *Checker) groupByRegistry(domains []string) map[string]string {
	result := make(map[string]string)

	for _, d := range domains {
		registry := c.getRegistryForDomain(d)
		result[d] = registry
	}

	return result
}

// getRegistryForDomain returns the registry hostname for a domain's TLD.
func (c *Checker) getRegistryForDomain(normalizedDomain string) string {
	parts := strings.Split(normalizedDomain, ".")
	if len(parts) < 2 {
		return "unknown"
	}
	tld := parts[len(parts)-1]

	// Check if we need WHOIS for this TLD.
	if NeedsWHOIS(tld) {
		return whoisServerForTLD(tld)
	}

	// Look up RDAP server from bootstrap.
	if c.bootstrap != nil {
		rdapURL, err := c.bootstrap.Lookup(tld)
		if err == nil {
			return extractRegistryHost(rdapURL)
		}
	}

	return "unknown"
}

// domainError creates a domain.ParseError-style error.
func domainError(reason, input string) error {
	return &domain.ParseError{
		Input: input,
		Phase: "check",
		Err:   reason,
	}
}
