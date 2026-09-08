package resilience

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sleepRecorder stands in for a real Sleep so tests observe the exact backoff
// schedule Do asks for without waiting for it. It also records the attempt
// sequence through fn via calls.
type sleepRecorder struct {
	delays []time.Duration
}

func (s *sleepRecorder) sleep(_ context.Context, d time.Duration) error {
	s.delays = append(s.delays, d)
	return nil
}

func (s *sleepRecorder) config(cfg RetryConfig) RetryConfig {
	cfg.Sleep = s.sleep
	return cfg
}

// transientErr is a deliberately unclassified error: Do treats unknown errors
// as transient, which is what most raw transport errors look like.
var transientErr = errors.New("connection reset by peer")

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.BaseDelay)
	assert.Equal(t, 32*time.Second, cfg.MaxDelay)
	assert.Equal(t, time.Duration(DefaultMaxElapsedTime), cfg.MaxElapsedTime)
	assert.Equal(t, 6, cfg.Attempts(), "5 retries + the initial attempt")
}

func TestDefaultInteractiveRetryConfig(t *testing.T) {
	cfg := DefaultInteractiveRetryConfig()
	assert.Equal(t, DefaultMaxRetries, cfg.MaxRetries)
	assert.Equal(t, DefaultBaseDelay, cfg.BaseDelay)
	assert.Equal(t, 8*time.Second, cfg.MaxDelay, "interactive cap is tightened to 8s")
	assert.Equal(t, 10*time.Second, cfg.MaxElapsedTime, "interactive budget is 10s")
}

func TestRetryConfigAttempts(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		want       int
	}{
		{"zero retries means one attempt", 0, 1},
		{"five retries means six attempts", 5, 6},
		{"negative retries clamp to one", -3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RetryConfig{MaxRetries: tt.maxRetries}
			assert.Equal(t, tt.want, cfg.Attempts())
		})
	}
}

func TestRetryConfigDelay(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RetryConfig
		attempt int
		want    time.Duration
	}{
		// The default schedule: 1s, 2s, 4s, 8s, 16s, then the 32s cap.
		{"default attempt 1", DefaultRetryConfig(), 1, 1 * time.Second},
		{"default attempt 2", DefaultRetryConfig(), 2, 2 * time.Second},
		{"default attempt 3", DefaultRetryConfig(), 3, 4 * time.Second},
		{"default attempt 4", DefaultRetryConfig(), 4, 8 * time.Second},
		{"default attempt 5", DefaultRetryConfig(), 5, 16 * time.Second},
		{"default attempt 6 lands on the cap", DefaultRetryConfig(), 6, 32 * time.Second},
		{"default attempt 7 saturates", DefaultRetryConfig(), 7, 32 * time.Second},
		{"default attempt 100 does not overflow", DefaultRetryConfig(), 100, 32 * time.Second},
		{"attempt 0 clamps to the first wait", DefaultRetryConfig(), 0, 1 * time.Second},
		{"negative attempt clamps to the first wait", DefaultRetryConfig(), -4, 1 * time.Second},
		// A tighter cap saturates earlier.
		{"interactive attempt 4", DefaultInteractiveRetryConfig(), 4, 8 * time.Second},
		{"interactive attempt 5 saturates at 8s", DefaultInteractiveRetryConfig(), 5, 8 * time.Second},
		{"no cap keeps doubling", RetryConfig{BaseDelay: time.Second}, 3, 4 * time.Second},
		{"one millisecond base", RetryConfig{BaseDelay: time.Millisecond, MaxDelay: 4 * time.Millisecond}, 4, 4 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.Delay(tt.attempt))
		})
	}
}

func TestDoRetriesThenSucceeds(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	var attempts []int
	err := Do(context.Background(), cfg, func(attempt int) error {
		attempts = append(attempts, attempt)
		if attempt < 4 {
			return WrapTransient(fmt.Errorf("attempt %d failed", attempt))
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4}, attempts)
	// Three failures back off 1s, 2s, 4s — the doubling schedule, not a
	// constant delay.
	assert.Equal(t, []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}, rec.delays)
}

func TestDoNoRetryOnPermanent(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	permanent := WrapPermanent(errors.New("404 not found"))
	var calls int
	err := Do(context.Background(), cfg, func(int) error {
		calls++
		return permanent
	})

	// The permanent error is returned as-is, unwrapped and unadorned, and the
	// loop never sleeps: retrying cannot help.
	assert.Same(t, permanent, err)
	assert.Equal(t, 1, calls)
	assert.Empty(t, rec.delays)
}

func TestDoRetriesUnclassifiedErrors(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	var calls int
	err := Do(context.Background(), cfg, func(attempt int) error {
		calls++
		if attempt == 1 {
			return transientErr // no class attached: network blip shape
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, calls, "an unclassified error is treated as transient")
	assert.Equal(t, []time.Duration{1 * time.Second}, rec.delays)
}

func TestDoExhaustsRetries(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	var calls int
	err := Do(context.Background(), cfg, func(int) error {
		calls++
		return transientErr
	})

	require.Error(t, err)
	var exhausted *AttemptsExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.Equal(t, 6, exhausted.Attempts, "initial attempt + 5 retries")
	assert.Equal(t, 6, calls)
	assert.Len(t, rec.delays, 5, "one sleep between each pair of attempts")
	assert.ErrorIs(t, err, ErrTransient, "exhaustion is itself transient")
	assert.ErrorIs(t, err, transientErr, "Unwrap exposes the final cause")
	assert.Contains(t, err.Error(), "6 attempts")
	assert.Contains(t, err.Error(), "5 retries")
}

func TestDoContextCancelledBeforeStart(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls int
	err := Do(ctx, cfg, func(int) error {
		calls++
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, calls, "a dead context never reaches fn")
	assert.Empty(t, rec.delays)
}

func TestDoContextCancelledDuringFn(t *testing.T) {
	rec := &sleepRecorder{}
	cfg := rec.config(DefaultRetryConfig())

	ctx, cancel := context.WithCancel(context.Background())

	err := Do(ctx, cfg, func(int) error {
		cancel() // the caller gave up while fn was running
		return transientErr
	})

	require.ErrorIs(t, err, context.Canceled,
		"a cancellation mid-attempt is reported as such, not dressed up as exhaustion")
	assert.Empty(t, rec.delays)
}

func TestDoContextCancellationInterruptsBackoff(t *testing.T) {
	started := make(chan struct{})
	cfg := DefaultRetryConfig()
	cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
		close(started) // we are parked in the backoff wait
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Do(ctx, cfg, func(int) error { return transientErr })
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Do did not return after the context was cancelled mid-backoff")
	}
}

func TestSleepContextReturnsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A cancelled context cuts a long wait short immediately.
	err := sleepContext(ctx, time.Hour)
	require.ErrorIs(t, err, context.Canceled)

	// A short wait completes and reports nil.
	err = sleepContext(context.Background(), time.Millisecond)
	assert.NoError(t, err)

	// A zero wait does not arm a timer at all.
	err = sleepContext(context.Background(), 0)
	assert.NoError(t, err)
}

func TestDoMaxElapsedTimeStopsTheLoop(t *testing.T) {
	// Real (short) sleeps: the 40ms second backoff alone overshoots the 25ms
	// budget, so the loop must stop after the first wait regardless of timing.
	cfg := RetryConfig{
		MaxRetries:     10,
		BaseDelay:      20 * time.Millisecond,
		MaxDelay:       50 * time.Millisecond,
		MaxElapsedTime: 25 * time.Millisecond,
	}

	var calls int
	err := Do(context.Background(), cfg, func(int) error {
		calls++
		return transientErr
	})

	var exhausted *AttemptsExhaustedError
	require.ErrorAs(t, err, &exhausted)
	// The loop must stop before a backoff would overshoot the budget: two
	// calls, one 20ms wait, no third attempt.
	assert.Equal(t, 2, calls)

	// Known quirk, characterised here rather than "fixed" (the package's
	// implementation is owned elsewhere): AttemptsExhaustedError.Attempts
	// carries the CONFIGURED attempt count (MaxRetries+1 = 11 here), not the
	// number of attempts actually made, so a budget-cut loop reports
	// "persisted after 11 attempts" having called fn twice. The failure-count
	// message is therefore an upper bound, and callers needing the real count
	// should watch fn themselves.
	assert.Equal(t, 11, exhausted.Attempts)
}

func TestDoMaxElapsedTimeDoesNotCutAShortSchedule(t *testing.T) {
	// Fake sleeps never advance the real clock, so a generous budget must let
	// every attempt through — proving the budget only bites when it is small.
	rec := &sleepRecorder{}
	cfg := rec.config(RetryConfig{
		MaxRetries:     5,
		BaseDelay:      1 * time.Second,
		MaxDelay:       32 * time.Second,
		MaxElapsedTime: 10 * time.Minute,
	})

	var calls int
	err := Do(context.Background(), cfg, func(int) error {
		calls++
		return transientErr
	})

	var exhausted *AttemptsExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.Equal(t, 6, exhausted.Attempts)
	assert.Equal(t, 6, calls)
	assert.Equal(t,
		[]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
		rec.delays)
}

func TestDoZeroValueConfigMakesOneAttempt(t *testing.T) {
	// The zero value never retries and never waits: one call, no sleeps, and
	// the exhaustion error reports a single attempt.
	var calls int
	err := Do(context.Background(), RetryConfig{}, func(int) error {
		calls++
		return transientErr
	})

	var exhausted *AttemptsExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.Equal(t, 1, exhausted.Attempts)
	assert.Equal(t, 1, calls)
}

func TestDoNilFunction(t *testing.T) {
	err := Do(context.Background(), DefaultRetryConfig(), nil)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "a programming error is permanent, not retryable")
}

func TestDoSleepErrorPropagates(t *testing.T) {
	sleepFailure := errors.New("sleep interrupted")
	cfg := DefaultRetryConfig()
	cfg.Sleep = func(context.Context, time.Duration) error { return sleepFailure }

	err := Do(context.Background(), cfg, func(int) error { return transientErr })
	assert.Same(t, sleepFailure, err)
}

func TestAttemptsExhaustedErrorRendering(t *testing.T) {
	last := errors.New("final cause")
	err := &AttemptsExhaustedError{Attempts: 3, Last: last}

	assert.Equal(t, 3, err.Attempts)
	assert.ErrorIs(t, err, ErrTransient)
	assert.ErrorIs(t, err, last)
	assert.False(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "3 attempts")
	assert.Contains(t, err.Error(), "2 retries")
	assert.True(t, strings.Contains(err.Error(), "final cause"))
}
