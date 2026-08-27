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

func TestUnit_Command_WrapperMode_ArgumentHoistingExpansion(t *testing.T) {
	t.Parallel()

	t.Run("Interleaved space and equals overrides across double-dash", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"python",
			"--cderun-env", "HOST_ENV=1",
			"--cderun-workdir=/app",
			"--",
			"app.py",
			"--cderun-env=CONTAINER_ENV=2",
			"--cderun-image", "python:3.11-slim",
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

		assert.Equal(t, "python:3.11-slim", cfg.Image)
		assert.Equal(t, "/app", cfg.Workdir)
		assert.Contains(t, cfg.Env, "HOST_ENV=1")
		assert.Contains(t, cfg.Env, "CONTAINER_ENV=2")
		assert.Equal(t, []string{"--", "app.py"}, cfg.Command)
	})
}

func TestUnit_Command_SymlinkMode_PassthroughExecution(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("python:\n  image: python:3.10-alpine\n  workdir: /workspace"),
		},
		Dirs: map[string]bool{"/project": true},
	}

	args := []string{"python", "script.py", "--arg1", "val1"}
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

	assert.Equal(t, "python:3.10-alpine", cfg.Image)
	assert.Equal(t, "/workspace", cfg.Workdir)
	assert.Equal(t, []string{"script.py", "--arg1", "val1"}, cfg.Command)
}

func TestUnit_Command_DryRun_JSONPayloadSensitiveMasking(t *testing.T) {
	t.Parallel()

	var outBuf bytes.Buffer
	args := []string{
		"cderun",
		"--dry-run",
		"--dry-run-format", "json",
		"--image", "alpine",
		"--sensitive-env", "*KEY*",
		"--sensitive-env", "*PASS*",
		"--env", "PUBLIC_VAR=hello",
		"--env", "SECRET_KEY=supersecret123",
		"--env", "MY_PASSWORD=adminpass",
		"echo", "test",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {}
		cmd.SetOut(&outBuf)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(outBuf.Bytes(), &payload)
	require.NoError(t, err, "Dry-run output must be valid JSON")

	assert.Equal(t, "alpine", payload["image"])

	envs, ok := payload["env"].([]any)
	require.True(t, ok)

	envMap := make(map[string]string)
	for _, e := range envs {
		str, isStr := e.(string)
		if isStr {
			for i := 0; i < len(str); i++ {
				if str[i] == '=' {
					envMap[str[:i]] = str[i+1:]
					break
				}
			}
		}
	}

	assert.Equal(t, "hello", envMap["PUBLIC_VAR"])
	assert.Equal(t, "[REDACTED]", envMap["SECRET_KEY"])
	assert.Equal(t, "[REDACTED]", envMap["MY_PASSWORD"])
}
