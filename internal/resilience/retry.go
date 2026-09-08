package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Default retry parameters. delay = BaseDelay * 2^(attempt-1), so with the
// defaults the five retries wait 1s, 2s, 4s, 8s and 16s — a cumulative 31s of
// backoff after the initial attempt.
const (
	// DefaultMaxRetries is the number of retries after the initial attempt.
	DefaultMaxRetries = 5
	// DefaultBaseDelay is the wait before the first retry.
	DefaultBaseDelay = 1 * time.Second
	// DefaultMaxDelay caps a single backoff interval.
	DefaultMaxDelay = 32 * time.Second
	// DefaultMaxElapsedTime (0) means no overall budget: the caller's context
	// deadline is the only wall-clock bound.
	DefaultMaxElapsedTime = 0
)

// RetryConfig controls the exponential-backoff loop in Do.
//
// The zero value performs a single attempt with no delay: it never retries.
// Use DefaultRetryConfig for the standard policy, or set the fields directly.
type RetryConfig struct {
	// MaxRetries is the number of retries after the initial attempt.
	// DefaultMaxRetries when set through DefaultRetryConfig.
	MaxRetries int

	// BaseDelay is the wait before the first retry. The wait before retry N
	// (1-based) is BaseDelay * 2^(N-1), capped at MaxDelay.
	BaseDelay time.Duration

	// MaxDelay caps a single backoff interval so long-running loops cannot
	// back off into minutes.
	MaxDelay time.Duration

	// MaxElapsedTime bounds the cumulative time spent in backoff across the
	// whole loop. Zero disables the budget — the caller's context deadline
	// then governs. Set it on interactive paths so retries cannot outlive the
	// request they serve (see DefaultInteractiveRetryConfig).
	MaxElapsedTime time.Duration

	// Sleep waits between attempts. It is called with the caller's context so
	// a cancelled context interrupts the backoff. nil installs sleepContext.
	// Tests replace it to observe the exact schedule without waiting.
	Sleep func(ctx context.Context, d time.Duration) error
}

// DefaultRetryConfig returns the standard policy: 5 retries after the initial
// attempt, 1s base delay doubling each attempt, 32s cap on a single wait and
// no overall time budget (the caller's context deadline governs).
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		BaseDelay:      DefaultBaseDelay,
		MaxDelay:       DefaultMaxDelay,
		MaxElapsedTime: DefaultMaxElapsedTime,
	}
}

// DefaultInteractiveRetryConfig returns the policy used on request paths, where
// the retry loop must finish well inside the server's 30s request timeout.
// It is DefaultRetryConfig with MaxDelay tightened to 8s and a 10s cumulative
// backoff budget, so the loop stops after roughly four attempts and leaves the
// caller time to answer.
func DefaultInteractiveRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		BaseDelay:      DefaultBaseDelay,
		MaxDelay:       8 * time.Second,
		MaxElapsedTime: 10 * time.Second,
	}
}

// Attempts returns the total number of calls to fn, retries included. Always
// at least one.
func (c RetryConfig) Attempts() int {
	if c.MaxRetries < 0 {
		return 1
	}
	return c.MaxRetries + 1
}

// Delay returns the wait before retry number attempt (1-based). It doubles
// per attempt and is capped at MaxDelay. The doubling is done in a loop rather
// than by shifting so a large attempt number saturates at the cap instead of
// overflowing time.Duration.
func (c RetryConfig) Delay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := c.BaseDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		// Saturate at the cap; a wrapped (overflowed) duration is treated the
		// same way.
		if d <= 0 || (c.MaxDelay > 0 && d >= c.MaxDelay) {
			if c.MaxDelay > 0 {
				return c.MaxDelay
			}
			break
		}
	}
	return d
}

// withDefaults fills unset fields. MaxRetries below zero, a non-positive
// BaseDelay and a missing Sleep are corrected; MaxDelay and MaxElapsedTime are
// left alone because zero is meaningful for both (no cap / no budget).
func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = DefaultBaseDelay
	}
	if c.Sleep == nil {
		c.Sleep = sleepContext
	}
	return c
}

// AttemptsExhaustedError reports that every attempt failed with a transient
// error. It is itself transient: whatever went wrong may still clear, so the
// caller should report it as an outage, not as a client mistake.
type AttemptsExhaustedError struct {
	// Attempts is the total number of attempts made (retries included).
	Attempts int
	// Last is the final underlying error.
	Last error
}

func (e *AttemptsExhaustedError) Error() string {
	return fmt.Sprintf("transient failure persisted after %d attempts (%d retries): %v",
		e.Attempts, e.Attempts-1, e.Last)
}

// Unwrap exposes the final underlying error.
func (e *AttemptsExhaustedError) Unwrap() error { return e.Last }

// Is matches the ErrTransient sentinel.
func (e *AttemptsExhaustedError) Is(target error) bool { return target == ErrTransient }

// Do calls fn repeatedly with exponential backoff until it succeeds, the error
// is permanent, the context is cancelled, or the retries run out.
//
// fn receives the 1-based attempt number. It returns nil on success. A
// transient (or unclassified) error is retried after
// BaseDelay * 2^(attempt-1); a permanent error aborts the loop immediately and
// is returned unwrapped. A context cancelled inside fn or during a backoff
// wait aborts the loop and returns the context error.
//
// MaxElapsedTime, when non-zero, stops the loop before a backoff would push
// the cumulative wait past the budget. Retries never widen the caller's
// deadline: a context deadline cuts the loop off mid-backoff.
//
// When every attempt fails the returned error is an AttemptsExhaustedError —
// its message names the attempt count and the final underlying error, so a log
// line distinguishes "the service was down for all 6 tries" from "the request
// was refused".
func Do(ctx context.Context, cfg RetryConfig, fn func(attempt int) error) error {
	if fn == nil {
		return WrapPermanent(errors.New("resilience: Do called with nil function"))
	}
	cfg = cfg.withDefaults()

	start := time.Now()
	attempts := cfg.Attempts()

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(attempt)
		if err == nil {
			return nil
		}
		lastErr = err

		// Permanent failures never succeed on retry — surface them as they
		// are so the caller sees the real cause.
		if Classify(err) == ClassPermanent {
			return err
		}

		// The caller gave up while fn was running; report that rather than
		// dressing it up as a service failure.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if attempt == attempts {
			break
		}

		delay := cfg.Delay(attempt)
		if cfg.MaxElapsedTime > 0 && time.Since(start)+delay > cfg.MaxElapsedTime {
			break
		}

		if err := cfg.Sleep(ctx, delay); err != nil {
			return err
		}
	}

	return &AttemptsExhaustedError{Attempts: attempts, Last: lastErr}
}

// sleepContext waits for d or until ctx is done, returning the context error
// on cancellation.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
