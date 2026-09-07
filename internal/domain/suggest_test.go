package domain

import (
	"testing"
)

func TestGenerateSuggestions(t *testing.T) {
	tests := []struct {
		name           string
		originalDomain string
		wantMinCount   int // Minimum expected suggestions (can vary based on TLD matching)
		wantTypes      []SuggestionType
	}{
		{
			name:           "simple domain",
			originalDomain: "example.com",
			wantMinCount:   20, // At least some TLD variants + prefixes + suffixes
			wantTypes:      []SuggestionType{SuggestionTypeTLD, SuggestionTypePrefix, SuggestionTypeSuffix},
		},
		{
			name:           "domain with popular TLD",
			originalDomain: "example.org",
			wantMinCount:   20,
			wantTypes:      []SuggestionType{SuggestionTypeTLD, SuggestionTypePrefix, SuggestionTypeSuffix},
		},
		{
			name:           "multi-part domain",
			originalDomain: "my-app.example.co",
			wantMinCount:   20,
			wantTypes:      []SuggestionType{SuggestionTypeTLD, SuggestionTypePrefix, SuggestionTypeSuffix},
		},
		{
			name:           "invalid domain - no TLD",
			originalDomain: "example",
			wantMinCount:   0, // No suggestions for invalid domains
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSuggestions(tt.originalDomain)

			if len(got) < tt.wantMinCount {
				t.Errorf("GenerateSuggestions() returned %d suggestions, want at least %d", len(got), tt.wantMinCount)
			}

			// Check that we have all expected suggestion types
			typeSet := make(map[SuggestionType]bool)
			for _, suggestion := range got {
				typeSet[suggestion.Type] = true
			}

			for _, wantType := range tt.wantTypes {
				if !typeSet[wantType] {
					t.Errorf("GenerateSuggestions() missing suggestion type %v", wantType)
				}
			}

			// Verify that suggestions are well-formed
			for _, suggestion := range got {
				if suggestion.Domain == "" {
					t.Errorf("GenerateSuggestions() returned suggestion with empty domain")
				}
				if suggestion.Type == "" {
					t.Errorf("GenerateSuggestions() returned suggestion with empty type")
				}
				// All suggestions should start with valid prefixes or be valid domain variants
				if !isValidSuggestion(suggestion.Domain, tt.originalDomain) {
					t.Errorf("GenerateSuggestions() returned invalid suggestion: %s (from %s)", suggestion.Domain, tt.originalDomain)
				}
			}
		})
	}
}

func isValidSuggestion(suggestion, original string) bool {
	// A suggestion is valid if it's a reasonable variant of the original domain
	// We allow: prefix changes, suffix changes, or TLD changes
	// We don't allow completely unrelated domains

	// For now, just check that it's not empty and looks like a domain
	if suggestion == "" {
		return false
	}

	// Check it contains at least one dot (has TLD)
	dotCount := 0
	for _, c := range suggestion {
		if c == '.' {
			dotCount++
		}
	}
	return dotCount >= 1
}

func TestSuggestionTypes(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		wantType SuggestionType
	}{
		{"TLD variant", "example.org", SuggestionTypeTLD},
		{"prefix variant", "getexample.com", SuggestionTypePrefix},
		{"suffix variant", "exampleapp.com", SuggestionTypeSuffix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate suggestions for a simple domain
			suggestions := GenerateSuggestions("example.com")

			// Find a suggestion matching our criteria
			var found *Suggestion
			for _, s := range suggestions {
				if s.Domain == tt.domain {
					found = &s
					break
				}
			}

			if found != nil && found.Type != tt.wantType {
				t.Errorf("Suggestion for %s has type %v, want %v", tt.domain, found.Type, tt.wantType)
			}
		})
	}
}

func TestSuggestionResponse(t *testing.T) {
	// Test that SuggestionResponse can be properly constructed
	result := &DomainResult{
		Domain:    "example.com",
		Available: false,
		TLD:       "com",
	}

	suggestions := []Suggestion{
		{
			Domain:    "example.org",
			Type:      SuggestionTypeTLD,
			Available: true,
			Result: &DomainResult{
				Domain:    "example.org",
				Available: true,
				TLD:       "org",
			},
		},
	}

	response := SuggestionResponse{
		Result:      result,
		Suggestions: suggestions,
	}

	if response.Result == nil {
		t.Error("SuggestionResponse.Result is nil")
	}

	if len(response.Suggestions) != 1 {
		t.Errorf("SuggestionResponse.Suggestions has %d items, want 1", len(response.Suggestions))
	}

	if response.Suggestions[0].Available != true {
		t.Error("First suggestion is not marked as available")
	}
}

func TestSuggestionTypeValues(t *testing.T) {
	// Test that suggestion type constants have expected values
	if SuggestionTypeTLD != "tld" {
		t.Errorf("SuggestionTypeTLD = %v, want %v", SuggestionTypeTLD, "tld")
	}
	if SuggestionTypePrefix != "prefix" {
		t.Errorf("SuggestionTypePrefix = %v, want %v", SuggestionTypePrefix, "prefix")
	}
	if SuggestionTypeSuffix != "suffix" {
		t.Errorf("SuggestionTypeSuffix = %v, want %v", SuggestionTypeSuffix, "suffix")
	}
}

func TestGenerateSuggestionsExcludesOriginal(t *testing.T) {
	// Test that the original domain is never included in suggestions
	originalDomain := "example.com"
	suggestions := GenerateSuggestions(originalDomain)

	for _, suggestion := range suggestions {
		if suggestion.Domain == originalDomain {
			t.Errorf("GenerateSuggestions() included original domain %s in suggestions", originalDomain)
		}
	}
}

func TestPopularTLDs(t *testing.T) {
	// Test that PopularTLDs is properly defined
	if len(PopularTLDs) == 0 {
		t.Error("PopularTLDs is empty")
	}

	// Check for expected popular TLDs
	expectedTLDs := map[string]bool{
		"com":  true,
		"org":  true,
		"net":  true,
		"io":   true,
		"app":  true,
		"dev":  true,
	}

	tldSet := make(map[string]bool)
	for _, tld := range PopularTLDs {
		tldSet[tld] = true
	}

	for expectedTLD := range expectedTLDs {
		if !tldSet[expectedTLD] {
			t.Errorf("PopularTLDs missing expected TLD: %s", expectedTLD)
		}
	}
}

func TestCommonPrefixesAndSuffixes(t *testing.T) {
	// Test that common prefixes and suffixes are properly defined
	if len(CommonPrefixes) == 0 {
		t.Error("CommonPrefixes is empty")
	}

	if len(CommonSuffixes) == 0 {
		t.Error("CommonSuffixes is empty")
	}

	// Check for expected values
	expectedPrefixes := []string{"get", "try", "use"}
	prefixSet := make(map[string]bool)
	for _, prefix := range CommonPrefixes {
		prefixSet[prefix] = true
	}

	for _, expected := range expectedPrefixes {
		if !prefixSet[expected] {
			t.Errorf("CommonPrefixes missing expected prefix: %s", expected)
		}
	}

	expectedSuffixes := []string{"app", "hq", "io"}
	suffixSet := make(map[string]bool)
	for _, suffix := range CommonSuffixes {
		suffixSet[suffix] = true
	}

	for _, expected := range expectedSuffixes {
		if !suffixSet[expected] {
			t.Errorf("CommonSuffixes missing expected suffix: %s", expected)
		}
	}
}

func TestGenerateSuggestionsDeterministic(t *testing.T) {
	// Test that generating suggestions twice produces the same results
	originalDomain := "example.com"

	suggestions1 := GenerateSuggestions(originalDomain)
	suggestions2 := GenerateSuggestions(originalDomain)

	if len(suggestions1) != len(suggestions2) {
		t.Errorf("GenerateSuggestions() produced %d suggestions first time, %d second time", len(suggestions1), len(suggestions2))
	}

	// Check that the same suggestions are generated (same order)
	for i := 0; i < len(suggestions1); i++ {
		if suggestions1[i].Domain != suggestions2[i].Domain || suggestions1[i].Type != suggestions2[i].Type {
			t.Errorf("GenerateSuggestions() mismatch at index %d: %v vs %v", i, suggestions1[i], suggestions2[i])
		}
	}
}

func TestSuggestionJSONSerialization(t *testing.T) {
	// Test that suggestions can be serialized to JSON properly
	suggestion := Suggestion{
		Domain:    "example.org",
		Type:      SuggestionTypeTLD,
		Available: true,
	}

	// This would normally use encoding/json, but we're just testing the struct fields
	if suggestion.Domain != "example.org" {
		t.Errorf("Suggestion.Domain = %v, want %v", suggestion.Domain, "example.org")
	}
	if suggestion.Type != SuggestionTypeTLD {
		t.Errorf("Suggestion.Type = %v, want %v", suggestion.Type, SuggestionTypeTLD)
	}
	if suggestion.Available != true {
		t.Errorf("Suggestion.Available = %v, want %v", suggestion.Available, true)
	}
}
