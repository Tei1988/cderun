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

// TestScenario_Command_DoubleDashLiteralArgs asserts that flags appearing after a double-dash "--"
// literal argument boundary are treated entirely as literal arguments for the subcommand,
// rather than being parsed as CLI options.
func TestScenario_Command_DoubleDashLiteralArgs(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	args := []string{
		"cderun",
		"--image", "alpine",
		"--tty=false",
		"node",
		"--",
		"--cderun-image=node:20-alpine",
		"--cderun-tty=true",
		"app.js",
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

	// Assertions:
	// 1. Image must remain "alpine", NOT "node:20-alpine" because the override was after "--"
	assert.Equal(t, "alpine", cfg.Image)
	// 2. TTY must remain false, NOT overridden to true
	assert.False(t, cfg.TTY)
	// 3. Subcommand boundary is correctly separated: subcommand is "node" and remainder is passthrough.
	// The literal arguments after "--" are perfectly retained.
	assert.Equal(t, []string{"--", "--cderun-image=node:20-alpine", "--cderun-tty=true", "app.js"}, cfg.Command)
}

// TestScenario_Command_UnicodeSymlinkNameResolution asserts Symlink Mode behavior under
// whitelisted ASCII names versus Unicode Cyrillic names. Since ValidateToolName restricts
// names to ASCII alphanumerics, dots, hyphens, and underscores, a Unicode symlink name
// must be rejected cleanly, whereas a valid ASCII name must resolve correctly.
func TestScenario_Command_UnicodeSymlinkNameResolution(t *testing.T) {
	t.Parallel()

	// 1. Valid whitelisted symlink name "my_python"
	t.Run("valid tool symlink execution", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("my_python:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./my_python", "-c", "print('hello')"}
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
		assert.Equal(t, []string{"-c", "print('hello')"}, cfg.Command)
	})

	// 2. Non-whitelisted unicode symlink name (Cyrillic 'о' -> \u043e) must fail with safe character validation error.
	t.Run("unicode tool symlink fails validation", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("pyth\u043e_n:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./pyth\u043e_n", "-c", "print('hello')"}
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

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in tool name")
	})
}

// customHangTimeoutMockRuntime simulates AttachContainer failing immediately or completing
// with an error, while WaitContainer is either blocked or exits on cue.
type customHangTimeoutMockRuntime struct {
	runtime.MockRuntime
	WaitChan  chan int
	AttachErr error
}

func (m *customHangTimeoutMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if ready != nil {
		close(ready)
	}
	return m.AttachErr
}

func (m *customHangTimeoutMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.WaitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// TestScenario_Command_ZeroHangTimeoutBlocking tests the hang timeout blocking logic
// when hang-timeout is 0 (blocks indefinitely until container finishes) versus when positive.
func TestScenario_Command_ZeroHangTimeoutBlocking(t *testing.T) {
	t.Parallel()

	t.Run("hang-timeout set to 0 blocks indefinitely on attach error", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		mock := &customHangTimeoutMockRuntime{
			WaitChan:  waitChan,
			AttachErr: errors.New("attach failure"),
		}
		mock.CreatedContainerID = "test-zero-timeout"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var execErr error
		done := make(chan struct{})

		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout=0"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return false }
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			done <- struct{}{}
		}()

		// Give it a moment to run and block
		time.Sleep(100 * time.Millisecond)

		select {
		case <-done:
			t.Fatal("Execution finished prematurely, expected it to block indefinitely because hang-timeout=0")
		default:
			// Still blocking as expected
		}

		// Now trigger container exit
		waitChan <- 42

		select {
		case <-done:
			// Successfully unblocked
			require.Error(t, execErr)
			var exitErr *ExitCodeError
			require.ErrorAs(t, execErr, &exitErr)
			assert.Equal(t, 42, exitErr.Code)
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not unblock after container exit")
		}
	})

	t.Run("positive hang-timeout returns on timeout during attach error", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		mock := &customHangTimeoutMockRuntime{
			WaitChan:  waitChan,
			AttachErr: errors.New("attach failure"),
		}
		mock.CreatedContainerID = "test-positive-timeout"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var execErr error
		done := make(chan struct{})

		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout=50ms"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return false }
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			done <- struct{}{}
		}()

		// Since hang-timeout is 50ms, it should return quite quickly even if WaitContainer is still blocked
		select {
		case <-done:
			require.Error(t, execErr)
			var exitErr *ExitCodeError
			require.ErrorAs(t, execErr, &exitErr)
			// WaitContainer did not finish, so default error code (or 125) is returned
			assert.Contains(t, exitErr.Error(), "attach failure")
		case <-time.After(1 * time.Second):
			t.Fatal("Execution hung indefinitely despite positive hang-timeout")
		}

		// Cleanup
		close(waitChan)
	})
}

// signalCapturingMockRuntime captures forwarded signals and can block on WaitContainer.
type signalCapturingMockRuntime struct {
	runtime.MockRuntime
	waitChan         chan int
	forwardedSignals []string
	signalErr        error
	mu               sync.Mutex
}

func (m *signalCapturingMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	err := m.signalErr
	m.mu.Unlock()
	return err
}

func (m *signalCapturingMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *signalCapturingMockRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}

// TestScenario_Command_UnixSignalWarningsAndHandling overrides setupSignals to simulate receiving
// signals during execution and asserts proper interception, forwarding, and warnings.
func TestScenario_Command_UnixSignalWarningsAndHandling(t *testing.T) {
	t.Parallel()

	t.Run("forward signal successfully", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		mock := &signalCapturingMockRuntime{
			waitChan: waitChan,
		}
		mock.CreatedContainerID = "signal-test"

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

		// Wait until signal setup is captured
		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGINT
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGINT
		sigChanMu.Unlock()

		// Verify signal was forwarded to the runtime
		assert.Eventually(t, func() bool {
			return len(mock.getSignals()) > 0
		}, 2*time.Second, 10*time.Millisecond)

		assert.Contains(t, mock.getSignals(), "SIGINT")

		// End execution
		waitChan <- 0
		<-done

		// Assert ExecuteContextWithOptions executed successfully and returned nil
		assert.NoError(t, execErr)
	})

	t.Run("handles forwarding error gracefully", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		mock := &signalCapturingMockRuntime{
			waitChan:  waitChan,
			signalErr: errors.New("injected signal error"),
		}
		mock.CreatedContainerID = "signal-test-error"

		var capturedSigChan chan os.Signal
		var sigChanMu sync.Mutex

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var buf bytes.Buffer
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
				cmd.SetErr(&buf)
			})
			close(done)
		}()

		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGTERM
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGTERM
		sigChanMu.Unlock()

		// Wait to ensure processing finishes
		time.Sleep(100 * time.Millisecond)

		// End execution
		waitChan <- 0
		<-done

		// Verify error was logged as warning
		assert.Contains(t, buf.String(), "failed to forward signal")

		// Assert ExecuteContextWithOptions executed successfully and returned nil
		assert.NoError(t, execErr)
	})
}

// TestScenario_Command_MissingSubcommandDryRunFailure asserts that dry-run with no subcommand
// and no image returns an appropriate error instead of succeeding or panicking.
func TestScenario_Command_MissingSubcommandDryRunFailure(t *testing.T) {
	t.Parallel()

	args := []string{
		"cderun",
		"--dry-run",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	// Assertion:
	// - Must return an error regarding missing image or subcommand
	require.Error(t, err)
}
