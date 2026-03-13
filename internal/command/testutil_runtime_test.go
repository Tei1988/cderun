//go:build runtime

package command

import (
	"strings"
	"testing"
)

func skipIfDockerBroken(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "failed to mount") && strings.Contains(msg, "invalid argument") {
		t.Skip("Skipping test due to Docker mount limitation in this environment (likely overlay-on-overlay)")
	}
	if strings.Contains(msg, "Data limit exceeded") || strings.Contains(msg, "pull rate limit") {
		t.Skip("Skipping test due to Docker Hub rate limit")
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection refused") {
		t.Skipf("Skipping test due to transient network/runtime issue: %v", err)
	}
	// Detect Docker SIGKILL timeout (likely environment resource constraint or slow CI)
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "SIGKILL") {
		t.Skip("Skipping test due to Docker SIGKILL timeout (likely environment resource constraint)")
	}
}
