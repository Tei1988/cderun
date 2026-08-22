package runtime

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/containerd/errdefs"
)

var signalRegex = regexp.MustCompile(`^(?i)[A-Z0-9]+$`)

var retryablePullMessages = []string{
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

var refreshableAuthKeywords = []string{
	"token expired",
	"expired token",
	"refresh token",
	"reauthenticate",
	"token refresh",
}

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

	// Typed error check: DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		// If it's "no such host", it's likely a typo or missing registry.
		if dnsErr.IsNotFound {
			return false
		}
		// Other DNS errors (e.g. server failure, timeout) are worth retrying.
		return true
	}

	// Typed error check: Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	for _, m := range retryablePullMessages {
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
	for _, kw := range refreshableAuthKeywords {
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
