// Package checker provides domain availability checking via RDAP.
package checker

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/coding/domain-check/internal/domain"
)

// DNSPreFilter checks domain availability using DNS NS lookups.
// If a domain has NS records, it's definitely registered.
// If no NS records exist, the result is inconclusive and RDAP should be consulted.
type DNSPreFilter struct {
	resolver *net.Resolver
}

// NewDNSPreFilter creates a new DNS pre-filter using the default resolver.
func NewDNSPreFilter() *DNSPreFilter {
	return &DNSPreFilter{
		resolver: net.DefaultResolver,
	}
}

// NewDNSPreFilterWithResolver creates a DNS pre-filter with a custom resolver.
// This is useful for testing or when a specific DNS server is required.
func NewDNSPreFilterWithResolver(resolver *net.Resolver) *DNSPreFilter {
	return &DNSPreFilter{
		resolver: resolver,
	}
}

// CheckResult is the result of a DNS pre-filter check.
type DNSCheckResult struct {
	// HasNS indicates whether NS records were found.
	HasNS bool
	// Nameservers contains the NS record values if found.
	Nameservers []string
	// Error is set if the DNS lookup failed (not NXDOMAIN).
	Error error
}

// Check performs a DNS NS lookup for the given domain.
// Returns a result indicating whether NS records were found.
//
// If NS records are found, the domain is definitely registered.
// If no NS records are found (NXDOMAIN or empty), the result is inconclusive
// and RDAP should be consulted for an authoritative check.
func (d *DNSPreFilter) Check(ctx context.Context, normalizedDomain string) *DNSCheckResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ns, err := d.resolver.LookupNS(ctx, normalizedDomain)
	if err != nil {
		// Check if this is an NXDOMAIN or other DNS error.
		if isNXDomain(err) {
			// No such domain - inconclusive, needs RDAP check.
			return &DNSCheckResult{HasNS: false}
		}
		// Other error (timeout, network issue, etc.) - return error.
		return &DNSCheckResult{Error: err}
	}

	if len(ns) > 0 {
		nameservers := make([]string, len(ns))
		for i, n := range ns {
			nameservers[i] = strings.ToLower(strings.TrimSuffix(n.Host, "."))
		}
		return &DNSCheckResult{
			HasNS:      true,
			Nameservers: nameservers,
		}
	}

	// No NS records found - inconclusive, needs RDAP check.
	return &DNSCheckResult{HasNS: false}
}

// CheckDomain is a convenience method that returns a DomainResult.
// If NS records are found, returns a DomainResult indicating the domain is registered.
// If no NS records or NXDOMAIN, returns nil to indicate RDAP should be consulted.
func (d *DNSPreFilter) CheckDomain(ctx context.Context, normalizedDomain string) *domain.DomainResult {
	start := time.Now()

	result := d.Check(ctx, normalizedDomain)

	if result.Error != nil {
		return &domain.DomainResult{
			Domain:     normalizedDomain,
			CheckedAt:  time.Now(),
			Source:     domain.SourceDNS,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      result.Error.Error(),
		}
	}

	if result.HasNS {
		return &domain.DomainResult{
			Domain:     normalizedDomain,
			Available:  false,
			CheckedAt:  time.Now(),
			Source:     domain.SourceDNS,
			DurationMs: time.Since(start).Milliseconds(),
			Registration: &domain.Registration{
				Nameservers: result.Nameservers,
			},
		}
	}

	// No NS records found - inconclusive, return nil to signal RDAP needed.
	return nil
}

// isNXDomain checks if the error indicates NXDOMAIN (no such domain).
func isNXDomain(err error) bool {
	if err == nil {
		return false
	}

	// net.DNSError has a IsNotFound field for NXDOMAIN.
	var dnsErr *net.DNSError
	if AsError(err, &dnsErr) {
		return dnsErr.IsNotFound
	}

	// Fallback: check for common NXDOMAIN error strings.
	errStr := err.Error()
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "NXDOMAIN") ||
		strings.Contains(errStr, "non-existent domain")
}

// AsError alias for errors.As to avoid importing errors just for this.
func AsError(err error, target any) bool {
	return err != nil && AsErrorImpl(err, target)
}

// Implementation helper - use standard errors.As pattern.
func AsErrorImpl(err error, target any) bool {
	// Use type assertion to call the DNSError method.
	if dnsErr, ok := err.(*net.DNSError); ok {
		*(target.(**net.DNSError)) = dnsErr
		return true
	}
	// Handle wrapped errors via Unwrap interface.
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return AsErrorImpl(unwrapper.Unwrap(), target)
	}
	return false
}
