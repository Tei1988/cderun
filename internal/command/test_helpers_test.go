package command

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

type testFileSystem struct {
	config.RealFileSystem
	wd  string
	env map[string]string
}

func (f *testFileSystem) Getwd() (string, error) {
	return f.wd, nil
}

func (f *testFileSystem) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(f.wd, path), nil
}

func (f *testFileSystem) Getenv(key string) string {
	if f.env != nil {
		if v, ok := f.env[key]; ok {
			return v
		}
	}
	return f.RealFileSystem.Getenv(key)
}

func (f *testFileSystem) LookupEnv(key string) (string, bool) {
	if f.env != nil {
		if v, ok := f.env[key]; ok {
			return v, true
		}
	}
	return f.RealFileSystem.LookupEnv(key)
}

func (f *testFileSystem) Setenv(key, value string) {
	if f.env == nil {
		f.env = make(map[string]string)
	}
	f.env[key] = value
}

// runCderun runs the cderun command in-process for integration testing.
// It captures stdout and stderr and returns the exit code.
// Note: This function uses bytes.Buffers and cmd.SetOut/SetErr for isolation.
// It uses ExecuteContextWithOptions to isolate command execution.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, nil, args...)
}

// runCderunWithSetup runs the cderun command with custom setup.
func runCderunWithSetup(setup func(o *rootOptions, cmd *cobra.Command), args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, setup, args...)
}

// runCderunWithStdin runs the cderun command in-process with a custom stdin.
func runCderunWithStdin(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(stdin, nil, args...)
}

// runCderunCore is the shared implementation for in-process command execution.
// It uses bytes.Buffer and cmd.SetOut/SetErr for isolation.
func runCderunCore(stdin io.Reader, setup func(o *rootOptions, cmd *cobra.Command), args ...string) (stdout, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	// Use a timeout context to prevent indefinite hangs in CI/E2E environments.
	// 30 seconds is a reasonable default for in-process execution tests.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	capturedExitCode := 0
	execErr := ExecuteContextWithOptions(ctx, append([]string{"cderun"}, args...), func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		if stdin != nil {
			cmd.SetIn(stdin)
		}
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
		if setup != nil {
			setup(o, cmd)
		}
	})

	return outBuf.String(), errBuf.String(), capturedExitCode, execErr
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
	// Detect Docker SIGKILL timeout (likely environment resource constraint or slow CI)
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "SIGKILL") {
		t.Skip("Skipping test due to Docker SIGKILL timeout (likely environment resource constraint)")
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

func setupTestDir(t *testing.T) (string, *testFileSystem, func(o *rootOptions, cmd *cobra.Command)) {
	t.Helper()
	tmpDir := t.TempDir()
	fs := &testFileSystem{wd: tmpDir, env: make(map[string]string)}
	return tmpDir, fs, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = fs
		o.configLoader = config.NewConfigLoaderWithFS(fs)
	}
}
