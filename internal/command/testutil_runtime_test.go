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
	if isRuntimeBackendError(msg) {
		t.Skipf("Skipping test due to runtime/backend limitation or setup issue: %v", err)
	}
}

func isRuntimeBackendError(msg string) bool {
	if (strings.Contains(msg, "failed to mount") || strings.Contains(msg, "failed to unmount")) &&
		(strings.Contains(msg, "invalid argument") || strings.Contains(msg, "operation not permitted")) {
		return true
	}
	if strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "mknod") || strings.Contains(msg, "cgroup") {
		return true
	}
	if strings.Contains(msg, "data limit exceeded") || strings.Contains(msg, "pull rate limit") || strings.Contains(msg, "toomanyrequests") || strings.Contains(msg, "429 too many requests") {
		return true
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "temporary failure in name resolution") || strings.Contains(msg, "tls handshake timeout") || strings.Contains(msg, "client.timeout") {
		return true
	}
	if strings.Contains(msg, "is the docker daemon running") || strings.Contains(msg, "cannot connect to the docker daemon") || strings.Contains(msg, "dial unix") {
		return true
	}
	if (strings.Contains(msg, "containerd runtime:") || strings.Contains(msg, "docker runtime:") || strings.Contains(msg, "podman runtime:")) && strings.Contains(msg, "not supported") {
		return true
	}
	if strings.Contains(msg, "not supported by containerd") || strings.Contains(msg, "is not supported for containerd") || strings.Contains(msg, "is not supported yet") {
		return true
	}
	if strings.Contains(msg, "rootless") || strings.Contains(msg, "subuid") {
		return true
	}
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "sigkill") {
		return true
	}
	if strings.Contains(msg, "deadlineexceeded") || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "rst_stream") {
		return true
	}
	return false
}
