// Package server provides Prometheus metrics for domain-check.
package server

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the service.
type Metrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	rdapRequests     *prometheus.CounterVec
	rdapDuration     *prometheus.HistogramVec
	cacheHits        *prometheus.CounterVec
	activeChecks     prometheus.Gauge
	bulkCheckSize    *prometheus.HistogramVec
	checksServed     prometheus.Counter
	bootstrapAge     *prometheus.GaugeVec
}

var (
	metricsOnce     sync.Once
	globalMetrics   *Metrics
	metricsRegistry *prometheus.Registry
)

// newMetrics creates and registers all Prometheus metrics.
func newMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		requestsTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "domcheck_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		rdapRequests: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_rdap_requests_total",
				Help: "Total number of RDAP requests to registries",
			},
			[]string{"registry", "status"},
		),
		rdapDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "domcheck_rdap_duration_seconds",
				Help:    "RDAP request latency in seconds",
				Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
			[]string{"registry"},
		),
		cacheHits: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_cache_hits_total",
				Help: "Total number of cache hits and misses",
			},
			[]string{"result"},
		),
		activeChecks: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{
				Name: "domcheck_active_checks",
				Help: "Number of in-flight domain check goroutines",
			},
		),
		bulkCheckSize: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "domcheck_bulk_check_size",
				Help:    "Number of domains in bulk check requests",
				Buckets: []float64{1, 2, 5, 10, 20, 30, 40, 50},
			},
			[]string{},
		),
		checksServed: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Name: "domcheck_checks_served_total",
				Help: "Total number of domain checks served",
			},
		),
		bootstrapAge: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "domcheck_bootstrap_age_seconds",
				Help: "Age of the IANA bootstrap cache in seconds",
			},
			[]string{},
		),
	}

	// Register Go runtime metrics.
	reg.MustRegister(prometheus.NewGoCollector())

	metricsRegistry = reg
	return m
}

// GetMetrics returns the global Metrics instance, creating it if necessary.
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = newMetrics()
	})
	return globalMetrics
}

// Handler returns an http.Handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{})
}

// RecordRequest records an HTTP request.
func (m *Metrics) RecordRequest(method, path string, status int, durationSeconds float64) {
	m.requestsTotal.WithLabelValues(method, path, statusCodeToString(status)).Inc()
	m.requestDuration.WithLabelValues(method, path).Observe(durationSeconds)
}

// RecordRDAPRequest records an RDAP request to a registry.
func (m *Metrics) RecordRDAPRequest(registry, status string, durationSeconds float64) {
	m.rdapRequests.WithLabelValues(registry, status).Inc()
	m.rdapDuration.WithLabelValues(registry).Observe(durationSeconds)
}

// RecordCacheHit records a cache access.
func (m *Metrics) RecordCacheHit(result string) {
	m.cacheHits.WithLabelValues(result).Inc()
}

// IncrementActiveChecks increments the in-flight checks counter.
func (m *Metrics) IncrementActiveChecks() {
	m.activeChecks.Inc()
}

// DecrementActiveChecks decrements the in-flight checks counter.
func (m *Metrics) DecrementActiveChecks() {
	m.activeChecks.Dec()
}

// RecordBulkCheck records a bulk check request size.
func (m *Metrics) RecordBulkCheck(size int) {
	m.bulkCheckSize.WithLabelValues().Observe(float64(size))
}

// IncrementChecksServed increments the total checks served counter.
func (m *Metrics) IncrementChecksServed() {
	m.checksServed.Inc()
}

// AddChecksServed adds n to the total checks served counter.
func (m *Metrics) AddChecksServed(n int) {
	m.checksServed.Add(float64(n))
}

// SetBootstrapAge sets the bootstrap cache age in seconds.
func (m *Metrics) SetBootstrapAge(ageSeconds float64) {
	m.bootstrapAge.WithLabelValues().Set(ageSeconds)
}

// statusCodeToString converts an HTTP status code to a string category.
func statusCodeToString(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "other"
	}
}
