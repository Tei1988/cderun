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

// TestUnit_Command_DoubleBraceEscaping_Execution verifies double-brace escaping syntax
// is honored and preserved during container configuration resolution before execution.
// Reference: docs/features/value-resolution.md (Double-Brace Escaping Syntax)
func TestUnit_Command_DoubleBraceEscaping_Execution(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
	}

	args := []string{
		"cderun",
		"--image", "alpine",
		"--env", "RAW_VALUE={{ {{HOME}} }}",
		"sh",
	}

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

	// Observable Behavior: RAW_VALUE is preserved literally as "{{HOME}}" without being resolved to home dir
	assert.Contains(t, cfg.Env, "RAW_VALUE={{HOME}}")
}

// TestUnit_Command_StrictResolution_UnknownExpressions verifies that unrecognized
// double-brace expressions (such as typos or unknown directives) trigger immediate errors.
// Reference: docs/features/value-resolution.md (Unrecognized and Unknown Expressions)
func TestUnit_Command_StrictResolution_UnknownExpressions(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
	}

	t.Run("uppercase magic word typo raises error", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"--image", "alpine",
			"--env", "TYPO={{HOM}}",
			"sh",
		}

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
		assert.Contains(t, err.Error(), "unknown directive or magic word")
	})

	t.Run("invalid directive prefix raises error", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"--image", "alpine",
			"--env", "TYPO={{envv:KEY}}",
			"sh",
		}

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
		assert.Contains(t, err.Error(), "unknown directive or magic word")
	})
}

// TestUnit_Command_ConsecutiveSignals_SighupAndSigquit verifies that rapid consecutive SIGHUP or SIGQUIT
// signals cancel the host-side execution context.
// Reference: docs/features/signal-handling-security.md (Signal Handling Security)
func TestUnit_Command_ConsecutiveSignals_SighupAndSigquit(t *testing.T) {
	t.Parallel()

	// 1. Rapid SIGHUP signals
	t.Run("rapid double SIGHUP cancels execution", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		startedChan := make(chan struct{})
		mock := &signalRecordingMockRuntime{
			waitChan:    waitChan,
			startedChan: startedChan,
		}
		mock.CreatedContainerID = "consecutive-sighup-test"

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

		require.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		select {
		case <-startedChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Container did not start in time")
		}

		sigChanMu.Lock()
		localSigChan := capturedSigChan
		sigChanMu.Unlock()

		// First SIGHUP
		localSigChan <- syscall.SIGHUP
		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGHUP")
		}, 2*time.Second, 10*time.Millisecond)

		// Second SIGHUP (triggers context cancellation)
		localSigChan <- syscall.SIGHUP

		select {
		case <-done:
			require.Error(t, execErr)
			require.ErrorIs(t, execErr, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not cancel after rapid second SIGHUP")
		}
	})

	// 2. Rapid SIGQUIT signals
	t.Run("rapid double SIGQUIT cancels execution", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		startedChan := make(chan struct{})
		mock := &signalRecordingMockRuntime{
			waitChan:    waitChan,
			startedChan: startedChan,
		}
		mock.CreatedContainerID = "consecutive-sigquit-test"

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

		require.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		select {
		case <-startedChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Container did not start in time")
		}

		sigChanMu.Lock()
		localSigChan := capturedSigChan
		sigChanMu.Unlock()

		// First SIGQUIT
		localSigChan <- syscall.SIGQUIT
		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGQUIT")
		}, 2*time.Second, 10*time.Millisecond)

		// Second SIGQUIT (triggers context cancellation)
		localSigChan <- syscall.SIGQUIT

		select {
		case <-done:
			require.Error(t, execErr)
			require.ErrorIs(t, execErr, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not cancel after rapid second SIGQUIT")
		}
	})
}

// TestUnit_Command_SymlinkMode_UnrecognizedTool_Unicode verifies unrecognized tool handling
// under Symlink Mode when triggered using unclean relative paths containing Unicode/CJK.
// Reference: docs/features/polyglot-entry.md (Phase 1/2 Binary Invocations)
func TestUnit_Command_SymlinkMode_UnrecognizedTool_Unicode(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	// Executed with unclean path and Unicode/CJK folder characters
	args := []string{"./tools/日本語/../unrecognized-tool", "run"}
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
	// Base name must be perfectly cleaned and isolated even when invalid tool
	assert.Equal(t, "unrecognized-tool", imgErr.Tool)
}

// TestUnit_Command_HangTimeout_NegativeDuration verifies duration validations on CLI level.
// Reference: docs/features/hang-timeout.md
func TestUnit_Command_HangTimeout_NegativeDuration(t *testing.T) {
	t.Parallel()

	args := []string{"cderun", "--image", "alpine", "--hang-timeout", "-10s", "sh"}
	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration error")
	assert.Contains(t, err.Error(), "duration cannot be negative")
}
