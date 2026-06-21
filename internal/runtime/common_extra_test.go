package runtime

import (
	"context"
	"errors"
	"io"
	"testing"

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
		{"context cancelled", context.Canceled, false},
		{"containerd unavailable", errdefs.ErrUnavailable, true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"timeout", errors.New("timeout"), true},
		{"deadline exceeded", errors.New("deadline exceeded"), true},
		{"unexpected eof", errors.New("unexpected eof"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"no such host", errors.New("no such host"), true},
		{"tls handshake timeout", errors.New("tls handshake timeout"), true},
		{"client.timeout exceeded", errors.New("client.timeout exceeded"), true},
		{"rate limit exceeded", errors.New("rate limit exceeded"), true},
		{"toomanyrequests", errors.New("toomanyrequests"), true},
		{"rate exceeded", errors.New("rate exceeded"), true},
		{"rate limit", errors.New("rate limit"), true},
		{"data limit exceeded", errors.New("data limit exceeded"), true},
		{"token expired", errors.New("token expired"), true},
		{"expired token", errors.New("expired token"), true},
		{"refresh token", errors.New("refresh token"), true},
		{"reauthenticate", errors.New("reauthenticate"), true},
		{"token refresh", errors.New("token refresh"), true},
		{"standard eof", io.EOF, false},
		{"unexpected eof (io)", io.ErrUnexpectedEOF, true},
		{"other error", errors.New("something went wrong"), false},
		{"mixed case", errors.New("Rate Limit Exceeded"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryablePullError(tt.err))
		})
	}
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
		{"forbidden", errors.New("forbidden"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTemporaryAuthError(tt.err))
		})
	}
}
