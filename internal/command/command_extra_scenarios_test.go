package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
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

// TestUnit_Command_WrapperMode_DoubleDashLiteralArgs validates that arguments
// placed after a double dash `--` are STILL hoisted as P1 internal overrides
// if they start with `--cderun-`.
func TestUnit_Command_WrapperMode_DoubleDashLiteralArgs(t *testing.T) {
	t.Parallel()

	t.Run("basic double-dash literal args after subcommand", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "alpine",
			"echo",
			"--",
			"--cderun-tty",
			"--cderun-image=node:20",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		// Image should be overridden to node:20 because it was hoisted
		assert.Equal(t, "node:20", cfg.Image)

		// Command should contain only "--" because the --cderun- flags were hoisted
		assert.Equal(t, []string{"--"}, cfg.Command)
	})
}

// TestUnit_Command_WrapperMode_HoistingWithEquals verifies hoisting logic and constraints on equals-sign flag overrides.
func TestUnit_Command_WrapperMode_HoistingWithEquals(t *testing.T) {
	t.Parallel()

	t.Run("valid hoisting with equals", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"sh",
			"--cderun-image=alpine:latest",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "alpine:latest", cfg.Image)
	})

	t.Run("hoisting value-taking flags with space separation", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image", "alpine:latest",
		}

		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "alpine:latest", cfg.Image)
	})
}

// TestUnit_Command_SymlinkMode_WithSpecialCharactersAndUnicode validates that polyglot/symlink mode
// preserves special characters and Unicode in pass-through arguments.
func TestUnit_Command_SymlinkMode_WithSpecialCharactersAndUnicode(t *testing.T) {
	t.Parallel()

	t.Run("symlink arguments with special characters and emojis", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./python", "-c", "print('こんにちは, 🔥 Emojis and 日本語!')"}
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
		assert.Equal(t, []string{"-c", "print('こんにちは, 🔥 Emojis and 日本語!')"}, cfg.Command)
	})

	t.Run("symlink file resolution with trailing slash and cleaning", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-alpine"),
			},
		}

		args := []string{"./python/", "-V"}
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
		assert.Equal(t, "python:3.11-alpine", cfg.Image)
		assert.Equal(t, []string{"-V"}, cfg.Command)
	})
}

// TestUnit_Command_Robustness_NullTimeoutBlocksIndefinitely validates that
// when the effective hang timeout is set to 0 (or <=0), the executor blocks
// indefinitely on waitDone, rather than triggering immediate timeout.
func TestUnit_Command_Robustness_NullTimeoutBlocksIndefinitely(t *testing.T) {
	t.Parallel()

	waitStarted := make(chan struct{})
	waitUnblock := make(chan struct{})
	mockRuntime := &cmdHangTimeoutMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		waitStarted: waitStarted,
		waitUnblock: waitUnblock,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout=0"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
	}()

	// Ensure WaitContainer was actually called (blocked state entered)
	select {
	case <-waitStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for WaitContainer to be called")
	}

	// Verify that execution did not immediately exit with timeout
	select {
	case err := <-errCh:
		t.Fatalf("Execute finished prematurely with: %v", err)
	case <-time.After(300 * time.Millisecond):
		// Unblocked execution did not timeout immediately
	}

	close(waitUnblock)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution to complete")
	}
}

// TestUnit_Command_Robustness_SignalKillForceTermination validates container inspection and
// SIGKILL force-termination behavior, exercising warning/error handling when SIGKILL fails or succeeds.
func TestUnit_Command_Robustness_SignalKillForceTermination(t *testing.T) {
	t.Parallel()

	o := &rootOptions{logger: &logging.Logger{}}

	t.Run("Signal fails and returns structured exit code error", func(t *testing.T) {
		var stderrBuf bytes.Buffer
		logger := logging.NewLogger()
		logger.Init("warn", "text", false)
		logger.SetOutput(&stderrBuf)
		oOpts := &rootOptions{logger: logger}

		mockRuntime := &cmdTerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal failure warning")},
			isRunning:   true,
		}

		_, err := oOpts.signalKillIfRunning(t.Context(), mockRuntime, "cont-force-kill-fail")
		require.NoError(t, err)
		assert.Contains(t, stderrBuf.String(), "failed to force terminate container: signal failure warning")
		assert.Equal(t, "cont-force-kill-fail", mockRuntime.signaledID)
		assert.Equal(t, "SIGKILL", mockRuntime.signaledSig)
	})

	t.Run("Signal succeeds without error", func(t *testing.T) {
		mockRuntime := &cmdTerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			isRunning:   true,
		}

		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "cont-force-kill-success")
		require.NoError(t, err)
		assert.Equal(t, "cont-force-kill-success", mockRuntime.signaledID)
		assert.Equal(t, "SIGKILL", mockRuntime.signaledSig)
	})
}

// TestUnit_Command_Robustness_MultipleRapidSignals simulates receiving rapid signals
// during active container execution to verify that subsequent signals cause host execution context cancellation.
func TestUnit_Command_Robustness_MultipleRapidSignals(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	mockRuntime := &cmdWaitBlockedMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		waitChan:    waitChan,
	}
	mockRuntime.CreatedContainerID = "rapid-signal-test"

	var capturedSigChan chan os.Signal
	var sigChanMu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var execErr error
	done := make(chan struct{})
	go func() {
		execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
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

	// Wait until signal channel is captured
	ok := assert.Eventually(t, func() bool {
		sigChanMu.Lock()
		defer sigChanMu.Unlock()
		return capturedSigChan != nil
	}, 2*time.Second, 10*time.Millisecond)
	require.True(t, ok, "timed out waiting for signal channel to be captured")

	var ch chan os.Signal
	sigChanMu.Lock()
	ch = capturedSigChan
	sigChanMu.Unlock()
	require.NotNil(t, ch, "captured signal channel must not be nil")

	// Send first SIGINT - should be forwarded (firstSignal is consumed)
	select {
	case ch <- syscall.SIGINT:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out sending first SIGINT")
	}

	// Verify it was forwarded to container
	assert.Eventually(t, func() bool {
		mockRuntime.mu.Lock()
		defer mockRuntime.mu.Unlock()
		return len(mockRuntime.signals) > 0
	}, 2*time.Second, 10*time.Millisecond)

	mockRuntime.mu.Lock()
	assert.Contains(t, mockRuntime.signals, "SIGINT")
	mockRuntime.mu.Unlock()

	// Send second SIGINT - should trigger host context cancellation via HandleSignal
	select {
	case ch <- syscall.SIGINT:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out sending second SIGINT")
	}

	// Wait for execution to finish
	select {
	case <-done:
		// Succeeded in cancelling context
		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), context.Canceled.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("Second rapid signal did not cancel the host context")
	}
}

// TestUnit_Command_DryRun_EmptySubcommandError validates that dry-run returns an error
// early if there is no subcommand specified.
func TestUnit_Command_DryRun_EmptySubcommandError(t *testing.T) {
	t.Parallel()

	args := []string{"cderun", "--image", "alpine", "--dry-run"}
	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
}

// Unique helper types to avoid redeclaration collisions

type cmdHangTimeoutMockRuntime struct {
	*runtime.MockRuntime
	waitStarted chan struct{}
	waitUnblock chan struct{}
}

func (m *cmdHangTimeoutMockRuntime) WaitContainer(ctx context.Context, id string) (int, error) {
	close(m.waitStarted)
	select {
	case <-m.waitUnblock:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

type cmdTerminationMockRuntime struct {
	*runtime.MockRuntime
	isRunning   bool
	signaledID  string
	signaledSig string
}

func (m *cmdTerminationMockRuntime) InspectContainer(ctx context.Context, id string) (bool, int, error) {
	return m.isRunning, 0, nil
}

func (m *cmdTerminationMockRuntime) SignalContainer(ctx context.Context, id string, sig string) error {
	m.signaledID = id
	m.signaledSig = sig
	return m.MockRuntime.SignalContainer(ctx, id, sig)
}

type cmdWaitBlockedMockRuntime struct {
	*runtime.MockRuntime
	waitChan chan int
	signals  []string
	mu       sync.Mutex
}

func (m *cmdWaitBlockedMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.signals = append(m.signals, sig)
	m.mu.Unlock()
	return nil
}

func (m *cmdWaitBlockedMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
