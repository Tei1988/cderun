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

func TestUnit_CommandExecution_ComprehensiveEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("WrapperModeHoistingWithDoubleDashAndOverrides", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		rawArgs := []string{
			"sh",
			"-c",
			"echo hello",
			"--cderun-dry-run",
			"--cderun-dry-run-format=json",
			"--cderun-image=busybox:latest",
			"--cderun-remove=false",
		}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
		})
		require.NoError(t, err)

		// Parse dry-run JSON output
		var dryRunOutput map[string]interface{}
		err = json.Unmarshal(stdout.Bytes(), &dryRunOutput)
		require.NoError(t, err)

		assert.Equal(t, "busybox:latest", dryRunOutput["image"])
		assert.Equal(t, false, dryRunOutput["remove"])
	})

	t.Run("SymlinkModeNonPrefixedOptionPassthrough", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		rawArgs := []string{
			"python",
			"-m",
			"http.server",
			"8000",
			"--cderun-dry-run",
			"--cderun-dry-run-format=json",
			"--cderun-image=python:3.11-slim",
		}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
		})
		require.NoError(t, err)

		var dryRunOutput map[string]interface{}
		err = json.Unmarshal(stdout.Bytes(), &dryRunOutput)
		require.NoError(t, err)

		// The forwarded command line should preserve non-prefixed flags
		cmdVal, ok := dryRunOutput["command"]
		require.True(t, ok)

		var strCmd []string
		if cmdSlice, ok := cmdVal.([]interface{}); ok {
			for _, v := range cmdSlice {
				strCmd = append(strCmd, v.(string))
			}
		}
		assert.Equal(t, []string{"-m", "http.server", "8000"}, strCmd)
	})
}
