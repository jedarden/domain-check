package domain

import (
	"strings"
)

// SuggestionType represents the type of suggestion.
type SuggestionType string

const (
	// SuggestionTypeTLD indicates the same name on a different TLD.
	SuggestionTypeTLD SuggestionType = "tld"
	// SuggestionTypePrefix indicates a name variant with a prefix.
	SuggestionTypePrefix SuggestionType = "prefix"
	// SuggestionTypeSuffix indicates a name variant with a suffix.
	SuggestionTypeSuffix SuggestionType = "suffix"
)

// Suggestion represents a domain suggestion.
type Suggestion struct {
	// Domain is the suggested domain name.
	Domain string `json:"domain"`
	// Type is the type of suggestion.
	Type SuggestionType `json:"type"`
	// Available indicates if the domain is available (true) or taken (false).
	// This field is populated after the domain is checked.
	Available bool `json:"available"`
	// Result contains the full check result for this suggestion.
	// Only populated if the domain was successfully checked.
	Result *DomainResult `json:"result,omitempty"`
}

// PopularTLDs is the list of popular TLDs to suggest alternatives on.
// These are the most commonly used TLDs that are broadly recognized and trusted.
var PopularTLDs = []string{
	"com", "org", "net", "io", "co", "app", "dev", "ai", "tech", "cloud",
}

// CommonPrefixes is the list of common prefixes to suggest for domain variants.
var CommonPrefixes = []string{
	"get", "try", "use", "my", "the", "go",
}

// CommonSuffixes is the list of common suffixes to suggest for domain variants.
var CommonSuffixes = []string{
	"app", "hq", "io", "co", "dev", "hq", "tech",
}

// GenerateSuggestions generates domain suggestions based on the original domain.
// It returns a list of candidate domain names that should be checked for availability.
// The original domain is excluded from suggestions.
func GenerateSuggestions(originalDomain string) []Suggestion {
	// Parse the original domain to extract name and TLD.
	parts := strings.Split(originalDomain, ".")
	if len(parts) < 2 {
		return nil
	}

	// Extract name (all parts except the last) and TLD (last part).
	name := strings.Join(parts[:len(parts)-1], ".")
	tld := parts[len(parts)-1]

	var suggestions []Suggestion

	// Generate TLD variants (same name on other popular TLDs).
	for _, popularTLD := range PopularTLDs {
		// Skip if it's the same TLD as the original.
		if popularTLD == tld {
			continue
		}
		suggestions = append(suggestions, Suggestion{
			Domain: name + "." + popularTLD,
			Type:   SuggestionTypeTLD,
		})
	}

	// Generate prefix variants.
	for _, prefix := range CommonPrefixes {
		suggestions = append(suggestions, Suggestion{
			Domain: prefix + name + "." + tld,
			Type:   SuggestionTypePrefix,
		})
	}

	// Generate suffix variants.
	for _, suffix := range CommonSuffixes {
		suggestions = append(suggestions, Suggestion{
			Domain: name + suffix + "." + tld,
			Type:   SuggestionTypeSuffix,
		})
	}

	return suggestions
}

// SuggestionResponse wraps a domain result with suggestions when enabled.
type SuggestionResponse struct {
	// Result is the original domain check result.
	Result *DomainResult `json:"result"`
	// Suggestions is the list of suggested alternatives (only when suggest=true).
	// Only available suggestions are included (taken domains are filtered out).
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}
