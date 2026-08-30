package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestDeepCoverage_WrapperModeHoisting(t *testing.T) {
	t.Parallel()

	t.Run("Hoist equals and space separated cderun flags across double dash", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})
		args := []string{
			"cderun",
			"node",
			"app.js",
			"--cderun-image=node:18-alpine",
			"--",
			"--cderun-tty",
			"true",
			"--app-flag",
			"value",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Hoisted flags should appear before the subcommand "node"
		assert.Equal(t, "cderun", processed[0])
		assert.Contains(t, processed[1], "--cderun-image")
		assert.Equal(t, "node", processed[len(processed)-6])
	})

	t.Run("Reject cderun flag before subcommand", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})
		args := []string{
			"cderun",
			"--cderun-image=alpine",
			"sh",
		}

		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})
}

func TestDeepCoverage_SymlinkModePassthrough(t *testing.T) {
	t.Parallel()

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	mockRt := runtime.NewMockRuntime()

	args := []string{"node", "--cderun-image=node:20-alpine", "app.js"}
	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		cmd.SetOut(stdoutBuf)
		cmd.SetErr(stderrBuf)
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}
	})

	require.NoError(t, err)
	assert.Equal(t, "node:20-alpine", mockRt.PulledImage)
	require.NotNil(t, mockRt.CreatedConfig)
	assert.Equal(t, []string{"app.js"}, mockRt.CreatedConfig.Command)
}

func TestDeepCoverage_DryRunJSONOutput(t *testing.T) {
	t.Parallel()

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	mockRt := runtime.NewMockRuntime()

	args := []string{
		"cderun",
		"--image",
		"python:3.11-slim",
		"--dry-run",
		"--dry-run-format",
		"json",
		"--env",
		"SECRET_KEY=super_secret_123",
		"python",
		"main.py",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		cmd.SetOut(stdoutBuf)
		cmd.SetErr(stderrBuf)
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}
	})

	require.NoError(t, err)

	var dryRunData map[string]any
	err = json.Unmarshal(stdoutBuf.Bytes(), &dryRunData)
	require.NoError(t, err, "stdout should contain valid JSON dry-run payload: %s", stdoutBuf.String())

	// Verify sensitive environment variable values are redacted
	if envs, ok := dryRunData["env"].([]any); ok {
		for _, e := range envs {
			if str, isStr := e.(string); isStr && bytes.HasPrefix([]byte(str), []byte("SECRET_KEY=")) {
				assert.Contains(t, str, "[REDACTED]")
			}
		}
	}
}
