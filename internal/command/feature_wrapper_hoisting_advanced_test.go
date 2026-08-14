package command

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/argument-parsing.md: Wrapper Mode Hoisting with Double-Dash
// Verify that space-separated and equals-separated --cderun- options are properly hoisted,
// even when placed after the subcommand, intermixed with positional arguments, subcommand flags, and double-dash dividers.
func TestUnit_Command_AdvancedWrapperHoisting_DoubleDashAndIntermixed(t *testing.T) {
	t.Parallel()

	t.Run("space-separated and equals-separated overrides intermixed with double dash", func(t *testing.T) {
		t.Parallel()

		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"python3",
			"-m", "http.server",
			"--cderun-image", "ubuntu:22.04",
			"--cderun-network=host",
			"--",
			"--cderun-env", "FOO=BAR",
			"8080",
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

		// Hoisted values
		assert.Equal(t, "ubuntu:22.04", cfg.Image)
		assert.Equal(t, "host", cfg.Network)
		assert.Contains(t, cfg.Env, "FOO=BAR")

		// Subcommand and arguments preserved in order
		assert.Equal(t, []string{"-m", "http.server", "--", "8080"}, cfg.Command)
	})

	t.Run("dry run outputs hoisted flag resolutions accurately", func(t *testing.T) {
		t.Parallel()

		var outBuf bytes.Buffer
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"sh",
			"-c", "echo dry",
			"--cderun-dry-run",
			"--cderun-image", "alpine:edge",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(&outBuf)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		output := outBuf.String()
		assert.Contains(t, output, "alpine:edge")
		assert.Contains(t, output, "echo dry")
	})
}

// docs/features/polyglot-entry.md: Symlink Mode Execution Invariants
// Verify relative path symlink execution, non-prefixed option passthrough, and path resolution invariants.
func TestUnit_Command_SymlinkMode_AdvancedInvariants(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/home/user/app",
		Files: map[string][]byte{
			"/home/user/app/.tools.yaml": []byte("gcc:\n  image: gcc:latest\n  workdir: /workspace"),
		},
	}

	t.Run("relative path symlink invocation preserves subcommand arguments", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"./bin/gcc", "main.c", "-o", "main", "-O2"}

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

		assert.Equal(t, "gcc:latest", cfg.Image)
		assert.Equal(t, []string{"main.c", "-o", "main", "-O2"}, cfg.Command)
		assert.Equal(t, filepath.Clean("/workspace"), filepath.Clean(cfg.Workdir))
	})

	t.Run("non-prefixed flag placed after subcommand passes through untouched in symlink mode", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"gcc", "--version", "--image", "ignored-image"}

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

		// Symlink tool definition image takes precedence over non-prefixed passthrough flag
		assert.Equal(t, "gcc:latest", cfg.Image)
		assert.Equal(t, []string{"--version", "--image", "ignored-image"}, cfg.Command)
	})
}
