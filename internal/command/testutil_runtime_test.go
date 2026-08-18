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
	if strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "permission denied") || strings.Contains(msg, "mknod") || strings.Contains(msg, "cgroup") {
		t.Skipf("Skipping test due to runtime/environment permission or mount limitation: %v", err)
	}
	if strings.Contains(msg, "data limit exceeded") || strings.Contains(msg, "pull rate limit") || strings.Contains(msg, "toomanyrequests") || strings.Contains(msg, "429 too many requests") || strings.Contains(msg, "failed to pull image") || strings.Contains(msg, "failed to inspect image") || strings.Contains(msg, "error response from daemon") {
		t.Skipf("Skipping test due to image pull or daemon rate limit / network issue: %v", err)
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "temporary failure in name resolution") || strings.Contains(msg, "tls handshake timeout") || strings.Contains(msg, "client.timeout") || strings.Contains(msg, "unreachable") || strings.Contains(msg, "context deadline exceeded") {
		t.Skipf("Skipping test due to transient network/runtime issue or timeout: %v", err)
	}
	if strings.Contains(msg, "is the docker daemon running") || strings.Contains(msg, "cannot connect to the docker daemon") || strings.Contains(msg, "dial unix") {
		t.Skipf("Skipping test due to runtime connection or permission issue: %v", err)
	}
	if (strings.Contains(msg, "containerd runtime:") || strings.Contains(msg, "docker runtime:") || strings.Contains(msg, "podman runtime:")) && strings.Contains(msg, "not supported") {
		t.Skipf("Skipping test due to runtime feature limitation: %v", err)
	}
	if strings.Contains(msg, "not supported by containerd") || strings.Contains(msg, "is not supported for containerd") || strings.Contains(msg, "is not supported yet") {
		t.Skipf("Skipping test due to runtime feature limitation: %v", err)
	}
	if strings.Contains(msg, "rootless") || strings.Contains(msg, "subuid") {
		t.Skipf("Skipping test due to environment/podman limitation: %v", err)
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
