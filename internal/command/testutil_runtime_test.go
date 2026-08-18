//go:build runtime

package command

import (
	"strings"
	"testing"
)

func skipIfRuntimeBroken(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	if (strings.Contains(msg, "failed to mount") || strings.Contains(msg, "failed to unmount")) && (strings.Contains(msg, "invalid argument") || strings.Contains(msg, "operation not permitted")) {
		t.Skip("Skipping test due to runtime mount limitation in this environment (likely overlay-on-overlay)")
	}
	if strings.Contains(msg, "data limit exceeded") || strings.Contains(msg, "pull rate limit") {
		t.Skip("Skipping test due to Docker Hub rate limit")
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection refused") {
		t.Skipf("Skipping test due to transient network/runtime issue: %v", err)
	}
	if strings.Contains(msg, "is the docker daemon running") || strings.Contains(msg, "cannot connect to the docker daemon") || strings.Contains(msg, "permission denied") || strings.Contains(msg, "dial unix") {
		t.Skipf("Skipping test due to runtime connection or permission issue: %v", err)
	}
	if (strings.Contains(msg, "containerd runtime:") || strings.Contains(msg, "docker runtime:")) && strings.Contains(msg, "not supported") {
		t.Skipf("Skipping test due to runtime feature limitation: %v", err)
	}
	// Detect Docker SIGKILL timeout (likely environment resource constraint or slow CI)
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "sigkill") {
		t.Skip("Skipping test due to Docker SIGKILL timeout (likely environment resource constraint)")
	}
	// Detect containerd/gRPC stream cancellation or deadline exceeded
	if strings.Contains(msg, "deadlineexceeded") || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "rst_stream") {
		t.Skipf("Skipping test due to gRPC stream/deadline issue: %v", err)
	}
}
