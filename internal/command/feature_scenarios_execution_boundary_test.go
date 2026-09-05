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

func TestUnit_Command_WrapperMode_HoistingAndWorkingDirBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Hoisting space and equals flags with custom workdir containing @ and +", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"node",
			"--cderun-workdir", "/workspace/.pnpm/@scope+pkg@1.0.0/node_modules",
			"--cderun-image=node:20-alpine",
			"--cderun-env", "NODE_ENV=production",
			"--",
			"index.js",
			"--port", "8080",
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

		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, "/workspace/.pnpm/@scope+pkg@1.0.0/node_modules", cfg.Workdir)
		assert.Contains(t, cfg.Env, "NODE_ENV=production")
		assert.Equal(t, []string{"--", "index.js", "--port", "8080"}, cfg.Command)
	})
}

func TestUnit_Command_SymlinkMode_PolyglotPassthroughWithSpecialWorkdir(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("pnpm:\n  image: node:20-alpine\n  workdir: /app/@scope+store@2.0.0"),
		},
		Dirs: map[string]bool{"/project": true},
	}

	args := []string{"pnpm", "install", "--frozen-lockfile"}
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

	assert.Equal(t, "node:20-alpine", cfg.Image)
	assert.Equal(t, "/app/@scope+store@2.0.0", cfg.Workdir)
	assert.Equal(t, []string{"install", "--frozen-lockfile"}, cfg.Command)
}

func TestUnit_Command_DryRun_JSONAndTextSensitiveRedactionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("JSON dry-run format redacts sensitive env variables", func(t *testing.T) {
		var outBuf bytes.Buffer
		args := []string{
			"cderun",
			"--dry-run",
			"--dry-run-format", "json",
			"--image", "golang:1.23",
			"--sensitive-env", "API_KEY",
			"--sensitive-env", "DB_PASS",
			"--env", "API_KEY=secret_api_key_value",
			"--env", "DB_PASS=secret_db_password",
			"--env", "APP_ENV=staging",
			"go", "test", "./...",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(&outBuf)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		rawJSON := outBuf.String()
		assert.NotContains(t, rawJSON, "secret_api_key_value")
		assert.NotContains(t, rawJSON, "secret_db_password")

		var payload map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &payload)
		require.NoError(t, err, "JSON dry-run output must be valid JSON")

		assert.Equal(t, "golang:1.23", payload["image"])

		envs, ok := payload["env"].([]any)
		require.True(t, ok)

		envMap := make(map[string]string)
		for _, e := range envs {
			if str, isStr := e.(string); isStr {
				for i := 0; i < len(str); i++ {
					if str[i] == '=' {
						envMap[str[:i]] = str[i+1:]
						break
					}
				}
			}
		}

		assert.Equal(t, "staging", envMap["APP_ENV"])
		assert.Equal(t, "[REDACTED]", envMap["API_KEY"])
		assert.Equal(t, "[REDACTED]", envMap["DB_PASS"])
	})

	t.Run("Simple text dry-run format redacts secrets", func(t *testing.T) {
		var outBuf bytes.Buffer
		args := []string{
			"cderun",
			"--dry-run",
			"--dry-run-format", "simple",
			"--image", "alpine",
			"--env", "AUTH_TOKEN=super_secret_token",
			"echo", "hello",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(&outBuf)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		outputText := outBuf.String()

		assert.Contains(t, outputText, "Image: alpine")
		assert.Contains(t, outputText, "Env: \"AUTH_TOKEN\"=\"[REDACTED]\"")
		assert.NotContains(t, outputText, "super_secret_token")
	})
}
