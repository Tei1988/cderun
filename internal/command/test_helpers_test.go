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
// It captures stdout and stderr and returns the exit code.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	o := newDefaultOptions()
	return runCderunWithOptions(context.Background(), o, args...)
}

// runCderunWithOptions runs the cderun command with specific options and context.
// It automatically prepends "cderun" to the arguments.
func runCderunWithOptions(ctx context.Context, o *rootOptions, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunRawWithOptions(ctx, o, append([]string{"cderun"}, args...))
}

// runCderunRawWithOptions runs the cderun command with specific options and context.
// It uses the provided arguments as-is.
func runCderunRawWithOptions(ctx context.Context, o *rootOptions, rawArgs []string) (stdout, stderr string, exitCode int, err error) {
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

	o.stdout = wOut
	o.stderr = wErr

	cmd := newRootCmd(o)
	cmd.SetOut(wOut)
	cmd.SetErr(wErr)

	// Mock exitFunc to capture exit code if not already mocked
	capturedExitCode := 0
	originalExitFunc := o.exitFunc
	o.exitFunc = func(code int) {
		capturedExitCode = code
		if originalExitFunc != nil {
			originalExitFunc(code)
		}
	}

	processedArgs, err := preprocessArgs(cmd, rawArgs)
	if err != nil {
		_ = wOut.Close()
		_ = wErr.Close()
		return "", "", capturedExitCode, err
	}

	if len(processedArgs) >= 1 {
		cmd.SetArgs(processedArgs[1:])
	} else {
		cmd.SetArgs([]string{})
	}

	execErr := cmd.ExecuteContext(ctx)

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
