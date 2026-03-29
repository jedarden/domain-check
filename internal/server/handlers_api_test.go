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

func TestCheckHandler_CachedResponse(t *testing.T) {
	// Test that cached responses are returned correctly
	expectedResult := &domain.DomainResult{
		Domain:     "cached.com",
		Available:  true,
		TLD:        "com",
		Source:     domain.SourceCache,
		Cached:     true,
		DurationMs: 0, // Cached responses may have 0 duration
	}

	mockCh := &mockChecker{
		result: expectedResult,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=cached.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp domain.DomainResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Cached != true {
		t.Errorf("expected cached=true, got %v", resp.Cached)
	}
	if resp.Source != domain.SourceCache {
		t.Errorf("expected source=%s, got %s", domain.SourceCache, resp.Source)
	}
}

func TestCheckHandler_CheckFailed(t *testing.T) {
	// Test internal server error when check fails
	mockCh := &mockChecker{
		err: context.DeadlineExceeded,
	}
	log := DefaultLogger("text", "error")
	handlers := NewAPIHandlers(mockCh, log)

	req := httptest.NewRequest("GET", "/api/v1/check?d=timeout.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "check_failed" {
		t.Errorf("expected error %q, got %q", "check_failed", resp.Error)
	}
}

func TestCheckHandler_FullRegistration(t *testing.T) {
	// Test that all registration fields are returned for taken domains
	expectedResult := &domain.DomainResult{
		Domain:    "google.com",
		Available: false,
		TLD:       "com",
		Source:    domain.SourceRDAP,
		Cached:    false,
		DurationMs: 250,
		Registration: &domain.Registration{
			Registrar:   "MarkMonitor Inc.",
			Created:     "1997-09-15T04:00:00Z",
			Expires:     "2028-09-14T04:00:00Z",
			Nameservers: []string{"ns1.google.com", "ns2.google.com", "ns3.google.com", "ns4.google.com"},
			Status:      []string{"client delete prohibited", "client transfer prohibited"},
		},
	}

	mockCh := &mockChecker{
		result: expectedResult,
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=google.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp domain.DomainResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Registration == nil {
		t.Fatalf("expected registration object, got nil")
	}
	if resp.Registration.Registrar != "MarkMonitor Inc." {
		t.Errorf("expected registrar %q, got %q", "MarkMonitor Inc.", resp.Registration.Registrar)
	}
	if resp.Registration.Created != "1997-09-15T04:00:00Z" {
		t.Errorf("expected created %q, got %q", "1997-09-15T04:00:00Z", resp.Registration.Created)
	}
	if len(resp.Registration.Nameservers) != 4 {
		t.Errorf("expected 4 nameservers, got %d", len(resp.Registration.Nameservers))
	}
	if len(resp.Registration.Status) != 2 {
		t.Errorf("expected 2 status values, got %d", len(resp.Registration.Status))
	}
}

func TestCheckHandler_ContentType(t *testing.T) {
	// Test that response has correct Content-Type
	mockCh := &mockChecker{
		result: &domain.DomainResult{Domain: "test.com", Available: true, TLD: "com"},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=test.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}
}

func TestCheckHandler_MethodNotAllowed(t *testing.T) {
	// Test that POST requests are rejected
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("POST", "/api/v1/check?d=test.com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "method_not_allowed" {
		t.Errorf("expected error %q, got %q", "method_not_allowed", resp.Error)
	}
}
