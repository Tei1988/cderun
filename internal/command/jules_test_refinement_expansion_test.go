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

// docs/features/polyglot-entry.md: Polyglot Entry Point / Symlink Mode
// Executing the CLI in Symlink Mode with non-ASCII CJK in arguments and dirty paths should clean and resolve the tool name correctly,
// and unrecognized tools should return config.ImageNotFoundError.
func TestUnit_Command_SymlinkMode_UnicodeAndUnrecognized(t *testing.T) {
	t.Parallel()

	// 1. Unicode/CJK dirty relative path tool execution
	t.Run("dirty relative path cleans and executes with unicode arguments", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./python/../python", "-c", "print('こんにちは, 🔥!')"}
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
		assert.Equal(t, []string{"-c", "print('こんにちは, 🔥!')"}, cfg.Command)
	})

	// 2. Unrecognized tool triggers config.ImageNotFoundError
	t.Run("unrecognized tool returns ImageNotFoundError", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		args := []string{"./unknown-tool", "run"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var imgErr *config.ImageNotFoundError
		require.ErrorAs(t, err, &imgErr)
		assert.Equal(t, "unknown-tool", imgErr.Tool)
	})
}

// docs/features/argument-parsing.md: Argument Hoisting Mechanics
// Value-taking override flags support space separation format, and double-dash (--) does not stop hoisting.
func TestUnit_Command_WrapperMode_OverridesAndHoisting(t *testing.T) {
	t.Parallel()

	// 1. Double-dash -- does NOT prevent hoisting of internal overrides in Wrapper Mode
	t.Run("double-dash does not prevent hoisting of cderun flags", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "alpine",
			"sh",
			"--",
			"--cderun-image=ubuntu",
			"-l",
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

		// Overridden image is hoisted and replaces "alpine" with "ubuntu"
		assert.Equal(t, "ubuntu", cfg.Image)
		// Passthrough arguments are preserved (including -- and -l)
		assert.Equal(t, []string{"--", "-l"}, cfg.Command)
	})

	// 2. Space separated value-taking overrides
	t.Run("space-separated value-taking overrides successfully extracted", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "alpine",
			"node",
			"--cderun-image", "node:20-alpine",
			"app.js",
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

		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, []string{"app.js"}, cfg.Command)
	})

	// 3. Error boundary: value-taking override missing its value (followed by another flag)
	t.Run("value-taking override missing value triggers error", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"--image", "alpine",
			"sh",
			"--cderun-image", "--cderun-network", "host",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})
}

// docs/features/signal-handling-security.md: Early Signal Handler
// Consecutive rapid signals triggers immediate cancellation of the host context.
func TestUnit_Command_RapidSignals_HostCancellation(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	mock := &julesSignalCapturingMock{
		waitChan: waitChan,
	}
	mock.CreatedContainerID = "rapid-signals-test"

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

	// Wait for signal handler to capture channel
	assert.Eventually(t, func() bool {
		sigChanMu.Lock()
		defer sigChanMu.Unlock()
		return capturedSigChan != nil
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	// Send first SIGINT
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Verify first signal is captured
	assert.Eventually(t, func() bool {
		return slices.Contains(mock.getSignals(), "SIGINT")
	}, 2*time.Second, 10*time.Millisecond)

	// Send second rapid SIGINT
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Execution should terminate due to context cancellation
	select {
	case <-done:
		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("consecutive rapid signals failed to trigger host cancellation")
	}
}

// docs/features/hang-timeout.md: Effective Hang Timeout
// Validates negative duration boundaries and invalid duration parsing error propagation.
func TestUnit_Command_ErrorBoundary_NegativeHangTimeout(t *testing.T) {
	t.Parallel()

	t.Run("negative hang timeout throws resolution error", func(t *testing.T) {
		t.Parallel()
		args := []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout", "-10s"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration error")
		assert.Contains(t, err.Error(), "duration cannot be negative")
	})

	t.Run("invalid duration format throws parsing error", func(t *testing.T) {
		t.Parallel()
		args := []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout", "invalid_format"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hang-timeout value")
	})
}

// julesSignalCapturingMock is a mock runtime that records forwarded signals, scoped specifically to avoid duplicate declaration collisions.
type julesSignalCapturingMock struct {
	runtime.MockRuntime
	waitChan         chan int
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *julesSignalCapturingMock) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	m.mu.Unlock()
	return nil
}

func (m *julesSignalCapturingMock) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *julesSignalCapturingMock) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}
