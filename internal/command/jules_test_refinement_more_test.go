package command

import (
	"context"
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

// docs/features/argument-parsing.md: Argument Hoisting Mechanics
// Value-taking override flags support space-separated format, and double-dash (--) does not stop hoisting.
// Value-taking overrides can be ordered alongside boolean overrides.
func TestUnit_Command_WrapperMode_MultiOverrideOrderingAndHoisting(t *testing.T) {
	t.Parallel()

	// 1. Mixing space-separated, equals-separated, and boolean P1 overrides
	t.Run("mix of space, equals, and boolean overrides correctly hoisted and consumed", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "alpine",
			"sh",
			"--cderun-tty",
			"--cderun-image", "ubuntu:latest",
			"--cderun-network=host",
			"--cderun-interactive=false",
			"echo", "hello",
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

		// Assertions to verify correct override extraction
		assert.Equal(t, "ubuntu:latest", cfg.Image)
		assert.Equal(t, "host", cfg.Network)
		assert.True(t, cfg.TTY)
		assert.False(t, cfg.Interactive)

		// Verify that the command arguments are correctly preserved in original order without overrides
		expectedCmd := []string{"echo", "hello"}
		assert.Equal(t, expectedCmd, cfg.Command)
	})

	// 2. Double-dash (--) behavior inside complex nested arguments
	t.Run("double-dash with complex nested flags does not prevent hoisting after double-dash", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "node:20",
			"npm",
			"run", "test",
			"--",
			"--grep", "auth",
			"--cderun-network", "none",
			"--cderun-privileged",
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

		// Assertions to verify overrides after double-dash are successfully hoisted
		assert.Equal(t, "none", cfg.Network)
		assert.True(t, cfg.Privileged)

		// Assertions to verify nested command and flags (including -- and its following non-cderun flags) are preserved
		expectedCmd := []string{"run", "test", "--", "--grep", "auth"}
		assert.Equal(t, expectedCmd, cfg.Command)
	})
}

// docs/features/polyglot-entry.md: Polyglot Entry Point / Symlink Mode
// Verify that non-ASCII Unicode and Emoji characters inside arguments are preserved flawlessly in Symlink Mode.
func TestUnit_Command_SymlinkMode_UnicodePreservation(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
		},
	}

	args := []string{"./python", "-c", "print('こんにちは, 🔥 世界! 🧪')"}
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

	assert.Equal(t, "python:3.11-slim", cfg.Image)
	assert.Equal(t, []string{"-c", "print('こんにちは, 🔥 世界! 🧪')"}, cfg.Command)
}

// docs/features/signal-handling-security.md: Early Signal Handler
// Verifies consecutive rapid SIGINT signals trigger immediate cancellation of the host context.
// Exercises cancellation specifically during a blocked SignalContainer call.
func TestUnit_Command_ConsecutiveSignals_RobustHostCancellation(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	receivedChan := make(chan struct{})
	mock := &blockingSignalCapturingRuntime{
		waitChan:     waitChan,
		receivedChan: receivedChan,
	}
	mock.CreatedContainerID = "consecutive-signals-test"

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

	time.Sleep(100 * time.Millisecond)

	// Send first SIGINT
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Wait for the mock's SignalContainer call to be entered and blocked
	select {
	case <-receivedChan:
		// First signal is captured and mock is blocked inside SignalContainer
	case <-time.After(2 * time.Second):
		t.Fatal("SignalContainer call was not entered/blocked in time")
	}

	// Verify first signal is recorded in the mock runtime
	assert.True(t, slices.Contains(mock.getSignals(), "SIGINT"))

	// Send second SIGINT rapidly while blocked to trigger immediate host cancellation
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Execution must terminate due to context cancellation
	select {
	case <-done:
		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("consecutive signals failed to trigger host context cancellation")
	}
}

// blockingSignalCapturingRuntime is a mock runtime that records forwarded signals and blocks,
// scoped specifically to avoid duplicate declaration collisions.
type blockingSignalCapturingRuntime struct {
	runtime.MockRuntime
	waitChan         chan int
	receivedChan     chan struct{}
	receivedOnce     sync.Once
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *blockingSignalCapturingRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	m.mu.Unlock()

	// Notify that the first signal was received and the call is blocking
	m.receivedOnce.Do(func() {
		close(m.receivedChan)
	})

	// Block until context is cancelled or timeout expires
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (m *blockingSignalCapturingRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *blockingSignalCapturingRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}
