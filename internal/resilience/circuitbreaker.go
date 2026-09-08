package resilience

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Default circuit-breaker thresholds. Five consecutive failures — one full
// DefaultRetryConfig retry cycle's worth — open the circuit for 30 seconds;
// two clean probes in half-open close it again.
const (
	// DefaultFailureThreshold is the number of consecutive failures that
	// opens the circuit.
	DefaultFailureThreshold = 5
	// DefaultOpenDuration is how long the circuit stays open before a probe
	// is allowed through.
	DefaultOpenDuration = 30 * time.Second
	// DefaultSuccessThreshold is the number of consecutive successes needed
	// in half-open to close the circuit again.
	DefaultSuccessThreshold = 2
)

// ErrCircuitOpen is returned when a call is attempted while the circuit is
// open. It means "this dependency has failed often enough that we stopped
// asking" — the caller should fail fast and surface an outage, not retry.
var ErrCircuitOpen = errors.New("circuit open: dependency is unavailable")

// CircuitState is the state of a CircuitBreaker.
type CircuitState int

const (
	// StateClosed lets every call through and counts failures.
	StateClosed CircuitState = iota
	// StateOpen rejects every call with ErrCircuitOpen until OpenDuration
	// elapses.
	StateOpen
	// StateHalfOpen lets a limited number of probe calls through to test
	// whether the dependency has recovered.
	StateHalfOpen
)

// String returns the lowercase state name used in logs and error messages.
func (s CircuitState) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// BreakerConfig configures a CircuitBreaker. Zero fields fall back to the
// Default* constants.
type BreakerConfig struct {
	// Name identifies the dependency in error messages and state-change
	// callbacks, e.g. the registry hostname.
	Name string

	// FailureThreshold is the number of consecutive failures that opens the
	// circuit.
	FailureThreshold int

	// OpenDuration is how long the circuit stays open before allowing a
	// half-open probe.
	OpenDuration time.Duration

	// SuccessThreshold is the number of consecutive successes needed in
	// half-open to close the circuit.
	SuccessThreshold int

	// Now returns the current time. nil installs time.Now. Tests replace it
	// to travel through the state machine without waiting.
	Now func() time.Time

	// OnStateChange, when non-nil, is called on every state transition. Use
	// it for logging or metrics; it runs while the breaker's lock is held, so
	// it must be cheap and must not call back into the breaker.
	OnStateChange func(name string, from, to CircuitState)
}

// CircuitBreaker stops a caller from hammering a dependency that is
// persistently unavailable.
//
// State machine:
//
//	closed --- failureThreshold consecutive failures ---> open
//	open   --- OpenDuration elapses ---> half-open
//	half-open --- successThreshold consecutive successes ---> closed
//	half-open --- any failure ---> open (for another OpenDuration)
//
// While open, Execute returns ErrCircuitOpen immediately without calling the
// dependency — the "fail fast" that turns an outage storm into a handful of
// probes. While half-open only one probe runs at a time, so a recovering
// service is not hit with the full backlog at once.
//
// A CircuitBreaker is safe for concurrent use.
type CircuitBreaker struct {
	mu  sync.Mutex
	cfg BreakerConfig

	state      CircuitState
	failures   int // consecutive failures in the current state
	successes  int // consecutive successes in half-open
	openedAt   time.Time
	probing    bool  // a half-open probe is in flight
	lastErr    error // most recent failure, for reporting
	totalOpens int   // times the circuit has opened
}

// NewCircuitBreaker creates a CircuitBreaker in the closed state.
func NewCircuitBreaker(cfg BreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = DefaultFailureThreshold
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = DefaultOpenDuration
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = DefaultSuccessThreshold
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &CircuitBreaker{cfg: cfg, state: StateClosed}
}

// Allow reports whether a call may proceed. It returns nil when the call
// should go ahead, or an error wrapping ErrCircuitOpen when the circuit is
// open (or when another half-open probe is already in flight).
//
// Every successful Allow must be followed by exactly one Record call with the
// call's outcome; Execute does that bookkeeping for you.
func (b *CircuitBreaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.cfg.Now()

	switch b.state {
	case StateClosed:
		return nil

	case StateOpen:
		if now.Sub(b.openedAt) < b.cfg.OpenDuration {
			return b.openErr(now)
		}
		// The open period has elapsed: let one probe test the dependency.
		b.setState(StateHalfOpen)
		b.failures = 0
		b.successes = 0
		b.probing = true
		return nil

	default: // StateHalfOpen
		if b.probing {
			return b.openErr(now)
		}
		b.probing = true
		return nil
	}
}

// Record reports the outcome of a call that Allow approved. A nil err is a
// success; a non-nil err is a failure regardless of its class, because the
// call did go out and did not work.
func (b *CircuitBreaker) Record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		b.probing = false
		if err == nil {
			b.failures = 0
			b.successes++
			if b.successes >= b.cfg.SuccessThreshold {
				b.setState(StateClosed)
			}
			return
		}
		b.lastErr = err
		b.successes = 0
		b.trip()
		return

	default: // StateClosed
		if err == nil {
			b.failures = 0
			return
		}
		b.lastErr = err
		b.failures++
		if b.failures >= b.cfg.FailureThreshold {
			b.trip()
		}
	}
}

// Execute runs fn if the circuit allows it, records the outcome and returns
// fn's error. When the circuit is open it returns an error wrapping
// ErrCircuitOpen without calling fn.
func (b *CircuitBreaker) Execute(fn func() error) error {
	if err := b.Allow(); err != nil {
		return err
	}
	err := fn()
	b.Record(err)
	return err
}

// State returns the current state.
func (b *CircuitBreaker) State() CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Failures returns the number of consecutive failures counted in the current
// state.
func (b *CircuitBreaker) Failures() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures
}

// LastError returns the most recent failure recorded, or nil.
func (b *CircuitBreaker) LastError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastErr
}

// TotalOpens returns how many times the circuit has opened since creation.
func (b *CircuitBreaker) TotalOpens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalOpens
}

// Reset closes the circuit and clears the failure count. Intended for tests
// and for an operator who has fixed the dependency and does not want to wait
// out the open period.
func (b *CircuitBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
	b.successes = 0
	b.failures = 0
	b.setState(StateClosed)
}

// trip opens the circuit. Callers must hold b.mu.
func (b *CircuitBreaker) trip() {
	b.openedAt = b.cfg.Now()
	b.totalOpens++
	b.setState(StateOpen)
}

// openErr builds the fail-fast error. Callers must hold b.mu.
func (b *CircuitBreaker) openErr(now time.Time) error {
	remaining := b.cfg.OpenDuration - now.Sub(b.openedAt)
	if remaining < 0 {
		remaining = 0
	}
	return WrapPermanent(fmt.Errorf("%w: %s (retry after %s)", ErrCircuitOpen, b.cfg.Name, remaining.Round(time.Millisecond)))
}

// setState moves to the given state and fires the callback. Callers must hold
// b.mu.
func (b *CircuitBreaker) setState(to CircuitState) {
	if b.state == to {
		return
	}
	from := b.state
	b.state = to
	if b.cfg.OnStateChange != nil {
		b.cfg.OnStateChange(b.cfg.Name, from, to)
	}
}

// BreakerSet hands out one CircuitBreaker per dependency name (typically a
// registry hostname) created on first use with a shared configuration, so each
// registry's failures are counted separately and a Verisign outage does not
// trip the Google Registry's circuit.
type BreakerSet struct {
	mu       sync.Mutex
	cfg      BreakerConfig
	breakers map[string]*CircuitBreaker
}

// NewBreakerSet creates a BreakerSet. The config is copied for each breaker it
// creates; Name is replaced with the dependency name.
func NewBreakerSet(cfg BreakerConfig) *BreakerSet {
	return &BreakerSet{cfg: cfg, breakers: make(map[string]*CircuitBreaker)}
}

// Get returns the breaker for name, creating it on first use.
func (s *BreakerSet) Get(name string) *CircuitBreaker {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b, ok := s.breakers[name]; ok {
		return b
	}
	cfg := s.cfg
	cfg.Name = name
	b := NewCircuitBreaker(cfg)
	s.breakers[name] = b
	return b
}

// States returns a snapshot of every breaker in the set, keyed by name. Used
// by diagnostics and tests.
func (s *BreakerSet) States() map[string]CircuitState {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]CircuitState, len(s.breakers))
	for name, b := range s.breakers {
		out[name] = b.State()
	}
	return out
}
