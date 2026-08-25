// Package cli provides command-line interface functionality for domain-check.
package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jedarden/domain-check/internal/bootstrap"
	"github.com/jedarden/domain-check/internal/domain"
	"github.com/jedarden/domain-check/internal/rdap"
	"github.com/jedarden/domain-check/internal/ratelimit"
)

// CheckConfig holds configuration for the check subcommand.
type CheckConfig struct {
	Domain       string        // Domain to check
	TLDs         []string      // TLDs to expand to (if empty, just check the input domain)
	Format       string        // Output format: text, json, csv
	Timeout      time.Duration // HTTP timeout for RDAP queries
	UserAgent    string        // User-Agent header for RDAP requests
	Watch        bool          // Watch mode: poll until domain becomes available
	WatchForever bool          // Continue watching even after domain becomes available
	WatchInterval time.Duration // Polling interval for watch mode (default 5m)
}

// Exit codes for the check subcommand.
const (
	ExitAvailable = 0 // Domain is available
	ExitTaken     = 1 // Domain is taken/registered
	ExitError     = 2 // Error occurred
)

// CheckResult holds the result of checking a single domain.
type CheckResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	TLD       string `json:"tld"`
	Error     string `json:"error,omitempty"`
}

// Check runs domain availability checks and outputs results.
// Returns the exit code based on the results.
func Check(ctx context.Context, cfg CheckConfig) int {
	// Build the list of domains to check.
	var domains []string
	if len(cfg.TLDs) > 0 {
		// Input is an SLD without TLD - validate it as a label.
		sld := strings.ToLower(strings.TrimSpace(cfg.Domain))
		if err := validateSLD(sld); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return ExitError
		}
		for _, tld := range cfg.TLDs {
			domains = append(domains, sld+"."+tld)
		}
	} else {
		// Input is a full domain - validate and normalize.
		parsed, err := domain.Parse(cfg.Domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return ExitError
		}
		domains = []string{parsed.Domain}
	}

	// Initialize the checker components.
	bootstrap, err := bootstrap.NewManager(ctx, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize bootstrap: %v\n", err)
		return ExitError
	}
	defer bootstrap.Stop()

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	rateLimiter := ratelimit.NewRateLimiter()

	rdapClient := rdap.NewRDAPClient(rdap.RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  rateLimiter,
		UserAgent:  cfg.UserAgent,
	})

	// If not in watch mode, run a single check and exit.
	if !cfg.Watch {
		return checkOnce(ctx, rdapClient, domains, cfg.Format)
	}

	// Watch mode: poll until all domains become available (or forever if --forever is set).
	interval := cfg.WatchInterval
	if interval == 0 {
		interval = 5 * time.Minute // Default interval
	}

	fmt.Fprintf(os.Stderr, "Watching %d domain(s), checking every %v...\n", len(domains), interval)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n\n")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial check immediately.
	if results := checkAndReport(ctx, rdapClient, domains, cfg.Format); results != nil {
		if shouldExitWatch(results, cfg.WatchForever) {
			return ExitAvailable
		}
	}

	// Poll periodically.
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\nWatch interrupted.\n")
			return ExitError
		case <-ticker.C:
			if results := checkAndReport(ctx, rdapClient, domains, cfg.Format); results != nil {
				if shouldExitWatch(results, cfg.WatchForever) {
					return ExitAvailable
				}
			}
		}
	}
}

// checkOnce performs a single check and returns the exit code.
func checkOnce(ctx context.Context, rdapClient *rdap.RDAPClient, domains []string, format string) int {
	results := make([]CheckResult, len(domains))
	for i, d := range domains {
		result, err := rdapClient.Check(ctx, d)
		if err != nil {
			results[i] = CheckResult{
				Domain: d,
				Error:  err.Error(),
			}
			continue
		}
		results[i] = CheckResult{
			Domain:    result.Domain,
			Available: result.Available,
			TLD:       result.TLD,
			Error:     result.Error,
		}
	}

	// Output results in the requested format.
	if err := outputResults(os.Stdout, results, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitError
	}

	// Determine exit code.
	return determineExitCode(results)
}

// checkAndReport performs a check, prints results with timestamp, and returns the results.
// Returns nil if an error occurred during output.
func checkAndReport(ctx context.Context, rdapClient *rdap.RDAPClient, domains []string, format string) []CheckResult {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] Checking...\n", timestamp)

	results := make([]CheckResult, len(domains))
	for i, d := range domains {
		result, err := rdapClient.Check(ctx, d)
		if err != nil {
			results[i] = CheckResult{
				Domain: d,
				Error:  err.Error(),
			}
			continue
		}
		results[i] = CheckResult{
			Domain:    result.Domain,
			Available: result.Available,
			TLD:       result.TLD,
			Error:     result.Error,
		}
	}

	// Output results in the requested format.
	if err := outputResults(os.Stdout, results, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return nil
	}

	return results
}

// shouldExitWatch determines if the watch should exit with success.
// Exits if all domains are available and --forever is not set.
func shouldExitWatch(results []CheckResult, watchForever bool) bool {
	if watchForever {
		return false
	}

	// Check if all domains are available with no errors.
	for _, r := range results {
		if r.Error != "" || !r.Available {
			return false
		}
	}

	// All domains are available!
	fmt.Fprintf(os.Stderr, "\n✓ All domain(s) are now available!\n")
	return true
}

// validateSLD validates a second-level domain label.
// It checks length (1-63 chars) and allowed characters (a-z, 0-9, hyphen).
func validateSLD(sld string) error {
	if sld == "" {
		return fmt.Errorf("domain label is empty")
	}
	if len(sld) > 63 {
		return fmt.Errorf("domain label exceeds 63 characters (%d)", len(sld))
	}
	if sld[0] == '-' {
		return fmt.Errorf("domain label starts with hyphen: %q", sld)
	}
	if sld[len(sld)-1] == '-' {
		return fmt.Errorf("domain label ends with hyphen: %q", sld)
	}
	for i, c := range sld {
		if !isLDH(c) {
			return fmt.Errorf("domain label contains invalid character %q at position %d", c, i+1)
		}
	}
	return nil
}

// isLDH reports whether c is a valid LDH character: a-z, 0-9, or hyphen.
func isLDH(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

// outputResults writes the results in the specified format.
func outputResults(w io.Writer, results []CheckResult, format string) error {
	switch format {
	case "json":
		return outputJSON(w, results)
	case "csv":
		return outputCSV(w, results)
	case "text":
		return outputText(w, results)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// outputJSON outputs results as JSON.
func outputJSON(w io.Writer, results []CheckResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// outputCSV outputs results as CSV.
func outputCSV(w io.Writer, results []CheckResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Write header.
	if err := cw.Write([]string{"domain", "available", "tld", "error"}); err != nil {
		return err
	}

	// Write rows.
	for _, r := range results {
		available := "false"
		if r.Available {
			available = "true"
		}
		if err := cw.Write([]string{r.Domain, available, r.TLD, r.Error}); err != nil {
			return err
		}
	}

	return nil
}

// outputText outputs results as human-readable text.
func outputText(w io.Writer, results []CheckResult) error {
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(w, "%s: ERROR - %s\n", r.Domain, r.Error)
			continue
		}
		if r.Available {
			fmt.Fprintf(w, "%s: AVAILABLE\n", r.Domain)
		} else {
			fmt.Fprintf(w, "%s: TAKEN\n", r.Domain)
		}
	}
	return nil
}

// determineExitCode returns the appropriate exit code based on results.
func determineExitCode(results []CheckResult) int {
	hasError := false
	hasTaken := false

	for _, r := range results {
		if r.Error != "" {
			hasError = true
		} else if !r.Available {
			hasTaken = true
		}
	}

	// Priority: error > taken > available
	if hasError {
		return ExitError
	}
	if hasTaken {
		return ExitTaken
	}
	return ExitAvailable
}
