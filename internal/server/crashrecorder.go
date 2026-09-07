// crashrecorder.go captures crash events so a crash can be investigated after
// the fact: an in-memory history for the health and diagnostics endpoints, a
// bounded set of JSON dumps on disk for post-mortems, and a crash-loop verdict
// published as both a metric and a health status.
//
// Panics are already converted into 500s by the Recover middleware; this type
// adds the *record* of them. Signal receptions are recorded too, but they are
// routine at deploy time and so are deliberately excluded from loop detection.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// Crash event kinds, used as the domcheck_crashes_total label value.
const (
	CrashKindPanic         = "panic"
	CrashKindSignal        = "signal"
	CrashKindShutdownError = "shutdown_error"
)

const (
	// DefaultCrashHistory caps the in-memory ring so a panic storm cannot
	// grow the process that is reporting on itself.
	DefaultCrashHistory = 50
	// DefaultCrashDumpMax caps crash dumps on disk; the oldest is pruned when
	// a new one is written.
	DefaultCrashDumpMax = 10
	// DefaultCrashLoopThreshold is how many crashes within the window count
	// as a crash loop.
	DefaultCrashLoopThreshold = 3
	// DefaultCrashLoopWindow is the window crash-loop detection looks inside.
	DefaultCrashLoopWindow = 5 * time.Minute
	// maxStackBytes caps a stored stack trace. A deep handler stack can reach
	// tens of KB; beyond that the head of the trace (where the panic site is)
	// is what matters.
	maxStackBytes = 16 * 1024
)

// CrashEvent is one captured crash, in the shape it is exposed on
// /api/v1/crashes and persisted as a dump file.
type CrashEvent struct {
	Time      time.Time `json:"time"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Signal    string    `json:"signal,omitempty"`
	Stack     string    `json:"stack,omitempty"`
	Method    string    `json:"method,omitempty"`
	Path      string    `json:"path,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
}

// CrashSummary is the crash-history view the health endpoint reports: the
// process-wide count, the crash-loop verdict, and the most recent events with
// their stack traces stripped (full stacks stay in /api/v1/crashes and on
// disk, so a health poll stays cheap).
type CrashSummary struct {
	Total        int          `json:"total"`
	LoopDetected bool         `json:"loop_detected"`
	Recent       []CrashEvent `json:"recent"`
}

// CrashConfig configures a CrashRecorder. Zero values fall back to the
// Default* constants; a nil Logger or nil Metrics is tolerated (the recorder
// then only keeps in-memory history).
type CrashConfig struct {
	Logger        *slog.Logger
	Metrics       *Metrics
	DumpDir       string        // directory for crash dump files; "" disables dumps
	MaxDumps      int           // dumps kept on disk (oldest pruned)
	History       int           // events kept in memory
	LoopThreshold int           // crashes within LoopWindow that count as a loop
	LoopWindow    time.Duration // window for crash-loop detection
}

// CrashRecorder captures crash events into a bounded in-memory history, dumps
// the most recent ones to disk, and decides when the process is in a crash
// loop. It is safe for concurrent use.
type CrashRecorder struct {
	mu      sync.Mutex
	cfg     CrashConfig
	history []CrashEvent
	total   int
	loop    bool
}

// NewCrashRecorder creates a recorder, creating DumpDir if needed. If the
// directory cannot be created, dumps are disabled for this recorder and the
// reason is logged — losing dumps is better than losing the history.
func NewCrashRecorder(cfg CrashConfig) *CrashRecorder {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if cfg.History <= 0 {
		cfg.History = DefaultCrashHistory
	}
	if cfg.MaxDumps <= 0 {
		cfg.MaxDumps = DefaultCrashDumpMax
	}
	if cfg.LoopThreshold <= 0 {
		cfg.LoopThreshold = DefaultCrashLoopThreshold
	}
	if cfg.LoopWindow <= 0 {
		cfg.LoopWindow = DefaultCrashLoopWindow
	}

	c := &CrashRecorder{cfg: cfg}

	if cfg.DumpDir != "" {
		if err := os.MkdirAll(cfg.DumpDir, 0o750); err != nil {
			c.cfg.DumpDir = ""
			cfg.Logger.Warn("crash dump directory unusable, dumps disabled",
				"dir", cfg.DumpDir, "error", err)
		} else {
			// Retention also applies across restarts: dumps left by a
			// previous run count against the same budget.
			c.pruneDumps()
		}
	}
	return c
}

var (
	crashMu       sync.Mutex
	globalCrashes *CrashRecorder
)

// InitCrashRecorder installs the process-wide recorder with runtime config.
// Call it once at startup, before any handler can record; later calls replace
// the recorder, so they are only meaningful before serving starts.
func InitCrashRecorder(cfg CrashConfig) *CrashRecorder {
	crashMu.Lock()
	defer crashMu.Unlock()
	globalCrashes = NewCrashRecorder(cfg)
	return globalCrashes
}

// GetCrashRecorder returns the process-wide recorder, creating one with
// default config when InitCrashRecorder was never called (tests, CLI paths).
func GetCrashRecorder() *CrashRecorder {
	crashMu.Lock()
	defer crashMu.Unlock()
	if globalCrashes == nil {
		globalCrashes = NewCrashRecorder(CrashConfig{})
	}
	return globalCrashes
}

// RecordPanic captures a panic recovered by the Recover middleware, capturing
// the stack when the caller did not supply one.
func (c *CrashRecorder) RecordPanic(rec any, r *http.Request, stack string) {
	ev := CrashEvent{
		Time:    time.Now(),
		Kind:    CrashKindPanic,
		Message: fmt.Sprint(rec),
		Stack:   truncateStack(stack),
	}
	if r != nil {
		ev.Method = r.Method
		ev.Path = r.URL.Path
		ev.RequestID = GetRequestID(r.Context())
		ev.ClientIP = GetClientIP(r.Context())
	}
	c.Record(ev)
}

// RecordSignal captures a caught OS signal. Signals are routine (a deploy
// sends SIGTERM), so they are recorded for history but never counted toward
// crash-loop detection, never dumped to disk, and excluded from
// domcheck_crashes_total — Server.Run reports them in
// domcheck_signal_receptions_total.
func (c *CrashRecorder) RecordSignal(sig string) {
	c.Record(CrashEvent{
		Time:    time.Now(),
		Kind:    CrashKindSignal,
		Signal:  sig,
		Message: "received " + sig,
	})
}

// RecordShutdownError captures a failed graceful shutdown: the process is
// going away and connections were not drained.
func (c *CrashRecorder) RecordShutdownError(err error, drained time.Duration) {
	c.Record(CrashEvent{
		Time:    time.Now(),
		Kind:    CrashKindShutdownError,
		Message: fmt.Sprintf("graceful shutdown failed after %s: %v", drained, err),
		Stack:   truncateStack(string(debug.Stack())),
	})
}

// Record captures one event: it enters the history, the metrics, and — for
// the crash kinds — the on-disk dump ring, and refreshes the crash-loop
// verdict.
func (c *CrashRecorder) Record(ev CrashEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.history = append(c.history, ev)
	if len(c.history) > c.cfg.History {
		c.history = c.history[len(c.history)-c.cfg.History:]
	}
	c.total++

	// Signal receptions stay out of the crash counter: a deploy sends
	// SIGTERM, and a crash total that rises on every deploy would trip any
	// naive alert on it. They are counted in domcheck_signal_receptions_total
	// instead, by Server.Run.
	if ev.Kind != CrashKindSignal && c.cfg.Metrics != nil {
		c.cfg.Metrics.RecordCrash(ev.Kind)
	}

	switch ev.Kind {
	case CrashKindPanic, CrashKindShutdownError:
		c.writeDumpLocked(ev)
	}
	c.evaluateLoopLocked()
}

// Snapshot returns the crash summary reported by the health endpoint.
func (c *CrashRecorder) Snapshot() CrashSummary {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := CrashSummary{Total: c.total, LoopDetected: c.loop}
	const recent = 5
	start := len(c.history) - recent
	if start < 0 {
		start = 0
	}
	s.Recent = make([]CrashEvent, 0, len(c.history)-start)
	// Newest first: the reader of a health payload cares about the last crash.
	for i := len(c.history) - 1; i >= start; i-- {
		ev := c.history[i]
		ev.Stack = ""
		s.Recent = append(s.Recent, ev)
	}
	return s
}

// Events returns the most recent n events in full, stacks included, newest
// first. Pass 0 for all retained events.
func (c *CrashRecorder) Events(n int) []CrashEvent {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n <= 0 || n > len(c.history) {
		n = len(c.history)
	}
	out := make([]CrashEvent, 0, n)
	for i := len(c.history) - 1; i >= len(c.history)-n; i-- {
		out = append(out, c.history[i])
	}
	return out
}

// Total returns the number of events captured since process start.
func (c *CrashRecorder) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// LoopDetected reports whether the process is currently in a crash loop.
func (c *CrashRecorder) LoopDetected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loop
}

// evaluateLoopLocked recounts crashes inside the window and publishes a
// changed verdict. Callers must hold c.mu.
func (c *CrashRecorder) evaluateLoopLocked() {
	cutoff := time.Now().Add(-c.cfg.LoopWindow)
	crashes := 0
	for i := len(c.history) - 1; i >= 0; i-- {
		if c.history[i].Time.Before(cutoff) {
			break
		}
		switch c.history[i].Kind {
		case CrashKindPanic, CrashKindShutdownError:
			crashes++
		}
	}

	detected := crashes >= c.cfg.LoopThreshold
	if detected == c.loop {
		return
	}
	c.loop = detected
	if c.cfg.Metrics != nil {
		c.cfg.Metrics.SetCrashLoopDetected(detected)
	}
	if detected {
		c.cfg.Logger.Error("crash loop detected",
			"crashes_in_window", crashes,
			"threshold", c.cfg.LoopThreshold,
			"window", c.cfg.LoopWindow.String(),
			"dump_dir", c.cfg.DumpDir,
		)
		return
	}
	// The gauge clears when the crashing events age out of the window.
	c.cfg.Logger.Info("crash loop cleared",
		"crashes_in_window", crashes,
		"threshold", c.cfg.LoopThreshold,
		"window", c.cfg.LoopWindow.String(),
	)
}

// writeDumpLocked persists one event as JSON and prunes the dump ring.
// Callers must hold c.mu. A dump failure is logged, never returned: the event
// is already in the history and failing the request over it would be worse.
func (c *CrashRecorder) writeDumpLocked(ev CrashEvent) {
	if c.cfg.DumpDir == "" {
		return
	}
	name := fmt.Sprintf("crash-%d-%s.json", ev.Time.UnixNano(), ev.Kind)
	path := filepath.Join(c.cfg.DumpDir, name)

	data, err := json.MarshalIndent(ev, "", "  ")
	if err == nil {
		err = os.WriteFile(path, data, 0o640)
	}
	if err != nil {
		c.cfg.Logger.Warn("failed to write crash dump", "path", path, "error", err)
		return
	}
	c.pruneDumps()
}

// pruneDumps keeps only the newest MaxDumps crash dumps. Filenames embed
// Unix nanoseconds, which sort chronologically for as long as Go supports
// time.UnixNano at all.
func (c *CrashRecorder) pruneDumps() {
	if c.cfg.DumpDir == "" {
		return
	}
	matches, err := filepath.Glob(filepath.Join(c.cfg.DumpDir, "crash-*.json"))
	if err != nil || len(matches) <= c.cfg.MaxDumps {
		return
	}
	sort.Strings(matches)
	for _, path := range matches[:len(matches)-c.cfg.MaxDumps] {
		if err := os.Remove(path); err != nil {
			c.cfg.Logger.Warn("failed to prune crash dump", "path", path, "error", err)
		}
	}
}

// truncateStack caps a stack trace at maxStackBytes, keeping the head, which
// is where the panic site and the request path live.
func truncateStack(stack string) string {
	stack = strings.TrimSpace(stack)
	if len(stack) <= maxStackBytes {
		return stack
	}
	return stack[:maxStackBytes] + "\n…truncated"
}
