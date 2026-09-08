package resilience

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock hands the breaker a controllable Now so tests move through the
// state machine without sleeping for OpenDuration. Single-goroutine use only.
type fakeClock struct {
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1_770_000_000, 0).UTC()}
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestBreaker(clock *fakeClock, threshold int) *CircuitBreaker {
	return NewCircuitBreaker(BreakerConfig{
		Name:             "test-dependency",
		FailureThreshold: threshold,
		OpenDuration:     30 * time.Second,
		SuccessThreshold: 2,
		Now:              clock.Now,
	})
}

func TestNewCircuitBreakerDefaults(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{})

	assert.Equal(t, StateClosed, b.State())
	assert.Equal(t, 0, b.Failures())
	assert.NoError(t, b.LastError())
	assert.Equal(t, 0, b.TotalOpens())
	assert.NoError(t, b.Allow(), "a fresh breaker lets calls through")

	// The defaults come from the package constants: 5 failures, 30s open,
	// 2 successes to recover.
	for i := 0; i < DefaultFailureThreshold-1; i++ {
		b.Record(errors.New("fail"))
	}
	assert.Equal(t, StateClosed, b.State(), "threshold-1 failures do not trip")
	b.Record(errors.New("fail"))
	assert.Equal(t, StateOpen, b.State(), "DefaultFailureThreshold failures trip")
	assert.Equal(t, DefaultFailureThreshold, b.Failures())
}

func TestBreakerTripsAfterThreshold(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 3)

	boom := errors.New("dependency down")
	for i := 1; i <= 2; i++ {
		require.NoError(t, b.Allow())
		b.Record(boom)
		assert.Equal(t, StateClosed, b.State(), "failure %d of 3 must not trip yet", i)
		assert.Equal(t, i, b.Failures())
	}

	require.NoError(t, b.Allow())
	b.Record(boom)
	assert.Equal(t, StateOpen, b.State())
	assert.Equal(t, 1, b.TotalOpens())
	assert.Equal(t, boom, b.LastError())
}

func TestBreakerFailsFastWhileOpen(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))

	// While open the dependency is never called: Execute returns the
	// fail-fast error without running fn.
	var calls int
	err := b.Execute(func() error { calls++; return nil })

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitOpen)
	assert.True(t, IsPermanent(err), "fail-fast is permanent: no retries")
	assert.Zero(t, calls)
	assert.Contains(t, err.Error(), "test-dependency")
	assert.Equal(t, StateOpen, b.State())
}

func TestBreakerOpenErrorNamesTheRetryDelay(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))
	clock.advance(10 * time.Second)

	err := b.Allow()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "20s", "the remaining open time is reported")
}

func TestBreakerHalfOpensAfterOpenDuration(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))

	// Just before the open period elapses the circuit still refuses calls.
	clock.advance(29 * time.Second)
	require.Error(t, b.Allow())
	assert.Equal(t, StateOpen, b.State())

	// At exactly the open duration the boundary is inclusive: one probe is
	// let through and the breaker reports half-open.
	clock.advance(time.Second)
	require.NoError(t, b.Allow())
	assert.Equal(t, StateHalfOpen, b.State())
}

func TestBreakerClosesAfterSuccessThreshold(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1) // success threshold is 2 via newTestBreaker

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))

	// First probe succeeds: still half-open, one more probe needed.
	clock.advance(30 * time.Second)
	require.NoError(t, b.Allow())
	b.Record(nil)
	assert.Equal(t, StateHalfOpen, b.State(), "one success is not enough to close")

	// Second probe succeeds: the circuit closes.
	require.NoError(t, b.Allow())
	b.Record(nil)
	assert.Equal(t, StateClosed, b.State())
	assert.Equal(t, 0, b.Failures(), "closing clears the failure count")
	assert.Equal(t, 1, b.TotalOpens())
}

func TestBreakerReopensOnHalfOpenFailure(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))

	// A probe that fails reopens the circuit for another full open period.
	clock.advance(30 * time.Second)
	require.NoError(t, b.Allow())
	b.Record(errors.New("still down"))

	assert.Equal(t, StateOpen, b.State())
	assert.Equal(t, 2, b.TotalOpens(), "the second open is counted separately")

	// And the new open period starts from the second trip, not the first.
	clock.advance(29 * time.Second)
	require.Error(t, b.Allow(), "the reopened circuit must wait its own open duration")
	clock.advance(time.Second)
	require.NoError(t, b.Allow())
	assert.Equal(t, StateHalfOpen, b.State())
}

func TestBreakerAllowsOneProbeAtATime(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))
	clock.advance(30 * time.Second)

	// The first Allow claims the probe slot; a concurrent caller is refused
	// until that probe is recorded.
	require.NoError(t, b.Allow())
	err := b.Allow()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitOpen)

	// Recording the probe frees the slot for the next caller.
	b.Record(nil)
	assert.Equal(t, StateHalfOpen, b.State())
	require.NoError(t, b.Allow())
}

func TestBreakerSuccessResetsFailureCount(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 3)

	boom := errors.New("dependency down")
	for i := 0; i < 2; i++ {
		require.NoError(t, b.Allow())
		b.Record(boom)
	}
	require.NoError(t, b.Allow())
	b.Record(nil) // a success clears the consecutive-failure count
	assert.Equal(t, 0, b.Failures())

	// Two further failures must therefore not trip a threshold of 3.
	for i := 0; i < 2; i++ {
		require.NoError(t, b.Allow())
		b.Record(boom)
	}
	assert.Equal(t, StateClosed, b.State())
	assert.Equal(t, 0, b.TotalOpens())
}

func TestBreakerExecuteRecordsOutcome(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 3)

	boom := errors.New("dependency down")

	// A failing Execute is recorded, so failures accumulate toward the trip.
	require.Error(t, b.Execute(func() error { return boom }))
	require.Error(t, b.Execute(func() error { return boom }))
	assert.Equal(t, 2, b.Failures())

	// A succeeding Execute resets the count, and fn's error is returned as-is.
	assert.NoError(t, b.Execute(func() error { return nil }))
	assert.Equal(t, 0, b.Failures())

	// The dependency's own error is surfaced, not replaced by the breaker's.
	sentinel := errors.New("upstream said no")
	err := b.Execute(func() error { return sentinel })
	assert.Same(t, sentinel, err)
}

func TestBreakerStateChangeCallbacks(t *testing.T) {
	clock := newFakeClock()
	var transitions []string
	b := NewCircuitBreaker(BreakerConfig{
		Name:             "callback-dependency",
		FailureThreshold: 1,
		OpenDuration:     30 * time.Second,
		SuccessThreshold: 1,
		Now:              clock.Now,
		OnStateChange: func(name string, from, to CircuitState) {
			assert.Equal(t, "callback-dependency", name)
			transitions = append(transitions, from.String()+"->"+to.String())
		},
	})

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down")) // closed -> open
	clock.advance(30 * time.Second)
	require.NoError(t, b.Allow()) // open -> half-open
	b.Record(nil)                 // half-open -> closed

	assert.Equal(t, []string{"closed->open", "open->half-open", "half-open->closed"}, transitions)
}

func TestBreakerReset(t *testing.T) {
	clock := newFakeClock()
	b := newTestBreaker(clock, 1)

	require.NoError(t, b.Allow())
	b.Record(errors.New("dependency down"))
	require.Equal(t, StateOpen, b.State())

	// An operator who has fixed the dependency need not wait out the open
	// period: Reset closes the circuit and clears the counters.
	b.Reset()
	assert.Equal(t, StateClosed, b.State())
	assert.Equal(t, 0, b.Failures())
	assert.NoError(t, b.Allow())
	assert.Equal(t, StateClosed, b.State(), "the next call is not a probe")
}

func TestBreakerSetIsolation(t *testing.T) {
	clock := newFakeClock()
	set := NewBreakerSet(BreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     30 * time.Second,
		SuccessThreshold: 1,
		Now:              clock.Now,
	})

	verisign := set.Get("rdap.verisign.com")
	google := set.Get("pubapi.registry.google")

	// Tripping Verisign must not touch Google Registry's circuit.
	require.Error(t, verisign.Execute(func() error { return errors.New("verisign down") }))
	assert.Equal(t, StateOpen, verisign.State())
	assert.Equal(t, StateClosed, google.State())

	// Each breaker carries its own dependency name, not the shared config's.
	assert.ErrorIs(t, verisign.Allow(), ErrCircuitOpen)
	require.NoError(t, google.Allow())
	assert.Equal(t, StateClosed, google.State(), "the unrelated circuit still serves traffic")

	states := set.States()
	assert.Equal(t, StateOpen, states["rdap.verisign.com"])
	assert.Equal(t, StateClosed, states["pubapi.registry.google"])
}

func TestBreakerSetGetReturnsSameInstance(t *testing.T) {
	set := NewBreakerSet(BreakerConfig{Now: newFakeClock().Now})

	first := set.Get("rdap.verisign.com")
	second := set.Get("rdap.verisign.com")
	assert.Same(t, first, second, "a name always maps to the same breaker")
	assert.Len(t, set.States(), 1)

	set.Get("pubapi.registry.google")
	assert.Len(t, set.States(), 2)
}
