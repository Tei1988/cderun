package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// customCanceledError is used to test errdefs.IsCanceled
type customCanceledError struct{ error }
func (e customCanceledError) Cancelled() {}

// customUnavailableError is used to test errdefs.IsUnavailable
type customUnavailableError struct{ error }
func (e customUnavailableError) Unavailable() {}

// customDeadlineExceededError is used to test errdefs.IsDeadlineExceeded
type customDeadlineExceededError struct{ error }
func (e customDeadlineExceededError) DeadlineExceeded() {}

func TestUnit_Runtime_Common_IsRetryablePullError_Extra(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("something went wrong"), false},
		{"containerd canceled", customCanceledError{error: errors.New("canceled")}, false},
		{"containerd unavailable", customUnavailableError{error: errors.New("unavailable")}, true},
		{"containerd deadline exceeded", customDeadlineExceededError{error: errors.New("deadline")}, true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"timeout", errors.New("timeout"), true},
		{"deadline exceeded string", errors.New("deadline exceeded"), true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"no such host", errors.New("no such host"), true},
		{"tls handshake timeout", errors.New("tls handshake timeout"), true},
		{"client.timeout exceeded", errors.New("client.timeout exceeded"), true},
		{"rate limit exceeded", errors.New("rate limit exceeded"), true},
		{"toomanyrequests", errors.New("toomanyrequests"), true},
		{"rate exceeded", errors.New("rate exceeded"), true},
		{"rate limit", errors.New("rate limit"), true},
		{"data limit exceeded", errors.New("data limit exceeded"), true},
		{"auth: token expired", errors.New("token expired"), true},
		{"auth: expired token", errors.New("expired token"), true},
		{"auth: refresh token", errors.New("refresh token"), true},
		{"auth: reauthenticate", errors.New("reauthenticate"), true},
		{"auth: token refresh", errors.New("token refresh"), true},
		{"mixed case: Connection Refused", errors.New("Connection Refused"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryablePullError(tt.err))
		})
	}
}

func TestUnit_Runtime_Common_IsTemporaryAuthError_Extra(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"not auth error", errors.New("not authorized"), false},
		{"token expired", errors.New("token expired"), true},
		{"expired token", errors.New("expired token"), true},
		{"refresh token", errors.New("refresh token"), true},
		{"reauthenticate", errors.New("reauthenticate"), true},
		{"token refresh", errors.New("token refresh"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTemporaryAuthError(tt.err))
		})
	}
}

func TestUnit_Runtime_Common_SleepFunc_Extra(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := SleepFunc(context.Background(), 1*time.Millisecond)
		require.NoError(t, err)
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := SleepFunc(ctx, 1*time.Second)
		require.ErrorIs(t, err, context.Canceled)
	})
}
