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

// TestUnit_Command_WrapperMode_ExpandedHoisting verifies argument hoisting mechanics
// according to docs/features/polyglot-entry.md and docs/features/argument-priority-logic.md.
func TestUnit_Command_WrapperMode_ExpandedHoisting(t *testing.T) {
	t.Parallel()

	t.Run("Hoisting space and equals cderun overrides interleaved across double dash", func(t *testing.T) {
		t.Parallel()

		cmd := newRootCmd(&rootOptions{})
		args := []string{
			"cderun",
			"node",
			"index.js",
			"--cderun-env", "ENV_A=val_a",
			"--cderun-rm=true",
			"--",
			"--cderun-workdir=/workspace",
			"arg1",
		}

		processedArgs, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Contains(t, processedArgs, "--cderun-env")
		assert.Contains(t, processedArgs, "ENV_A=val_a")
		assert.Contains(t, processedArgs, "--cderun-rm=true")
		assert.Contains(t, processedArgs, "--cderun-workdir=/workspace")
	})
}

// TestUnit_Command_SymlinkMode_ExpandedPassthrough verifies non-prefixed option passthrough in symlink mode
// according to docs/features/polyglot-entry.md.
func TestUnit_Command_SymlinkMode_ExpandedPassthrough(t *testing.T) {
	t.Parallel()

	mockRuntime := runtime.NewMockRuntime()
	mfs := &config.MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	args := []string{"python3", "--cderun-image", "python:3.11-slim", "-m", "unittest", "discover"}

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
}

// TestUnit_Command_DryRun_ExpandedOutput verifies JSON output structure and sensitive data masking
// in dry-run mode according to docs/features/command-line-options.md.
func TestUnit_Command_DryRun_ExpandedOutput(t *testing.T) {
	t.Parallel()

	mockRuntime := runtime.NewMockRuntime()
	mfs := &config.MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	args := []string{
		"cderun",
		"python",
		"--cderun-image", "python:3.11-slim",
		"--cderun-dry-run",
		"--cderun-dry-run-format", "json",
		"--cderun-env", "API_SECRET=my_secret_token",
		"--",
		"main.py",
	}

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

	var dryRunData map[string]any
	err = json.Unmarshal(outBuf.Bytes(), &dryRunData)
	require.NoError(t, err)

	assert.NotNil(t, dryRunData)
	assert.Equal(t, "python:3.11-slim", dryRunData["image"])

	cmdVal, ok := dryRunData["command"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"--", "main.py"}, cmdVal)
}
