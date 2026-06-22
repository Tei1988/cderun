package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
)

func TestUnit_Common_IsRetryablePullError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"timeout", errors.New("timeout"), true},
		{"deadline exceeded string", errors.New("deadline exceeded"), true},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"io EOF", io.EOF, false},
		{"no such host", errors.New("no such host"), true},
		{"rate limit exceeded", errors.New("rate limit exceeded"), true},
		{"toomanyrequests", errors.New("toomanyrequests"), true},
		{"token expired", errors.New("token expired"), true},
		{"generic error", errors.New("generic error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryablePullError(tt.err))
		})
	}

	t.Run("containerd errors", func(t *testing.T) {
		// Use errdefs.IsCanceled to verify we are testing what containerd uses
		assert.False(t, IsRetryablePullError(context.Canceled))
		assert.True(t, errdefs.IsCanceled(context.Canceled))
	})
}

func TestUnit_Common_IsTemporaryAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"token expired", errors.New("token expired"), true},
		{"expired token", errors.New("expired token"), true},
		{"refresh token", errors.New("refresh token"), true},
		{"reauthenticate", errors.New("reauthenticate"), true},
		{"token refresh", errors.New("token refresh"), true},
		{"unauthorized", errors.New("unauthorized"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTemporaryAuthError(tt.err))
		})
	}
}

func TestUnit_Common_SleepFunc(t *testing.T) {
	t.Run("sleep completes", func(t *testing.T) {
		start := time.Now()
		err := SleepFunc(context.Background(), 10*time.Millisecond)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond)
	})

	t.Run("sleep cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()
		err := SleepFunc(ctx, 100*time.Millisecond)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
