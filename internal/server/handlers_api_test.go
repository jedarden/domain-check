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
	"github.com/coding/domain-check/internal/config"
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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, log, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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
	handlers := NewAPIHandlers(mockCh, nil, nil)

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

// ============================================================================
// Integration Tests — Full Middleware Chain (30 scenarios)
// These tests verify behavior through Router() with all middleware applied:
// RequestID → ClientIP → Logging → SecurityHeaders → CORS → RateLimit → Handler
// ============================================================================

// setupIntegrationRouter creates a Router with full middleware chain and a mock checker.
func setupIntegrationRouter(ch DomainChecker) http.Handler {
	cfg := config.Defaults()
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	return Router(&cfg, log, rl, ch, nil, nil, nil)
}

// --- Scenarios 1-7: Single Check (GET /api/v1/check) ---

func TestIntegration_SingleCheck(t *testing.T) {
	t.Run("01-available domain", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "intavail123.com",
				Available:  true,
				TLD:        "com",
				Source:     domain.SourceRDAP,
				Cached:     false,
				DurationMs: 42,
			},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=intavail123.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp domain.DomainResult
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Available != true {
			t.Errorf("expected available=true, got %v", resp.Available)
		}
		if resp.Source != domain.SourceRDAP {
			t.Errorf("expected source rdap, got %s", resp.Source)
		}
		if resp.Cached != false {
			t.Errorf("expected cached=false, got %v", resp.Cached)
		}
	})

	t.Run("02-taken domain with registration data", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "inttaken.com",
				Available:  false,
				TLD:        "com",
				Source:     domain.SourceRDAP,
				Cached:     false,
				DurationMs: 150,
				Registration: &domain.Registration{
					Registrar:   "MarkMonitor Inc.",
					Created:     "1997-09-15T04:00:00Z",
					Expires:     "2028-09-14T04:00:00Z",
					Nameservers: []string{"ns1.google.com", "ns2.google.com"},
					Status:      []string{"client delete prohibited", "client transfer prohibited"},
				},
			},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=inttaken.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp domain.DomainResult
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Available != false {
			t.Errorf("expected available=false, got %v", resp.Available)
		}
		if resp.Registration == nil {
			t.Fatal("expected registration data for taken domain")
		}
		if resp.Registration.Registrar == "" {
			t.Error("expected non-empty registrar")
		}
		if resp.Registration.Created == "" {
			t.Error("expected non-empty created date")
		}
		if resp.Registration.Expires == "" {
			t.Error("expected non-empty expires date")
		}
		if len(resp.Registration.Nameservers) == 0 {
			t.Error("expected at least one nameserver")
		}
		if len(resp.Registration.Status) == 0 {
			t.Error("expected at least one status value")
		}
	})

	t.Run("03-empty domain parameter", func(t *testing.T) {
		mockCh := &mockChecker{}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "missing_parameter" {
			t.Errorf("expected missing_parameter, got %s", resp.Error)
		}
	})

	t.Run("04-invalid domain", func(t *testing.T) {
		mockCh := &mockChecker{}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=not+a+domain", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "invalid_domain" {
			t.Errorf("expected invalid_domain, got %s", resp.Error)
		}
	})

	t.Run("05-unsupported TLD", func(t *testing.T) {
		mockCh := &mockChecker{
			err: checker.ErrTLDNotFound,
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=intunsup.xyz", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "unsupported_tld" {
			t.Errorf("expected unsupported_tld, got %s", resp.Error)
		}
	})

	t.Run("06-missing domain parameter", func(t *testing.T) {
		mockCh := &mockChecker{}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "missing_parameter" {
			t.Errorf("expected missing_parameter, got %s", resp.Error)
		}
	})

	t.Run("07-cached response", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "intcached.com",
				Available:  true,
				TLD:        "com",
				Source:     domain.SourceCache,
				Cached:     true,
				DurationMs: 0,
			},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=intcached.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp domain.DomainResult
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Cached != true {
			t.Errorf("expected cached=true, got %v", resp.Cached)
		}
		if resp.Source != domain.SourceCache {
			t.Errorf("expected source cache, got %s", resp.Source)
		}
	})
}

// --- Scenarios 8-10: Multi-TLD (GET /api/v1/check?tlds=) ---

func TestIntegration_MultiTLD(t *testing.T) {
	t.Run("08-valid pair com,org", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"intmulti1.com": {Domain: "intmulti1.com", Available: false, TLD: "com", Source: domain.SourceRDAP},
				"intmulti1.org": {Domain: "intmulti1.org", Available: true, TLD: "org", Source: domain.SourceRDAP},
			},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=intmulti1&tlds=com,org", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp MultiTLDResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected total 2, got %d", resp.Total)
		}
		if resp.Succeeded != 2 {
			t.Errorf("expected succeeded 2, got %d", resp.Succeeded)
		}
		if resp.Failed != 0 {
			t.Errorf("expected failed 0, got %d", resp.Failed)
		}
		if len(resp.Results) != 2 {
			t.Errorf("expected 2 results, got %d", len(resp.Results))
		}
	})

	t.Run("09-partial failure", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"intmulti2.com": {Domain: "intmulti2.com", Available: false, TLD: "com", Source: domain.SourceRDAP},
			},
			errors: map[string]string{
				"intmulti2.invalidtld": "unsupported tld",
			},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=intmulti2&tlds=com,invalidtld", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp MultiTLDResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Succeeded != 1 {
			t.Errorf("expected succeeded 1, got %d", resp.Succeeded)
		}
		if resp.Failed != 1 {
			t.Errorf("expected failed 1, got %d", resp.Failed)
		}

		// Verify the failed result has an error message
		var foundError bool
		for _, r := range resp.Results {
			if r.Error != "" {
				foundError = true
				break
			}
		}
		if !foundError {
			t.Error("expected at least one result with error field set")
		}
	})

	t.Run("10-no tlds falls through to single check", func(t *testing.T) {
		mockCh := &mockChecker{}
		router := setupIntegrationRouter(mockCh)

		// Without tlds parameter, handler does a single domain check.
		// "example" is a single label → domain.Parse rejects it.
		req := httptest.NewRequest("GET", "/api/v1/check?d=example", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		// Should be invalid_domain from single-check path, not a multi-TLD error
		if resp.Error != "invalid_domain" {
			t.Errorf("expected invalid_domain (single-check path), got %s", resp.Error)
		}
	})
}

// --- Scenarios 11-17: Bulk (POST /api/v1/bulk) ---

func TestIntegration_Bulk(t *testing.T) {
	t.Run("11-valid bulk request", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"intbulk1.com": {Domain: "intbulk1.com", Available: true, TLD: "com", Source: domain.SourceRDAP},
				"intbulk2.com": {Domain: "intbulk2.com", Available: false, TLD: "com", Source: domain.SourceRDAP},
				"intbulk3.com": {Domain: "intbulk3.com", Available: true, TLD: "com", Source: domain.SourceRDAP},
			},
		}
		router := setupIntegrationRouter(mockCh)

		body := `{"domains": ["intbulk1.com", "intbulk2.com", "intbulk3.com"]}`
		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp BulkCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Total != 3 {
			t.Errorf("expected total 3, got %d", resp.Total)
		}
		if resp.Succeeded != 3 {
			t.Errorf("expected succeeded 3, got %d", resp.Succeeded)
		}
		if len(resp.Results) != 3 {
			t.Errorf("expected 3 results, got %d", len(resp.Results))
		}
	})

	t.Run("12-max 50 domains", func(t *testing.T) {
		mockCh := &mockBulkChecker{results: make(map[string]*domain.DomainResult)}
		for i := 0; i < 50; i++ {
			d := fmt.Sprintf("intmax50-%d.com", i)
			mockCh.results[d] = &domain.DomainResult{Domain: d, Available: true, TLD: "com", Source: domain.SourceRDAP}
		}
		router := setupIntegrationRouter(mockCh)

		domains := make([]string, 50)
		for i := 0; i < 50; i++ {
			domains[i] = fmt.Sprintf("intmax50-%d.com", i)
		}
		body, _ := json.Marshal(BulkRequest{Domains: domains})
		req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp BulkCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Total != 50 {
			t.Errorf("expected total 50, got %d", resp.Total)
		}
		if resp.Succeeded != 50 {
			t.Errorf("expected succeeded 50, got %d", resp.Succeeded)
		}
	})

	t.Run("13-over limit 51 domains", func(t *testing.T) {
		mockCh := &mockBulkChecker{}
		router := setupIntegrationRouter(mockCh)

		domains := make([]string, 51)
		for i := 0; i < 51; i++ {
			domains[i] = fmt.Sprintf("intover51-%d.com", i)
		}
		body, _ := json.Marshal(BulkRequest{Domains: domains})
		req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "too_many_domains" {
			t.Errorf("expected too_many_domains, got %s", resp.Error)
		}
	})

	t.Run("14-empty array", func(t *testing.T) {
		mockCh := &mockBulkChecker{}
		router := setupIntegrationRouter(mockCh)

		body := `{"domains": []}`
		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "empty_array" {
			t.Errorf("expected empty_array, got %s", resp.Error)
		}
	})

	t.Run("15-oversized body", func(t *testing.T) {
		mockCh := &mockBulkChecker{}
		router := setupIntegrationRouter(mockCh)

		// Build a body > 64KB with 50 domains (max count allowed)
		largeLabel := strings.Repeat("a", 1400)
		domains := make([]string, 50)
		for i := 0; i < 50; i++ {
			domains[i] = largeLabel + ".com"
		}
		body, _ := json.Marshal(BulkRequest{Domains: domains})
		if len(body) <= MaxBulkBodySize {
			t.Fatalf("test body too small (%d bytes), need > %d", len(body), MaxBulkBodySize)
		}
		req := httptest.NewRequest("POST", "/api/v1/bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "body_too_large" {
			t.Errorf("expected body_too_large, got %s", resp.Error)
		}
	})

	t.Run("16-invalid JSON", func(t *testing.T) {
		mockCh := &mockBulkChecker{}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader("this is not json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != "invalid_json" {
			t.Errorf("expected invalid_json, got %s", resp.Error)
		}
	})

	t.Run("17-mixed success and failure", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"intmix1.com": {Domain: "intmix1.com", Available: true, TLD: "com", Source: domain.SourceRDAP},
				"intmix3.com": {Domain: "intmix3.com", Available: false, TLD: "com", Source: domain.SourceRDAP},
			},
			errors: map[string]string{
				"intmix2.com": "timeout checking registry",
			},
		}
		router := setupIntegrationRouter(mockCh)

		// Include an invalid domain that fails validation before reaching the checker
		body := `{"domains": ["intmix1.com", "intmix2.com", "intmix3.com", "invalid!"]}`
		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp BulkCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 4 {
			t.Errorf("expected total 4, got %d", resp.Total)
		}
		if resp.Succeeded != 2 {
			t.Errorf("expected succeeded 2, got %d", resp.Succeeded)
		}
		if resp.Failed != 2 {
			t.Errorf("expected failed 2, got %d", resp.Failed)
		}

		// Each result must have either a result or an error
		for _, r := range resp.Results {
			if r.Result == nil && r.Error == "" {
				t.Errorf("domain %s: expected either result or error", r.Domain)
			}
		}
	})
}

// --- Scenarios 18-21: Rate Limiting ---

func TestIntegration_RateLimit(t *testing.T) {
	t.Run("18-web limit 11th request blocked", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "rlweb.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		// Web endpoint is GET / — burst is 10
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.99.1.1:12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("web request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 11th should be rate limited
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.99.1.1:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("11th web request: expected 429, got %d", rec.Code)
		}
	})

	t.Run("19-API limit 61st request blocked", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "rlapi.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		// API burst is 60
		for i := 0; i < 60; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check?d=rlapi.com", nil)
			req.RemoteAddr = "10.99.2.1:12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("API request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// 61st should be rate limited
		req := httptest.NewRequest("GET", "/api/v1/check?d=rlapi.com", nil)
		req.RemoteAddr = "10.99.2.1:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("61st API request: expected 429, got %d", rec.Code)
		}
	})

	t.Run("20-different IPs have isolated limits", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "rlip.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		// Exhaust API limit for IP A (60 requests)
		for i := 0; i < 60; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check?d=rlip.com", nil)
			req.RemoteAddr = "10.99.3.1:12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("IP-A request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// IP A should now be blocked
		req := httptest.NewRequest("GET", "/api/v1/check?d=rlip.com", nil)
		req.RemoteAddr = "10.99.3.1:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("IP-A after limit: expected 429, got %d", rec.Code)
		}

		// IP B should still succeed (isolated limit)
		req = httptest.NewRequest("GET", "/api/v1/check?d=rlip.com", nil)
		req.RemoteAddr = "10.99.3.2:12345"
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("IP-B: expected 200, got %d", rec.Code)
		}
	})

	t.Run("21-retry-after header on 429", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "rlretry.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		// Exhaust web limit (10 requests)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.99.4.1:12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		// Next request should get 429 with Retry-After
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.99.4.1:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}

		retryAfter := rec.Header().Get("Retry-After")
		if retryAfter == "" {
			t.Error("expected Retry-After header to be set")
		}

		// Body should contain rate limit error
		body := rec.Body.String()
		if !contains(body, "rate limit exceeded") {
			t.Errorf("expected body to contain 'rate limit exceeded', got %s", body)
		}
	})
}

// --- Scenarios 22-27: Security Headers ---

func TestIntegration_Security(t *testing.T) {
	t.Run("22-Content-Security-Policy header", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "sec1.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=sec1.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		csp := rec.Header().Get("Content-Security-Policy")
		if csp == "" {
			t.Fatal("expected Content-Security-Policy header")
		}

		requiredDirectives := []string{
			"default-src 'none'",
			"script-src 'self'",
			"frame-ancestors 'none'",
		}
		for _, dir := range requiredDirectives {
			if !contains(csp, dir) {
				t.Errorf("CSP missing directive: %s", dir)
			}
		}
	})

	t.Run("23-X-Content-Type-Options nosniff", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("expected nosniff, got %s", got)
		}
	})

	t.Run("24-X-Frame-Options DENY", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("expected DENY, got %s", got)
		}
	})

	t.Run("25-X-Request-Id header", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		rid := rec.Header().Get("X-Request-Id")
		if rid == "" {
			t.Fatal("expected X-Request-Id header")
		}
		if len(rid) != 16 {
			t.Errorf("expected 16-char request ID, got %d chars: %s", len(rid), rid)
		}
	})

	t.Run("26-CORS preflight", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("OPTIONS", "/api/v1/check", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Errorf("expected Access-Control-Allow-Origin https://example.com, got %s", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("expected Access-Control-Allow-Methods header")
		}
		if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
			t.Error("expected Access-Control-Allow-Headers header")
		}
		if got := rec.Header().Get("Access-Control-Max-Age"); got == "" {
			t.Error("expected Access-Control-Max-Age header")
		}
	})

	t.Run("27-API response Content-Type application/json", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{Domain: "ctint.com", Available: true, TLD: "com"},
		}
		router := setupIntegrationRouter(mockCh)

		req := httptest.NewRequest("GET", "/api/v1/check?d=ctint.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("expected application/json, got %s", got)
		}
	})
}

// --- Scenarios 28-30: Health Check ---

func TestIntegration_HealthCheck(t *testing.T) {
	t.Run("28-normal health check returns ok", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode health response: %v", err)
		}
		if health["status"] != "ok" {
			t.Errorf("expected status ok, got %v", health["status"])
		}
	})

	t.Run("29-health response structure and headers", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		// Content-Type must be JSON
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("expected application/json, got %s", got)
		}

		// Security headers must be present on health endpoint too
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("expected X-Content-Type-Options nosniff, got %s", got)
		}
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("expected X-Frame-Options DENY, got %s", got)
		}

		// Request ID must be present
		if got := rec.Header().Get("X-Request-Id"); got == "" {
			t.Error("expected X-Request-Id header")
		}

		// Response must be valid JSON
		var health map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if _, ok := health["status"]; !ok {
			t.Error("expected 'status' field in health response")
		}
	})

	t.Run("30-health endpoint not rate limited", func(t *testing.T) {
		router := setupIntegrationRouter(&mockChecker{})

		// Health has no rate limiting — 50 rapid requests should all succeed
		for i := 0; i < 50; i++ {
			req := httptest.NewRequest("GET", "/health", nil)
			req.RemoteAddr = "10.99.5.1:12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("health request %d: expected 200, got %d", i, rec.Code)
			}
		}
	})
}

// Mock BootstrapProvider for testing
type mockBootstrap struct {
	tlds    []string
	updated time.Time
}

func (m *mockBootstrap) ServerCount() int {
	return len(m.tlds)
}

func (m *mockBootstrap) Updated() time.Time {
	if m.updated.IsZero() {
		return time.Now()
	}
	return m.updated
}

func (m *mockBootstrap) TLDs() []string {
	return m.tlds
}

// TLDs Endpoint Tests

func TestTLDsHandler_Success(t *testing.T) {
	tlds := []string{"com", "org", "net", "dev", "io", "app", "xyz"}
	mockBS := &mockBootstrap{
		tlds:    tlds,
		updated: time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp TLDsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != len(tlds) {
		t.Errorf("expected count %d, got %d", len(tlds), resp.Count)
	}

	if len(resp.TLDs) != len(tlds) {
		t.Errorf("expected %d TLDs, got %d", len(tlds), len(resp.TLDs))
	}

	if resp.BootstrapUpdated != "2026-04-20T12:00:00Z" {
		t.Errorf("expected bootstrap_updated 2026-04-20T12:00:00Z, got %s", resp.BootstrapUpdated)
	}

	// Verify TLDs are sorted
	for i := 1; i < len(resp.TLDs); i++ {
		if resp.TLDs[i-1] > resp.TLDs[i] {
			t.Errorf("TLDs not sorted: %s comes before %s", resp.TLDs[i-1], resp.TLDs[i])
		}
	}
}

func TestTLDsHandler_EmptyTLDs(t *testing.T) {
	mockBS := &mockBootstrap{
		tlds:    []string{},
		updated: time.Now(),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp TLDsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 0 {
		t.Errorf("expected count 0, got %d", resp.Count)
	}

	if len(resp.TLDs) != 0 {
		t.Errorf("expected 0 TLDs, got %d", len(resp.TLDs))
	}
}

func TestTLDsHandler_UnsortedInput(t *testing.T) {
	mockBS := &mockBootstrap{
		tlds:    []string{"xyz", "com", "net", "aaa", "org"},
		updated: time.Now(),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp TLDsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify TLDs are sorted alphabetically
	expected := []string{"aaa", "com", "net", "org", "xyz"}
	for i, tld := range resp.TLDs {
		if tld != expected[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expected[i], tld)
		}
	}
}

func TestTLDsHandler_NilBootstrap(t *testing.T) {
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "bootstrap_unavailable" {
		t.Errorf("expected error %q, got %q", "bootstrap_unavailable", resp.Error)
	}
}

func TestTLDsHandler_MethodNotAllowed(t *testing.T) {
	mockBS := &mockBootstrap{
		tlds:    []string{"com", "org"},
		updated: time.Now(),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("POST", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

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

func TestTLDsHandler_ContentType(t *testing.T) {
	mockBS := &mockBootstrap{
		tlds:    []string{"com", "org"},
		updated: time.Now(),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}
}

func TestTLDsHandler_LargeList(t *testing.T) {
	// Create a large list of TLDs
	tlds := make([]string, 500)
	for i := 0; i < 500; i++ {
		tlds[i] = fmt.Sprintf("tld%d", i)
	}

	mockBS := &mockBootstrap{
		tlds:    tlds,
		updated: time.Now(),
	}
	mockCh := &mockChecker{}
	handlers := NewAPIHandlers(mockCh, nil, mockBS)

	req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
	rec := httptest.NewRecorder()

	handlers.TLDsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp TLDsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 500 {
		t.Errorf("expected count 500, got %d", resp.Count)
	}

	if len(resp.TLDs) != 500 {
		t.Errorf("expected 500 TLDs, got %d", len(resp.TLDs))
	}
}

func TestIntegration_TLDsEndpoint(t *testing.T) {
	t.Run("tlds-endpoint-returns-sorted-list", func(t *testing.T) {
		mockBS := &mockBootstrap{
			tlds:    []string{"com", "org", "net", "dev", "io"},
			updated: time.Now(),
		}
		mockCh := &mockChecker{}
		cfg := config.Defaults()
		log := DefaultLogger("text", "error")

		// Create a custom router with our mock bootstrap
		mux := http.NewServeMux()
		apiHandlers := NewAPIHandlers(mockCh, log, mockBS)
		mux.HandleFunc("GET /api/v1/tlds", apiHandlers.TLDsHandler)
		handler := Chain(mux, RequestID, ClientIP(false), Logging(log), SecurityHeaders, CORS(&cfg))

		req := httptest.NewRequest("GET", "/api/v1/tlds", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp TLDsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 5 {
			t.Errorf("expected count 5, got %d", resp.Count)
		}

		// Verify security headers
		if got := rec.Header().Get("Content-Security-Policy"); got == "" {
			t.Error("expected CSP header")
		}
		if got := rec.Header().Get("X-Request-Id"); got == "" {
			t.Error("expected X-Request-Id header")
		}
	})
}
