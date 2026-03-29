// Package cli provides command-line interface functionality for domain-check.
package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coding/domain-check/internal/checker"
	"github.com/coding/domain-check/internal/domain"
)

// BulkConfig holds configuration for the bulk subcommand.
type BulkConfig struct {
	File        string        // Path to file containing domains (one per line)
	Format      string        // Output format: text, json, csv
	Concurrency int           // Number of concurrent checks
	Timeout     time.Duration // HTTP timeout for RDAP queries
	UserAgent   string        // User-Agent header for RDAP requests
	ShowProgress bool         // Show progress indicator
}

// BulkCheckResult holds the result of checking a single domain in bulk mode.
type BulkCheckResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	TLD       string `json:"tld"`
	Source    string `json:"source,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProgressWriter handles progress output for bulk operations.
type ProgressWriter struct {
	total      int32
	completed  int32
	start      time.Time
	showProgress bool
	mu         sync.Mutex
	lastUpdate time.Time
}

// NewProgressWriter creates a new progress writer.
func NewProgressWriter(total int, showProgress bool) *ProgressWriter {
	return &ProgressWriter{
		total:        int32(total),
		showProgress: showProgress,
		start:        time.Now(),
	}
}

// Increment atomically increments the completed count and updates progress.
func (p *ProgressWriter) Increment() {
	if !p.showProgress {
		return
	}

	completed := atomic.AddInt32(&p.completed, 1)

	// Throttle updates to avoid flickering (max 10 updates/sec).
	p.mu.Lock()
	now := time.Now()
	if now.Sub(p.lastUpdate) < 100*time.Millisecond {
		p.mu.Unlock()
		return
	}
	p.lastUpdate = now
	p.mu.Unlock()

	// Calculate progress.
	percent := float64(completed) / float64(atomic.LoadInt32(&p.total)) * 100
	elapsed := time.Since(p.start).Round(time.Millisecond)

	// Calculate estimated time remaining.
	var eta time.Duration
	if completed > 0 {
		rate := float64(completed) / elapsed.Seconds()
		remaining := float64(atomic.LoadInt32(&p.total)-completed) / rate
		eta = time.Duration(remaining * float64(time.Second))
	}

	// Print progress to stderr (so stdout remains clean for results).
	fmt.Fprintf(os.Stderr, "\rProgress: %d/%d (%.1f%%) | Elapsed: %s | ETA: %s    ",
		completed, atomic.LoadInt32(&p.total), percent, elapsed.Round(time.Second), eta.Round(time.Second))
}

// Finish clears the progress line and prints a summary.
func (p *ProgressWriter) Finish() {
	if !p.showProgress {
		return
	}

	elapsed := time.Since(p.start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "\rCompleted: %d domains in %s (%.1f domains/sec)    \n",
		atomic.LoadInt32(&p.completed), elapsed, float64(p.completed)/elapsed.Seconds())
}

// Bulk runs bulk domain availability checks and outputs results.
// Returns the exit code based on the results.
func Bulk(ctx context.Context, cfg BulkConfig) int {
	// Read domains from file.
	domains, err := readDomainsFromFile(cfg.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read domains file: %v\n", err)
		return ExitError
	}

	if len(domains) == 0 {
		fmt.Fprintln(os.Stderr, "error: no domains found in file")
		return ExitError
	}

	// Initialize the checker components.
	bootstrap, err := checker.NewBootstrapManager(ctx, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize bootstrap: %v\n", err)
		return ExitError
	}
	defer bootstrap.Stop()

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	rateLimiter := checker.NewRateLimiter()

	rdapClient := checker.NewRDAPClient(checker.RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  rateLimiter,
		UserAgent:  cfg.UserAgent,
	})

	// Create the checker with bulk configuration.
	bulkConfig := checker.DefaultBulkCheckConfig()
	if cfg.Concurrency > 0 {
		bulkConfig.GlobalConcurrency = cfg.Concurrency
	}

	chk := checker.NewChecker(checker.CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
		BulkConfig: bulkConfig,
	})

	// For text format, stream results as they complete.
	// For json/csv, collect all results first.
	if cfg.Format == "text" {
		return bulkStreaming(ctx, chk, domains, cfg)
	}

	return bulkCollected(ctx, chk, domains, cfg)
}

// bulkStreaming handles bulk checks with streaming text output.
func bulkStreaming(ctx context.Context, chk *checker.Checker, domains []string, cfg BulkConfig) int {
	progress := NewProgressWriter(len(domains), cfg.ShowProgress)

	// Create a channel for results.
	type checkResult struct {
		domain string
		result *domain.DomainResult
		err    error
	}
	results := make(chan checkResult, cfg.Concurrency)

	// Start a goroutine to print results as they come in.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range results {
			progress.Increment()

			if r.err != nil {
				fmt.Fprintf(os.Stdout, "%s: ERROR - %s\n", r.domain, r.err)
				continue
			}
			if r.result.Available {
				fmt.Fprintf(os.Stdout, "%s: AVAILABLE\n", r.result.Domain)
			} else {
				fmt.Fprintf(os.Stdout, "%s: TAKEN\n", r.result.Domain)
			}
		}
	}()

	// Process domains with limited concurrency.
	var sem chan struct{}
	if cfg.Concurrency > 0 {
		sem = make(chan struct{}, cfg.Concurrency)
	} else {
		sem = make(chan struct{}, 20) // default concurrency
	}

	var hasError atomic.Bool
	var hasTaken atomic.Bool

	for _, d := range domains {
		// Check context before starting.
		select {
		case <-ctx.Done():
			hasError.Store(true)
			break
		default:
		}

		sem <- struct{}{} // acquire

		go func(domain string) {
			defer func() { <-sem }() // release

			res, err := chk.Check(ctx, domain)
			results <- checkResult{domain: domain, result: res, err: err}

			if err != nil {
				hasError.Store(true)
			} else if res != nil && !res.Available {
				hasTaken.Store(true)
			}
		}(d)
	}

	// Wait for all goroutines to complete.
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(results)
	wg.Wait()

	progress.Finish()

	// Determine exit code.
	if hasError.Load() {
		return ExitError
	}
	if hasTaken.Load() {
		return ExitTaken
	}
	return ExitAvailable
}

// bulkCollected handles bulk checks with collected output (json/csv).
func bulkCollected(ctx context.Context, chk *checker.Checker, domains []string, cfg BulkConfig) int {
	progress := NewProgressWriter(len(domains), cfg.ShowProgress)

	// Run bulk check using the checker's built-in bulk method.
	bulkResult := chk.CheckBulk(ctx, domains)

	progress.Finish()

	// Convert results to output format.
	outputResults := make([]BulkCheckResult, 0, len(domains))

	// Process successful results.
	for _, d := range domains {
		normalized := strings.ToLower(strings.TrimSpace(d))
		normalized = strings.TrimRight(normalized, ".")

		if r, ok := bulkResult.Results[normalized]; ok {
			outputResults = append(outputResults, BulkCheckResult{
				Domain:     r.Domain,
				Available:  r.Available,
				TLD:        r.TLD,
				Source:     string(r.Source),
				DurationMs: r.DurationMs,
				Error:      r.Error,
			})
		} else if errMsg, ok := bulkResult.Errors[normalized]; ok {
			outputResults = append(outputResults, BulkCheckResult{
				Domain: normalized,
				Error:  errMsg,
			})
		} else {
			// Domain was in input but not in results or errors (shouldn't happen).
			outputResults = append(outputResults, BulkCheckResult{
				Domain: normalized,
				Error:  "no result returned",
			})
		}
	}

	// Output results.
	var err error
	switch cfg.Format {
	case "json":
		err = outputBulkJSON(os.Stdout, outputResults)
	case "csv":
		err = outputBulkCSV(os.Stdout, outputResults)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitError
	}

	// Determine exit code.
	if len(bulkResult.Errors) > 0 {
		return ExitError
	}
	for _, r := range bulkResult.Results {
		if !r.Available {
			return ExitTaken
		}
	}
	return ExitAvailable
}

// readDomainsFromFile reads domains from a file, one per line.
// Blank lines and lines starting with # are ignored.
func readDomainsFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse and validate the domain.
		parsed, err := domain.Parse(line)
		if err != nil {
			// Log warning but continue with other domains.
			fmt.Fprintf(os.Stderr, "warning: skipping invalid domain %q: %v\n", line, err)
			continue
		}

		domains = append(domains, parsed.Domain)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return domains, nil
}

// outputBulkJSON outputs bulk results as a JSON array.
func outputBulkJSON(w io.Writer, results []BulkCheckResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// outputBulkCSV outputs bulk results as CSV.
func outputBulkCSV(w io.Writer, results []BulkCheckResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Write header.
	if err := cw.Write([]string{"domain", "available", "tld", "source", "duration_ms", "error"}); err != nil {
		return err
	}

	// Write rows.
	for _, r := range results {
		available := "false"
		if r.Available {
			available = "true"
		}
		duration := fmt.Sprintf("%d", r.DurationMs)
		if err := cw.Write([]string{r.Domain, available, r.TLD, r.Source, duration, r.Error}); err != nil {
			return err
		}
	}

	return nil
}
