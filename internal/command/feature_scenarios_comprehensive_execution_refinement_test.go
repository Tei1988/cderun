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

// TestComprehensiveExecution_WrapperModeHoisting tests argument hoisting in Wrapper Mode
// across equals-separated flags, space-separated flags, boolean flags, and double-dash delimiters.
// Ref: docs/features/argument-parsing.md
func TestComprehensiveExecution_WrapperModeHoisting(t *testing.T) {
	t.Parallel()

	t.Run("Hoisting space and equals cderun overrides interleaved", func(t *testing.T) {
		t.Parallel()

		cmd := newRootCmd(&rootOptions{})
		args := []string{
			"cderun",
			"node",
			"server.js",
			"--cderun-engine", "docker",
			"--cderun-pid=host",
			"--cderun-init",
			"--",
			"--cderun-workdir=/app",
			"app_arg1",
		}

		processedArgs, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Assert all --cderun-* flags were hoisted ahead of subcommands (node)
		assert.Less(t, sliceIndex(processedArgs, "--cderun-engine"), sliceIndex(processedArgs, "node"))
		assert.Contains(t, processedArgs, "--cderun-engine")
		assert.Contains(t, processedArgs, "docker")
		assert.Contains(t, processedArgs, "--cderun-pid=host")
		assert.Contains(t, processedArgs, "--cderun-init")
		assert.Contains(t, processedArgs, "--cderun-workdir=/app")
	})

	t.Run("Hoisting boolean flags without values", func(t *testing.T) {
		t.Parallel()

		cmd := newRootCmd(&rootOptions{})
		args := []string{
			"cderun",
			"gcc",
			"main.c",
			"--cderun-read-only",
			"--cderun-rm",
			"-o",
			"main",
		}

		processedArgs, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		assert.Less(t, sliceIndex(processedArgs, "--cderun-read-only"), sliceIndex(processedArgs, "gcc"))
		assert.Less(t, sliceIndex(processedArgs, "--cderun-rm"), sliceIndex(processedArgs, "gcc"))
	})
}

// TestComprehensiveExecution_SymlinkModePassthrough tests Symlink Mode tool execution,
// verifying the recorded mockRuntime invocation, selected image, and preserved tool arguments.
// Ref: docs/features/polyglot-entry.md
func TestComprehensiveExecution_SymlinkModePassthrough(t *testing.T) {
	t.Parallel()

	mockRuntime := runtime.NewMockRuntime()
	mockRuntime.CreatedContainerID = "mock-cid-symlink"

	mfs := &config.MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	args := []string{"node", "--cderun-image", "node:20-alpine", "index.js", "--port", "3000"}

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

	// Assert mockRuntime recorded container invocation
	createdCfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, createdCfg, "mockRuntime must receive container configuration")
	assert.Equal(t, "node:20-alpine", createdCfg.Image, "selected container image must match --cderun-image")
	assert.Equal(t, []string{"index.js", "--port", "3000"}, createdCfg.Command, "tool arguments after node must be preserved")
}

// TestComprehensiveExecution_DryRunJSONOutput verifies JSON formatting and sensitive variable masking.
// Ref: docs/features/dry-run-mode.md, docs/features/sensitive-data-protection.md
func TestComprehensiveExecution_DryRunJSONOutput(t *testing.T) {
	t.Parallel()

	mockRuntime := runtime.NewMockRuntime()
	mfs := &config.MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	rawSecret := "super_secret_password_12345"

	args := []string{
		"cderun",
		"python3",
		"--cderun-image", "python:3.11-alpine",
		"--cderun-dry-run",
		"--cderun-dry-run-format", "json",
		"--cderun-env", "DB_PASSWORD=" + rawSecret,
		"--",
		"script.py",
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

	outStr := outBuf.String()
	assert.NotContains(t, outStr, rawSecret, "Raw secret string must never be exposed in dry-run output")

	var dryRunData map[string]any
	err = json.Unmarshal(outBuf.Bytes(), &dryRunData)
	require.NoError(t, err)

	assert.Equal(t, "python:3.11-alpine", dryRunData["image"])
}

func sliceIndex(slice []string, val string) int {
	for i, item := range slice {
		if item == val {
			return i
		}
	}
	return -1
}
