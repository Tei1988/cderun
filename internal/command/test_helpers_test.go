package command

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCderun runs the cderun command in-process for integration testing.
// It captures stdout and stderr and returns the exit code.
// Note: This function is now safe for parallel execution.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunWithOptions(nil, args...)
}

// runCderunWithOptions runs the cderun command in-process with custom options.
func runCderunWithOptions(o *rootOptions, args ...string) (stdout, stderr string, exitCode int, err error) {
	if o == nil {
		o = defaultOptions()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	o.out = stdoutBuf
	o.err = stderrBuf

	// Mock exitFunc to capture exit code
	capturedExitCode := 0
	o.exitFunc = func(code int) {
		capturedExitCode = code
	}

	execErr := ExecuteWithOptions(context.Background(), append([]string{"cderun"}, args...), o)

	return stdoutBuf.String(), stderrBuf.String(), capturedExitCode, execErr
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
