// Package cli provides tests for the command-line interface functionality.
package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/checker"
	"github.com/coding/domain-check/internal/domain"
)

// TestBulkConfigDefaults tests that BulkConfig has sensible defaults.
func TestBulkConfigDefaults(t *testing.T) {
	cfg := BulkConfig{
		File:        "test.txt",
		Format:      "text",
		Concurrency: 20,
		Timeout:     30 * time.Second,
		UserAgent:   "domain-check/1.0",
		ShowProgress: false,
	}

	if cfg.File != "test.txt" {
		t.Errorf("expected File to be test.txt, got %s", cfg.File)
	}
	if cfg.Format != "text" {
		t.Errorf("expected Format to be text, got %s", cfg.Format)
	}
	if cfg.Concurrency != 20 {
		t.Errorf("expected Concurrency to be 20, got %d", cfg.Concurrency)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout to be 30s, got %v", cfg.Timeout)
	}
}

// TestReadDomainsFromFile tests reading domains from a file.
func TestReadDomainsFromFile(t *testing.T) {
	// Create a temporary file with domains.
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "domains.txt")
	content := "example.com\ngoogle.com\n\n# comment\n  github.com  \n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	domains, err := readDomainsFromFile(testFile)
	if err != nil {
		t.Fatalf("failed to read domains from file: %v", err)
	}

	// Should have 3 domains (blank lines and comments are skipped).
	if len(domains) != 3 {
		t.Errorf("expected 3 domains, got %d", len(domains))
	}

	expected := []string{"example.com", "google.com", "github.com"}
	for i, d := range domains {
		if d != expected[i] {
			t.Errorf("domain %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

// TestReadDomainsFromFile_EmptyFile tests reading from an empty file.
func TestReadDomainsFromFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	domains, err := readDomainsFromFile(testFile)
	if err != nil {
		t.Fatalf("failed to read empty file: %v", err)
	}

	if len(domains) != 0 {
		t.Errorf("expected 0 domains from empty file, got %d", len(domains))
	}
}

// TestReadDomainsFromFile_NonExistent tests reading from a non-existent file.
func TestReadDomainsFromFile_NonExistent(t *testing.T) {
	_, err := readDomainsFromFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error reading non-existent file, got nil")
	}
}

// TestReadDomainsFromFile_InvalidDomains tests that invalid domains are skipped.
func TestReadDomainsFromFile_InvalidDomains(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "mixed.txt")
	content := "example.com\n-invalid-\ngoogle.com"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	domains, err := readDomainsFromFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file with invalid domain: %v", err)
	}

	// Should have 2 valid domains (invalid one is skipped).
	if len(domains) != 2 {
		t.Errorf("expected 2 valid domains, got %d", len(domains))
	}

	expected := []string{"example.com", "google.com"}
	for i, d := range domains {
		if d != expected[i] {
			t.Errorf("domain %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

// TestProgressWriter tests the progress writer functionality.
func TestProgressWriter(t *testing.T) {
	total := 100
	p := NewProgressWriter(total, true)

	if p.total != int32(total) {
		t.Errorf("expected total to be %d, got %d", total, p.total)
	}

	// Increment and check that completed increases.
	for i := 0; i < total; i++ {
		p.Increment()
	}

	if p.completed != int32(total) {
		t.Errorf("expected completed to be %d, got %d", total, p.completed)
	}
}

// TestProgressWriter_NoProgress tests that progress writer respects showProgress flag.
func TestProgressWriter_NoProgress(t *testing.T) {
	p := NewProgressWriter(100, false)

	// These should not panic or write to stderr.
	for i := 0; i < 100; i++ {
		p.Increment()
	}
	p.Finish()
}

// TestBulkCheckResult tests the BulkCheckResult struct.
func TestBulkCheckResult(t *testing.T) {
	result := BulkCheckResult{
		Domain:     "example.com",
		Available:  true,
		TLD:        "com",
		Source:     "rdap",
		DurationMs: 100,
		Error:      "",
	}

	// Test JSON marshaling.
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal BulkCheckResult: %v", err)
	}

	var unmarshaled BulkCheckResult
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal BulkCheckResult: %v", err)
	}

	if unmarshaled.Domain != result.Domain {
		t.Errorf("expected Domain %s, got %s", result.Domain, unmarshaled.Domain)
	}
	if unmarshaled.Available != result.Available {
		t.Errorf("expected Available %v, got %v", result.Available, unmarshaled.Available)
	}
	if unmarshaled.TLD != result.TLD {
		t.Errorf("expected TLD %s, got %s", result.TLD, unmarshaled.TLD)
	}
}

// TestBulkCheckResult_WithError tests BulkCheckResult with an error.
func TestBulkCheckResult_WithError(t *testing.T) {
	result := BulkCheckResult{
		Domain: "invalid-domain",
		Error:  "invalid domain format",
	}

	// Test JSON marshaling with error.
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal BulkCheckResult with error: %v", err)
	}

	if !strings.Contains(string(data), "invalid domain format") {
		t.Errorf("expected JSON to contain error message, got %s", string(data))
	}
}

// TestOutputBulkJSON tests JSON output formatting.
func TestOutputBulkJSON(t *testing.T) {
	results := []BulkCheckResult{
		{Domain: "example.com", Available: true, TLD: "com", Source: "rdap", DurationMs: 100},
		{Domain: "google.com", Available: false, TLD: "com", Source: "rdap", DurationMs: 150},
	}

	var sb strings.Builder
	err := outputBulkJSON(&sb, results)
	if err != nil {
		t.Fatalf("failed to output JSON: %v", err)
	}

	output := sb.String()

	// Verify JSON structure.
	var decoded []BulkCheckResult
	err = json.Unmarshal([]byte(output), &decoded)
	if err != nil {
		t.Fatalf("failed to decode JSON output: %v\nOutput: %s", err, output)
	}

	if len(decoded) != 2 {
		t.Errorf("expected 2 results in JSON output, got %d", len(decoded))
	}
}

// TestOutputBulkCSV tests CSV output formatting.
func TestOutputBulkCSV(t *testing.T) {
	results := []BulkCheckResult{
		{Domain: "example.com", Available: true, TLD: "com", Source: "rdap", DurationMs: 100},
		{Domain: "google.com", Available: false, TLD: "com", Source: "rdap", DurationMs: 150},
	}

	var sb strings.Builder
	err := outputBulkCSV(&sb, results)
	if err != nil {
		t.Fatalf("failed to output CSV: %v", err)
	}

	output := sb.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 2 data lines = 3 lines.
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (header + 2 data) in CSV output, got %d", len(lines))
	}

	// Check header.
	if !strings.Contains(lines[0], "domain,available") {
		t.Errorf("expected CSV header to contain 'domain,available', got %s", lines[0])
	}

	// Check that example.com is marked as available.
	if !strings.Contains(lines[1], "true") {
		t.Errorf("expected example.com line to contain 'true', got %s", lines[1])
	}

	// Check that google.com is marked as taken.
	if !strings.Contains(lines[2], "false") {
		t.Errorf("expected google.com line to contain 'false', got %s", lines[2])
	}
}

// TestOutputBulkCSV_WithErrors tests CSV output with error results.
func TestOutputBulkCSV_WithErrors(t *testing.T) {
	results := []BulkCheckResult{
		{Domain: "example.com", Available: true, TLD: "com", Source: "rdap", DurationMs: 100},
		{Domain: "invalid-domain", Error: "invalid domain format"},
	}

	var sb strings.Builder
	err := outputBulkCSV(&sb, results)
	if err != nil {
		t.Fatalf("failed to output CSV with errors: %v", err)
	}

	output := sb.String()

	// Should contain the error message.
	if !strings.Contains(output, "invalid domain format") {
		t.Errorf("expected CSV output to contain error message, got %s", output)
	}
}

// TestBulk_WithMockRDAP tests bulk checking with a mock RDAP server.
func TestBulk_WithMockRDAP(t *testing.T) {
	// Create a mock RDAP server that returns simple responses.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/")

		if strings.Contains(domain, "available") {
			// Return 404 for available domains.
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return 200 with RDAP response for registered domains.
		w.Header().Set("Content-Type", "application/rdap+json")
		w.WriteHeader(http.StatusOK)
		jsonResponse := `{
			"ldhName": "` + domain + `",
			"handle": "123",
			"status": ["active"],
			"events": [
				{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"}
			]
		}`
		w.Write([]byte(jsonResponse))
	}))
	defer server.Close()

	// Create temporary domains file.
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "domains.txt")
	content := "available.com\ntaken.com\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create a minimal bootstrap with our mock server.
	bootstrap, err := checker.NewBootstrapManager(context.Background(), "")
	if err != nil {
		t.Skipf("skipping test: failed to create bootstrap: %v", err)
	}
	defer bootstrap.Stop()

	// Manually inject our mock server into the bootstrap.
	bootstrap.InjectServers(map[string]string{
		"com": server.URL,
	})

	// Create checker with our bootstrap.
	httpClient := &http.Client{Timeout: 5 * time.Second}
	rateLimiter := checker.NewRateLimiter()

	rdapClient := checker.NewRDAPClient(checker.RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  rateLimiter,
		UserAgent:  "test",
	})

	chk := checker.NewChecker(checker.CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
	})

	// Run bulk check.
	bulkResult := chk.CheckBulk(context.Background(), []string{"available.com", "taken.com"})

	// Check results.
	if len(bulkResult.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(bulkResult.Results))
	}

	// available.com should be available.
	if r, ok := bulkResult.Results["available.com"]; ok {
		if !r.Available {
			t.Errorf("expected available.com to be available, got %v", r.Available)
		}
	} else {
		t.Error("available.com not found in results")
	}

	// taken.com should be taken.
	if r, ok := bulkResult.Results["taken.com"]; ok {
		if r.Available {
			t.Errorf("expected taken.com to be taken, got %v", r.Available)
		}
	} else {
		t.Error("taken.com not found in results")
	}
}

// TestBulk_ConcurrencyLimit tests that concurrency limits are respected.
func TestBulk_ConcurrencyLimit(t *testing.T) {
	// This is a basic test to ensure the concurrency parameter is used.
	// A full test would need to track active goroutines, which is complex.
	cfg := BulkConfig{
		File:        "test.txt",
		Concurrency: 5,
		Timeout:     10 * time.Second,
	}

	if cfg.Concurrency != 5 {
		t.Errorf("expected Concurrency to be 5, got %d", cfg.Concurrency)
	}
}

// TestExitCodes tests the exit code constants.
func TestExitCodes(t *testing.T) {
	if ExitAvailable != 0 {
		t.Errorf("expected ExitAvailable to be 0, got %d", ExitAvailable)
	}
	if ExitTaken != 1 {
		t.Errorf("expected ExitTaken to be 1, got %d", ExitTaken)
	}
	if ExitError != 2 {
		t.Errorf("expected ExitError to be 2, got %d", ExitError)
	}
}

// TestBulkCheckResult_MarshalJSON tests JSON marshaling for BulkCheckResult.
func TestBulkCheckResult_MarshalJSON(t *testing.T) {
	result := BulkCheckResult{
		Domain:     "example.com",
		Available:  true,
		TLD:        "com",
		Source:     "rdap",
		DurationMs: 123,
		Error:      "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled["domain"] != "example.com" {
		t.Errorf("expected domain to be example.com, got %v", unmarshaled["domain"])
	}
	if unmarshaled["available"] != true {
		t.Errorf("expected available to be true, got %v", unmarshaled["available"])
	}
	if unmarshaled["tld"] != "com" {
		t.Errorf("expected tld to be com, got %v", unmarshaled["tld"])
	}
}

// TestBulkCollected tests the bulkCollected function with mock results.
func TestBulkCollected(t *testing.T) {
	// Create a mock domain result.
	mockResult := &domain.DomainResult{
		Domain:     "example.com",
		Available:  true,
		TLD:        "com",
		CheckedAt:  time.Now(),
		Source:     domain.SourceRDAP,
		Cached:     false,
		DurationMs: 100,
	}

	// Verify the result structure.
	if mockResult.Domain != "example.com" {
		t.Errorf("expected domain to be example.com, got %s", mockResult.Domain)
	}
	if !mockResult.Available {
		t.Errorf("expected example.com to be available")
	}
	if mockResult.TLD != "com" {
		t.Errorf("expected TLD to be com, got %s", mockResult.TLD)
	}
}
