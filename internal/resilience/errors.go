// Package resilience provides health checks, retry with exponential backoff,
// and circuit breaking for domain-check's external service dependencies
// (RDAP registries, the inference gateway, other HTTP endpoints).
//
// The three building blocks compose:
//
//	retry := resilience.DefaultRetryConfig()
//	err := resilience.Do(ctx, retry, func(attempt int) error {
//	    if err := breaker.Allow(); err != nil {
//	        return resilience.WrapPermanent(err) // fail fast, no retries
//	    }
//	    ...
//	})
//
// Errors carry a class (transient or permanent) so the retry loop — and the
// operators reading its output — can tell a blip worth retrying from a
// condition that will never succeed however many times it is attempted.
package resilience

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Classification sentinels. An error wrapped in TransientError or
// PermanentError satisfies errors.Is against the matching sentinel, so callers
// can test a class without knowing the concrete type:
//
//	if resilience.IsTransient(err) { ... }
var (
	// ErrTransient marks an error as worth retrying.
	ErrTransient = errors.New("transient failure")
	// ErrPermanent marks an error that will not succeed on retry.
	ErrPermanent = errors.New("permanent failure")
)

// Class says whether an error is worth retrying.
type Class int

const (
	// ClassUnknown means the error carries no classification. Callers decide
	// their own default; Do treats it as transient (network errors are
	// usually blips) while CheckHealth reports it verbatim.
	ClassUnknown Class = iota
	// ClassTransient errors are worth retrying: a bad gateway, a dropped
	// connection, a rate-limit window.
	ClassTransient
	// ClassPermanent errors are not: a cancelled context, an allowlist
	// rejection, a 404.
	ClassPermanent
)

// String returns the lowercase class name used in error messages and logs.
func (c Class) String() string {
	switch c {
	case ClassTransient:
		return "transient"
	case ClassPermanent:
		return "permanent"
	default:
		return "unknown"
	}
}

// TransientError marks its wrapped error as retryable.
type TransientError struct {
	// Op describes what was being attempted, e.g. "rdap query example.com".
	// Empty is allowed and omits the prefix.
	Op  string
	Err error
}

func (e *TransientError) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("%v: %v", ErrTransient, e.Err)
	}
	return fmt.Sprintf("%s: %v (transient, retry may succeed)", e.Op, e.Err)
}

// Unwrap exposes the wrapped error so errors.Is/As keep working through it.
func (e *TransientError) Unwrap() error { return e.Err }

// Is matches the ErrTransient sentinel so errors.Is(err, ErrTransient) works.
func (e *TransientError) Is(target error) bool { return target == ErrTransient }

// PermanentError marks its wrapped error as not worth retrying.
type PermanentError struct {
	// Op describes what was being attempted. Empty is allowed.
	Op  string
	Err error
}

func (e *PermanentError) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("%v: %v", ErrPermanent, e.Err)
	}
	return fmt.Sprintf("%s: %v (permanent, retry will not help)", e.Op, e.Err)
}

// Unwrap exposes the wrapped error so errors.Is/As keep working through it.
func (e *PermanentError) Unwrap() error { return e.Err }

// Is matches the ErrPermanent sentinel so errors.Is(err, ErrPermanent) works.
func (e *PermanentError) Is(target error) bool { return target == ErrPermanent }

// WrapTransient returns err classified as transient, or nil if err is nil.
func WrapTransient(err error) error {
	if err == nil {
		return nil
	}
	return &TransientError{Err: err}
}

// WrapPermanent returns err classified as permanent, or nil if err is nil.
func WrapPermanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsTransient reports whether err is classified transient.
func IsTransient(err error) bool { return errors.Is(err, ErrTransient) }

// IsPermanent reports whether err is classified permanent.
func IsPermanent(err error) bool { return errors.Is(err, ErrPermanent) }

// Classify returns the class carried by err. Context cancellation and a bare
// ErrCircuitOpen are permanent: both mean "stop, further attempts are futile".
// An unrecognised error is ClassUnknown.
func Classify(err error) Class {
	switch {
	case err == nil:
		return ClassUnknown
	case errors.Is(err, ErrTransient):
		return ClassTransient
	case errors.Is(err, ErrPermanent), errors.Is(err, ErrCircuitOpen):
		return ClassPermanent
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ClassPermanent
	default:
		return ClassUnknown
	}
}

// TransientHTTPStatuses lists the HTTP status codes worth retrying, with the
// reason each is considered transient. 502 and 503 are the cases named in the
// retry strategy; 408, 429 and 504 are the standard companions (a timed-out
// request, an exhausted rate-limit window, an upstream that answered too
// late).
//
// 500 is deliberately absent: registries return it for malformed queries, so
// the same answer would come back on every retry and only burn the request
// budget.
var TransientHTTPStatuses = map[int]string{
	http.StatusRequestTimeout:     "request timeout",
	http.StatusTooManyRequests:    "rate limited",
	http.StatusBadGateway:         "bad gateway",
	http.StatusServiceUnavailable: "service unavailable",
	http.StatusGatewayTimeout:     "gateway timeout",
}

// IsTransientStatus reports whether the HTTP status code is worth retrying.
func IsTransientStatus(code int) bool {
	_, ok := TransientHTTPStatuses[code]
	return ok
}

// StatusReason returns the human-readable reason a status is transient, or ""
// for statuses outside TransientHTTPStatuses.
func StatusReason(code int) string { return TransientHTTPStatuses[code] }

// StatusClass classifies an HTTP status code: transient for the codes in
// TransientHTTPStatuses, permanent for every other non-2xx, unknown for 2xx
// (which is not an error at all).
func StatusClass(code int) Class {
	if code >= 200 && code < 300 {
		return ClassUnknown
	}
	if IsTransientStatus(code) {
		return ClassTransient
	}
	return ClassPermanent
}
