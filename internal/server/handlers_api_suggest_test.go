package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/domain-check/internal/checker"
	"github.com/jedarden/domain-check/internal/domain"
)

// mockCheckerForSuggestions is a mock that simulates domain availability checks.
type mockCheckerForSuggestions struct {
	checkResults map[string]*domain.DomainResult
}

func (m *mockCheckerForSuggestions) Check(ctx context.Context, domainName string) (*domain.DomainResult, error) {
	if result, ok := m.checkResults[domainName]; ok {
		return result, nil
	}
	// Default: domain is available
	return &domain.DomainResult{
		Domain:    domainName,
		Available: true,
		TLD:       "com", // Simplified
	}, nil
}

func (m *mockCheckerForSuggestions) CheckBulk(ctx context.Context, domains []string) *checker.BulkResult {
	result := &checker.BulkResult{
		Results: make(map[string]*domain.DomainResult),
		Errors:  make(map[string]string),
	}

	for _, domainName := range domains {
		if checkResult, ok := m.checkResults[domainName]; ok {
			result.Results[domainName] = checkResult
		} else {
			// Default: domain is available
			result.Results[domainName] = &domain.DomainResult{
				Domain:    domainName,
				Available: true,
				TLD:       "com",
			}
		}
	}

	return result
}

func TestCheckHandler_WithSuggestions(t *testing.T) {
	tests := []struct {
		name           string
		domain         string
		suggestEnabled bool
		setupMock      map[string]*domain.DomainResult
		wantStatus     int
		wantSuggestions bool
	}{
		{
			name:           "unavailable domain with suggestions enabled",
			domain:         "taken.com",
			suggestEnabled: true,
			setupMock: map[string]*domain.DomainResult{
				"taken.com": {
					Domain:    "taken.com",
					Available: false,
					TLD:       "com",
				},
			},
			wantStatus:      http.StatusOK,
			wantSuggestions: true,
		},
		{
			name:           "available domain with suggestions enabled",
			domain:         "available.com",
			suggestEnabled: true,
			setupMock: map[string]*domain.DomainResult{
				"available.com": {
					Domain:    "available.com",
					Available: true,
					TLD:       "com",
				},
			},
			wantStatus:      http.StatusOK,
			wantSuggestions: false, // Available domains don't get suggestions
		},
		{
			name:           "unavailable domain with suggestions disabled",
			domain:         "taken.com",
			suggestEnabled: false,
			setupMock: map[string]*domain.DomainResult{
				"taken.com": {
					Domain:    "taken.com",
					Available: false,
					TLD:       "com",
				},
			},
			wantStatus:      http.StatusOK,
			wantSuggestions: false, // Suggestions not requested
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChecker := &mockCheckerForSuggestions{checkResults: tt.setupMock}
			log := slog.Default()
			h := NewAPIHandlers(mockChecker, log, nil, nil, nil)

			// Build request URL
			url := "/api/v1/check?d=" + tt.domain
			if tt.suggestEnabled {
				url += "&suggest=true"
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			h.CheckHandler(w, req)

			// Check status code
			if w.Code != tt.wantStatus {
				t.Errorf("CheckHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			// Parse response
			var responseBody map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&responseBody); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check for suggestions field
			_, hasSuggestions := responseBody["suggestions"]
			if hasSuggestions != tt.wantSuggestions {
				t.Errorf("CheckHandler() has suggestions = %v, want %v", hasSuggestions, tt.wantSuggestions)
			}

			// If suggestions are expected, verify they're in the right format
			if tt.wantSuggestions && hasSuggestions {
				suggestions, ok := responseBody["suggestions"].([]interface{})
				if !ok {
					t.Error("Suggestions field is not an array")
				} else {
					// Each suggestion should have domain, type, and available fields
					for _, s := range suggestions {
						suggestion, ok := s.(map[string]interface{})
						if !ok {
							t.Error("Suggestion is not an object")
							continue
						}
						if _, hasDomain := suggestion["domain"]; !hasDomain {
							t.Error("Suggestion missing 'domain' field")
						}
						if _, hasType := suggestion["type"]; !hasType {
							t.Error("Suggestion missing 'type' field")
						}
						if _, hasAvailable := suggestion["available"]; !hasAvailable {
							t.Error("Suggestion missing 'available' field")
						}
					}
				}
			}
		})
	}
}

func TestCheckHandler_SuggestionsWithBulkChecker(t *testing.T) {
	// Test that suggestions work correctly with a real bulk checker mock
	takenDomain := "example.com"
	mockChecker := &mockCheckerForSuggestions{
		checkResults: map[string]*domain.DomainResult{
			takenDomain: {
				Domain:    takenDomain,
				Available: false,
				TLD:       "com",
			},
		},
	}

	log := slog.Default()
	h := NewAPIHandlers(mockChecker, log, nil, nil, nil)

	// Request with suggestions enabled
	url := "/api/v1/check?d=" + takenDomain + "&suggest=true"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	h.CheckHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CheckHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	// Parse response
	var responseBody domain.SuggestionResponse
	if err := json.NewDecoder(w.Body).Decode(&responseBody); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify the response structure
	if responseBody.Result == nil {
		t.Error("Response missing 'result' field")
	}
	if responseBody.Result == nil || responseBody.Result.Available {
		t.Error("Original domain should be marked as unavailable")
	}

	// Check that suggestions are present and only include available domains
	if len(responseBody.Suggestions) == 0 {
		t.Error("Expected suggestions to be generated, got none")
	}

	for _, suggestion := range responseBody.Suggestions {
		if !suggestion.Available {
			t.Errorf("Suggestion %s is not available (only available suggestions should be included)", suggestion.Domain)
		}
		if suggestion.Result == nil {
			t.Errorf("Suggestion %s missing result data", suggestion.Domain)
		}
	}
}

func TestCheckHandler_SuggestionsUnavailableDomain(t *testing.T) {
	// Test that suggestions are generated when the original domain is unavailable
	unavailableDomain := "unavailable.com"
	mockChecker := &mockCheckerForSuggestions{
		checkResults: map[string]*domain.DomainResult{
			unavailableDomain: {
				Domain:    unavailableDomain,
				Available: false,
				TLD:       "com",
			},
		},
	}

	log := slog.Default()
	h := NewAPIHandlers(mockChecker, log, nil, nil, nil)

	url := "/api/v1/check?d=" + unavailableDomain + "&suggest=true"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	h.CheckHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CheckHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	var responseBody domain.SuggestionResponse
	if err := json.NewDecoder(w.Body).Decode(&responseBody); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have suggestions since the domain is unavailable
	if len(responseBody.Suggestions) == 0 {
		t.Error("Expected suggestions for unavailable domain, got none")
	}

	// All suggestions should be available (taken ones filtered out)
	for _, suggestion := range responseBody.Suggestions {
		if !suggestion.Available {
			t.Errorf("Suggestion %s should be available (taken suggestions should be filtered)", suggestion.Domain)
		}
	}
}

func TestCheckHandler_SuggestionsAvailableDomain(t *testing.T) {
	// Test that no suggestions are generated when the original domain is available
	availableDomain := "available.com"
	mockChecker := &mockCheckerForSuggestions{
		checkResults: map[string]*domain.DomainResult{
			availableDomain: {
				Domain:    availableDomain,
				Available: true,
				TLD:       "com",
			},
		},
	}

	log := slog.Default()
	h := NewAPIHandlers(mockChecker, log, nil, nil, nil)

	url := "/api/v1/check?d=" + availableDomain + "&suggest=true"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	h.CheckHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CheckHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	var responseBody map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&responseBody); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should not have suggestions since the domain is available
	if _, hasSuggestions := responseBody["suggestions"]; hasSuggestions {
		t.Error("Available domain should not generate suggestions")
	}

	// Verify the domain is marked as available
	if available, ok := responseBody["available"].(bool); !ok || !available {
		t.Error("Available domain should be marked as available in response")
	}
}
