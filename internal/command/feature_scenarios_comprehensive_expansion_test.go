package command

import (
	"context"
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

// docs/features/argument-parsing.md: Flag Preprocessing and Adjacent Value Handling
// Test preprocessor edge cases where a value-taking override flag is missing its value.
func TestUnit_Command_Preprocessor_OverrideMissingValue(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "cderun"}
	// Define the flags that our parser will lookup
	cmd.Flags().String("cderun-image", "", "")
	cmd.Flags().String("cderun-workdir", "", "")

	t.Run("missing value at the end of arguments list", func(t *testing.T) {
		args := []string{"cderun", "sh", "--cderun-image"}
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})

	t.Run("missing value followed by another override flag", func(t *testing.T) {
		args := []string{"cderun", "sh", "--cderun-image", "--cderun-workdir", "/app"}
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})
}

// docs/features/polyglot-entry.md: Security Validations on Symlink Names
// Test that symlink names are strictly validated and malicious or invalid ones are rejected.
func TestUnit_Command_SymlinkMode_Security_Validations(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:18-alpine\n"),
		},
	}

	t.Run("homograph/non-ASCII character rejected", func(t *testing.T) {
		// Use Cyrillic 'о' (\u043e) in the name 'nоde'
		args := []string{"./n\u043ede", "--version"}
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

	t.Run("parent directory reference in base name rejected", func(t *testing.T) {
		// Executable name resolves to ".."
		args := []string{"/usr/bin/..", "--version"}
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
		assert.Contains(t, err.Error(), "parent directory reference not allowed for tool name")
	})

	t.Run("current directory reference in base name rejected", func(t *testing.T) {
		// Executable name resolves to "."
		args := []string{"/usr/bin/.", "--version"}
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
		assert.Contains(t, err.Error(), "current or parent directory reference not allowed for tool name")
	})

	t.Run("invalid special characters rejected", func(t *testing.T) {
		args := []string{"./node#bad", "--version"}
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

// docs/features/signal-handling-security.md: Early Signal Handler
// Verifies rapid SIGHUP signal forwarding and context cancellation is robust.
func TestUnit_Command_SIGHUP_Forwarding_And_Teardown(t *testing.T) {
	t.Parallel()

	waitChan := make(chan int)
	receivedChan := make(chan struct{})
	startedChan := make(chan struct{})
	mock := &sighupSignalCapturingRuntime{
		waitChan:     waitChan,
		receivedChan: receivedChan,
		startedChan:  startedChan,
	}
	mock.CreatedContainerID = "sighup-forwarding-test"

	var capturedSigChan chan os.Signal
	var sigChanMu sync.Mutex

	var stopSignalCount int
	var stopSignalMu sync.Mutex

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
			o.stopSignalHandling = func(sigChan chan os.Signal) {
				stopSignalMu.Lock()
				stopSignalCount++
				stopSignalMu.Unlock()
			}
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

	// Send SIGHUP directly once the barrier is satisfied
	sigChanMu.Lock()
	localSigChan := capturedSigChan
	sigChanMu.Unlock()
	localSigChan <- syscall.SIGHUP

	// Wait for the mock's SignalContainer call to be entered
	select {
	case <-receivedChan:
		// SIGHUP received by mock
	case <-time.After(2 * time.Second):
		t.Fatal("SignalContainer call was not entered/blocked in time")
	}

	// Verify SIGHUP is recorded in the mock runtime
	assert.Contains(t, mock.getSignals(), "SIGHUP")

	// Release blocked call to trigger teardown
	cancel()

	select {
	case <-done:
		require.Error(t, execErr)
		require.ErrorIs(t, execErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("teardown failed after context cancellation")
	}

	// Assert that teardown invoked the stopSignalHandling callback
	stopSignalMu.Lock()
	count := stopSignalCount
	stopSignalMu.Unlock()
	assert.Equal(t, 1, count, "stopSignalHandling should be called exactly once during teardown")
}

// sighupSignalCapturingRuntime is a mock runtime that records SIGHUP signals.
type sighupSignalCapturingRuntime struct {
	runtime.MockRuntime
	waitChan         chan int
	receivedChan     chan struct{}
	startedChan      chan struct{}
	startOnce        sync.Once
	receivedOnce     sync.Once
	forwardedSignals []string
	mu               sync.Mutex
}

func (m *sighupSignalCapturingRuntime) StartContainer(ctx context.Context, id string) error {
	if m.startedChan != nil {
		m.startOnce.Do(func() {
			close(m.startedChan)
		})
	}
	return m.MockRuntime.StartContainer(ctx, id)
}

func (m *sighupSignalCapturingRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.forwardedSignals = append(m.forwardedSignals, sig)
	m.mu.Unlock()

	m.receivedOnce.Do(func() {
		close(m.receivedChan)
	})
	return nil
}

func (m *sighupSignalCapturingRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *sighupSignalCapturingRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.forwardedSignals))
	copy(sigs, m.forwardedSignals)
	return sigs
}
