package command

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/argument-parsing.md: Phase 1 (P1) Internal Overrides Hoisting
// Test complex adjacent override flag configurations:
// 1. Space-separated value-taking override flag consuming next adjacent value.
// 2. Boolean overrides (like --cderun-tty and --cderun-read-only) hoisting autonomously without consuming subsequent adjacent arguments.
// 3. Mixing standard flags, subcommand, and P1 overrides while maintaining strict positions of passthrough arguments.
func TestUnit_Command_Preprocessor_AdvancedAdjacentOverrides(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}

	t.Run("boolean overrides do not consume next arguments and hoist autonomously", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-read-only", // boolean flag
			"some_arg",           // should not be consumed as value for read-only
			"--cderun-tty=false", // inline override
			"--cderun-image", "ubuntu:22.04", // space-separated value-taking flag
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		// Assert resolved configurations
		assert.True(t, cfg.ReadOnly)
		assert.False(t, cfg.TTY)
		assert.Equal(t, "ubuntu:22.04", cfg.Image)
		// Assert "some_arg" was preserved as passthrough
		assert.Equal(t, []string{"some_arg"}, cfg.Command)
	})

	t.Run("mixing standard flags and P1 overrides maintains relative positions", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"--tty", // Standard flag (P2)
			"sh",
			"-l",             // standard passthrough
			"--cderun-image", "alpine", // P1 override
			"-c", "echo 'hi'", // standard passthrough
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		assert.Equal(t, "alpine", cfg.Image)
		// Standard P2 tty is processed by resolver
		assert.True(t, cfg.TTY)
		// Passthrough arguments are preserved in their relative order
		assert.Equal(t, []string{"-l", "-c", "echo 'hi'"}, cfg.Command)
	})
}

// docs/features/polyglot-entry.md: Polyglot Entry Point (Symlink Mode)
// Test Symlink Mode with specific argument hoisting restrictions:
// 1. Standard non-prefixed flags (e.g., --tty, --env) after the subcommand must not be hoisted.
// 2. Symlink binary executed with relative unclean directory paths (e.g. "./path/to/../node") is cleaned and resolved.
// 3. UTF-8 and CJK Unicode arguments are preserved flawlessly in Symlink Mode.
func TestUnit_Command_SymlinkMode_HoistingRestrictionsAndUnicode(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:18-alpine\npython:\n  image: python:3-slim"),
		},
	}

	t.Run("only cderun-prefixed flags are hoisted while standard flags remain passthrough", func(t *testing.T) {
		// Invoked as symlink node
		args := []string{"./node", "--env", "FOO=bar", "app.js", "--cderun-image", "node:20-alpine"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		// Hoisted override image is applied
		assert.Equal(t, "node:20-alpine", cfg.Image)
		// Standard flag --env is kept as literal passthrough
		expectedCmd := []string{"--env", "FOO=bar", "app.js"}
		assert.Equal(t, expectedCmd, cfg.Command)
	})

	t.Run("unclean symlink path resolved and CJK arguments preserved", func(t *testing.T) {
		args := []string{"./some_dir/../python", "-c", "print('こんにちは, Go!')"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		assert.Equal(t, "python:3-slim", cfg.Image)
		assert.Equal(t, []string{"-c", "print('こんにちは, Go!')"}, cfg.Command)
	})
}

// docs/features/signal-handling-security.md: Early Signal Handler
// Verifies consecutive rapid signals (e.g., SIGHUP or SIGQUIT) trigger rapid cancellation and robust teardown of the host context.
func TestUnit_Command_ConsecutiveSignals_SIGQUIT_Cancellation(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	receivedChan := make(chan struct{})
	startedChan := make(chan struct{})
	mock := &extraRefinementSignalCapturingRuntime{
		waitChan:     waitChan,
		receivedChan: receivedChan,
		startedChan:  startedChan,
	}
	mock.CreatedContainerID = "sigquit-consecutive-signals-test"

	var capturedSigChan chan os.Signal
	var sigChanMu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var execErr error
	done := make(chan struct{})
	go func() {
		execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			o.setupSignals = func(sigChan chan os.Signal) {
				sigChanMu.Lock()
				capturedSigChan = sigChan
				sigChanMu.Unlock()
			}
			o.stopSignalHandling = func(sigChan chan os.Signal) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
		close(done)
	}()

	// Wait for signal handler to capture channel.
	require.Eventually(t, func() bool {
		sigChanMu.Lock()
		defer sigChanMu.Unlock()
		return capturedSigChan != nil
	}, 2*time.Second, 10*time.Millisecond)

	// Wait for deterministic container startup barrier
	select {
	case <-startedChan:
		// State confirmed
	case <-time.After(2 * time.Second):
		t.Fatal("StartContainer was not executed in time")
	}

	// Send SIGQUIT first
	sigChanMu.Lock()
	localSigChan := capturedSigChan
	sigChanMu.Unlock()
	localSigChan <- syscall.SIGQUIT

	// Wait for mock's SignalContainer to block
	select {
	case <-receivedChan:
		// Blocks inside SignalContainer
	case <-time.After(2 * time.Second):
		t.Fatal("SignalContainer not entered/blocked in time")
	}

	assert.True(t, slices.Contains(mock.getSignals(), "SIGQUIT"))

	// Send SIGQUIT second time rapidly to trigger cancellation
	sigChanMu.Lock()
	localSigChan = capturedSigChan
	sigChanMu.Unlock()
	localSigChan <- syscall.SIGQUIT

	// Teardown should complete with context canceled
	select {
	case <-done:
		require.Error(t, execErr)
		require.True(t, errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded))
	case <-time.After(2 * time.Second):
		t.Fatal("consecutive SIGQUIT signals failed to trigger host context cancellation")
	}
}

// extraRefinementSignalCapturingRuntime is a unique mock runtime to capture signals without declaring duplicate symbols.
type extraRefinementSignalCapturingRuntime struct {
	runtime.MockRuntime
	waitChan         chan int
	receivedChan     chan struct{}
	startedChan      chan struct{}
	receivedOnce     sync.Once
	startOnce        sync.Once
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *extraRefinementSignalCapturingRuntime) StartContainer(ctx context.Context, id string) error {
	if m.startedChan != nil {
		m.startOnce.Do(func() {
			close(m.startedChan)
		})
	}
	return m.MockRuntime.StartContainer(ctx, id)
}

func (m *extraRefinementSignalCapturingRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	m.mu.Unlock()

	m.receivedOnce.Do(func() {
		close(m.receivedChan)
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (m *extraRefinementSignalCapturingRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *extraRefinementSignalCapturingRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}
