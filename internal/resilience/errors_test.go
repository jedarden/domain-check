package resilience

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassString(t *testing.T) {
	assert.Equal(t, "unknown", ClassUnknown.String())
	assert.Equal(t, "transient", ClassTransient.String())
	assert.Equal(t, "permanent", ClassPermanent.String())
	assert.Equal(t, "unknown", Class(99).String(), "out-of-range values fall back to unknown")
}

func TestWrapAndClassify(t *testing.T) {
	cause := errors.New("boom")

	tests := []struct {
		name string
		err  error
		want Class
	}{
		{"nil error", nil, ClassUnknown},
		{"unclassified error", cause, ClassUnknown},
		{"transient sentinel", ErrTransient, ClassTransient},
		{"permanent sentinel", ErrPermanent, ClassPermanent},
		{"wrapped transient", WrapTransient(cause), ClassTransient},
		{"wrapped permanent", WrapPermanent(cause), ClassPermanent},
		{"transient through a wrapper", fmt.Errorf("calling upstream: %w", WrapTransient(cause)), ClassTransient},
		{"circuit open is permanent", fmt.Errorf("call refused: %w", ErrCircuitOpen), ClassPermanent},
		{"context cancellation is permanent", context.Canceled, ClassPermanent},
		{"context deadline is permanent", context.DeadlineExceeded, ClassPermanent},
		// The explicit class marker wins: the sentinel check precedes the
		// built-in context rules, so a caller that wraps a cancellation in
		// TransientError has declared it retryable.
		{"cancelled context wrapped in transient", WrapTransient(context.Canceled), ClassTransient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Classify(tt.err))
		})
	}
}

func TestIsTransientAndIsPermanent(t *testing.T) {
	cause := errors.New("boom")

	assert.True(t, IsTransient(WrapTransient(cause)))
	assert.False(t, IsPermanent(WrapTransient(cause)))
	assert.True(t, IsPermanent(WrapPermanent(cause)))
	assert.False(t, IsTransient(WrapPermanent(cause)))

	// A bare cause carries no class at all.
	assert.False(t, IsTransient(cause))
	assert.False(t, IsPermanent(cause))

	// Nil is neither.
	assert.False(t, IsTransient(nil))
	assert.False(t, IsPermanent(nil))

	// errors.Is against the sentinels works through arbitrary wrapping.
	wrapped := fmt.Errorf("rdap query: %w", WrapTransient(cause))
	assert.ErrorIs(t, wrapped, ErrTransient)
	assert.NotErrorIs(t, wrapped, ErrPermanent)
}

func TestClassifiedErrorMessages(t *testing.T) {
	cause := errors.New("connection refused")

	t.Run("transient with op", func(t *testing.T) {
		err := &TransientError{Op: "rdap query example.com", Err: cause}
		assert.Contains(t, err.Error(), "rdap query example.com")
		assert.Contains(t, err.Error(), "connection refused")
		assert.Contains(t, err.Error(), "transient")
		assert.ErrorIs(t, err, ErrTransient)
		assert.ErrorIs(t, err, cause, "Unwrap keeps the original cause reachable")
	})

	t.Run("transient without op", func(t *testing.T) {
		err := &TransientError{Err: cause}
		assert.Contains(t, err.Error(), "transient failure")
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("permanent with op", func(t *testing.T) {
		err := &PermanentError{Op: "validate domain", Err: cause}
		assert.Contains(t, err.Error(), "validate domain")
		assert.Contains(t, err.Error(), "connection refused")
		assert.Contains(t, err.Error(), "permanent")
		assert.ErrorIs(t, err, ErrPermanent)
		assert.ErrorIs(t, err, cause)
	})

	t.Run("permanent without op", func(t *testing.T) {
		err := &PermanentError{Err: cause}
		assert.Contains(t, err.Error(), "permanent failure")
		assert.Contains(t, err.Error(), "connection refused")
	})
}

func TestWrapHelpersReturnNilForNil(t *testing.T) {
	assert.NoError(t, WrapTransient(nil))
	assert.NoError(t, WrapPermanent(nil))
}

func TestTransientHTTPStatuses(t *testing.T) {
	// The retryable set: the documented 502/503 plus the standard companions.
	for _, code := range []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		assert.Contains(t, TransientHTTPStatuses, code, "HTTP %d should be transient", code)
		assert.True(t, IsTransientStatus(code), "HTTP %d should be transient", code)
		assert.NotEmpty(t, StatusReason(code), "HTTP %d should carry a reason", code)
	}

	// 500 is deliberately absent: registries return it for malformed queries,
	// so retrying would only burn the request budget.
	assert.False(t, IsTransientStatus(http.StatusInternalServerError))
	assert.False(t, IsTransientStatus(http.StatusNotFound))
	assert.False(t, IsTransientStatus(http.StatusBadRequest))
	assert.False(t, IsTransientStatus(http.StatusOK))
	assert.False(t, IsTransientStatus(599))
	assert.Empty(t, StatusReason(http.StatusInternalServerError))
}

func TestStatusClass(t *testing.T) {
	tests := []struct {
		name string
		code int
		want Class
	}{
		{"200 is not an error", http.StatusOK, ClassUnknown},
		{"204 is not an error", http.StatusNoContent, ClassUnknown},
		{"299 is still 2xx", 299, ClassUnknown},
		{"408 request timeout", http.StatusRequestTimeout, ClassTransient},
		{"429 rate limited", http.StatusTooManyRequests, ClassTransient},
		{"502 bad gateway", http.StatusBadGateway, ClassTransient},
		{"503 service unavailable", http.StatusServiceUnavailable, ClassTransient},
		{"504 gateway timeout", http.StatusGatewayTimeout, ClassTransient},
		{"404 not found", http.StatusNotFound, ClassPermanent},
		{"400 bad request", http.StatusBadRequest, ClassPermanent},
		{"500 internal error", http.StatusInternalServerError, ClassPermanent},
		{"501 not implemented", http.StatusNotImplemented, ClassPermanent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StatusClass(tt.code))
		})
	}
}

func TestStatusReasonMatchesSet(t *testing.T) {
	// Every entry in the map must be non-empty and reachable through
	// StatusReason, so a log line never renders "HTTP 503 ()".
	for code := range TransientHTTPStatuses {
		require.NotEmpty(t, TransientHTTPStatuses[code])
		assert.Equal(t, TransientHTTPStatuses[code], StatusReason(code))
	}
}
