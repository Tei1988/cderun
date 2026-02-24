package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

// runCderun runs the cderun command in-process for integration testing.
// It captures stdout and stderr and returns the exit code.
// Note: This function modifies global state (os.Stdout, os.Stderr)
// and is NOT safe for parallel execution with t.Parallel().
// It uses ExecuteContextWithOptions to isolate command execution.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, args...)
}

// runCderunWithStdin runs the cderun command in-process with a custom stdin.
//
//nolint:unused
func runCderunWithStdin(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(stdin, args...)
}

// runCderunCore is the shared implementation for in-process command execution.
func runCderunCore(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
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
	execErr := ExecuteContextWithOptions(context.TODO(), append([]string{"cderun"}, args...), func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		if stdin != nil {
			cmd.SetIn(stdin)
		}
		// Note: We don't set cmd.SetOut/Err here because we are capturing via os.Pipe
		// and ExecuteContextWithOptions uses newRootCmd which defaults to os.Stdin/Out/Err.
	})

	_ = wOut.Close()
	_ = wErr.Close()

	stdout = <-stdoutChan
	stderr = <-stderrChan

	return stdout, stderr, capturedExitCode, execErr
}

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
}

func withMockRuntime(mock runtime.ContainerRuntime, extras ...func(o *rootOptions, cmd *cobra.Command)) func(o *rootOptions, cmd *cobra.Command) {
	return func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		// Default exitFunc to a no-op to prevent tests from terminating the process.
		// Callers can still override this behavior by providing an extra setup function.
		o.exitFunc = func(code int) {}

		for _, extra := range extras {
			extra(o, cmd)
		}
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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	r, w, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stderr
	os.Stderr = w
	defer func() {
		os.Stderr = saved
		_ = r.Close()
	}()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	return buf.String()
}
