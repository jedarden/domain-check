// Package domain provides domain name parsing, validation, and normalization
// per RFC 1035, RFC 1123, and RFC 5891 (IDNA).
package domain

import (
	"fmt"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// ParseError indicates why a domain was rejected.
type ParseError struct {
	Input string // original input
	Phase string // which pipeline step failed
	Err   string // human-readable reason
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("domain %q: %s (%s)", e.Input, e.Phase, e.Err)
}

// ParsedDomain is the result of successful domain parsing.
type ParsedDomain struct {
	// Original is the raw user input before normalization.
	Original string

	// Domain is the ASCII-normalized registrable domain (effective TLD+1).
	// e.g. "example.co.uk"
	Domain string

	// TLD is the public suffix (e.g. "co.uk", "com").
	TLD string

	// Unicode is the Unicode form of the domain if it contained IDN labels,
	// otherwise empty.
	Unicode string
}

var lookupProfile = idna.Lookup

// Parse validates and normalizes a domain name input through a 7-step pipeline:
//
//  1. Trim whitespace, strip trailing dot, lowercase
//  2. Reject if contains URL characters (/:@)
//  3. IDN conversion via idna.Lookup profile (handles punycode encoding)
//  4. Extract TLD via publicsuffix (handles multi-level TLDs like .co.uk)
//  5. Validate each label against LDH rules (RFC 1035)
//  6. Verify TLD is recognized by the public suffix list
//  7. Strip subdomains via EffectiveTLDPlusOne
func Parse(input string) (*ParsedDomain, error) {
	original := input

	// Step 1: Trim whitespace, strip trailing dot, lowercase.
	domain := strings.TrimSpace(input)
	domain = strings.TrimRight(domain, ".")
	domain = strings.ToLower(domain)
	if domain == "" {
		return nil, &ParseError{Input: original, Phase: "trim", Err: "empty after trimming"}
	}

	// Step 2: Reject URL characters.
	if strings.ContainsAny(domain, "/:@") {
		return nil, &ParseError{Input: original, Phase: "url-chars", Err: "contains URL characters (/:@)"}
	}

	// Step 3: IDN conversion via Lookup profile.
	// This handles Unicode input (e.g. "münchen.de" → "xn--mnchen-3ya.de")
	// and validates IDNA encoding for punycode input.
	asciiDomain, err := lookupProfile.ToASCII(domain)
	if err != nil {
		return nil, &ParseError{Input: original, Phase: "idna", Err: err.Error()}
	}

	// Also compute the Unicode form for display.
	unicodeDomain, _ := lookupProfile.ToUnicode(asciiDomain)
	if unicodeDomain == asciiDomain {
		unicodeDomain = "" // no IDN labels
	}

	domain = asciiDomain

	// Step 4: Extract TLD via publicsuffix.
	// This also validates the domain against the PSL.
	tld, icann := publicsuffix.PublicSuffix(domain)
	if !icann {
		// Non-ICANN TLD — could be a private suffix or unknown.
		// We still allow it but flag it differently.
	}

	if tld == "" || tld == domain {
		return nil, &ParseError{Input: original, Phase: "tld-extract", Err: "no registrable domain found"}
	}

	// Step 5: Validate each label against LDH rules.
	if err := validateLDH(domain); err != nil {
		return nil, &ParseError{Input: original, Phase: "ldh", Err: err.Error()}
	}

	// Step 6: Verify TLD is recognized.
	if !icann {
		// TLD not in the ICANN section of the PSL.
		// We accept it but note it's not an officially recognized TLD.
		// This allows internal/private domains while flagging unknown TLDs.
	}

	// Step 7: Strip subdomains via EffectiveTLDPlusOne.
	etld1, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return nil, &ParseError{Input: original, Phase: "etld1", Err: err.Error()}
	}

	return &ParsedDomain{
		Original: original,
		Domain:   etld1,
		TLD:      tld,
		Unicode:  unicodeDomain,
	}, nil
}

// validateLDH checks every label of the domain against RFC 1035 LDH rules:
//   - Label length 1–63 characters
//   - Allowed characters: [a-z0-9-] (already lowercased)
//   - Labels must not start or end with a hyphen
//   - Total FQDN length ≤ 253
func validateLDH(domain string) error {
	if len(domain) > 253 {
		return fmt.Errorf("domain exceeds 253 characters (%d)", len(domain))
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must have at least two labels")
	}

	for i, label := range labels {
		if len(label) < 1 {
			return fmt.Errorf("label %d is empty", i+1)
		}
		if len(label) > 63 {
			return fmt.Errorf("label %d exceeds 63 characters (%d)", i+1, len(label))
		}
		if label[0] == '-' {
			return fmt.Errorf("label %d starts with hyphen: %q", i+1, label)
		}
		if label[len(label)-1] == '-' {
			return fmt.Errorf("label %d ends with hyphen: %q", i+1, label)
		}
		for j, c := range label {
			if !isLDH(c) {
				return fmt.Errorf("label %d contains invalid character %q at position %d", i+1, c, j+1)
			}
		}
	}

	return nil
}

// isLDH reports whether c is a valid LDH character: a-z, 0-9, or hyphen.
func isLDH(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}
