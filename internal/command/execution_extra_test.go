package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_Execution_Extra(t *testing.T) {
	t.Run("propagate non-zero exit code from container", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreatedContainerID: "c1",
			ExitCode:           123,
		}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 123, exitErr.Code)
	})

	emptySubcmdTests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{
			name:       "empty subcommand returns error in dry-run (T42 regression)",
			args:       []string{"cderun", "--dry-run", ""},
			wantErrMsg: "--dry-run requires a subcommand",
		},
		{
			name: "empty subcommand does not panic in normal run (T42 regression)",
			args: []string{"cderun", ""},
		},
	}

	for _, tt := range emptySubcmdTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var runErr error
			require.NotPanics(t, func() {
				runErr = ExecuteContextWithOptions(ctx, tt.args, func(o *rootOptions, cmd *cobra.Command) {
					o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
						return &runtime.MockRuntime{}, nil
					}
					o.exitFunc = func(code int) {}
					o.isTerminal = func(fd int) bool { return false }
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
				})
			})
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, runErr, tt.wantErrMsg)
			} else {
				assert.NoError(t, runErr)
			}
		})
	}

	t.Run("propagate internal error with 125 exit code", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreateErr: errors.New("runtime error"),
		}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "runtime error")
	})
}

func TestUnit_Command_Execution_AttachError_HangTimeoutZero(t *testing.T) {
	mock := &runtime.MockRuntime{
		CreatedContainerID: "c1",
	}

	// Attach fails before container exit
	mock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		return errors.New("attach failed prematurely")
	}

	// WaitContainer has some latency to verify we don't immediately return
	mock.WaitFunc = func(ctx context.Context, containerID string) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return 42, nil
		}
	}

	ctx := context.Background()
	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--hang-timeout", "0", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}
		o.isTerminal = func(fd int) bool { return false }
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)
	// Even though attach failed (downgraded to warning), it should wait for container exit code (42),
	// and propagate 42 (non-zero).
	assert.Equal(t, 42, exitErr.Code)
	assert.NoError(t, exitErr.Err)
}

func TestUnit_Command_Execution_AttachError_HangTimeoutWithTimeout(t *testing.T) {
	mock := &runtime.MockRuntime{
		CreatedContainerID: "c1",
	}

	// Attach fails before container exit
	mock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		return errors.New("attach failed prematurely")
	}

	// WaitContainer has long latency (larger than hang timeout)
	mock.WaitFunc = func(ctx context.Context, containerID string) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(1 * time.Second):
			return 42, nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--hang-timeout", "50ms", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}
		o.isTerminal = func(fd int) bool { return false }
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	// Because 50ms hang timeout fired before 1s container exit, and attach failure is downgraded to warning,
	// the runner exits with code 125 due to a timeout.
	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 125, exitErr.Code)
	assert.Contains(t, exitErr.Error(), "timeout waiting for container to exit after attach error")
	cancel()
}

func TestUnit_Command_Execution_SIGKILLTimeout(t *testing.T) {
	mock := &runtime.MockRuntime{
		CreatedContainerID: "sigkill-timeout-c1",
	}

	// Attach finishes immediately and successfully
	mock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		return nil
	}

	mock.InspectFunc = func(ctx context.Context, id string) (bool, int, error) {
		return true, 0, nil
	}

	// WaitContainer blocks indefinitely, so it won't exit even after SIGKILL
	mock.WaitFunc = func(ctx context.Context, containerID string) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(5 * time.Second):
			return 0, nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--hang-timeout", "50ms", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}
		o.isTerminal = func(fd int) bool { return false }
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)

	// Assert that mock.SignalContainer recorded that SIGKILL was requested
	var signaled string
	mock.WithLockedMock(func(m *runtime.MockRuntime) {
		signaled = m.Signal
	})
	assert.Equal(t, "SIGKILL", signaled)

	assert.Equal(t, 125, exitErr.Code)
	assert.Contains(t, exitErr.Error(), "failed to exit after SIGKILL timeout")
}

func TestUnit_Command_Execution_SIGKILLFailure_NormalExit(t *testing.T) {
	mock := &runtime.MockRuntime{
		CreatedContainerID: "sigkill-fail-c1",
	}

	// Attach finishes immediately and successfully
	mock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		return nil
	}

	mock.InspectFunc = func(ctx context.Context, id string) (bool, int, error) {
		return true, 0, nil
	}

	// SignalContainer fails when SIGKILL is sent
	mock.SignalErr = errors.New("injected SIGKILL failure")

	// WaitContainer blocks for 70ms (longer than first 50ms, but exits before second 50ms timeout)
	mock.WaitFunc = func(ctx context.Context, containerID string) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(70 * time.Millisecond):
			return 42, nil
		}
	}

	var stderrBuf bytes.Buffer
	logger := logging.NewLogger()
	logger.Init("warn", "text", false)
	logger.SetOutput(&stderrBuf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--log-level", "warn", "--image", "alpine", "--hang-timeout", "50ms", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.logger = logger
		o.exitFunc = func(code int) {}
		o.isTerminal = func(fd int) bool { return false }
		cmd.SetOut(io.Discard)
		cmd.SetErr(&stderrBuf)
	})

	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)
	// The container's exit code should be returned successfully because the container exited on its own,
	// and the SIGKILL failure is downgraded to a warning.
	assert.Equal(t, 42, exitErr.Code)
	assert.Contains(t, stderrBuf.String(), "failed to force terminate container: injected SIGKILL failure")
}
