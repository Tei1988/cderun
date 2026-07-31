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

// cmdSignalCapturingMock is a mock runtime that records forwarded signals.
type cmdSignalCapturingMock struct {
	runtime.MockRuntime
	waitChan         chan int
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *cmdSignalCapturingMock) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	m.mu.Unlock()
	return nil
}

func (m *cmdSignalCapturingMock) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *cmdSignalCapturingMock) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}

// TestUnit_Command_Robustness_ConsecutiveSignals validates that receiving
// consecutive rapid signals (e.g. double SIGINT) triggers immediate cancellation on the host.
func TestUnit_Command_Robustness_ConsecutiveSignals(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	mock := &cmdSignalCapturingMock{
		waitChan: waitChan,
	}
	mock.CreatedContainerID = "signal-consecutive-test"

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

	// Inject startup begun or wait briefly for Execute to register state.
	// Since we are running execution, once we are block-waiting in WaitContainer (which runs after container is started),
	// the first signal will be forwarded, and the second signal will trigger cancellation on the host.
	time.Sleep(100 * time.Millisecond)

	// Send first SIGINT
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Verify first signal was received and forwarded, and has NOT cancelled yet
	assert.Eventually(t, func() bool {
		return slices.Contains(mock.getSignals(), "SIGINT")
	}, 2*time.Second, 10*time.Millisecond)

	// Send second SIGINT
	sigChanMu.Lock()
	capturedSigChan <- syscall.SIGINT
	sigChanMu.Unlock()

	// Now wait for ExecuteContextWithOptions to return context.Canceled
	select {
	case <-done:
		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("second signal failed to trigger context cancellation")
	}
}

// TestUnit_Command_WrapperMode_ValueTakingOverridesWithoutEquals validates that
// value-taking overrides specified without the equals sign (e.g. `--cderun-image node`) are strictly rejected.
func TestUnit_Command_WrapperMode_ValueTakingOverridesWithoutEquals(t *testing.T) {
	t.Parallel()

	args := []string{
		"cderun",
		"--image", "alpine",
		"sh",
		"--cderun-image", "ubuntu", // without equals sign format
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must use '=' format to specify its value")
}

// TestUnit_Command_SymlinkMode_CleanedPathsAndUnicode validates polyglot/symlink mode
// with cleaned paths and unicode arguments, ensuring correct command-line parsing.
func TestUnit_Command_SymlinkMode_CleanedPathsAndUnicode(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
		},
	}

	// Executing python through relative dirty path with Unicode arguments
	args := []string{"./python/../python", "-c", "print('こんにちは, 🔥 Emojis and 日本語!')"}
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
}
