package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_NerdctlLifecycle_PullImage_BoundaryPaths tests NerdctlRuntime PullImage boundary
// and error paths including missing image fallback failure, structural verification rejection,
// and SleepFunc cancellation during exponential backoff.
// Corresponds to docs/features/direct-container-execution.md and docs/features/multi-runtime-support.md.
func TestUnit_NerdctlLifecycle_PullImage_BoundaryPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-pull.sh")

	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
case "$1" in
    image)
        # Always report image missing
        exit 1
        ;;
    pull)
        # Fail with retryable pull error text
        echo "failed to fetch: network connection reset" >&2
        exit 1
        ;;
    *)
        exit 1
        ;;
esac
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("PullImage missing policy pulls and returns retry error when not found", func(t *testing.T) {
		pullErr := rt.PullImage(ctx, "absent-image:v1", "missing", 2, 5*time.Millisecond)
		require.Error(t, pullErr)
		assert.Contains(t, pullErr.Error(), "failed to pull image absent-image:v1 with nerdctl")
	})

	t.Run("PullImage structural verification failure on control characters in image name", func(t *testing.T) {
		// NewCLIArgBuilder VerifyStructure rejects control characters (\x01)
		pullErr := rt.PullImage(ctx, "malformed\x01image:latest", "always", 1, 10*time.Millisecond)
		require.Error(t, pullErr)
		assert.Contains(t, pullErr.Error(), "nerdctl pull structural verification failed")
	})

	t.Run("PullImage context canceled during retry backoff sleep", func(t *testing.T) {
		ctxTimeout, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		pullErr := rt.PullImage(ctxTimeout, "retry-sleep-cancel:v1", "always", 5, time.Hour)
		require.Error(t, pullErr)
		assert.ErrorIs(t, pullErr, context.DeadlineExceeded)
	})
}

// TestUnit_NerdctlLifecycle_CreateContainer_ErrorPaths tests NerdctlRuntime CreateContainer
// handling of CLIArgBuilder verification rejections, exit errors without stderr, and binary failure.
// Corresponds to docs/features/direct-container-execution.md.
func TestUnit_NerdctlLifecycle_CreateContainer_ErrorPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-create.sh")

	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
if [ "$1" = "create" ]; then
    shift
    for arg in "$@"; do
        if [ "$arg" = "silent-fail:latest" ]; then
            # Exit with non-zero status without printing anything to stderr
            exit 1
        fi
    done
fi
exit 0
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("CreateContainer structural verification failure on control character in command", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo\x07bell"},
		}
		_, createErr := rt.CreateContainer(ctx, cfg)
		require.Error(t, createErr)
		assert.Contains(t, createErr.Error(), "nerdctl create structural verification failed")
	})

	t.Run("CreateContainer exit error without stderr payload", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "silent-fail:latest",
		}
		_, createErr := rt.CreateContainer(ctx, cfg)
		require.Error(t, createErr)
		assert.Contains(t, createErr.Error(), "nerdctl create failed: exit status 1")
		assert.NotContains(t, createErr.Error(), "stderr:")
	})

	t.Run("CreateContainer binary execution failure on non-existent binary", func(t *testing.T) {
		missingRt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(filepath.Join(tmpDir, "non-existent-binary")))
		require.NoError(t, err)

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
		}
		_, createErr := missingRt.CreateContainer(ctx, cfg)
		require.Error(t, createErr)
		assert.Contains(t, createErr.Error(), "nerdctl create failed")
	})
}

// TestUnit_NerdctlLifecycle_OperationsAndInspect_ErrorPaths tests NerdctlRuntime operations
// (WaitContainer, InspectContainer, AttachContainer, SignalContainer, RemoveContainer, StartContainer)
// under various error modes (non-ExitError, malformed outputs, and nil channels).
// Corresponds to docs/features/direct-container-execution.md.
func TestUnit_NerdctlLifecycle_OperationsAndInspect_ErrorPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-ops.sh")

	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
cmd="$1"
shift

target=""
for arg in "$@"; do
    target="$arg"
done

case "$cmd" in
    wait)
        if [ "$target" = "silent-fail" ]; then
            exit 1 # exit without stderr
        fi
        echo "0"
        exit 0
        ;;
    inspect)
        if [ "$target" = "silent-fail" ]; then
            exit 1 # exit without stderr
        elif [ "$target" = "single-field" ]; then
            echo "true"
            exit 0
        elif [ "$target" = "empty-inspect" ]; then
            echo ""
            exit 0
        fi
        echo "container missing" >&2
        exit 1
        ;;
    attach)
        if [ "$target" = "attach-fail" ]; then
            echo "failed to attach" >&2
            exit 1
        fi
        exit 0
        ;;
    start)
        if [ "$target" = "start-fail" ]; then
            echo "failed to start" >&2
            exit 1
        fi
        exit 0
        ;;
    kill)
        if [ "$target" = "kill-fail" ]; then
            echo "failed to send signal" >&2
            exit 1
        fi
        exit 0
        ;;
    rm)
        if [ "$target" = "rm-fail" ]; then
            echo "failed to remove" >&2
            exit 1
        fi
        exit 0
        ;;
    *)
        exit 0
        ;;
esac
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("WaitContainer exit error without stderr", func(t *testing.T) {
		code, waitErr := rt.WaitContainer(ctx, "silent-fail")
		assert.Equal(t, -1, code)
		require.Error(t, waitErr)
		assert.Contains(t, waitErr.Error(), "nerdctl wait failed: exit status 1")
		assert.NotContains(t, waitErr.Error(), "stderr:")
	})

	t.Run("WaitContainer non-existent binary returns error", func(t *testing.T) {
		missingRt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(filepath.Join(tmpDir, "missing-bin")))
		require.NoError(t, err)

		code, waitErr := missingRt.WaitContainer(ctx, "c123")
		assert.Equal(t, -1, code)
		require.Error(t, waitErr)
		assert.Contains(t, waitErr.Error(), "nerdctl wait failed")
	})

	t.Run("InspectContainer exit error without stderr", func(t *testing.T) {
		running, exitCode, inspectErr := rt.InspectContainer(ctx, "silent-fail")
		assert.False(t, running)
		assert.Equal(t, -1, exitCode)
		require.Error(t, inspectErr)
		assert.Contains(t, inspectErr.Error(), "nerdctl inspect failed: exit status 1")
		assert.NotContains(t, inspectErr.Error(), "stderr:")
	})

	t.Run("InspectContainer malformed output with single field", func(t *testing.T) {
		running, exitCode, inspectErr := rt.InspectContainer(ctx, "single-field")
		assert.False(t, running)
		assert.Equal(t, -1, exitCode)
		require.Error(t, inspectErr)
		assert.Contains(t, inspectErr.Error(), "malformed nerdctl inspect output: true")
	})

	t.Run("InspectContainer malformed output with empty string", func(t *testing.T) {
		running, exitCode, inspectErr := rt.InspectContainer(ctx, "empty-inspect")
		assert.False(t, running)
		assert.Equal(t, -1, exitCode)
		require.Error(t, inspectErr)
		assert.Contains(t, inspectErr.Error(), "malformed nerdctl inspect output")
	})

	t.Run("AttachContainer with nil ready channel", func(t *testing.T) {
		var stdout bytes.Buffer
		attachErr := rt.AttachContainer(ctx, "c123", false, nil, &stdout, nil, nil)
		assert.NoError(t, attachErr)
	})

	t.Run("AttachContainer startup failure on missing binary", func(t *testing.T) {
		missingRt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(filepath.Join(tmpDir, "missing-bin")))
		require.NoError(t, err)

		ready := make(chan struct{})
		attachErr := missingRt.AttachContainer(ctx, "c123", false, nil, nil, nil, ready)
		require.Error(t, attachErr)
		assert.Contains(t, attachErr.Error(), "nerdctl attach start failed")
	})

	t.Run("StartContainer failure reporting combined output", func(t *testing.T) {
		startErr := rt.StartContainer(ctx, "start-fail")
		require.Error(t, startErr)
		assert.Contains(t, startErr.Error(), "nerdctl start failed")
		assert.Contains(t, startErr.Error(), "failed to start")
	})

	t.Run("SignalContainer failure reporting combined output", func(t *testing.T) {
		sigErr := rt.SignalContainer(ctx, "kill-fail", "SIGKILL")
		require.Error(t, sigErr)
		assert.Contains(t, sigErr.Error(), "nerdctl kill failed")
		assert.Contains(t, sigErr.Error(), "failed to send signal")
	})

	t.Run("RemoveContainer failure reporting combined output", func(t *testing.T) {
		rmErr := rt.RemoveContainer(ctx, "rm-fail")
		require.Error(t, rmErr)
		assert.Contains(t, rmErr.Error(), "nerdctl rm failed")
		assert.Contains(t, rmErr.Error(), "failed to remove")
	})

	t.Run("ResizeContainerTTY boundary dimensions", func(t *testing.T) {
		customLogger := logging.NewLogger()
		loggedRt, err := NewNerdctlRuntime("/sock", WithNerdctlLogger(customLogger))
		require.NoError(t, err)

		assert.NoError(t, loggedRt.ResizeContainerTTY(ctx, "c123", 0, 0))
		assert.NoError(t, loggedRt.ResizeContainerTTY(ctx, "c123", 65535, 65535))
	})
}
