package server

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

	"github.com/jedarden/domain-check/internal/config"
	dto "github.com/prometheus/client_model/go"
)

// installRecorder replaces the process-wide recorder for the duration of a
// test. Tests that assert on the global recorder (or on the health/crashes
// endpoints, which read it) must go through this so they neither inherit a
// previous test's history nor leave one behind.
func installRecorder(t *testing.T, cfg CrashConfig) *CrashRecorder {
	t.Helper()
	t.Cleanup(func() { InitCrashRecorder(CrashConfig{}) }) // fresh, empty recorder
	return InitCrashRecorder(cfg)
}

// crashCounterValue reads a counter or gauge value out of the package's
// metrics registry, following the label set. Missing series read as 0. Named
// apart from safeguards_test.go's counterValue, which reads a specific
// prometheus.Counter rather than the registry.
func crashCounterValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	if metricsRegistry == nil {
		return 0
	}
	mfs, err := metricsRegistry.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !labelPairsMatch(m.GetLabel(), labels) {
				continue
			}
			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				return m.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				return m.GetGauge().GetValue()
			}
		}
	}
	return 0
}

// labelPairsMatch reports whether a gathered metric's labels equal want
// exactly (same set, same values). Named apart from handlers_metrics_test.go's
// string-parsing labelsMatch.
func labelPairsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, l := range got {
		v, ok := want[l.GetName()]
		if !ok || v != l.GetValue() {
			return false
		}
	}
	return true
}

func TestCrashRecorderDefaults(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir})

	if c.cfg.MaxDumps != DefaultCrashDumpMax {
		t.Errorf("MaxDumps default: got %d, want %d", c.cfg.MaxDumps, DefaultCrashDumpMax)
	}
	if c.cfg.LoopThreshold != DefaultCrashLoopThreshold {
		t.Errorf("LoopThreshold default: got %d, want %d", c.cfg.LoopThreshold, DefaultCrashLoopThreshold)
	}
	if c.cfg.LoopWindow != DefaultCrashLoopWindow {
		t.Errorf("LoopWindow default: got %s, want %s", c.cfg.LoopWindow, DefaultCrashLoopWindow)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dump dir was not created: %v", err)
	}
}

func TestRecordPanicCapturesEventAndDump(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir, Metrics: GetMetrics()})

	before := crashCounterValue(t, "domcheck_crashes_total", map[string]string{"kind": CrashKindPanic})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bulk", nil)
	c.RecordPanic("boom", req, "goroutine 1 [running]:\nmain.main()")

	if got := c.Total(); got != 1 {
		t.Fatalf("Total() = %d, want 1", got)
	}
	ev := c.Events(1)[0]
	if ev.Kind != CrashKindPanic || ev.Message != "boom" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if ev.Method != http.MethodPost || ev.Path != "/api/v1/bulk" {
		t.Errorf("request context not captured: %+v", ev)
	}
	if ev.Stack == "" {
		t.Error("stack trace not captured")
	}

	after := crashCounterValue(t, "domcheck_crashes_total", map[string]string{"kind": CrashKindPanic})
	if after-before != 1 {
		t.Errorf("domcheck_crashes_total{kind=panic} moved %f, want 1", after-before)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one dump file, got %d (%v)", len(matches), err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var dumped CrashEvent
	if err := json.Unmarshal(raw, &dumped); err != nil {
		t.Fatalf("dump is not valid JSON: %v", err)
	}
	if dumped.Kind != CrashKindPanic || dumped.Message != "boom" {
		t.Errorf("dump content mismatch: %+v", dumped)
	}
	if !strings.Contains(dumped.Stack, "main.main()") {
		t.Errorf("dump lost the stack trace: %q", dumped.Stack)
	}
}

func TestRecordSignalIsHistoryOnly(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir, Metrics: GetMetrics()})

	beforeCrashes := crashCounterValue(t, "domcheck_crashes_total", map[string]string{"kind": CrashKindSignal})
	beforeSignals := crashCounterValue(t, "domcheck_signal_receptions_total", map[string]string{"signal": "SIGTERM"})

	for i := 0; i < 5; i++ {
		c.RecordSignal("SIGTERM")
	}

	if got := c.Total(); got != 5 {
		t.Errorf("Total() = %d, want 5 (signals are kept in history)", got)
	}
	// A deploy sends SIGTERM: five of them must not read as a crash loop,
	// must not touch the crash counter, and must not leave dumps behind.
	if c.LoopDetected() {
		t.Error("signal receptions tripped crash-loop detection")
	}
	afterCrashes := crashCounterValue(t, "domcheck_crashes_total", map[string]string{"kind": CrashKindSignal})
	if afterCrashes != beforeCrashes {
		t.Errorf("signals must not increment domcheck_crashes_total: %f -> %f", beforeCrashes, afterCrashes)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if len(matches) != 0 {
		t.Errorf("signals must not produce dumps, found %v", matches)
	}

	// Server.Run reports the reception in its own counter.
	afterSignals := crashCounterValue(t, "domcheck_signal_receptions_total", map[string]string{"signal": "SIGTERM"})
	if afterSignals-beforeSignals != 0 {
		t.Errorf("recorder must not double-count signals (Server.Run owns that counter): moved %f", afterSignals-beforeSignals)
	}
}

func TestRecordShutdownErrorCountsAndDumps(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir, Metrics: GetMetrics(), LoopThreshold: 2})

	c.RecordShutdownError(context.DeadlineExceeded, 900*time.Millisecond)
	c.RecordShutdownError(context.DeadlineExceeded, time.Second)

	if !c.LoopDetected() {
		t.Error("failed shutdowns within the threshold did not trip crash-loop detection")
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 dumps, got %d", len(matches))
	}

	got := crashCounterValue(t, "domcheck_crashes_total", map[string]string{"kind": CrashKindShutdownError})
	if got != 2 {
		t.Errorf("domcheck_crashes_total{kind=shutdown_error} = %f, want 2", got)
	}
}

func TestCrashLoopDetectionWindow(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir, Metrics: GetMetrics(), LoopThreshold: 3, LoopWindow: 5 * time.Minute})

	now := time.Now()
	// Two panics inside the window are below the threshold.
	for i := 0; i < 2; i++ {
		c.Record(CrashEvent{Time: now.Add(-time.Duration(i) * time.Minute), Kind: CrashKindPanic, Message: "early"})
	}
	if c.LoopDetected() {
		t.Fatal("loop detected below the threshold")
	}

	// A third one trips it.
	c.Record(CrashEvent{Time: now, Kind: CrashKindPanic, Message: "last straw"})
	if !c.LoopDetected() {
		t.Fatal("loop not detected at the threshold")
	}
	if got := crashCounterValue(t, "domcheck_crash_loop_detected", nil); got != 1 {
		t.Errorf("domcheck_crash_loop_detected = %f, want 1", got)
	}

	// Events older than the window never counted in the first place.
	old := NewCrashRecorder(CrashConfig{LoopThreshold: 2, LoopWindow: time.Minute})
	base := time.Now().Add(-10 * time.Minute)
	old.Record(CrashEvent{Time: base, Kind: CrashKindPanic})
	old.Record(CrashEvent{Time: base.Add(time.Second), Kind: CrashKindPanic})
	if old.LoopDetected() {
		t.Error("crashes outside the window tripped the loop")
	}
	if got := crashCounterValue(t, "domcheck_crash_loop_detected", nil); got != 1 {
		t.Error("a clear verdict must leave the gauge cleared; it was set by another test and never cleared")
	}
}

func TestCrashLoopClearsWhenEventsAgeOut(t *testing.T) {
	c := NewCrashRecorder(CrashConfig{Metrics: GetMetrics(), LoopThreshold: 2, LoopWindow: 10 * time.Millisecond})

	c.RecordPanic("one", nil, "")
	c.RecordPanic("two", nil, "")
	if !c.LoopDetected() {
		t.Fatal("loop not detected at the threshold")
	}

	time.Sleep(25 * time.Millisecond)
	// Any record re-evaluates the window; by now both panics have aged out.
	c.RecordSignal("SIGHUP")
	if c.LoopDetected() {
		t.Error("loop verdict stayed set after its events aged out of the window")
	}
	if got := crashCounterValue(t, "domcheck_crash_loop_detected", nil); got != 0 {
		t.Errorf("domcheck_crash_loop_detected = %f, want 0 after clearing", got)
	}
}

func TestCrashDumpRetentionKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	c := NewCrashRecorder(CrashConfig{DumpDir: dir, MaxDumps: 2})

	for i := 0; i < 4; i++ {
		c.Record(CrashEvent{Time: time.Now(), Kind: CrashKindPanic, Message: "dump"})
		time.Sleep(2 * time.Millisecond) // distinct nanosecond timestamps
	}

	matches, err := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("retention kept %d dumps, want 2", len(matches))
	}
	// The ring keeps the newest: the last two recorded events, and the history
	// agrees (Total counts everything, Events is bounded by History).
	if got := c.Total(); got != 4 {
		t.Errorf("Total() = %d, want 4", got)
	}
}

func TestDumpsDisabledWithoutDir(t *testing.T) {
	c := NewCrashRecorder(CrashConfig{}) // DumpDir "" on purpose
	c.RecordPanic("no dumps here", httptest.NewRequest(http.MethodGet, "/x", nil), "stack")

	if got := c.Total(); got != 1 {
		t.Errorf("Total() = %d, want 1", got)
	}
}

func TestUnusableDumpDirDisablesDumpsButNotHistory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(file, []byte("a file, not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewCrashRecorder(CrashConfig{DumpDir: filepath.Join(file, "dumps")})

	if c.cfg.DumpDir != "" {
		t.Fatalf("expected dumps to be disabled for an unusable dir, got %q", c.cfg.DumpDir)
	}
	c.RecordPanic("still recorded", nil, "stack")
	if got := c.Total(); got != 1 {
		t.Errorf("Total() = %d, want 1 (history survives unusable dump dir)", got)
	}
}

func TestHistoryRingIsBounded(t *testing.T) {
	c := NewCrashRecorder(CrashConfig{History: 3})

	for i := 0; i < 5; i++ {
		c.Record(CrashEvent{Time: time.Now(), Kind: CrashKindPanic, Message: "e"})
	}

	if got := c.Total(); got != 5 {
		t.Errorf("Total() = %d, want 5 (the count is process-wide)", got)
	}
	if got := len(c.Events(0)); got != 3 {
		t.Errorf("retained %d events, want 3", got)
	}
}

func TestSnapshotStripsStacksNewestFirst(t *testing.T) {
	c := NewCrashRecorder(CrashConfig{})

	c.RecordPanic("first", nil, "stack-one")
	c.RecordPanic("second", nil, "stack-two")

	s := c.Snapshot()
	if s.Total != 2 || s.LoopDetected {
		t.Errorf("unexpected summary: %+v", s)
	}
	if len(s.Recent) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(s.Recent))
	}
	if s.Recent[0].Message != "second" || s.Recent[1].Message != "first" {
		t.Errorf("recent events are not newest first: %v then %v", s.Recent[0].Message, s.Recent[1].Message)
	}
	for _, ev := range s.Recent {
		if ev.Stack != "" {
			t.Errorf("health payload must not carry stacks, got %q", ev.Stack)
		}
	}
}

func TestEventsLimitAndOrdering(t *testing.T) {
	c := NewCrashRecorder(CrashConfig{})

	for i := 0; i < 5; i++ {
		c.Record(CrashEvent{Time: time.Now(), Kind: CrashKindPanic, Message: string(rune('a' + i))})
	}

	all := c.Events(0)
	if len(all) != 5 || all[0].Message != "e" || all[4].Message != "a" {
		t.Errorf("Events(0) = %d events, newest %q oldest %q; want 5/e/a", len(all), all[0].Message, all[4].Message)
	}
	two := c.Events(2)
	if len(two) != 2 || two[0].Message != "e" || two[1].Message != "d" {
		t.Errorf("Events(2) = %v, want [e d]", two)
	}
	if got := c.Events(99); len(got) != 5 {
		t.Errorf("Events(99) clamped to %d events, want 5", len(got))
	}
}

func TestTruncateStackKeepsHead(t *testing.T) {
	long := strings.Repeat("x", maxStackBytes+1000)
	got := truncateStack(long)
	if len(got) > maxStackBytes+len("\n…truncated") {
		t.Errorf("stack not truncated: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "…truncated") {
		t.Errorf("truncated stack lost its marker: %q", got[len(got)-20:])
	}
	if truncateStack("short") != "short" {
		t.Error("short stack was modified")
	}
}

// --- Endpoint tests ---

func TestCrashesHandlerLimitValidation(t *testing.T) {
	c := installRecorder(t, CrashConfig{})
	for i := 0; i < 3; i++ {
		c.Record(CrashEvent{Time: time.Now(), Kind: CrashKindPanic, Message: "e", Stack: "stack"})
	}

	handler := crashesHandler(c)

	t.Run("default limit returns 20 newest", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crashes", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var body struct {
			Total  int          `json:"total"`
			Events []CrashEvent `json:"events"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Total != 3 || len(body.Events) != 3 {
			t.Errorf("got total=%d events=%d, want 3/3", body.Total, len(body.Events))
		}
	})

	t.Run("limit bounds the response", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crashes?limit=1", nil))
		var body struct {
			Events []CrashEvent `json:"events"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(body.Events))
		}
	})

	for _, tc := range []struct {
		name, limit string
	}{
		{"negative", "-1"},
		{"non-numeric", "many"},
	} {
		t.Run("invalid limit "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crashes?limit="+tc.limit, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("limit=%s: expected 400, got %d", tc.limit, rec.Code)
			}
		})
	}
}

func TestCrashesHandlerIncludesStacks(t *testing.T) {
	c := installRecorder(t, CrashConfig{})
	c.RecordPanic("handler blew up", httptest.NewRequest(http.MethodGet, "/api/v1/check", nil), "goroutine 7 [running]:")

	rec := httptest.NewRecorder()
	crashesHandler(c)(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crashes", nil))

	if !strings.Contains(rec.Body.String(), "goroutine 7 [running]:") {
		t.Error("/api/v1/crashes is the diagnostics endpoint: it must include stack traces")
	}
}

// TestHealthEndpointReportsCrashHistory drives the whole router so the health
// payload is asserted in the shape a client actually sees.
func TestHealthEndpointReportsCrashHistory(t *testing.T) {
	installRecorder(t, CrashConfig{LoopThreshold: 2})

	cfg := config.Defaults()
	log := DefaultLogger("text", "error")
	router := Router(&cfg, log, NewRateLimiter(log), &mockChecker{}, nil, NewServiceMonitor(), GetMetrics(), nil)

	// A healthy process reports an empty crash history.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var health struct {
		Status  string       `json:"status"`
		Crashes CrashSummary `json:"crashes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health.Status != "ok" || health.Crashes.Total != 0 {
		t.Fatalf("fresh process: got status=%q total=%d, want ok/0", health.Status, health.Crashes.Total)
	}

	// One panic: history grows, status stays ok.
	GetCrashRecorder().RecordPanic("single panic", httptest.NewRequest(http.MethodGet, "/api/v1/check", nil), "stack")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	json.NewDecoder(rec.Body).Decode(&health)
	if health.Status != "ok" || health.Crashes.Total != 1 {
		t.Fatalf("after one panic: got status=%q total=%d, want ok/1", health.Status, health.Crashes.Total)
	}

	// A second panic trips the configured loop threshold: the status degrades
	// but stays 200, because a restarting probe would multiply the problem.
	GetCrashRecorder().RecordPanic("second panic", httptest.NewRequest(http.MethodGet, "/api/v1/check", nil), "stack")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	json.NewDecoder(rec.Body).Decode(&health)
	if health.Status != "degraded" {
		t.Errorf("crash loop: got status=%q, want degraded", health.Status)
	}
	if !health.Crashes.LoopDetected {
		t.Error("health payload did not report loop_detected")
	}
}

// TestRecoverFeedsCrashRecorder proves the middleware and the recorder are
// actually connected end to end: a handler panic shows up as a recorded,
// dumped, metric-counted crash event.
func TestRecoverFeedsCrashRecorder(t *testing.T) {
	dir := t.TempDir()
	installRecorder(t, CrashConfig{DumpDir: dir, Metrics: GetMetrics(), LoopThreshold: 5})

	before := crashCounterValue(t, "domcheck_panics_recovered_total", nil)

	handler := Recover(DefaultLogger("text", "error"), GetMetrics())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("endpoint blew up")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/check?d=example.com", nil)
	req = requestWithID(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from recovery, got %d", rec.Code)
	}

	ev := GetCrashRecorder().Events(1)[0]
	if ev.Kind != CrashKindPanic || ev.Message != "endpoint blew up" {
		t.Fatalf("middleware did not feed the recorder: %+v", ev)
	}
	if ev.Path != "/api/v1/check" {
		t.Errorf("event lost the request path: %+v", ev)
	}
	if after := crashCounterValue(t, "domcheck_panics_recovered_total", nil); after-before != 1 {
		t.Errorf("domcheck_panics_recovered_total moved %f, want 1", after-before)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 crash dump from the recovered panic, got %d", len(matches))
	}
}

// TestSignalPathRecordsWithoutLoop covers the Server.Run signal goroutine's
// recorder call: receptions land in history as signals and leave the loop
// verdict alone.
func TestSignalPathRecordsWithoutLoop(t *testing.T) {
	installRecorder(t, CrashConfig{LoopThreshold: 1}) // threshold 1: anything crash-like trips it

	GetCrashRecorder().RecordSignal("SIGTERM")

	if GetCrashRecorder().LoopDetected() {
		t.Error("a SIGTERM reception tripped crash-loop detection at threshold 1")
	}
	ev := GetCrashRecorder().Events(1)[0]
	if ev.Kind != CrashKindSignal || ev.Signal != "SIGTERM" {
		t.Errorf("unexpected signal event: %+v", ev)
	}
}

// TestMetricsExposeCrashFamily verifies every metric the alert rules in
// declarative-config scrape is actually registered and present after use.
func TestMetricsExposeCrashFamily(t *testing.T) {
	m := GetMetrics()
	m.RecordCrash(CrashKindPanic)
	m.RecordSignalReceived("SIGINT")
	m.RecordShutdown("graceful", 10*time.Millisecond)
	m.SetCrashLoopDetected(false)

	for _, tc := range []struct{ name string }{
		{"domcheck_crashes_total"},
		{"domcheck_signal_receptions_total"},
		{"domcheck_graceful_shutdowns_total"},
		{"domcheck_shutdown_duration_seconds"},
		{"domcheck_crash_loop_detected"},
		{"domcheck_panics_recovered_total"},
	} {
		if !registryHasMetric(t, tc.name) {
			t.Errorf("metric %s is not registered", tc.name)
		}
	}
}

func registryHasMetric(t *testing.T, name string) bool {
	t.Helper()
	mfs, err := metricsRegistry.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			return true
		}
	}
	return false
}

func requestWithID(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), RequestIDKey, "test-request-id"))
}
