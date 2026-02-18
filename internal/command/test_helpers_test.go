package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCderun runs the cderun command in-process for integration testing.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, args...)
}

// runCderunWithStdin runs the cderun command in-process with a custom stdin.
func runCderunWithStdin(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(stdin, args...)
}

// runCderunWithOptions runs the cderun command with custom options setup.
func runCderunWithOptions(setup func(*rootOptions), args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCoreWithOptions(nil, setup, args...)
}

// runCderunCore is the shared implementation for in-process command execution.
func runCderunCore(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCoreWithOptions(stdin, nil, args...)
}

// runCderunCoreWithOptions is the shared implementation with custom options support.
func runCderunCoreWithOptions(stdin io.Reader, setup func(*rootOptions), args ...string) (stdout, stderr string, exitCode int, err error) {
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
	internalSetup := func(o *rootOptions) {
		o.in = stdin
		o.out = wOut
		o.err = wErr
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		if setup != nil {
			setup(o)
		}
	}

	execErr := ExecuteContextWithOptions(context.Background(), append([]string{"cderun"}, args...), internalSetup)

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
