package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_CommandExecution_AdvancedScenarios(t *testing.T) {
	t.Parallel()

	t.Run("WrapperModeHoistingWithSpaceAndEqualsAndDoubleDash", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		rawArgs := []string{
			"cderun",
			"node",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
			"--cderun-image", "node:18-alpine",
			"--cderun-remove=false",
			"--cderun-workdir=/app",
			"--",
			"-e",
			"console.log('hello')",
		}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
		})
		require.NoError(t, err)

		var dryRunOutput map[string]any
		err = json.Unmarshal(stdout.Bytes(), &dryRunOutput)
		require.NoError(t, err)

		assert.Equal(t, "node:18-alpine", dryRunOutput["image"])
		assert.Equal(t, false, dryRunOutput["remove"])
		assert.Equal(t, "/app", dryRunOutput["workdir"])

		cmdVal, ok := dryRunOutput["command"]
		require.True(t, ok)

		var strCmd []string
		if cmdSlice, ok := cmdVal.([]any); ok {
			for _, v := range cmdSlice {
				strCmd = append(strCmd, v.(string))
			}
		}
		// The forwarded target command should preserve the double-dash and target flags
		assert.Equal(t, []string{"--", "-e", "console.log('hello')"}, strCmd)
	})

	t.Run("SymlinkModeExecutionAndPassthrough", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		rawArgs := []string{
			"./python",
			"-c",
			"import sys; print(sys.version)",
			"--cderun-dry-run",
			"--cderun-dry-run-format=json",
			"--cderun-image=python:3.11-slim",
		}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
		})
		require.NoError(t, err)

		var dryRunOutput map[string]any
		err = json.Unmarshal(stdout.Bytes(), &dryRunOutput)
		require.NoError(t, err)

		assert.Equal(t, "python:3.11-slim", dryRunOutput["image"])

		cmdVal, ok := dryRunOutput["command"]
		require.True(t, ok)

		var strCmd []string
		if cmdSlice, ok := cmdVal.([]any); ok {
			for _, v := range cmdSlice {
				strCmd = append(strCmd, v.(string))
			}
		}
		assert.Equal(t, []string{"-c", "import sys; print(sys.version)"}, strCmd)
	})

	t.Run("DryRunOutputSimpleVsJsonFormatInvariants", func(t *testing.T) {
		t.Parallel()

		// Test Simple Format Output
		var stdoutSimple bytes.Buffer
		var stderrSimple bytes.Buffer

		rawArgsSimple := []string{
			"cderun",
			"sh",
			"--cderun-dry-run",
			"--cderun-dry-run-format=simple",
			"--cderun-image=alpine:latest",
			"-c",
			"uptime",
		}

		err := ExecuteContextWithOptions(context.Background(), rawArgsSimple, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdoutSimple)
			cmd.SetErr(&stderrSimple)
		})
		require.NoError(t, err)

		simpleOutput := stdoutSimple.String()
		assert.Contains(t, simpleOutput, "Image: alpine:latest")
		assert.Contains(t, simpleOutput, "Command: \"-c\" \"uptime\"")

		// Test JSON Format Output
		var stdoutJSON bytes.Buffer
		var stderrJSON bytes.Buffer

		rawArgsJSON := []string{
			"cderun",
			"sh",
			"--cderun-dry-run",
			"--cderun-dry-run-format=json",
			"--cderun-image=alpine:latest",
			"-c",
			"uptime",
		}

		err = ExecuteContextWithOptions(context.Background(), rawArgsJSON, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdoutJSON)
			cmd.SetErr(&stderrJSON)
		})
		require.NoError(t, err)

		var dryRunJSON map[string]any
		err = json.Unmarshal(stdoutJSON.Bytes(), &dryRunJSON)
		require.NoError(t, err)

		assert.Equal(t, "alpine:latest", dryRunJSON["image"])
		assert.Contains(t, dryRunJSON, "network")
		assert.Contains(t, dryRunJSON, "workdir")
	})
}
