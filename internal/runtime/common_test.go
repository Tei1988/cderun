package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

type mockCancelled struct{}

func (mockCancelled) Error() string { return "cancelled" }
func (mockCancelled) Cancelled()    {}

type mockDeadlineExceeded struct{}

func (mockDeadlineExceeded) Error() string     { return "deadline exceeded" }
func (mockDeadlineExceeded) DeadlineExceeded() {}

type mockUnavailable struct{}

func (mockUnavailable) Error() string { return "unavailable" }
func (mockUnavailable) Unavailable()  {}

func TestUnit_Common_IsRetryablePullError_Typed(t *testing.T) {
	t.Run("DNSError - NotFound", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: true}
		assert.False(t, IsRetryablePullError(err))
	})

	t.Run("DNSError - Temporary", func(t *testing.T) {
		err := &net.DNSError{IsTemporary: true}
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("NetError - Timeout", func(t *testing.T) {
		err := &timeoutErr{}
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("String matching still works", func(t *testing.T) {
		assert.True(t, IsRetryablePullError(errors.New("connection refused")))
		assert.True(t, IsRetryablePullError(errors.New("toomanyrequests")))
		assert.True(t, IsRetryablePullError(errors.New("connection reset")))
		assert.True(t, IsRetryablePullError(errors.New("unexpected EOF")))
		assert.True(t, IsRetryablePullError(errors.New("TLS handshake timeout")))
		assert.True(t, IsRetryablePullError(errors.New("Client.Timeout exceeded")))
		assert.True(t, IsRetryablePullError(errors.New("rate exceeded")))
		assert.True(t, IsRetryablePullError(errors.New("rate limit")))
		assert.True(t, IsRetryablePullError(errors.New("data limit exceeded")))
	})

	t.Run("Wrapped errors", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: true}
		wrappedProperly := fmt.Errorf("wrap: %w", err)
		assert.False(t, IsRetryablePullError(wrappedProperly))

		timeout := &timeoutErr{}
		wrappedTimeout := fmt.Errorf("wrap: %w", timeout)
		assert.True(t, IsRetryablePullError(wrappedTimeout))
	})

	t.Run("errdefs checks", func(t *testing.T) {
		// context.Canceled is not retryable
		assert.False(t, IsRetryablePullError(context.Canceled))
		// mock error with Cancelled() method
		assert.False(t, IsRetryablePullError(mockCancelled{}))

		// errdefs.ErrUnavailable is retryable
		assert.True(t, IsRetryablePullError(errdefs.ErrUnavailable))
		// mock error with Unavailable() method
		assert.True(t, IsRetryablePullError(mockUnavailable{}))

		// context.DeadlineExceeded is retryable
		assert.True(t, IsRetryablePullError(context.DeadlineExceeded))
		// mock error with DeadlineExceeded() method
		assert.True(t, IsRetryablePullError(mockDeadlineExceeded{}))
	})

	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(nil))
	})
}

func TestUnit_Common_IsTemporaryAuthError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("unauthorized"), false},
		{errors.New("token expired"), true},
		{errors.New("expired token"), true},
		{errors.New("refresh token"), true},
		{errors.New("reauthenticate"), true},
		{errors.New("token refresh"), true},
		{fmt.Errorf("wrap: %w", errors.New("token expired")), true},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTemporaryAuthError(tt.err))
		})
	}
}

func TestUnit_Common_SleepFunc(t *testing.T) {
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
