package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ScenariosRefinement_ExecutionAndHoisting(t *testing.T) {
	t.Parallel()

	t.Run("wrapper_mode_flag_hoisting_with_double_dash_dividers", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		mockRt := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id-123",
		}

		args := []string{
			"cderun",
			"python",
			"script.py",
			"--cderun-remove=true",
			"--cderun-image", "python:3.11-slim",
			"--",
			"--cderun-env", "FOO=BAR",
			"extra_arg",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			}
		})

		require.NoError(t, err)
		cfg := mockRt.GetCreatedConfig()
		require.NotNil(t, cfg)

		assert.Equal(t, "python:3.11-slim", cfg.Image)
		assert.Contains(t, cfg.Env, "FOO=BAR")
		assert.True(t, cfg.Remove)
		assert.Equal(t, []string{"script.py", "--", "extra_arg"}, cfg.Command)
	})

	t.Run("symlink_mode_non_prefixed_option_passthrough", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			ExecPath: "/usr/local/bin/python",
		}

		mockRt := &runtime.MockRuntime{
			CreatedContainerID: "symlink-container-id",
		}

		args := []string{
			"python",
			"-v",
			"--cderun-image=python:3.11",
			"script.py",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			}
		})

		require.NoError(t, err)
		cfg := mockRt.GetCreatedConfig()
		require.NotNil(t, cfg)

		assert.Equal(t, "python:3.11", cfg.Image)
		assert.Equal(t, []string{"-v", "script.py"}, cfg.Command)
	})

	t.Run("dryrun_json_output_formatting_invariant", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		outBuf := &bytes.Buffer{}

		args := []string{
			"cderun",
			"--dry-run",
			"--dry-run-format", "json",
			"--image", "alpine:latest",
			"echo", "hello",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			cmd.SetOut(outBuf)
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
		})

		require.NoError(t, err)

		var dryRunData map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &dryRunData)
		require.NoError(t, err)

		assert.Equal(t, "alpine:latest", dryRunData["image"])
	})
}
