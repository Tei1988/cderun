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

// TestScenario_Command_WrapperMode_ComplexHoisting_And_Dividers tests Wrapper Mode argument hoisting
// with intermixed space-separated, equals-separated, boolean, value-taking overrides, and '--' dividers.
// References: docs/features/command-line-options.md
func TestScenario_Command_WrapperMode_ComplexHoisting_And_Dividers(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/home/user/workspace",
		HomeDir: "/home/user",
	}

	t.Run("space and equals overrides intermixed with double-dash divider", func(t *testing.T) {
		t.Parallel()

		mockRt := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id-123",
		}

		args := []string{
			"node",
			"--cderun-image", "node:20-alpine",
			"--cderun-workdir", "/app/src",
			"--cderun-env=NODE_ENV=production",
			"--cderun-remove",
			"--cderun-shm-size", "1g",
			"--",
			"--eval", "console.log('hello')",
			"--literal-arg-with-hyphen", // after -- divider, should be treated as literal argument
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			}
		})

		require.NoError(t, err)
		capturedConfig := mockRt.GetCreatedConfig()
		require.NotNil(t, capturedConfig)

		assert.Equal(t, "node:20-alpine", capturedConfig.Image)
		assert.Equal(t, "/app/src", capturedConfig.Workdir)
		assert.Equal(t, "1g", capturedConfig.ShmSize)
		assert.Contains(t, capturedConfig.Env, "NODE_ENV=production")
		assert.True(t, capturedConfig.Remove)

		// Verification of positional arguments passed to the container command including divider
		assert.Equal(t, []string{"--", "--eval", "console.log('hello')", "--literal-arg-with-hyphen"}, capturedConfig.Command)
	})
}

// TestScenario_Command_SymlinkMode_Passthrough tests Symlink Mode execution where cderun is invoked via symlink.
// Non-prefixed flags belong to the target binary and must be preserved as literal arguments.
// References: docs/features/command-line-options.md
func TestScenario_Command_SymlinkMode_Passthrough(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:       "/home/user/workspace",
		HomeDir:  "/home/user",
		ExecPath: "/usr/local/bin/python3", // Symlink Mode trigger
	}

	mockRt := &runtime.MockRuntime{
		CreatedContainerID: "symlink-container-id",
	}

	// In Symlink Mode, invocation args start with symlink binary name or args
	args := []string{
		"python3",
		"-c", "print('hello')",
		"--version",
		"--cderun-image=python:3.11-slim",
		"--cderun-workdir=/app",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}
	})

	require.NoError(t, err)
	capturedConfig := mockRt.GetCreatedConfig()
	require.NotNil(t, capturedConfig)

	assert.Equal(t, "python:3.11-slim", capturedConfig.Image)
	assert.Equal(t, "/app", capturedConfig.Workdir)
	// -c, print('hello'), --version must be passed directly as container command arguments
	assert.Equal(t, []string{"-c", "print('hello')", "--version"}, capturedConfig.Command)
}

// TestScenario_Command_DryRun_ComplexOptions_JSONOutput tests --dry-run and JSON format output
// verifying that complex configuration fields (sysctl, ulimits, gpus, shm-size, env masking) are serialized properly.
// References: docs/features/command-line-options.md, docs/features/sensitive-data-protection.md
func TestScenario_Command_DryRun_ComplexOptions_JSONOutput(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/home/user/workspace",
		HomeDir: "/home/user",
		Env: map[string]string{
			"SECRET_KEY": "my-super-secret-key-999",
		},
	}

	outBuf := &bytes.Buffer{}

	args := []string{
		"cderun",
		"--dry-run",
		"--dry-run-format", "json",
		"--remove=false",
		"--image", "alpine:latest",
		"--sysctl", "net.ipv4.ip_forward=1",
		"--shm-size", "512m",
		"--pids-limit", "100",
		"--restart", "on-failure:3",
		"--env", "SECRET_KEY={{env:SECRET_KEY}}",
		"--sensitive-env", "*SECRET*",
		"sh", "-c", "echo dryrun",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		cmd.SetOut(outBuf)
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
	})

	require.NoError(t, err)

	outputJSON := outBuf.Bytes()
	require.NotEmpty(t, outputJSON)

	var dryRunData map[string]any
	err = json.Unmarshal(outputJSON, &dryRunData)
	require.NoError(t, err, "dry-run JSON output must be valid JSON: %s", string(outputJSON))

	// Verify image resolution and fields in dry-run snapshot
	assert.Equal(t, "alpine:latest", dryRunData["image"])
	assert.Equal(t, "512m", dryRunData["shm_size"])

	pidsLimit, ok := dryRunData["pids_limit"].(float64)
	require.True(t, ok)
	assert.InEpsilon(t, float64(100), pidsLimit, 0.0001)

	sysctls, ok := dryRunData["sysctls"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", sysctls["net.ipv4.ip_forward"])

	assert.Equal(t, "on-failure:3", dryRunData["restart"])

	// Verify sensitive environment variable masking in dry-run JSON
	envArray, ok := dryRunData["env"].([]any)
	require.True(t, ok)

	foundMaskedSecret := false
	for _, e := range envArray {
		if s, ok := e.(string); ok && s == "SECRET_KEY=[REDACTED]" {
			foundMaskedSecret = true
			break
		}
	}
	assert.True(t, foundMaskedSecret, "SECRET_KEY must be masked as [REDACTED] in dry-run output, got: %v", envArray)
}

// TestScenario_Command_PreCanceled_Context_And_Interruption tests pre-canceled context handling prior to command execution.
// References: docs/features/command-line-options.md
func TestScenario_Command_PreCanceled_Context_And_Interruption(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/home/user/workspace",
		HomeDir: "/home/user",
	}

	t.Run("pre-canceled context execution returns context error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Pre-cancel context

		args := []string{"cderun", "--image", "alpine", "sh"}

		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
		})

		assert.ErrorIs(t, err, context.Canceled)
	})
}

// TestScenario_Command_DiagMode_Output tests diagnostic mode (--diagnosis) output generation.
// References: docs/features/command-line-options.md
func TestScenario_Command_DiagMode_Output(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/home/user/workspace",
		HomeDir: "/home/user",
	}

	outBuf := &bytes.Buffer{}

	args := []string{
		"cderun",
		"--diagnosis",
		"--image", "alpine",
		"sh",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		cmd.SetOut(outBuf)
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
	})

	require.NoError(t, err)
	diagOutput := outBuf.String()
	assert.Contains(t, diagOutput, "runtime:")
	assert.Contains(t, diagOutput, "configs:")
}
