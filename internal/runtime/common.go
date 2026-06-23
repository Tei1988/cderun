package runtime

import (
	"context"
	"errors"
	"io"
	"net"
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
	if errdefs.IsCanceled(err) || errors.Is(err, context.Canceled) {
		return false
	}

	// containerd/errdefs
	if errdefs.IsUnavailable(err) || errdefs.IsDeadlineExceeded(err) {
		return true
	}

	// Standard context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// EOF errors
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}

		// DNS Error handling
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			// If it's "no such host", it's usually a configuration error or permanent name issue (NXDOMAIN).
			// NXDOMAIN is indicated by IsNotFound=true.
			if dnsErr.IsNotFound {
				return false
			}
			return true // Other DNS errors (e.g. temporary server failure) are retryable
		}
	}

	msg := strings.ToLower(err.Error())

	// "no such host" should not be retried as it's likely a configuration error.
	if strings.Contains(msg, "no such host") {
		return false
	}

	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"timed out",
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
