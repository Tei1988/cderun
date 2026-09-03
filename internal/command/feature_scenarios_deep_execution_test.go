package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_DeepExecution_PreprocessArgs(t *testing.T) {
	t.Parallel()

	t.Run("hoist equals and space separated internal override flags in wrapper mode", func(t *testing.T) {
		opts := &rootOptions{}
		cmd := newRootCmd(opts)

		args := []string{
			"cderun",
			"run",
			"index.js",
			"--cderun-image=node:22-alpine",
			"--cderun-engine=docker",
			"--port",
			"3000",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Internal override flags are hoisted right after subcommand
		assert.Equal(t, "cderun", processed[0])
		assert.Equal(t, "--cderun-image=node:22-alpine", processed[1])
		assert.Equal(t, "--cderun-engine=docker", processed[2])
		assert.Equal(t, "run", processed[3])
		assert.Contains(t, processed, "index.js")
		assert.Contains(t, processed, "--port")
		assert.Contains(t, processed, "3000")
	})

	t.Run("error when cderun internal override flag is placed before subcommand in wrapper mode", func(t *testing.T) {
		opts := &rootOptions{}
		cmd := newRootCmd(opts)

		args := []string{
			"cderun",
			"--cderun-image=alpine:latest",
			"run",
			"index.js",
		}

		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})
}

func TestUnit_Command_DeepExecution_SymlinkPolyglotMode(t *testing.T) {
	t.Run("execute symlink polyglot mode with mock runtime", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "symlink-container-123"

		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var stdout bytes.Buffer

		execErr := ExecuteContextWithOptions(ctx, []string{"node", "--cderun-image=node:20-alpine", "index.js", "--port", "3000"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&stdout)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, execErr)

		createdCfg := mock.GetCreatedConfig()
		require.NotNil(t, createdCfg)
		assert.Equal(t, "node:20-alpine", createdCfg.Image)
		assert.Contains(t, createdCfg.Command, "index.js")
		assert.Contains(t, createdCfg.Command, "--port")
		assert.Contains(t, createdCfg.Command, "3000")
	})
}

func TestUnit_Command_DeepExecution_DryRunJSONFormatting(t *testing.T) {
	t.Parallel()

	opts := &rootOptions{}
	cmd := newRootCmd(opts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo hello"},
		Env: []string{
			"SECRET_KEY=supersecret123",
			"PUBLIC_CONFIG=normal_value",
		},
	}

	resolved := &config.ResolvedConfig{
		Image:        "alpine:latest",
		DryRun:       true,
		DryRunFormat: "json",
		SensitiveEnv: []string{"*SECRET*"},
	}

	err := opts.handleDryRun(cmd, cc, resolved)
	require.NoError(t, err)

	outStr := buf.String()
	assert.NotEmpty(t, outStr)

	var payload map[string]interface{}
	err = json.Unmarshal([]byte(outStr), &payload)
	require.NoError(t, err)

	assert.Equal(t, "alpine:latest", payload["image"])

	envRaw, ok := payload["env"].([]interface{})
	require.True(t, ok)

	envSlice := make([]string, len(envRaw))
	for i, item := range envRaw {
		envSlice[i] = item.(string)
	}

	assert.Contains(t, envSlice, "SECRET_KEY=[REDACTED]")
	assert.Contains(t, envSlice, "PUBLIC_CONFIG=normal_value")
}
