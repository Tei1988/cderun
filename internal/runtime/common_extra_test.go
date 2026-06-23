package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
)

func TestUnit_Common_IsRetryablePullError_Extra(t *testing.T) {
	t.Run("context.Canceled returns false", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(context.Canceled))
	})

	t.Run("errdefs.IsUnavailable returns true", func(t *testing.T) {
		err := errdefs.ErrUnavailable
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("context.DeadlineExceeded returns true", func(t *testing.T) {
		assert.True(t, IsRetryablePullError(context.DeadlineExceeded))
	})

	t.Run("all retryable messages", func(t *testing.T) {
		messages := []string{
			"connection refused",
			"connection reset",
			"timeout",
			"deadline exceeded",
			"unexpected eof",
			"i/o timeout",
			"tls handshake timeout",
			"client.timeout exceeded",
			"rate limit exceeded",
			"toomanyrequests",
			"rate exceeded",
			"rate limit",
			"data limit exceeded",
		}
		for _, msg := range messages {
			assert.True(t, IsRetryablePullError(errors.New(msg)), "Expected retryable for: %s", msg)
		}

		// "no such host" should NOT be retryable (T12)
		assert.False(t, IsRetryablePullError(errors.New("no such host")))
	})
}

func TestUnit_Common_IsTemporaryAuthError_Extra(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsTemporaryAuthError(nil))
	})

	t.Run("refreshable keywords", func(t *testing.T) {
		keywords := []string{
			"token expired",
			"expired token",
			"refresh token",
			"reauthenticate",
			"token refresh",
		}
		for _, kw := range keywords {
			assert.True(t, IsTemporaryAuthError(fmt.Errorf("error with %s keyword", kw)))
		}
	})

	t.Run("non-refreshable error", func(t *testing.T) {
		assert.False(t, IsTemporaryAuthError(errors.New("permanent auth failure")))
	})
}
