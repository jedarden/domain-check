package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

// mockBulkChecker is a mock checker that supports bulk operations.
type mockBulkChecker struct {
	mockChecker
	results map[string]*domain.DomainResult
	errors  map[string]string
}

func (m *mockBulkChecker) CheckBulk(ctx context.Context, domains []string) *checker.BulkResult {
	result := &checker.BulkResult{
		Results: make(map[string]*domain.DomainResult),
		Errors:  make(map[string]string),
	}

	for _, d := range domains {
		if err, ok := m.errors[d]; ok {
			result.Errors[d] = err
		} else if res, ok := m.results[d]; ok {
			result.Results[d] = res
		} else {
			result.Errors[d] = "domain not found in mock"
		}
	}

	return result
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

// Multi-TLD via CheckHandler Tests (GET /api/v1/check?d=example&tlds=com,org)

func TestCheckHandler_MultiTLDViaCheckEndpoint(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
			"example.org": {
				Domain:    "example.org",
				Available: true,
				TLD:       "org",
				Source:    domain.SourceRDAP,
			},
			"example.dev": {
				Domain:    "example.dev",
				Available: true,
				TLD:       "dev",
				Source:    domain.SourceRDAP,
			},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example&tlds=com,org,dev", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "example" {
		t.Errorf("expected name %q, got %q", "example", resp.Name)
	}
	if resp.Total != 3 {
		t.Errorf("expected total %d, got %d", 3, resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected succeeded %d, got %d", 3, resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("expected failed %d, got %d", 0, resp.Failed)
	}
	if len(resp.Results) != 3 {
		t.Errorf("expected %d results, got %d", 3, len(resp.Results))
	}
}

func TestCheckHandler_MultiTLDPartialSuccess(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
		},
		errors: map[string]string{
			"example.net": "timeout",
			"example.io":  "unsupported tld",
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example&tlds=com,net,io", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 3 {
		t.Errorf("expected total %d, got %d", 3, resp.Total)
	}
	if resp.Succeeded != 1 {
		t.Errorf("expected succeeded %d, got %d", 1, resp.Succeeded)
	}
	if resp.Failed != 2 {
		t.Errorf("expected failed %d, got %d", 2, resp.Failed)
	}
}

func TestCheckHandler_MultiTLDWithEmptyTLDs(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example&tlds=,,,", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid_parameter" {
		t.Errorf("expected error %q, got %q", "invalid_parameter", resp.Error)
	}
}

func TestCheckHandler_MultiTLDInvalidName(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=-bad&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid_domain" {
		t.Errorf("expected error %q, got %q", "invalid_domain", resp.Error)
	}
}

func TestCheckHandler_MultiTLDContentType(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {Domain: "example.com", Available: true, TLD: "com"},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check?d=example&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.CheckHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}
}

// Multi-TLD Handler Tests

func TestMultiTLDHandler_MissingNameParameter(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?tlds=com,net", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

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

func TestMultiTLDHandler_MissingTLDsParameter(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

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

func TestMultiTLDHandler_EmptyTLDsList(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=,,,", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid_parameter" {
		t.Errorf("expected error %q, got %q", "invalid_parameter", resp.Error)
	}
}

func TestMultiTLDHandler_InvalidDomainName(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	tests := []struct {
		name        string
		domain      string
		expectError string
	}{
		{"empty", "", "missing_parameter"}, // Empty after normalization is caught as missing
		{"starts with hyphen", "-example", "invalid_domain"},
		{"ends with hyphen", "example-", "invalid_domain"},
		{"invalid chars", "exa!mple", "invalid_domain"},
		{"with space", "exa mple", "invalid_domain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL-encode the domain parameter
			req := httptest.NewRequest("GET", "/api/v1/check/multi?d="+url.QueryEscape(tt.domain)+"&tlds=com", nil)
			rec := httptest.NewRecorder()

			handlers.MultiTLDHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
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

func TestMultiTLDHandler_TooLongDomainName(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	// Create a domain name that's exactly 64 characters (exceeds 63 char limit)
	longDomain := ""
	for i := 0; i < 64; i++ {
		longDomain += "a"
	}

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d="+longDomain+"&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid_domain" {
		t.Errorf("expected error %q, got %q", "invalid_domain", resp.Error)
	}
}

func TestMultiTLDHandler_Success(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
			"example.net": {
				Domain:    "example.net",
				Available: true,
				TLD:       "net",
				Source:    domain.SourceRDAP,
			},
			"example.org": {
				Domain:    "example.org",
				Available: true,
				TLD:       "org",
				Source:    domain.SourceRDAP,
			},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=com,net,org", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "example" {
		t.Errorf("expected name %q, got %q", "example", resp.Name)
	}
	if resp.Total != 3 {
		t.Errorf("expected total %d, got %d", 3, resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected succeeded %d, got %d", 3, resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("expected failed %d, got %d", 0, resp.Failed)
	}
	if len(resp.Results) != 3 {
		t.Errorf("expected %d results, got %d", 3, len(resp.Results))
	}
}

func TestMultiTLDHandler_PartialSuccess(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
			"example.org": {
				Domain:    "example.org",
				Available: true,
				TLD:       "org",
				Source:    domain.SourceRDAP,
			},
		},
		errors: map[string]string{
			"example.net":  "timeout",
			"example.dev":  "rate limit exceeded",
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=com,net,org,dev", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 4 {
		t.Errorf("expected total %d, got %d", 4, resp.Total)
	}
	if resp.Succeeded != 2 {
		t.Errorf("expected succeeded %d, got %d", 2, resp.Succeeded)
	}
	if resp.Failed != 2 {
		t.Errorf("expected failed %d, got %d", 2, resp.Failed)
	}

	// Verify error results have error field set
	for _, r := range resp.Results {
		if r.TLD == "net" || r.TLD == "dev" {
			if r.Error == "" {
				t.Errorf("expected error for %s, got empty", r.Domain)
			}
		} else {
			if r.Result == nil {
				t.Errorf("expected result for %s, got nil", r.Domain)
			}
		}
	}
}

func TestMultiTLDHandler_TLDNormalization(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: true,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	// Test TLDs with dots and mixed case
	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=Example&tlds=.COM,Net", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "example" {
		t.Errorf("expected normalized name %q, got %q", "example", resp.Name)
	}
	if resp.Total != 2 {
		t.Errorf("expected total %d, got %d", 2, resp.Total)
	}

	// Check that TLDs were normalized
	tlds := make(map[string]bool)
	for _, r := range resp.Results {
		tlds[r.TLD] = true
	}
	if !tlds["com"] {
		t.Error("expected normalized TLD 'com' in results")
	}
	if !tlds["net"] {
		t.Error("expected normalized TLD 'net' in results")
	}
}

func TestMultiTLDHandler_MethodNotAllowed(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("POST", "/api/v1/check/multi?d=example&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

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

func TestMultiTLDHandler_ContentType(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {Domain: "example.com", Available: true, TLD: "com"},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}
}

func TestMultiTLDHandler_Duration(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {Domain: "example.com", Available: true, TLD: "com"},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", resp.Duration)
	}
	if resp.Duration < time.Millisecond {
		// Duration should be at least a millisecond (or close to it)
		t.Logf("duration was very fast: %v", resp.Duration)
	}
}

func TestMultiTLDHandler_FallbackToSequential(t *testing.T) {
	// Test that sequential fallback works when bulk checker is not available
	mockCh := &mockChecker{
		result: &domain.DomainResult{
			Domain:    "example.com",
			Available: true,
			TLD:       "com",
			Source:    domain.SourceRDAP,
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/check/multi?d=example&tlds=com", nil)
	rec := httptest.NewRecorder()

	handlers.MultiTLDHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp MultiTLDResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "example" {
		t.Errorf("expected name %q, got %q", "example", resp.Name)
	}
}

func TestParseTLDList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"com,net,org", []string{"com", "net", "org"}},
		{"  com , net , org  ", []string{"com", "net", "org"}},
		{".com,.net", []string{"com", "net"}},
		{"COM,NET,ORG", []string{"com", "net", "org"}},
		{"com,,net", []string{"com", "net"}},
		{"", []string{}},
		{",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTLDList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d TLDs, got %d", len(tt.expected), len(result))
				return
			}
			for i, tld := range result {
				if tld != tt.expected[i] {
					t.Errorf("expected TLD %q at index %d, got %q", tt.expected[i], i, tld)
				}
			}
		})
	}
}

func TestValidateDomainName(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		expectErr bool
	}{
		{"valid simple", "example", false},
		{"valid with numbers", "example123", false},
		{"valid with hyphen", "my-example", false},
		{"empty", "", true},
		{"starts with hyphen", "-example", true},
		{"ends with hyphen", "example-", true},
		{"too long", strings.Repeat("a", 64), true},
		{"invalid char", "exa!mple", true},
		{"with space", "exa mple", true},
		{"max valid length", strings.Repeat("a", 63), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomainName(tt.domain)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// Bulk Handler Tests

func TestBulkHandler_EmptyArray(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": []}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "empty_array" {
		t.Errorf("expected error %q, got %q", "empty_array", resp.Error)
	}
}

func TestBulkHandler_TooManyDomains(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	// Create 51 domains (exceeds limit of 50)
	domains := make([]string, 51)
	for i := 0; i < 51; i++ {
		domains[i] = fmt.Sprintf("domain%d.com", i)
	}

	body, _ := json.Marshal(BulkRequest{Domains: domains})
	req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "too_many_domains" {
		t.Errorf("expected error %q, got %q", "too_many_domains", resp.Error)
	}
}

func TestBulkHandler_InvalidJSON(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid_json" {
		t.Errorf("expected error %q, got %q", "invalid_json", resp.Error)
	}
}

func TestBulkHandler_BodyTooLarge(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	// Create a body larger than 64 KB with 50 domains (max allowed count)
	// Each domain needs to be >1304 chars to exceed 64KB with 50 domains + JSON overhead
	largeDomain := strings.Repeat("a", 1400)
	domains := make([]string, 50)
	for i := 0; i < 50; i++ {
		domains[i] = largeDomain + ".com"
	}

	body, _ := json.Marshal(BulkRequest{Domains: domains})
	req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "body_too_large" {
		t.Errorf("expected error %q, got %q", "body_too_large", resp.Error)
	}
}

func TestBulkHandler_Success(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
			"example.org": {
				Domain:    "example.org",
				Available: true,
				TLD:       "org",
				Source:    domain.SourceRDAP,
			},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": ["example.com", "example.org"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("expected total %d, got %d", 2, resp.Total)
	}
	if resp.Succeeded != 2 {
		t.Errorf("expected succeeded %d, got %d", 2, resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("expected failed %d, got %d", 0, resp.Failed)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected %d results, got %d", 2, len(resp.Results))
	}
}

func TestBulkHandler_PartialSuccess(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: false,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
		},
		errors: map[string]string{
			"example.net": "timeout",
			"example.org": "rate limit exceeded",
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": ["example.com", "example.net", "example.org"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 3 {
		t.Errorf("expected total %d, got %d", 3, resp.Total)
	}
	if resp.Succeeded != 1 {
		t.Errorf("expected succeeded %d, got %d", 1, resp.Succeeded)
	}
	if resp.Failed != 2 {
		t.Errorf("expected failed %d, got %d", 2, resp.Failed)
	}
}

func TestBulkHandler_InvalidDomain(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {
				Domain:    "example.com",
				Available: true,
				TLD:       "com",
				Source:    domain.SourceRDAP,
			},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	// Include an invalid domain in the request
	body := `{"domains": ["example.com", "invalid!", "example.org"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 3 total: 2 successful + 1 validation error
	if resp.Total != 3 {
		t.Errorf("expected total %d, got %d", 3, resp.Total)
	}
}

func TestBulkHandler_MaxDomains(t *testing.T) {
	// Test that exactly 50 domains works (the maximum allowed)
	mockCh := &mockBulkChecker{
		results: make(map[string]*domain.DomainResult),
	}
	for i := 0; i < 50; i++ {
		mockCh.results[fmt.Sprintf("domain%d.com", i)] = &domain.DomainResult{
			Domain:    fmt.Sprintf("domain%d.com", i),
			Available: true,
			TLD:       "com",
			Source:    domain.SourceRDAP,
		}
	}
	handlers := NewAPIHandlers(mockCh, nil)

	// Create exactly 50 domains
	domains := make([]string, 50)
	for i := 0; i < 50; i++ {
		domains[i] = fmt.Sprintf("domain%d.com", i)
	}

	body, _ := json.Marshal(BulkRequest{Domains: domains})
	req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 50 {
		t.Errorf("expected total %d, got %d", 50, resp.Total)
	}
	if resp.Succeeded != 50 {
		t.Errorf("expected succeeded %d, got %d", 50, resp.Succeeded)
	}
}

func TestBulkHandler_MethodNotAllowed(t *testing.T) {
	mockCh := &mockBulkChecker{}
	handlers := NewAPIHandlers(mockCh, nil)

	req := httptest.NewRequest("GET", "/api/v1/bulk", nil)
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

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

func TestBulkHandler_ContentType(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {Domain: "example.com", Available: true, TLD: "com"},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": ["example.com"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}
}

func TestBulkHandler_Duration(t *testing.T) {
	mockCh := &mockBulkChecker{
		results: map[string]*domain.DomainResult{
			"example.com": {Domain: "example.com", Available: true, TLD: "com"},
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": ["example.com"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", resp.Duration)
	}
}

func TestBulkHandler_FallbackToSequential(t *testing.T) {
	// Test that sequential fallback works when bulk checker is not available
	mockCh := &mockChecker{
		result: &domain.DomainResult{
			Domain:    "example.com",
			Available: true,
			TLD:       "com",
			Source:    domain.SourceRDAP,
		},
	}
	handlers := NewAPIHandlers(mockCh, nil)

	body := `{"domains": ["example.com"]}`
	req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handlers.BulkHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BulkCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("expected total %d, got %d", 1, resp.Total)
	}
	if resp.Succeeded != 1 {
		t.Errorf("expected succeeded %d, got %d", 1, resp.Succeeded)
	}
}
