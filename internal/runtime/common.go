package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/containerd/errdefs"
)

// IsRetryablePullError returns true if the error from a pull operation is likely transient and worth retrying.
func IsRetryablePullError(err error) bool {
	if err == nil {
		return false
	}

	// Explicit cancellation should not be retried.
	if errdefs.IsCanceled(err) {
		return false
	}

	// containerd/errdefs
	if errdefs.IsUnavailable(err) || errdefs.IsDeadlineExceeded(err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadline exceeded",
		"unexpected eof",
		"i/o timeout",
		"no such host",
		"tls handshake timeout",
		"client.timeout exceeded",
		"rate limit exceeded",
		"toomanyrequests",
		"rate exceeded",
		"rate limit",
		"data limit exceeded",
		"eof",
	}

	for _, m := range retryableMessages {
		if strings.Contains(msg, m) {
			return true
		}
	}

	return IsTemporaryAuthError(err)
}

// IsTemporaryAuthError returns true if the error is a temporary authentication or authorization failure.
func IsTemporaryAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Only return true for transient/refreshable conditions.
	refreshableKeywords := []string{
		"token expired",
		"expired token",
		"refresh token",
		"reauthenticate",
		"token refresh",
	}
	for _, kw := range refreshableKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// SleepFunc is a helper for cancellable sleep.
func SleepFunc(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
