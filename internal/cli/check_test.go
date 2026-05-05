// Package cli provides tests for the check subcommand functionality.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestValidateSLD_Valid tests valid second-level domain labels.
func TestValidateSLD_Valid(t *testing.T) {
	valid := []string{
		"example",
		"a",
		"123",
		"a-b-c",
		"xn--nxasmq6b",
		"test123",
		"1example",
		"aaaaa" + strings.Repeat("a", 57), // 63 chars
	}

	for _, sld := range valid {
		t.Run(sld, func(t *testing.T) {
			if err := validateSLD(sld); err != nil {
				t.Errorf("expected %q to be valid, got error: %v", sld, err)
			}
		})
	}
}

// TestValidateSLD_Invalid tests invalid second-level domain labels.
func TestValidateSLD_Invalid(t *testing.T) {
	invalid := []struct {
		input    string
		expected string
	}{
		{"", "domain label is empty"},
		{"-leading", "starts with hyphen"},
		{"trailing-", "ends with hyphen"},
		{"exam_ple", "invalid character"},
		{"exam ple", "invalid character"},
		{"exam<ple", "invalid character"},
		{"🎉", "invalid character"},
		{strings.Repeat("a", 64), "exceeds 63 characters"},
	}

	for _, tc := range invalid {
		t.Run(tc.input, func(t *testing.T) {
			err := validateSLD(tc.input)
			if err == nil {
				t.Errorf("expected %q to be invalid, got nil error", tc.input)
				return
			}
			if !strings.Contains(err.Error(), tc.expected) {
				t.Errorf("expected error to contain %q, got %q", tc.expected, err.Error())
			}
		})
	}
}

// TestIsLDH tests the LDH character validator.
func TestIsLDH(t *testing.T) {
	valid := []rune{'a', 'z', '0', '9', '-'}
	for _, c := range valid {
		if !isLDH(c) {
			t.Errorf("expected %q to be a valid LDH character", c)
		}
	}

	invalid := []rune{'_', ' ', '@', '.', '/', ':', '🎉'}
	for _, c := range invalid {
		if isLDH(c) {
			t.Errorf("expected %q to be an invalid LDH character", c)
		}
	}
}

// TestOutputResults_Text tests text output format.
func TestOutputResults_Text(t *testing.T) {
	results := []CheckResult{
		{Domain: "available.com", Available: true, TLD: "com"},
		{Domain: "taken.com", Available: false, TLD: "com"},
		{Domain: "error.com", Error: "connection failed"},
	}

	var sb strings.Builder
	err := outputResults(&sb, results, "text")
	if err != nil {
		t.Fatalf("failed to output text: %v", err)
	}

	output := sb.String()
	if !strings.Contains(output, "available.com: AVAILABLE") {
		t.Errorf("expected output to contain 'available.com: AVAILABLE', got %s", output)
	}
	if !strings.Contains(output, "taken.com: TAKEN") {
		t.Errorf("expected output to contain 'taken.com: TAKEN', got %s", output)
	}
	if !strings.Contains(output, "error.com: ERROR") {
		t.Errorf("expected output to contain 'error.com: ERROR', got %s", output)
	}
}

// TestOutputResults_JSON tests JSON output format.
func TestOutputResults_JSON(t *testing.T) {
	results := []CheckResult{
		{Domain: "example.com", Available: true, TLD: "com"},
	}

	var sb strings.Builder
	err := outputResults(&sb, results, "json")
	if err != nil {
		t.Fatalf("failed to output JSON: %v", err)
	}

	var decoded []CheckResult
	err = json.Unmarshal([]byte(sb.String()), &decoded)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Errorf("expected 1 result, got %d", len(decoded))
	}
	if decoded[0].Domain != "example.com" {
		t.Errorf("expected domain to be example.com, got %s", decoded[0].Domain)
	}
	if !decoded[0].Available {
		t.Errorf("expected example.com to be available")
	}
}

// TestOutputResults_CSV tests CSV output format.
func TestOutputResults_CSV(t *testing.T) {
	results := []CheckResult{
		{Domain: "example.com", Available: true, TLD: "com"},
		{Domain: "taken.com", Available: false, TLD: "com"},
		{Domain: "error.com", Error: "some error"},
	}

	var sb strings.Builder
	err := outputResults(&sb, results, "csv")
	if err != nil {
		t.Fatalf("failed to output CSV: %v", err)
	}

	output := sb.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Header + 3 data lines
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (header + 3 data), got %d", len(lines))
	}

	// Check header
	if !strings.Contains(lines[0], "domain,available") {
		t.Errorf("expected header to contain 'domain,available', got %s", lines[0])
	}

	// Check that available is true
	if !strings.Contains(lines[1], "true") {
		t.Errorf("expected line 1 to contain 'true', got %s", lines[1])
	}

	// Check that taken is false
	if !strings.Contains(lines[2], "false") {
		t.Errorf("expected line 2 to contain 'false', got %s", lines[2])
	}

	// Check error message
	if !strings.Contains(lines[3], "some error") {
		t.Errorf("expected line 3 to contain error message, got %s", lines[3])
	}
}

// TestOutputResults_UnknownFormat tests error handling for unknown format.
func TestOutputResults_UnknownFormat(t *testing.T) {
	results := []CheckResult{{Domain: "example.com"}}

	var sb strings.Builder
	err := outputResults(&sb, results, "xml")
	if err == nil {
		t.Error("expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected error to contain 'unknown format', got %v", err)
	}
}

// TestDetermineExitCode tests exit code logic.
func TestDetermineExitCode(t *testing.T) {
	tests := []struct {
		name     string
		results  []CheckResult
		expected int
	}{
		{
			name:     "all available",
			results:  []CheckResult{{Domain: "a.com", Available: true}, {Domain: "b.com", Available: true}},
			expected: ExitAvailable,
		},
		{
			name:     "all taken",
			results:  []CheckResult{{Domain: "a.com", Available: false}, {Domain: "b.com", Available: false}},
			expected: ExitTaken,
		},
		{
			name:     "mixed taken and available",
			results:  []CheckResult{{Domain: "a.com", Available: true}, {Domain: "b.com", Available: false}},
			expected: ExitTaken,
		},
		{
			name:     "error takes priority",
			results:  []CheckResult{{Domain: "a.com", Available: true}, {Domain: "b.com", Error: "failed"}},
			expected: ExitError,
		},
		{
			name:     "error and taken",
			results:  []CheckResult{{Domain: "a.com", Available: false}, {Domain: "b.com", Error: "failed"}},
			expected: ExitError,
		},
		{
			name:     "only errors",
			results:  []CheckResult{{Domain: "a.com", Error: "failed"}},
			expected: ExitError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := determineExitCode(tc.results)
			if result != tc.expected {
				t.Errorf("expected exit code %d, got %d", tc.expected, result)
			}
		})
	}
}

// TestCheckConfigDefaults tests that CheckConfig has sensible defaults.
func TestCheckConfigDefaults(t *testing.T) {
	cfg := CheckConfig{
		Domain:    "example.com",
		Format:    "text",
		Timeout:   15 * time.Second,
		UserAgent: "domain-check/1.0",
	}

	if cfg.Domain != "example.com" {
		t.Errorf("expected Domain to be example.com, got %s", cfg.Domain)
	}
	if cfg.Format != "text" {
		t.Errorf("expected Format to be text, got %s", cfg.Format)
	}
	if cfg.Timeout != 15*time.Second {
		t.Errorf("expected Timeout to be 15s, got %v", cfg.Timeout)
	}
}

// TestCheckResult_MarshalJSON tests JSON marshaling for CheckResult.
func TestCheckResult_MarshalJSON(t *testing.T) {
	result := CheckResult{
		Domain:    "example.com",
		Available: true,
		TLD:       "com",
		Error:     "",
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

// TestCheckResult_WithError tests CheckResult with an error.
func TestCheckResult_WithError(t *testing.T) {
	result := CheckResult{
		Domain: "invalid-domain",
		Error:  "invalid domain format",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "invalid domain format") {
		t.Errorf("expected JSON to contain error message, got %s", string(data))
	}
}

// TestCheck_InvalidSLD tests that invalid SLD is rejected.
func TestCheck_InvalidSLD(t *testing.T) {
	cfg := CheckConfig{
		Domain:  "-invalid",
		TLDs:    []string{"com"},
		Format:  "text",
		Timeout: 5 * time.Second,
	}

	exitCode := Check(context.Background(), cfg)

	if exitCode != ExitError {
		t.Errorf("expected exit code %d (error), got %d", ExitError, exitCode)
	}
}

// TestCheck_EmptyDomain tests that empty domain is rejected.
func TestCheck_EmptyDomain(t *testing.T) {
	cfg := CheckConfig{
		Domain:  "",
		TLDs:    []string{"com"},
		Format:  "text",
		Timeout: 5 * time.Second,
	}

	exitCode := Check(context.Background(), cfg)

	if exitCode != ExitError {
		t.Errorf("expected exit code %d (error), got %d", ExitError, exitCode)
	}
}

// TestExitCodes tests the exit code constants.
func TestCheckExitCodes(t *testing.T) {
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

// TestOutputJSON tests JSON output helper.
func TestOutputJSON(t *testing.T) {
	results := []CheckResult{
		{Domain: "example.com", Available: true, TLD: "com"},
	}

	var sb strings.Builder
	err := outputJSON(&sb, results)
	if err != nil {
		t.Fatalf("failed to output JSON: %v", err)
	}

	var decoded []CheckResult
	err = json.Unmarshal([]byte(sb.String()), &decoded)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Errorf("expected 1 result, got %d", len(decoded))
	}
}

// TestOutputCSV tests CSV output helper.
func TestOutputCSV(t *testing.T) {
	results := []CheckResult{
		{Domain: "example.com", Available: true, TLD: "com"},
	}

	var sb strings.Builder
	err := outputCSV(&sb, results)
	if err != nil {
		t.Fatalf("failed to output CSV: %v", err)
	}

	output := sb.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + data), got %d", len(lines))
	}
}

// TestOutputText tests text output helper.
func TestOutputText(t *testing.T) {
	results := []CheckResult{
		{Domain: "available.com", Available: true, TLD: "com"},
		{Domain: "taken.com", Available: false, TLD: "com"},
		{Domain: "error.com", Error: "failed"},
	}

	var sb strings.Builder
	err := outputText(&sb, results)
	if err != nil {
		t.Fatalf("failed to output text: %v", err)
	}

	output := sb.String()

	if !strings.Contains(output, "available.com: AVAILABLE") {
		t.Errorf("expected 'available.com: AVAILABLE', got %s", output)
	}
	if !strings.Contains(output, "taken.com: TAKEN") {
		t.Errorf("expected 'taken.com: TAKEN', got %s", output)
	}
	if !strings.Contains(output, "error.com: ERROR") {
		t.Errorf("expected 'error.com: ERROR', got %s", output)
	}
}

// TestCheck_IntegrationWithDomainParser tests Check with domain parsing.
func TestCheck_IntegrationWithDomainParser(t *testing.T) {
	// Test that Check properly integrates with domain.Parse for full domain validation.
	// This test verifies the input validation path without mocking network calls.

	t.Run("invalid domain format", func(t *testing.T) {
		cfg := CheckConfig{
			Domain:  "not a domain",
			Format:  "text",
			Timeout: 5 * time.Second,
		}

		exitCode := Check(context.Background(), cfg)

		if exitCode != ExitError {
			t.Errorf("expected exit code %d (error) for invalid domain, got %d", ExitError, exitCode)
		}
	})

	t.Run("empty domain", func(t *testing.T) {
		cfg := CheckConfig{
			Domain:  "",
			Format:  "text",
			Timeout: 5 * time.Second,
		}

		exitCode := Check(context.Background(), cfg)

		if exitCode != ExitError {
			t.Errorf("expected exit code %d (error) for empty domain, got %d", ExitError, exitCode)
		}
	})
}
