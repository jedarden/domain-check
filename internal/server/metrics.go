// Package server provides Prometheus metrics for domain-check.
package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the service.
type Metrics struct {
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	rdapRequests      *prometheus.CounterVec
	rdapDuration      *prometheus.HistogramVec
	cacheHits         *prometheus.CounterVec
	activeChecks      prometheus.Gauge
	bulkCheckSize     *prometheus.HistogramVec
	checksServed      prometheus.Counter
	bootstrapAge      *prometheus.GaugeVec
	panicsRecovered   prometheus.Counter
	requestTimeouts   prometheus.Counter
	crashesTotal      *prometheus.CounterVec
	signalReceived    *prometheus.CounterVec
	signalHandlerRuns *prometheus.CounterVec
	signalHandlerDur  *prometheus.HistogramVec
	shutdowns         *prometheus.CounterVec
	shutdownDur       prometheus.Histogram
	crashLoop         prometheus.Gauge
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
		panicsRecovered: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Name: "domcheck_panics_recovered_total",
				Help: "Total number of handler panics recovered by middleware",
			},
		),
		requestTimeouts: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Name: "domcheck_request_timeouts_total",
				Help: "Total number of requests that exceeded the request timeout",
			},
		),
		crashesTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_crashes_total",
				// Signals are deliberately absent: a deploy sends SIGTERM and a
				// crash total that rises on every deploy would trip naive
				// alerts. Receptions are counted in
				// domcheck_signal_receptions_total instead.
				Help: "Total number of crash events captured by the process, by kind (panic, shutdown_error)",
			},
			[]string{"kind"},
		),
		signalReceived: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_signal_receptions_total",
				Help: "Total number of OS signals received by the process, by signal name",
			},
			[]string{"signal"},
		),
		signalHandlerRuns: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_signal_handler_executions_total",
				Help: "Total number of completed signal handler executions - a caught signal whose graceful shutdown finished - by signal and drain outcome. SIGHUP shares the shutdown path with SIGINT/SIGTERM; a SIGHUP cascade shows up as signal=\"SIGHUP\".",
			},
			[]string{"signal", "outcome"},
		),
		signalHandlerDur: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "domcheck_signal_handler_duration_seconds",
				Help:    "Time from signal receipt to the end of the graceful shutdown it triggered",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15},
			},
			[]string{"signal"},
		),
		shutdowns: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "domcheck_graceful_shutdowns_total",
				Help: "Total number of server shutdowns, by outcome (graceful, forced, error)",
			},
			[]string{"outcome"},
		),
		shutdownDur: promauto.With(reg).NewHistogram(
			prometheus.HistogramOpts{
				Name:    "domcheck_shutdown_duration_seconds",
				Help:    "Time spent draining connections during graceful shutdown",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15},
			},
		),
		crashLoop: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{
				Name: "domcheck_crash_loop_detected",
				Help: "1 when the configured number of crashes occurred within the crash-loop window, else 0",
			},
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

// RecordPanicRecovered increments the recovered-panic counter. The Recover
// middleware calls it once per handler panic it converts into a 500 response.
func (m *Metrics) RecordPanicRecovered() {
	m.panicsRecovered.Inc()
}

// RecordRequestTimeout increments the request-timeout counter. The Timeout
// middleware calls it when a request outlives its budget; a client that
// disconnects early is not a timeout and is not counted.
func (m *Metrics) RecordRequestTimeout() {
	m.requestTimeouts.Inc()
}

// RecordCrash increments the crash-event counter for the given kind. The
// CrashRecorder calls it once per captured event; panics are additionally
// counted by RecordPanicRecovered, which predates the crash family.
func (m *Metrics) RecordCrash(kind string) {
	m.crashesTotal.WithLabelValues(kind).Inc()
}

// RecordSignalReceived increments the signal counter for the named signal
// (e.g. "SIGTERM"). Server.Run calls it when the process catches a signal.
func (m *Metrics) RecordSignalReceived(sig string) {
	m.signalReceived.WithLabelValues(sig).Inc()
}

// RecordSignalHandled records one completed signal handler execution: which
// signal it served, how the drain it triggered ended, and how long the whole
// handler took from signal receipt to shutdown completion. Server.Run calls
// it after the drain, so a SIGHUP cascade - the recurring infrastructure
// failure mode in this fleet - is observable on /metrics as it happens rather
// than only in post-mortem logs. Prometheus counters and histograms are
// goroutine-safe, which matters here because a signal lands on whichever
// goroutine ends up selecting on the signal channel.
func (m *Metrics) RecordSignalHandled(sig, outcome string, d time.Duration) {
	m.signalHandlerRuns.WithLabelValues(sig, outcome).Inc()
	m.signalHandlerDur.WithLabelValues(sig).Observe(d.Seconds())
}

// RecordShutdown records one server shutdown: its outcome ("graceful" when
// every connection drained inside the budget, "forced" when the drain timed
// out, "error" when Shutdown itself failed) and how long the drain took.
func (m *Metrics) RecordShutdown(outcome string, d time.Duration) {
	m.shutdowns.WithLabelValues(outcome).Inc()
	m.shutdownDur.Observe(d.Seconds())
}

// SetCrashLoopDetected publishes the crash-loop verdict as a 0/1 gauge so a
// single Prometheus rule can alert on it. The CrashRecorder calls it every
// time the verdict could have changed.
func (m *Metrics) SetCrashLoopDetected(detected bool) {
	if detected {
		m.crashLoop.Set(1)
		return
	}
	m.crashLoop.Set(0)
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
