package command

import (
	"context"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// TestScenario_Command_WrapperModeHoistingAndDoubleDash tests Wrapper Mode argument hoisting
// with both equals-separated and space-separated flags before and after double-dash boundaries.
func TestScenario_Command_WrapperModeHoistingAndDoubleDash(t *testing.T) {
	t.Parallel()

	t.Run("Hoists space-separated and equals-separated flags after double-dash", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image", "alpine:latest",
			"node",
			"--",
			"--cderun-image", "node:20-alpine",
			"--cderun-workdir", "/app",
			"--cderun-tty=true",
			"server.js",
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

		// Assert hoisted P1 overrides:
		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, "/app", cfg.Workdir)
		assert.True(t, cfg.TTY)
		// Passthrough arguments after -- are retained correctly
		assert.Equal(t, []string{"--", "server.js"}, cfg.Command)
	})
}

// TestScenario_Command_SymlinkModePassthrough verifies Symlink Mode tool invocations
// and non-prefixed option passthrough execution.
func TestScenario_Command_SymlinkModePassthrough(t *testing.T) {
	t.Parallel()

	t.Run("Symlink tool invocation passes non-prefixed flags to container command", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./python", "-m", "http.server", "8000"}
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
		assert.Equal(t, []string{"-m", "http.server", "8000"}, cfg.Command)
	})
}
