package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_IsRetryablePullError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"wrapped context canceled", fmt.Errorf("wrapped: %w", context.Canceled), false},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"containerd unavailable", fmt.Errorf("wrapped: %w", errdefs.ErrUnavailable), true},
		{"connection refused string", errors.New("connection refused"), true},
		{"connection reset string", errors.New("connection reset by peer"), true},
		{"timeout string", errors.New("operation timeout"), true},
		{"timed out string", errors.New("operation timed out"), true},
		{"unexpected eof string", errors.New("unexpected EOF"), true},
		{"no such host string", errors.New("no such host"), false},
		{"net.DNSError not found", &net.DNSError{IsNotFound: true, Err: "no such host", Name: "example.com"}, false},
		{"net.DNSError temporary", &net.DNSError{IsTemporary: true, Err: "server fail", Name: "example.com"}, true},
		{"net.OpError timeout", &net.OpError{Op: "read", Net: "tcp", Err: &timeoutError{}}, true},
		{"net.OpError not timeout", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("other")}, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"rate limit exceeded string", errors.New("rate limit exceeded"), true},
		{"toomanyrequests string", errors.New("toomanyrequests"), true},
		{"token expired string", errors.New("token expired"), true},
		{"generic error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRetryablePullError(tt.err))
		})
	}
}

func TestUnit_IsTemporaryAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"token expired", errors.New("your token expired"), true},
		{"refresh token", errors.New("need to refresh token"), true},
		{"generic error", errors.New("auth failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTemporaryAuthError(tt.err))
		})
	}
}

func TestUnit_Sleep_Completes(t *testing.T) {
	start := time.Now()
	err := SleepFunc(context.Background(), 10*time.Millisecond)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond)
}

func TestUnit_Sleep_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	err := SleepFunc(ctx, 100*time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
