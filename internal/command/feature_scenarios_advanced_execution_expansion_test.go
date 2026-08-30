package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cderun/internal/logging"
	"cderun/internal/runtime"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestScenario_Command_AdvancedWrapperAndSymlinkExecution verifies argument hoisting
// across -- dividers, symlink passthrough execution, and dry-run JSON sensitive masking.
// Reference: docs/features/container-command-execution.md
func TestScenario_Command_AdvancedWrapperAndSymlinkExecution(t *testing.T) {
	t.Parallel()

	t.Run("WrapperMode_ArgumentHoistingAcrossDoubleDash", func(t *testing.T) {
		tempDir := t.TempDir()
		toolsFile := filepath.Join(tempDir, ".tools.yaml")
		err := os.WriteFile(toolsFile, []byte(`
node:
  image: node:18-alpine
`), 0644)
		require.NoError(t, err)

		var stdoutBuf, stderrBuf bytes.Buffer
		args := []string{
			"cderun",
			"--tool-config", toolsFile,
			"--dry-run",
			"--dry-run-format", "json",
			"node",
			"app.js",
			"--",
			"--cderun-env", "FOO=BAR",
			"--cderun-image", "node:20-alpine",
		}

		ctx := context.Background()
		err = ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return runtime.NewMockRuntime(), nil
			}
			cmd.SetOut(&stdoutBuf)
			cmd.SetErr(&stderrBuf)
		})
		require.NoError(t, err)

		var output map[string]any
		err = json.Unmarshal(stdoutBuf.Bytes(), &output)
		require.NoError(t, err)

		imageVal, ok := output["image"].(string)
		require.True(t, ok)
		require.Equal(t, "node:20-alpine", imageVal)
	})

	t.Run("SymlinkMode_PassthroughExecution", func(t *testing.T) {
		tempDir := t.TempDir()
		toolsFile := filepath.Join(tempDir, ".tools.yaml")
		err := os.WriteFile(toolsFile, []byte(`
python:
  image: python:3.11-alpine
`), 0644)
		require.NoError(t, err)

		var stdoutBuf, stderrBuf bytes.Buffer
		args := []string{
			"python",
			"--cderun-tool-config", toolsFile,
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
			"script.py",
		}

		ctx := context.Background()
		err = ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return runtime.NewMockRuntime(), nil
			}
			cmd.SetOut(&stdoutBuf)
			cmd.SetErr(&stderrBuf)
		})
		require.NoError(t, err)

		var output map[string]any
		err = json.Unmarshal(stdoutBuf.Bytes(), &output)
		require.NoError(t, err)

		cmdVal, ok := output["command"].([]any)
		require.True(t, ok)
		require.Equal(t, "script.py", cmdVal[0])
	})

	t.Run("DryRunJSON_SensitiveEnvRedaction", func(t *testing.T) {
		var stdoutBuf, stderrBuf bytes.Buffer
		args := []string{
			"cderun",
			"--image", "alpine:latest",
			"--dry-run",
			"--dry-run-format", "json",
			"--env", "API_KEY=secret_12345",
			"--env", "DATABASE_PASSWORD=super_secret_pass",
			"alpine",
			"echo", "test",
		}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return runtime.NewMockRuntime(), nil
			}
			cmd.SetOut(&stdoutBuf)
			cmd.SetErr(&stderrBuf)
		})
		require.NoError(t, err)

		var output map[string]any
		err = json.Unmarshal(stdoutBuf.Bytes(), &output)
		require.NoError(t, err)

		envList, ok := output["env"].([]any)
		require.True(t, ok)

		foundAPIKeyRedacted := false
		foundDBPassRedacted := false

		for _, item := range envList {
			str, ok := item.(string)
			if !ok {
				continue
			}
			if str == "API_KEY=[REDACTED]" {
				foundAPIKeyRedacted = true
			}
			if str == "DATABASE_PASSWORD=[REDACTED]" {
				foundDBPassRedacted = true
			}
		}

		require.True(t, foundAPIKeyRedacted)
		require.True(t, foundDBPassRedacted)
	})
}
