package runtime

import (
	"strings"
)

func isRetryablePullError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// List of keywords indicating transient registry or connection issues.
	retryableKeywords := []string{
		"toomanyrequests", "rate exceeded", "rate limit", "data limit exceeded",
		"i/o timeout", "connection refused", "connection reset", "eof",
	}

	for _, kw := range retryableKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}

	return isTemporaryAuthError(err)
}

func isTemporaryAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Catch temporary token issues or specific hints for re-authentication
	return strings.Contains(msg, "token expired")
}
