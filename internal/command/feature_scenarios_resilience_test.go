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

func TestUnit_Command_ScenariosResilience_ExecutionAndHoisting(t *testing.T) {
	t.Parallel()

	t.Run("wrapper_mode_mixed_hoisting_with_space_and_equals_overrides", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		mockRt := &runtime.MockRuntime{
			CreatedContainerID: "resilience-container-123",
		}

		args := []string{
			"cderun",
			"gcc",
			"main.c",
			"-O2",
			"--cderun-shm-size", "512m",
			"--cderun-read-only=true",
			"--cderun-pid=host",
			"--cderun-image", "gcc:13",
			"-o", "main",
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

		assert.Equal(t, "gcc:13", cfg.Image)
		assert.Equal(t, "512m", cfg.ShmSize)
		assert.True(t, cfg.ReadOnly)
		assert.Equal(t, "host", cfg.Pid)
		assert.Equal(t, []string{"main.c", "-O2", "-o", "main"}, cfg.Command)
	})

	t.Run("symlink_mode_complex_tool_invocations", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			ExecPath: "/usr/local/bin/cargo",
		}

		mockRt := &runtime.MockRuntime{
			CreatedContainerID: "symlink-cargo-id",
		}

		args := []string{
			"cargo",
			"build",
			"--release",
			"--cderun-image=rust:1.75-slim",
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

		assert.Equal(t, "rust:1.75-slim", cfg.Image)
		assert.Equal(t, []string{"build", "--release"}, cfg.Command)
	})

	t.Run("dryrun_json_output_sensitive_env_masking_invariant", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
			Env: map[string]string{
				"SECRET_TOKEN": "super-secret-pass",
			},
		}

		outBuf := &bytes.Buffer{}

		args := []string{
			"cderun",
			"--dry-run",
			"--dry-run-format", "json",
			"--image", "alpine:latest",
			"--env", "SECRET_TOKEN={{env:SECRET_TOKEN}}",
			"--shm-size", "256m",
			"env",
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
		assert.Equal(t, "256m", dryRunData["shm_size"])

		envSlice, ok := dryRunData["env"].([]any)
		require.True(t, ok)
		require.Len(t, envSlice, 1)
		assert.Equal(t, "SECRET_TOKEN=[REDACTED]", envSlice[0])
	})
}
