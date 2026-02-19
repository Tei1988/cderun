package command

import (
	"bytes"
	"cderun/internal/runtime"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCderun runs the cderun command in-process for integration testing.
// It captures stdout and stderr and returns the exit code.
// Note: This function no longer modifies global state for rootCmd and opts,
// but it still modifies process-global streams (os.Stdout, os.Stderr)
// and is NOT safe for parallel execution with t.Parallel().
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, args...)
}

// runCderunWithOptions runs the cderun command in-process with a custom setup for options.
func runCderunWithOptions(stdin io.Reader, setup func(*rootOptions), args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCoreFull(stdin, setup, args...)
}

// runCderunCore is the shared implementation for in-process command execution.
func runCderunCore(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCoreFull(stdin, nil, args...)
}

// runCderunCoreFull is the full implementation for in-process command execution.
func runCderunCoreFull(stdin io.Reader, setup func(*rootOptions), args ...string) (stdout, stderr string, exitCode int, err error) {
	savedStdout := os.Stdout
	savedStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		return "", "", 0, err
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		return "", "", 0, err
	}

	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = savedStdout
		os.Stderr = savedStderr
	}()

	stdoutChan := make(chan string)
	stderrChan := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		stdoutChan <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		stderrChan <- buf.String()
	}()

	capturedExitCode := 0
	execErr := ExecuteContextWithOptions(context.Background(), append([]string{"cderun"}, args...), func(o *rootOptions) {
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		if stdin != nil {
			o.in = stdin
		}
		o.out = wOut
		o.err = wErr

		// Use the real runtime factory by default for integration tests
		o.runtimeFactory = func(name string, socket string) (runtime.ContainerRuntime, error) {
			switch name {
			case "docker":
				return runtime.NewDockerRuntime(socket)
			case "podman":
				return runtime.NewPodmanRuntime(socket)
			default:
				return nil, fmt.Errorf("unsupported runtime %q", name)
			}
		}

		if setup != nil {
			setup(o)
		}
	})

	_ = wOut.Close()
	_ = wErr.Close()

	stdout = <-stdoutChan
	stderr = <-stderrChan

	return stdout, stderr, capturedExitCode, execErr
}

func skipIfDockerBroken(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "failed to mount") && strings.Contains(err.Error(), "invalid argument") {
		t.Skip("Skipping test due to Docker mount limitation in this environment (likely overlay-on-overlay)")
	}
	if err != nil && strings.Contains(err.Error(), "Data limit exceeded") {
		t.Skip("Skipping test due to Docker Hub rate limit")
	}
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	restoreWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(restoreWd) })
	return tmpDir
}
