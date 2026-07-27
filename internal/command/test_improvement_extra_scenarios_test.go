package command

import (
	"context"
	"errors"
	"io"
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
// placed after a double dash `--` are NOT hoisted as P1 internal overrides
// even if they start with `--cderun-`.
func TestUnit_Command_WrapperMode_DoubleDashLiteralArgs(t *testing.T) {
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

	// Image should NOT be overridden to node:20 because it was after `--`
	assert.Equal(t, "alpine", cfg.Image)

	// Command should preserve `--` and all subsequent arguments literally
	assert.Equal(t, []string{"--", "--cderun-tty", "--cderun-image=node:20"}, cfg.Command)
}

// TestUnit_Command_SymlinkMode_WithSpecialCharactersAndUnicode validates that polyglot/symlink mode
// preserves special characters and Unicode in pass-through arguments.
func TestUnit_Command_SymlinkMode_WithSpecialCharactersAndUnicode(t *testing.T) {
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
}

// TestUnit_Command_Robustness_NullTimeoutBlocksIndefinitely validates that
// when the effective hang timeout is set to 0 (or <=0), the executor blocks
// indefinitely on waitDone, rather than triggering immediate timeout.
func TestUnit_Command_Robustness_NullTimeoutBlocksIndefinitely(t *testing.T) {
	t.Parallel()

	waitStarted := make(chan struct{})
	waitUnblock := make(chan struct{})
	mockRuntime := &hangTimeoutMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		waitStarted: waitStarted,
		isRunning:   true,
	}
	mockRuntime.WaitFunc = func(ctx context.Context, id string) (int, error) {
		close(waitStarted)
		<-waitUnblock
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout", "0"}, func(o *rootOptions, cmd *cobra.Command) {
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
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal failure warning")},
			IsRunning:   true,
		}

		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "cont-force-kill-fail")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to force terminate container: signal failure warning")
		assert.Equal(t, "cont-force-kill-fail", mockRuntime.SignaledContainerID)
	})

	t.Run("Signal succeeds without error", func(t *testing.T) {
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			IsRunning:   true,
		}

		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "cont-force-kill-success")
		require.NoError(t, err)
		assert.Equal(t, "cont-force-kill-success", mockRuntime.SignaledContainerID)
	})
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

// TestUnit_Command_InvalidConfigValidation validates that invalid configurations (like invalid ports or group-adds)
// are strictly caught and rejected under ExecuteContextWithOptions command execution.
func TestUnit_Command_InvalidConfigValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid ports are rejected", func(t *testing.T) {
		args := []string{"cderun", "--image", "alpine", "--expose", "invalid-port", "sh"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for expose")
	})

	t.Run("invalid group-add is rejected", func(t *testing.T) {
		args := []string{"cderun", "--image", "alpine", "--group-add", "wheel-admin!", "sh"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for group-add")
	})
}

// TestUnit_Command_EnvVarResolution verifies that environment variables containing resolution expressions
// are fully resolved before being passed to the container creation configuration.
func TestUnit_Command_EnvVarResolution(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/my-workspace",
	}

	args := []string{
		"cderun",
		"--image", "alpine",
		"--env", "WORKSPACE_PATH={{PWD}}",
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

	// Verify that {{PWD}} expression was resolved to the WD of mfs "/my-workspace"
	assert.Contains(t, cfg.Env, "WORKSPACE_PATH=/my-workspace")
}
