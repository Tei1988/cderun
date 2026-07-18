package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// TestUnit_Command_WrapperMode_ComplexOverrides validates the precedence and hoisting mechanism
// where standard P2 flags (before subcommand) are overridden by P1 internal overrides (after subcommand).
func TestUnit_Command_WrapperMode_ComplexOverrides(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	args := []string{
		"cderun",
		"--image", "alpine",
		"--tty=false",
		"node",
		"--cderun-tty=true",
		"--cderun-image=node:20-alpine",
		"app.js",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.isTerminal = func(fd int) bool { return true }
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)

	// Observable Behavior Assertions:
	// 1. Image must be overridden by P1 override (--cderun-image=node:20-alpine takes precedence over --image alpine)
	assert.Equal(t, "node:20-alpine", cfg.Image)
	// 2. TTY must be overridden by P1 override (--cderun-tty=true takes precedence over --tty=false)
	assert.True(t, cfg.TTY)
	// 3. Subcommand boundary is correctly separated: subcommand is "node" and remainder is passthrough.
	assert.Equal(t, []string{"app.js"}, cfg.Command)
}

// TestUnit_Command_SymlinkMode_AbsoluteAndRelativePaths validates Polyglot Mode (Symlink Mode).
// We verify executing the binary via relative/absolute paths correctly resolves tools in tools.yaml
// and retains any passthrough arguments perfectly.
func TestUnit_Command_SymlinkMode_AbsoluteAndRelativePaths(t *testing.T) {
	t.Parallel()

	// 1. Relative path `./python`
	t.Run("relative path tool execution", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python:\n  image: python:3.11-slim"),
			},
		}

		args := []string{"./python", "-m", "http.server"}
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

		// Assertions:
		// - Correct tool was resolved from the binary path "./python" => "python"
		// - Resolved image matches tools.yaml mapping
		assert.Equal(t, "python:3.11-slim", cfg.Image)
		// - Passthrough arguments "-m http.server" are retained
		assert.Equal(t, []string{"-m", "http.server"}, cfg.Command)
	})

	// 2. Absolute path `/usr/bin/python3`
	t.Run("absolute path tool execution", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte("python3:\n  image: python:3.11-alpine"),
			},
		}

		args := []string{"/usr/bin/python3", "-c", "import sys; print(sys.version)"}
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

		// Assertions:
		// - Base name resolved: "/usr/bin/python3" => "python3"
		assert.Equal(t, "python:3.11-alpine", cfg.Image)
		assert.Equal(t, []string{"-c", "import sys; print(sys.version)"}, cfg.Command)
	})
}

// TestUnit_Command_SymlinkMode_UnrecognizedTool asserts the security boundaries and correct behavior
// when an unmapped tool name is executed in polyglot mode.
func TestUnit_Command_SymlinkMode_UnrecognizedTool(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	args := []string{"./unrecognized-tool", "run"}
	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	// Assertions:
	// - Must return a proper ImageNotFoundError instead of silently succeeding or panicking.
	var imgErr *config.ImageNotFoundError
	require.ErrorAs(t, err, &imgErr)
	assert.Equal(t, "unrecognized-tool", imgErr.Tool)
}

// TestUnit_Command_StrictEnv_MissingVar verifies that if strict-env is set, and a requested env var
// is absent from the host, the execution throws a required environment variable not found error.
func TestUnit_Command_StrictEnv_MissingVar(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		Env: map[string]string{
			"EXISTING_VAR": "value",
		},
	}

	args := []string{
		"cderun",
		"--image", "alpine",
		"--strict-env",
		"--env", "EXISTING_VAR",
		"--env", "REQUIRED_BUT_MISSING",
		"sh",
	}

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

	// Assertion:
	// - Returns the correct strict validation error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required environment variable not found: \"REQUIRED_BUT_MISSING\"")
}

// TestUnit_Command_DryRun_JSONOutputValidity validates that `--dry-run` and `--dry-run-format json`
// returns a valid, parsable JSON structure of the container execution configuration.
func TestUnit_Command_DryRun_JSONOutputValidity(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	args := []string{
		"cderun",
		"--image", "node:20-alpine",
		"--dry-run",
		"--dry-run-format", "json",
		"--env", "APP_ENV=production",
		"node", "-e", "console.log('hello')",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		cmd.SetOut(&buf)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)

	// Observable Behavior Assertions:
	// 1. Let's make sure the output is non-empty
	output := buf.Bytes()
	require.NotEmpty(t, output)

	// 2. Unmarshal back into ContainerConfig to check validity and fields
	var cfg container.ContainerConfig
	err = json.Unmarshal(output, &cfg)
	require.NoError(t, err, "Output should be valid JSON matching ContainerConfig")

	assert.Equal(t, "node:20-alpine", cfg.Image)
	assert.Equal(t, []string{"-e", "console.log('hello')"}, cfg.Command)
	// By default, sensitive-env is unset, so environment variables are masked.
	assert.Contains(t, cfg.Env, "APP_ENV=[REDACTED]")
}

// TestUnit_Command_TerminationAndExitCodes tests propagation of exit codes via simulated execution,
// verifying standard termination statuses (e.g., container execution exit codes, and OCI creation failures exit code 125).
func TestUnit_Command_TerminationAndExitCodes(t *testing.T) {
	t.Parallel()

	// Scenario A: Container exits with non-zero exit status (e.g. 127)
	t.Run("non-zero exit code propagation", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-cont-127",
			ExitCode:           127,
		}

		args := []string{"cderun", "--image", "alpine", "sh"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 127, exitErr.Code)
	})

	// Scenario B: Runtime creation fails completely, throwing standard 125 internal error code
	t.Run("container creation failure propagates 125 error code", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{
			CreateErr: errors.New("low-level OCI spec creation failure"),
		}

		args := []string{"cderun", "--image", "alpine", "sh"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "low-level OCI spec creation failure")
	})
}

// TestUnit_Command_WrapperMode_SubcommandSameAsFlag verifies that when a subcommand name
// is identical to a standard flag name (e.g. executing a tool named 'image'),
// cderun correctly distinguishes the subcommand and passes any remainder as passthrough arguments.
func TestUnit_Command_WrapperMode_SubcommandSameAsFlag(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("image:\n  image: alpine:3.18"),
		},
	}

	args := []string{
		"cderun",
		"--image", "alpine:3.19",
		"image", // Subcommand has the same name as the '--image' flag
		"list",  // Passthrough arg
	}

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

	// Observable Behavior Assertions:
	// - Image resolved must be 'alpine:3.19' (as specified by the '--image' flag, keeping registry parity)
	assert.Equal(t, "alpine:3.19", cfg.Image)
	// - Subcommand 'image' is consumed, and 'list' is passed through
	assert.Equal(t, []string{"list"}, cfg.Command)
}

// TestUnit_Command_SymlinkMode_MultipleOverrides validates that executing a symlinked tool
// with multiple '--cderun-*' overrides successfully hoists and processes all of them.
func TestUnit_Command_SymlinkMode_MultipleOverrides(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:18"),
		},
	}

	args := []string{
		"node",
		"app.js",
		"--cderun-image=node:20",
		"--cderun-tty=true",
		"--cderun-env=VAR=val",
	}

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

	// Observable Behavior Assertions:
	// - Image is overridden by '--cderun-image' (while maintaining registry parity)
	assert.Equal(t, "node:20", cfg.Image)
	// - TTY is enabled via '--cderun-tty'
	assert.True(t, cfg.TTY)
	// - Passthrough arg 'app.js' is kept
	assert.Equal(t, []string{"app.js"}, cfg.Command)
	// - Environment variable VAR is specified and preserved (or masked as per config)
	assert.Contains(t, cfg.Env, "VAR=val")
}
