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

// docs/features/argument-parsing.md: Adjacent Flag Value Restriction
// A value-taking P1 override flag cannot consume another adjacent P1 override flag as its value.
// It must immediately fail with a "requires a value" error.
func TestUnit_Command_Preprocessor_AdjacentValueRestriction(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}

	t.Run("adjacent override flag triggers error", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image",        // value-taking flag
			"--cderun-network=host", // next adjacent arg is also a cderun flag
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `override flag "--cderun-image" requires a value`)
	})

	t.Run("override flag at the end of command triggers error", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `override flag "--cderun-image" requires a value`)
	})

	t.Run("override flag followed by non-cderun value succeeds", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image", "alpine:latest",
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
		assert.Equal(t, "alpine:latest", cfg.Image)
	})
}

// docs/features/argument-parsing.md: Wrapper Mode Complex Hoisting
// Verify that the wrapper preprocessor handles multiple consecutive flags (both value-taking and boolean ones)
// and properly hoists them while preserving standard subcommand flags and their positions.
func TestUnit_Command_WrapperMode_ComplexHoistingAndPreservation(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	args := []string{
		"cderun",
		"--image", "alpine",
		"sh",
		"-c", "echo 'hello'",
		"--cderun-tty=true",
		"--cderun-network", "none",
		"--cderun-interactive", // boolean flag (NoOptDefVal is usually non-empty or empty depending on bool definition)
		"--cderun-privileged=false",
		"-v",
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

	// Assert resolved overrides
	assert.True(t, cfg.TTY)
	assert.Equal(t, "none", cfg.Network)
	assert.True(t, cfg.Interactive)
	assert.False(t, cfg.Privileged)

	// Assert subcommand and flags are preserved in original order
	expectedCmd := []string{"-c", "echo 'hello'", "-v"}
	assert.Equal(t, expectedCmd, cfg.Command)
}

// docs/features/polyglot-entry.md: Polyglot Entry Point / Symlink Mode
// Verify that Symlink Mode works with unclean paths, Unicode and CJK arguments,
// and correctly resolves and executes the subcommands.
func TestUnit_Command_SymlinkMode_UncleanPathAndCJK(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-alpine\nnode:\n  image: node:20"),
		},
	}

	// 1. Unclean Symlink Path (e.g., ./subdir/../python)
	t.Run("unclean symlink path resolved cleanly", func(t *testing.T) {
		args := []string{"./subdir/../python", "-c", "print('hello')"}
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
		assert.Equal(t, []string{"-c", "print('hello')"}, cfg.Command)
	})

	// 2. Unicode and CJK Characters
	t.Run("cjk and unicode characters inside arguments preserved", func(t *testing.T) {
		args := []string{"./node", "-e", "console.log('こんにちは 世界！ 🚀')"}
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
		assert.Equal(t, "node:20", cfg.Image)
		assert.Equal(t, []string{"-e", "console.log('こんにちは 世界！ 🚀')"}, cfg.Command)
	})
}

// docs/features/signal-handling-security.md: Early Signal Handler
// Verifies consecutive rapid signals (e.g., SIGHUP or SIGQUIT) trigger rapid cancellation and robust teardown of the host context.
func TestUnit_Command_ConsecutiveSignals_SIGHUP_Cancellation(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	receivedChan := make(chan struct{})
	startedChan := make(chan struct{})
	mock := &refinementSignalCapturingRuntime{
		waitChan:     waitChan,
		receivedChan: receivedChan,
		startedChan:  startedChan,
	}
	mock.CreatedContainerID = "sighup-consecutive-signals-test"

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

	// Send SIGHUP first
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGHUP
	sigChanMu.Unlock()

	// Wait for mock's SignalContainer to block
	select {
	case <-receivedChan:
		// Blocks inside SignalContainer
	case <-time.After(2 * time.Second):
		t.Fatal("SignalContainer not entered/blocked in time")
	}

	assert.True(t, slices.Contains(mock.getSignals(), "SIGHUP"))

	// Send SIGHUP second time rapidly to trigger cancellation
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGHUP
	sigChanMu.Unlock()

	// Teardown should complete with context canceled
	select {
	case <-done:
		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("consecutive SIGHUP signals failed to trigger host context cancellation")
	}
}

// refinementSignalCapturingRuntime is a unique mock runtime to capture signals without declaring duplicate symbols.
type refinementSignalCapturingRuntime struct {
	runtime.MockRuntime
	waitChan         chan int
	receivedChan     chan struct{}
	startedChan      chan struct{}
	receivedOnce     sync.Once
	startOnce        sync.Once
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *refinementSignalCapturingRuntime) StartContainer(ctx context.Context, id string) error {
	if m.startedChan != nil {
		m.startOnce.Do(func() {
			close(m.startedChan)
		})
	}
	return m.MockRuntime.StartContainer(ctx, id)
}

func (m *refinementSignalCapturingRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
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

func (m *refinementSignalCapturingRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *refinementSignalCapturingRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}
