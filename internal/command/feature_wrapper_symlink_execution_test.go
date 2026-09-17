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

func TestUnit_Command_WrapperModeHoisting_Refinement(t *testing.T) {
	t.Parallel()

	t.Run("hoisting_value_taking_cderun_flags_interleaved", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"cderun",
			"node",
			"script.js",
			"--cderun-image", "node:18-alpine",
			"--cderun-shm-size", "1g",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
			"--",
			"extra_arg1",
			"extra_arg2",
		}

		mockRuntime := runtime.NewMockRuntime()
		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})
		require.NoError(t, err)

		var dryRunOutput map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &dryRunOutput)
		require.NoError(t, err, "dry-run output should be valid JSON: %s", outBuf.String())

		assert.Equal(t, "node:18-alpine", dryRunOutput["image"])
		assert.Equal(t, "1g", dryRunOutput["shm_size"])

		cmdArgs, ok := dryRunOutput["command"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"script.js", "--", "extra_arg1", "extra_arg2"}, cmdArgs)
	})

	t.Run("symlink_mode_non_prefixed_passthrough", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"npm",
			"run",
			"build",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
			"--verbose",
			"--production",
		}

		mockRuntime := runtime.NewMockRuntime()
		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("npm:\n  image: node:18\n"),
			},
		}

		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})
		require.NoError(t, err)

		var dryRunOutput map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &dryRunOutput)
		require.NoError(t, err, "dry-run output should be valid JSON: %s", outBuf.String())

		assert.Equal(t, "node:18", dryRunOutput["image"])

		cmdArgs, ok := dryRunOutput["command"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"run", "build", "--verbose", "--production"}, cmdArgs)
	})
}
