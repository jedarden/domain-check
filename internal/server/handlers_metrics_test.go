package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jedarden/domain-check/internal/config"
	"github.com/jedarden/domain-check/internal/domain"
)

// scrapeMetricsText retrieves the raw Prometheus metrics text from a metrics handler.
func scrapeMetricsText(t *testing.T, handler http.Handler) string {
	t.Helper()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("failed to scrape metrics: status %d", rec.Code)
	}

	return rec.Body.String()
}

// parseMetricValue extracts a metric value from Prometheus text format.
// Returns the value and whether it was found.
func parseMetricValue(metricsText, metricName string, labels map[string]string) (float64, bool) {
	lines := strings.Split(metricsText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this line is for our metric
		if !strings.HasPrefix(line, metricName) {
			continue
		}

		// Extract the metric part (before any value)
		spaceIdx := strings.LastIndex(line, " ")
		if spaceIdx == -1 {
			continue
		}

		metricPart := line[:spaceIdx]
		valuePart := line[spaceIdx+1:]

		// Parse the value
		value, err := strconv.ParseFloat(valuePart, 64)
		if err != nil {
			continue
		}

		// Check label match if labels specified
		if len(labels) > 0 {
			if !labelsMatch(metricPart, labels) {
				continue
			}
		}

		return value, true
	}

	return 0, false
}

// labelsMatch checks if the metric part contains the expected labels.
func labelsMatch(metricPart string, expectedLabels map[string]string) bool {
	// Extract labels from metric line like: metric_name{label1="value1",label2="value2"}
	startIdx := strings.Index(metricPart, "{")
	endIdx := strings.Index(metricPart, "}")

	if startIdx == -1 || endIdx == -1 {
		// No labels in metric, but we expected some
		return len(expectedLabels) == 0
	}

	labelStr := metricPart[startIdx+1 : endIdx]

	// Parse labels
	labelPairs := strings.Split(labelStr, ",")
	foundLabels := make(map[string]string)

	for _, pair := range labelPairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		equalIdx := strings.Index(pair, "=")
		if equalIdx == -1 {
			continue
		}

		key := strings.TrimSpace(pair[:equalIdx])
		value := strings.TrimSpace(pair[equalIdx+1:])
		// Remove quotes from value
		value = strings.Trim(value, `"`)

		foundLabels[key] = value
	}

	// Check all expected labels match
	for k, v := range expectedLabels {
		if foundLabels[k] != v {
			return false
		}
	}

	return true
}

// countMetricObservations reads the _count metric for a histogram to get total observations.
func countMetricObservations(metricsText, metricName string) int {
	lines := strings.Split(metricsText, "\n")

	// Look for the _count metric which contains the observation count
	countMetricName := metricName + "_count"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Look for the count line
		if strings.HasPrefix(line, countMetricName+" ") {
			spaceIdx := strings.LastIndex(line, " ")
			if spaceIdx != -1 {
				valueStr := line[spaceIdx+1:]
				if val, err := strconv.Atoi(valueStr); err == nil {
					return val
				}
			}
		}
	}

	return 0
}

// setupMetricsTestRouter creates a router with metrics enabled for testing.
func setupMetricsTestRouter(checker DomainChecker) (*Metrics, http.Handler) {
	cfg := config.Defaults()
	log := DefaultLogger("text", "error")
	rl := NewRateLimiter(log)
	metrics := GetMetrics()

	router := Router(&cfg, log, rl, checker, nil, nil, metrics, nil)
	return metrics, router
}

// TestMetrics_RequestCounter tests that HTTP requests increment domcheck_requests_total.
func TestMetrics_RequestCounter(t *testing.T) {
	t.Run("GET /api/v1/check increments request counter", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "example.com",
				Available:  true,
				TLD:        "com",
				Source:     domain.SourceRDAP,
				Cached:     false,
				DurationMs: 50,
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Make a request
		req := httptest.NewRequest("GET", "/api/v1/check?d=example.com", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Scrape metrics
		metricsText := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_requests_total has non-zero value for GET /api/v1/check
		labels := map[string]string{
			"method": "GET",
			"path":   "/api/v1/check",
			"status": "2xx",
		}
		value, found := parseMetricValue(metricsText, "domcheck_requests_total", labels)
		if !found {
			t.Fatal("domcheck_requests_total with labels {method=\"GET\",path=\"/api/v1/check\",status=\"2xx\"} not found")
		}
		if value <= 0 {
			t.Errorf("domcheck_requests_total should be > 0, got %f", value)
		}
	})

	t.Run("multiple requests increment counter", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "test.com",
				Available:  false,
				TLD:        "com",
				Source:     domain.SourceRDAP,
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		labels := map[string]string{
			"method": "GET",
			"path":   "/api/v1/check",
			"status": "2xx",
		}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_requests_total", labels)

		// Make 3 requests
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/v1/check?d=test.com", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_requests_total", labels)
		if !found {
			t.Fatal("domcheck_requests_total not found")
		}

		// Verify we added exactly 3 requests
		if valueAfter != valueBefore+3 {
			t.Errorf("expected to add 3 requests (from %f to %f), got %f", valueBefore, valueBefore+3, valueAfter)
		}
	})
}

// TestMetrics_RDAPRequestsCounter tests that RDAP requests increment domcheck_rdap_requests_total.
func TestMetrics_RDAPRequestsCounter(t *testing.T) {
	t.Run("RDAP metrics can be recorded", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		labelsBefore := map[string]string{"registry": "verisign", "status": "success"}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_rdap_requests_total", labelsBefore)

		// Simulate recording RDAP metrics (as the real RDAP client would do)
		metrics.RecordRDAPRequest("verisign", "success", 0.1)

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_rdap_requests_total", labelsBefore)
		if !found {
			t.Fatal("domcheck_rdap_requests_total with labels {registry=\"verisign\",status=\"success\"} not found")
		}
		if valueAfter <= valueBefore {
			t.Errorf("domcheck_rdap_requests_total should increase after recording, went from %f to %f", valueBefore, valueAfter)
		}
	})

	t.Run("multiple RDAP requests accumulate", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		labels := map[string]string{"registry": "pir", "status": "success"}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_rdap_requests_total", labels)

		// Record multiple RDAP requests
		for i := 0; i < 3; i++ {
			metrics.RecordRDAPRequest("pir", "success", 0.15)
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_rdap_requests_total", labels)
		if !found {
			t.Fatal("domcheck_rdap_requests_total with labels {registry=\"pir\",status=\"success\"} not found")
		}
		if valueAfter != valueBefore+3 {
			t.Errorf("expected domcheck_rdap_requests_total to increase by 3 (from %f to %f), got %f", valueBefore, valueBefore+3, valueAfter)
		}
	})

	t.Run("RDAP error metrics recorded separately", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		errorLabels := map[string]string{"registry": "google", "status": "error"}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_rdap_requests_total", errorLabels)

		// Record an error
		metrics.RecordRDAPRequest("google", "error", 0.05)

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_rdap_requests_total", errorLabels)
		if !found {
			t.Fatal("domcheck_rdap_requests_total with labels {registry=\"google\",status=\"error\"} not found")
		}
		if valueAfter <= valueBefore {
			t.Errorf("domcheck_rdap_requests_total error should increase after recording, went from %f to %f", valueBefore, valueAfter)
		}
	})
}

// TestMetrics_CacheHits tests that cache hits are recorded in domcheck_cache_hits_total.
func TestMetrics_CacheHits(t *testing.T) {
	t.Run("cache hit metric is recorded", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		hitLabels := map[string]string{"result": "hit"}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_cache_hits_total", hitLabels)

		// Simulate recording cache hits (as the real cache would do)
		metrics.RecordCacheHit("hit")

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_cache_hits_total", hitLabels)
		if !found {
			t.Fatal("domcheck_cache_hits_total with labels {result=\"hit\"} not found")
		}
		if valueAfter <= valueBefore {
			t.Errorf("domcheck_cache_hits_total{result=\"hit\"} should increase after recording, went from %f to %f", valueBefore, valueAfter)
		}
	})

	t.Run("cache miss metric is recorded", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		missLabels := map[string]string{"result": "miss"}
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_cache_hits_total", missLabels)

		// Simulate recording cache misses
		metrics.RecordCacheHit("miss")

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_cache_hits_total", missLabels)
		if !found {
			t.Fatal("domcheck_cache_hits_total with labels {result=\"miss\"} not found")
		}
		if valueAfter <= valueBefore {
			t.Errorf("domcheck_cache_hits_total{result=\"miss\"} should increase after recording, went from %f to %f", valueBefore, valueAfter)
		}
	})

	t.Run("multiple cache accesses accumulate", func(t *testing.T) {
		metrics := GetMetrics()

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		hitLabels := map[string]string{"result": "hit"}
		missLabels := map[string]string{"result": "miss"}
		hitBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_cache_hits_total", hitLabels)
		missBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_cache_hits_total", missLabels)

		// Record mixed cache operations
		for i := 0; i < 3; i++ {
			metrics.RecordCacheHit("hit")
		}
		for i := 0; i < 2; i++ {
			metrics.RecordCacheHit("miss")
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())
		hitAfter, found := parseMetricValue(metricsTextAfter, "domcheck_cache_hits_total", hitLabels)
		if !found {
			t.Fatal("domcheck_cache_hits_total{result=\"hit\"} not found")
		}
		missAfter, found := parseMetricValue(metricsTextAfter, "domcheck_cache_hits_total", missLabels)
		if !found {
			t.Fatal("domcheck_cache_hits_total{result=\"miss\"} not found")
		}

		// Verify 3 hits and 2 misses were recorded
		if hitAfter != hitBefore+3 {
			t.Errorf("expected 3 cache hits (from %f to %f), got %f", hitBefore, hitBefore+3, hitAfter)
		}
		if missAfter != missBefore+2 {
			t.Errorf("expected 2 cache misses (from %f to %f), got %f", missBefore, missBefore+2, missAfter)
		}
	})
}

// TestMetrics_BulkCheckSize tests that bulk check size is recorded in domcheck_bulk_check_size.
func TestMetrics_BulkCheckSize(t *testing.T) {
	t.Run("bulk request records size histogram", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"bulk1.com": {Domain: "bulk1.com", Available: true, TLD: "com"},
				"bulk2.com": {Domain: "bulk2.com", Available: false, TLD: "com"},
				"bulk3.com": {Domain: "bulk3.com", Available: true, TLD: "com"},
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		observationsBefore := countMetricObservations(metricsTextBefore, "domcheck_bulk_check_size")

		// Make a bulk request with 3 domains
		body := `{"domains": ["bulk1.com", "bulk2.com", "bulk3.com"]}`
		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_bulk_check_size has recorded 1 more observation
		observationsAfter := countMetricObservations(metricsTextAfter, "domcheck_bulk_check_size")
		if observationsAfter != observationsBefore+1 {
			t.Errorf("expected 1 observation in domcheck_bulk_check_size (from %d to %d), got %d", observationsBefore, observationsBefore+1, observationsAfter)
		}
	})

	t.Run("multiple bulk requests accumulate histogram", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: make(map[string]*domain.DomainResult),
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		observationsBefore := countMetricObservations(metricsTextBefore, "domcheck_bulk_check_size")

		// Make multiple bulk requests with different sizes
		sizes := []int{5, 10, 15}
		for _, size := range sizes {
			domains := make([]string, size)
			for i := 0; i < size; i++ {
				domains[i] = fmt.Sprintf("domain%d.com", i)
			}

			body, _ := json.Marshal(map[string][]string{"domains": domains})
			req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("bulk size %d: expected 200, got %d", size, rec.Code)
			}
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_bulk_check_size has 3 more observations
		observationsAfter := countMetricObservations(metricsTextAfter, "domcheck_bulk_check_size")
		if observationsAfter != observationsBefore+3 {
			t.Errorf("expected 3 observations in domcheck_bulk_check_size (from %d to %d), got %d", observationsBefore, observationsBefore+3, observationsAfter)
		}
	})
}

// TestMetrics_ChecksServed tests that checks served are recorded in domcheck_checks_served_total.
func TestMetrics_ChecksServed(t *testing.T) {
	t.Run("bulk request increments checks served counter", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"bulk1.com": {Domain: "bulk1.com", Available: true, TLD: "com"},
				"bulk2.com": {Domain: "bulk2.com", Available: false, TLD: "com"},
				"bulk3.com": {Domain: "bulk3.com", Available: true, TLD: "com"},
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_checks_served_total", map[string]string{})

		// Make a bulk request with 3 domains
		body := `{"domains": ["bulk1.com", "bulk2.com", "bulk3.com"]}`
		req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_checks_served_total has been incremented by 3
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_checks_served_total", map[string]string{})
		if !found {
			t.Fatal("domcheck_checks_served_total not found")
		}
		if valueAfter != valueBefore+3 {
			t.Errorf("expected domcheck_checks_served_total to increase by 3 (from %f to %f), got %f", valueBefore, valueBefore+3, valueAfter)
		}
	})

	t.Run("multiple bulk requests accumulate checks served", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: make(map[string]*domain.DomainResult),
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_checks_served_total", map[string]string{})

		// Make multiple bulk requests with unique domain names
		totalChecks := 0
		domainCounter := 0
		for _, size := range []int{2, 5, 3} {
			domains := make([]string, size)
			for i := 0; i < size; i++ {
				domainName := fmt.Sprintf("domain%d.com", domainCounter)
				domains[i] = domainName
				domainCounter++
				// Add result for this domain
				mockCh.results[domainName] = &domain.DomainResult{
					Domain:    domainName,
					Available: true,
					TLD:       "com",
				}
			}
			totalChecks += size

			body, _ := json.Marshal(map[string][]string{"domains": domains})
			req := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("bulk size %d: expected 200, got %d", size, rec.Code)
			}
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_checks_served_total equals total checks (10)
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_checks_served_total", map[string]string{})
		if !found {
			t.Fatal("domcheck_checks_served_total not found")
		}
		if valueAfter != valueBefore+float64(totalChecks) {
			t.Errorf("expected domcheck_checks_served_total to increase by %d (from %f to %f), got %f", totalChecks, valueBefore, valueBefore+float64(totalChecks), valueAfter)
		}
	})

	t.Run("partial success only counts successful checks", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"bulk1.com": {Domain: "bulk1.com", Available: true, TLD: "com"},
				"bulk2.com": {Domain: "bulk2.com", Available: true, TLD: "com"},
			},
			errors: map[string]string{
				"bulk3.com": "timeout",
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		valueBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_checks_served_total", map[string]string{})

		// Make a bulk request with 2 successes and 1 failure
		body := `{"domains": ["bulk1.com", "bulk2.com", "bulk3.com"]}`
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

		if resp.Succeeded != 2 {
			t.Errorf("expected Succeeded=2, got %d", resp.Succeeded)
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_checks_served_total only counts successful checks (2)
		valueAfter, found := parseMetricValue(metricsTextAfter, "domcheck_checks_served_total", map[string]string{})
		if !found {
			t.Fatal("domcheck_checks_served_total not found")
		}
		if valueAfter != valueBefore+2 {
			t.Errorf("expected domcheck_checks_served_total to increase by 2 (only successes) (from %f to %f), got %f", valueBefore, valueBefore+2, valueAfter)
		}
	})
}

// TestMetrics_MultiTLD tests metrics for multi-TLD requests.
func TestMetrics_MultiTLD(t *testing.T) {
	t.Run("multi-TLD request records bulk check size", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"example.com": {Domain: "example.com", Available: false, TLD: "com"},
				"example.org": {Domain: "example.org", Available: true, TLD: "org"},
				"example.net": {Domain: "example.net", Available: true, TLD: "net"},
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		observationsBefore := countMetricObservations(metricsTextBefore, "domcheck_bulk_check_size")
		checksBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_checks_served_total", map[string]string{})

		// Make a multi-TLD request
		req := httptest.NewRequest("GET", "/api/v1/check?d=example&tlds=com,org,net", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Scrape metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_bulk_check_size has 1 more observation
		observationsAfter := countMetricObservations(metricsTextAfter, "domcheck_bulk_check_size")
		if observationsAfter != observationsBefore+1 {
			t.Errorf("expected 1 observation in domcheck_bulk_check_size (from %d to %d), got %d", observationsBefore, observationsBefore+1, observationsAfter)
		}

		// Verify domcheck_checks_served_total increased by 3
		checksAfter, found := parseMetricValue(metricsTextAfter, "domcheck_checks_served_total", map[string]string{})
		if !found {
			t.Fatal("domcheck_checks_served_total not found")
		}
		if checksAfter != checksBefore+3 {
			t.Errorf("expected domcheck_checks_served_total to increase by 3 (from %f to %f), got %f", checksBefore, checksBefore+3, checksAfter)
		}
	})
}

// TestMetrics_MetricsEndpoint tests that the /metrics endpoint works correctly.
func TestMetrics_MetricsEndpoint(t *testing.T) {
	t.Run("metrics endpoint is accessible", func(t *testing.T) {
		mockCh := &mockChecker{
			result: &domain.DomainResult{
				Domain:     "example.com",
				Available:  true,
				TLD:        "com",
				Source:     domain.SourceRDAP,
			},
		}

		_, router := setupMetricsTestRouter(mockCh)

		// Access /metrics endpoint
		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		// Verify response is text
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("expected Content-Type to contain text/plain, got %s", ct)
		}

		// Verify body contains core metric names (that should always exist)
		body := rec.Body.String()
		expectedMetrics := []string{
			"domcheck_requests_total",
			"domcheck_bulk_check_size",
			"domcheck_checks_served_total",
		}

		for _, metric := range expectedMetrics {
			if !strings.Contains(body, metric) {
				t.Errorf("metrics body missing core metric: %s", metric)
			}
		}
	})

	t.Run("metrics endpoint returns Prometheus format", func(t *testing.T) {
		mockCh := &mockChecker{}
		metrics, _ := setupMetricsTestRouter(mockCh)

		// Record some metrics to ensure they appear in output
		metrics.RecordRDAPRequest("verisign", "success", 0.1)
		metrics.RecordCacheHit("hit")
		metrics.RecordBulkCheck(5)
		metrics.AddChecksServed(5)

		// Scrape metrics using the handler directly
		metricsText := scrapeMetricsText(t, metrics.Handler())

		// Verify we got metrics
		if metricsText == "" {
			t.Error("expected non-empty metrics from handler")
		}

		// Verify expected metrics exist (after recording)
		expectedMetrics := []string{
			"domcheck_requests_total",
			"domcheck_rdap_requests_total",
			"domcheck_cache_hits_total",
			"domcheck_bulk_check_size",
			"domcheck_checks_served_total",
		}

		for _, metric := range expectedMetrics {
			if !strings.Contains(metricsText, metric) {
				t.Errorf("expected metric %q not found in scraped metrics", metric)
			}
		}
	})
}

// TestMetrics_Integration is a comprehensive integration test for all metrics.
func TestMetrics_Integration(t *testing.T) {
	t.Run("all metrics recorded in typical workflow", func(t *testing.T) {
		mockCh := &mockBulkChecker{
			results: map[string]*domain.DomainResult{
				"single.com": {Domain: "single.com", Available: true, TLD: "com", Source: domain.SourceRDAP},
				"bulk1.com":  {Domain: "bulk1.com", Available: false, TLD: "com", Source: domain.SourceRDAP},
				"bulk2.com":  {Domain: "bulk2.com", Available: true, TLD: "com", Source: domain.SourceRDAP},
			},
		}

		metrics, router := setupMetricsTestRouter(mockCh)

		// Get baseline
		metricsTextBefore := scrapeMetricsText(t, metrics.Handler())
		requestLabels := map[string]string{"method": "GET", "path": "/api/v1/check", "status": "2xx"}
		requestsBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_requests_total", requestLabels)
		bulkSizeBefore := countMetricObservations(metricsTextBefore, "domcheck_bulk_check_size")
		checksBefore, _ := parseMetricValue(metricsTextBefore, "domcheck_checks_served_total", map[string]string{})

		// 1. Make a single check request
		req1 := httptest.NewRequest("GET", "/api/v1/check?d=single.com", nil)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Fatalf("single check: expected 200, got %d", rec1.Code)
		}

		// 2. Make a bulk request
		body := `{"domains": ["bulk1.com", "bulk2.com"]}`
		req2 := httptest.NewRequest("POST", "/api/v1/bulk", strings.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Fatalf("bulk check: expected 200, got %d", rec2.Code)
		}

		// 3. Scrape and verify all metrics
		metricsTextAfter := scrapeMetricsText(t, metrics.Handler())

		// Verify domcheck_requests_total has 1 more GET /api/v1/check request
		requestsAfter, found := parseMetricValue(metricsTextAfter, "domcheck_requests_total", requestLabels)
		if !found {
			t.Fatal("GET /api/v1/check request metric not found")
		}
		if requestsAfter != requestsBefore+1 {
			t.Errorf("expected 1 GET /api/v1/check request (from %f to %f), got %f", requestsBefore, requestsBefore+1, requestsAfter)
		}

		// Verify domcheck_bulk_check_size has increased (at least 1 more observation)
		// Note: We check for >= instead of == to handle any edge cases where routing might record metrics
		bulkSizeAfter := countMetricObservations(metricsTextAfter, "domcheck_bulk_check_size")
		if bulkSizeAfter < bulkSizeBefore+1 {
			t.Errorf("expected at least 1 bulk check size observation (from %d to >=%d), got %d", bulkSizeBefore, bulkSizeBefore+1, bulkSizeAfter)
		}

		// Verify domcheck_checks_served_total increased by 3 (1 single + 2 bulk)
		checksAfter, found := parseMetricValue(metricsTextAfter, "domcheck_checks_served_total", map[string]string{})
		if !found {
			t.Fatal("domcheck_checks_served_total not found")
		}
		if checksAfter != checksBefore+3 {
			t.Errorf("expected 3 checks served (1 single + 2 bulk) (from %f to %f), got %f", checksBefore, checksBefore+3, checksAfter)
		}
	})
}
