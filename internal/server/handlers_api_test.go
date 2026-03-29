package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coding/domain-check/internal/checker"
	"github.com/coding/domain-check/internal/domain"
)

// mockChecker is a mock checker for testing.
type mockChecker struct {
	result *domain.DomainResult
	err    error
}

func (m *mockChecker) Check(ctx context.Context, domain string) (*domain.DomainResult, error) {
	return m.result, m.err
}

func TestCheckHandler_MissingParameter(t *testing.T) {
	// Create mock checker
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	// Create request without domain parameter
	req := httptest.NewRequest("GET", "/api/v1/check", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "missing_parameter" {
		t.Errorf("expected error %q, got %q", "missing_parameter", resp.Error)
	}
}

func TestCheckHandler_InvalidDomain(t *testing.T) {
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	tests := []struct {
		name        string
		domain      string
		expectError string
	}{
		{"empty domain", "", "missing_parameter"}, // Empty param is caught as missing
		{"invalid chars", "example!.com", "invalid_domain"},
		{"url in domain", "http://example.com", "invalid_domain"},
		{"single label", "example", "invalid_domain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/check?d="+tt.domain, nil)
		rec := httptest.NewRecorder()

		handlers.CheckHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			return
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

			if resp.Error != tt.expectError {
				t.Errorf("expected error %q, got %q", tt.expectError, resp.Error)
			}
		})
	}
}

func TestCheckHandler_UnsupportedTLD(t *testing.T) {
	mockCh := &mockChecker{
		err: checker.ErrTLDNotFound,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example.xyz", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "unsupported_tld" {
		t.Errorf("expected error %q, got %q", "unsupported_tld", resp.Error)
	}
}

func TestCheckHandler_Success(t *testing.T) {
	expectedResult := &domain.DomainResult{
		Domain:     "example.com",
		Available:  false,
		TLD:        "com",
		Source:     domain.SourceRDAP,
		Cached:     false,
		DurationMs: 150,
		Registration: &domain.Registration{
			Registrar: "Example Registrar",
			Created:  "2020-01-01T00:00:00Z",
			Expires:  "2025-01-01T00:00:00Z",
		},
	}

	mockCh := &mockChecker{
		result: expectedResult,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp domain.DomainResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Domain != expectedResult.Domain {
		t.Errorf("expected domain %q, got %q", expectedResult.Domain, resp.Domain)
	}
	if resp.Available != expectedResult.Available {
		t.Errorf("expected available %v, got %v", expectedResult.Available, resp.Available)
	}
	if resp.TLD != expectedResult.TLD {
		t.Errorf("expected TLD %q, got %q", expectedResult.TLD, resp.TLD)
	}
}

func TestCheckHandler_AvailableDomain(t *testing.T) {
	expectedResult := &domain.DomainResult{
		Domain:     "available123.com",
		Available:  true,
		TLD:        "com",
		Source:     domain.SourceRDAP,
		Cached:     false,
		DurationMs: 50,
	}

	mockCh := &mockChecker{
		result: expectedResult,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=available123.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp domain.DomainResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Available != true {
		t.Errorf("expected available=true, got %v", resp.Available)
	}
	if resp.Registration != nil {
		t.Errorf("expected nil registration for available domain, got %v", resp.Registration)
	}
}

func TestCheckHandler_IDNDomain(t *testing.T) {
	// Test that IDN domains are normalized properly
	expectedResult := &domain.DomainResult{
		Domain:     "xn--mnchen-3ya.de",
		Available:  false,
		TLD:        "de",
		Source:     domain.SourceRDAP,
		Cached:     false,
		DurationMs: 100,
	}

	mockCh := &mockChecker{
		result: expectedResult,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	// Unicode input should be normalized to ASCII punycode
	req := httptest.NewRequest("GET", "/api/v1/check?d=m%C3%BCnchen.de", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp domain.DomainResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The checker should have received the normalized domain
	if resp.Domain != "xn--mnchen-3ya.de" {
		t.Errorf("expected normalized domain %q, got %q", "xn--mnchen-3ya.de", resp.Domain)
	}
}
