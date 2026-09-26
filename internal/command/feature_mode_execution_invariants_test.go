package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Command_WrapperMode_ComplexHoistingInvariants(t *testing.T) {
	t.Parallel()

	t.Run("hoisting_boolean_and_value_flags_intermixed_after_subcommand_flags", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"cderun",
			"python3",
			"app.py",
			"--config=prod.json",
			"--cderun-image", "python:3.11-slim",
			"--cderun-interactive",
			"--cderun-tty",
			"--cderun-read-only",
			"--cderun-user", "1001:1001",
			"--cderun-workdir", "/app",
			"--cderun-entrypoint", "/usr/local/bin/python3",
			"--verbose",
			"--",
			"extra-arg",
			"--subcommand-flag",
		}

		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		assert.Equal(t, "python:3.11-slim", cfg.Image)
		assert.True(t, cfg.Interactive)
		assert.True(t, cfg.TTY)
		assert.True(t, cfg.ReadOnly)
		assert.Equal(t, "1001:1001", cfg.User)
		assert.Equal(t, "/app", cfg.Workdir)
		assert.Equal(t, []string{"/usr/local/bin/python3"}, cfg.Entrypoint)

		// Subcommand flags & arguments after double dash preserved in exact order
		assert.Equal(t, []string{"app.py", "--config=prod.json", "--verbose", "--", "extra-arg", "--subcommand-flag"}, cfg.Command)
	})

	t.Run("dry_run_json_output_verifies_wrapper_hoisting", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"cderun",
			"node",
			"server.js",
			"--cderun-image", "node:20-alpine",
			"--cderun-init",
			"--cderun-privileged",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
		}

		mockRuntime := &runtime.MockRuntime{}
		outBuf := &bytes.Buffer{}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			cmd.SetOut(outBuf)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		var res map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &res)
		require.NoError(t, err, "dry run output should be valid JSON: %s", outBuf.String())

		assert.Equal(t, "node:20-alpine", res["image"])
		assert.Equal(t, true, res["init"])
		assert.Equal(t, true, res["privileged"])

		cmdArgs, ok := res["command"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"server.js"}, cmdArgs)
	})
}

func TestUnit_Command_SymlinkMode_PrecedenceMatrix(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte(`
go:
  image: golang:1.21-alpine
  workdir: /go/src/app
  env:
    - CGO_ENABLED=0
`),
			"/project/.cderun.yaml": []byte(`
defaults:
  network: host
  workdir: /default/dir
`),
		},
	}

	t.Run("symlink_execution_precedence_overrides", func(t *testing.T) {
		t.Parallel()

		// Invoking as "go" in Symlink Mode with CLI override `--cderun-workdir`
		args := []string{
			"go",
			"test",
			"./...",
			"--cderun-workdir", "/custom/workdir",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
		}

		outBuf := &bytes.Buffer{}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		var res map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &res)
		require.NoError(t, err)

		// CLI flag workdir overrides tool config workdir
		assert.Equal(t, "/custom/workdir", res["workdir"])
		// Image from .tools.yaml tool definition
		assert.Equal(t, "golang:1.21-alpine", res["image"])
		// Network from .cderun.yaml defaults
		assert.Equal(t, "host", res["network"])
	})
}

func TestUnit_Command_AdHocMode_NoImageRejection(t *testing.T) {
	t.Parallel()

	t.Run("direct_execution_without_image_returns_error", func(t *testing.T) {
		t.Parallel()

		args := []string{"cderun", "sh", "-c", "echo hello"}

		mfs := &config.MockFileSystem{
			WD: "/empty",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool")
	})
}
