package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	dockererrdefs "github.com/docker/docker/errdefs"
)

// IsRetryablePullError returns true if the error from a pull operation is likely transient and worth retrying.
func IsRetryablePullError(err error) bool {
	if err == nil {
		return false
	}

	// containerd/errdefs
	if errdefs.IsUnavailable(err) || errdefs.IsDeadlineExceeded(err) || errdefs.IsCanceled(err) {
		return true
	}

	// docker/errdefs
	// SA1019: dockererrdefs.IsSystem is deprecated: use containerd [cerrdefs.IsInternal]
	if dockererrdefs.IsSystem(err) || dockererrdefs.IsUnknown(err) || dockererrdefs.IsDeadline(err) || dockererrdefs.IsCancelled(err) { //nolint:staticcheck
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
		"request canceled",
		"rate limit exceeded",
		"toomanyrequests",
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
	return strings.Contains(msg, "unauthorized") || strings.Contains(msg, "403 forbidden") || strings.Contains(msg, "token expired")
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
