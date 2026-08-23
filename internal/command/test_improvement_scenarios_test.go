package command

import (
	"bytes"
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
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// TestScenario_Command_DoubleDashLiteralArgs asserts that flags appearing after a double-dash "--"
// literal argument boundary are STILL hoisted as P1 internal overrides if they start with `--cderun-`.
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
	// 1. Image must be overridden to "node:20-alpine" because it was hoisted.
	assert.Equal(t, "node:20-alpine", cfg.Image)
	// 2. TTY must be overridden to true.
	assert.True(t, cfg.TTY)
	// 3. Subcommand boundary is correctly separated: subcommand is "node" and remainder is passthrough.
	// The other literal arguments after "--" are perfectly retained.
	assert.Equal(t, []string{"--", "app.js"}, cfg.Command)
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
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout=0s"}, func(o *rootOptions, cmd *cobra.Command) {
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

		// Since hang-timeout is 50ms, it should return quite quickly even if WaitContainer is still blocked.
		// Since a timeout occurs, we expect exit code 125.
		select {
		case <-done:
			require.Error(t, execErr)
			var exitErr *ExitCodeError
			require.ErrorAs(t, execErr, &exitErr)
			assert.Equal(t, 125, exitErr.Code)
			assert.Contains(t, exitErr.Error(), "timeout waiting for container to exit after attach error")
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
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--log-level", "warn", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
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
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--log-level", "warn", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
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

type signalAbortMockRuntime struct {
	runtime.MockRuntime
	PullChan      chan struct{}
	PullBlocked   chan struct{}
	CreateChan    chan struct{}
	CreateBlocked chan struct{}
	StartChan     chan struct{}
	StartBlocked  chan struct{}

	PullImageCalled       bool
	CreateContainerCalled bool
	StartContainerCalled  bool
	RemoveContainerCalled bool

	signals []string

	mu sync.Mutex
}

func (m *signalAbortMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.signals = append(m.signals, sig)
	m.mu.Unlock()
	return nil
}

func (m *signalAbortMockRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.signals))
	copy(sigs, m.signals)
	return sigs
}

func (m *signalAbortMockRuntime) PullImage(ctx context.Context, image string, policy string, maxRetries int, backoff time.Duration) error {
	m.mu.Lock()
	m.PullImageCalled = true
	m.mu.Unlock()
	if m.PullBlocked != nil {
		m.PullBlocked <- struct{}{}
	}
	if m.PullChan != nil {
		select {
		case <-m.PullChan:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (m *signalAbortMockRuntime) CreateContainer(ctx context.Context, cc *container.ContainerConfig) (string, error) {
	m.mu.Lock()
	m.CreateContainerCalled = true
	m.mu.Unlock()
	if m.CreateBlocked != nil {
		m.CreateBlocked <- struct{}{}
	}
	if m.CreateChan != nil {
		select {
		case <-m.CreateChan:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "test-abort-id", nil
}

func (m *signalAbortMockRuntime) StartContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	m.StartContainerCalled = true
	m.mu.Unlock()
	if m.StartBlocked != nil {
		m.StartBlocked <- struct{}{}
	}
	if m.StartChan != nil {
		select {
		case <-m.StartChan:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (m *signalAbortMockRuntime) RemoveContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	m.RemoveContainerCalled = true
	m.mu.Unlock()
	return nil
}

func (m *signalAbortMockRuntime) isPullImageCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.PullImageCalled
}

func (m *signalAbortMockRuntime) isCreateContainerCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CreateContainerCalled
}

func (m *signalAbortMockRuntime) isStartContainerCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.StartContainerCalled
}

func (m *signalAbortMockRuntime) isRemoveContainerCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.RemoveContainerCalled
}

func TestScenario_Command_EarlySignalHandlingAndGaps(t *testing.T) {
	t.Parallel()

	t.Run("Signal received during PullImage cancels execution without container creation", func(t *testing.T) {
		t.Parallel()
		pullChan := make(chan struct{})
		pullBlocked := make(chan struct{}, 1)
		mock := &signalAbortMockRuntime{
			PullChan:    pullChan,
			PullBlocked: pullBlocked,
		}

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

		// Wait until execution is blocked in PullImage
		select {
		case <-pullBlocked:
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not reach PullImage")
		}

		// Verify signal channel is captured
		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGINT
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGINT
		sigChanMu.Unlock()

		// Wait for execution to finish
		<-done

		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
		assert.True(t, mock.isPullImageCalled())
		assert.False(t, mock.isCreateContainerCalled())
		assert.False(t, mock.isRemoveContainerCalled())
	})

	t.Run("Signal received during CreateContainer cancels execution without starting", func(t *testing.T) {
		t.Parallel()
		createChan := make(chan struct{})
		createBlocked := make(chan struct{}, 1)
		mock := &signalAbortMockRuntime{
			CreateChan:    createChan,
			CreateBlocked: createBlocked,
		}

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

		// Wait until execution is blocked in CreateContainer
		select {
		case <-createBlocked:
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not reach CreateContainer")
		}

		// Verify signal channel is captured
		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGINT
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGINT
		sigChanMu.Unlock()

		// Wait for execution to finish
		<-done

		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
		assert.True(t, mock.isPullImageCalled())
		assert.True(t, mock.isCreateContainerCalled())
		assert.False(t, mock.isStartContainerCalled())
		// Since CreateContainer aborted/failed to complete normally, container was not created successfully
		assert.False(t, mock.isRemoveContainerCalled())
	})

	t.Run("Signal received after CreateContainer but before StartContainer halts starting and cleans up", func(t *testing.T) {
		t.Parallel()
		attachBlocked := make(chan struct{}, 1)
		mock := &signalAbortMockRuntime{}
		mock.CreatedContainerID = "test-halt-cleanup"
		mock.AttachFunc = func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			attachBlocked <- struct{}{}
			// Block until ctx (which is attachCtx/ctxG) is cancelled by the signal
			<-ctx.Done()
			return ctx.Err()
		}

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

		// Wait until execution is blocked inside AttachContainer
		select {
		case <-attachBlocked:
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not reach AttachContainer")
		}

		// Verify signal channel is captured
		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGINT
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGINT
		sigChanMu.Unlock()

		// Wait for execution to finish
		<-done

		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
		assert.True(t, mock.isPullImageCalled())
		assert.True(t, mock.isCreateContainerCalled())
		assert.False(t, mock.isStartContainerCalled())
		assert.True(t, mock.isRemoveContainerCalled())
	})

	t.Run("Signal received during StartContainer is deferred and forwarded after startup completes", func(t *testing.T) {
		t.Parallel()
		startBlocked := make(chan struct{}, 1)
		startChan := make(chan struct{})
		mock := &signalAbortMockRuntime{
			StartBlocked: startBlocked,
			StartChan:    startChan,
		}
		mock.CreatedContainerID = "test-inflight-forward"

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

		// Wait until execution is blocked inside StartContainer
		select {
		case <-startBlocked:
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not reach StartContainer")
		}

		// Verify signal channel is captured
		assert.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Send simulated SIGINT during StartContainer startup phase
		sigChanMu.Lock()
		capturedSigChan <- syscall.SIGINT
		sigChanMu.Unlock()

		// Wait deterministically for the signal to be processed and deferred/recorded by the mock
		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGINT")
		}, 2*time.Second, 10*time.Millisecond)

		// Unblock StartContainer so startup completes
		close(startChan)

		// Wait for execution to finish
		<-done

		// Since StartContainer was in-flight, the signal was deferred and forwarded rather than cancelling,
		// so execute should have run successfully and returned nil (assuming mock WaitContainer returned 0).
		require.NoError(t, execErr)
		assert.True(t, mock.isStartContainerCalled())
	})

	t.Run("Forward SIGHUP and SIGQUIT successfully to container", func(t *testing.T) {
		t.Parallel()

		// 1. SIGHUP Verification
		t.Run("SIGHUP", func(t *testing.T) {
			waitChan := make(chan int)
			mock := &signalCapturingMockRuntime{
				waitChan: waitChan,
			}
			mock.CreatedContainerID = "sighup-test"

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

			assert.Eventually(t, func() bool {
				sigChanMu.Lock()
				defer sigChanMu.Unlock()
				return capturedSigChan != nil
			}, 2*time.Second, 10*time.Millisecond)

			// Send simulated SIGHUP
			sigChanMu.Lock()
			capturedSigChan <- syscall.SIGHUP
			sigChanMu.Unlock()

			// Verify SIGHUP was forwarded
			assert.Eventually(t, func() bool {
				return slices.Contains(mock.getSignals(), "SIGHUP")
			}, 2*time.Second, 10*time.Millisecond)

			assert.Contains(t, mock.getSignals(), "SIGHUP")

			// End execution
			waitChan <- 0
			<-done
			require.NoError(t, execErr)
		})

		// 2. SIGQUIT Verification
		t.Run("SIGQUIT", func(t *testing.T) {
			waitChan := make(chan int)
			mock := &signalCapturingMockRuntime{
				waitChan: waitChan,
			}
			mock.CreatedContainerID = "sigquit-test"

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

			assert.Eventually(t, func() bool {
				sigChanMu.Lock()
				defer sigChanMu.Unlock()
				return capturedSigChan != nil
			}, 2*time.Second, 10*time.Millisecond)

			// Send simulated SIGQUIT
			sigChanMu.Lock()
			capturedSigChan <- syscall.SIGQUIT
			sigChanMu.Unlock()

			// Verify SIGQUIT was forwarded
			assert.Eventually(t, func() bool {
				return slices.Contains(mock.getSignals(), "SIGQUIT")
			}, 2*time.Second, 10*time.Millisecond)

			assert.Contains(t, mock.getSignals(), "SIGQUIT")

			// End execution
			waitChan <- 0
			<-done
			require.NoError(t, execErr)
		})
	})
}
