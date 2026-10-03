package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_PolyglotSymlink_SpaceSeparatedOverrides(t *testing.T) {
	t.Parallel()

	args := []string{
		"python3",
		"app.py",
		"--cderun-image", "python:3.11-slim",
		"--cderun-sysctl", "net.ipv4.ip_forward=1",
		"--cderun-env", "APP_ENV=test",
		"--cderun-dry-run",
		"--cderun-dry-run-format", "json",
		"--verbose",
	}

	mockRuntime := runtime.NewMockRuntime()
	mfs := &config.MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("python3:\n  image: python:3.9\n"),
		},
	}

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

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

	var dryRunOutput map[string]any
	err = json.Unmarshal(outBuf.Bytes(), &dryRunOutput)
	require.NoError(t, err, "dry-run output should be valid JSON: %s", outBuf.String())

	// P1 override python:3.11-slim should take precedence over .tools.yaml python:3.9
	assert.Equal(t, "python:3.11-slim", dryRunOutput["image"])

	cmdArgs, ok := dryRunOutput["command"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"app.py", "--verbose"}, cmdArgs)

	envList, ok := dryRunOutput["env"].([]any)
	require.True(t, ok)
	// By default sensitive environment variables in dry-run output are masked (Mask-all)
	assert.Contains(t, envList, "APP_ENV=[REDACTED]")

	sysctls, ok := dryRunOutput["sysctls"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", sysctls["net.ipv4.ip_forward"])
}

func TestUnit_Command_WrapperMode_ComplexHoistingAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("interleaved_value_taking_flags_and_missing_value_error", func(t *testing.T) {
		t.Parallel()

		// Test missing value error for value-taking flag
		argsMissing := []string{
			"cderun",
			"bash",
			"--cderun-image",
		}

		errMissing := ExecuteContextWithOptions(context.Background(), argsMissing, nil)
		require.Error(t, errMissing)
		assert.Contains(t, errMissing.Error(), "cderun internal override flag \"--cderun-image\" requires a value")

		// Test pre-subcommand flag error
		argsPreSubcmd := []string{
			"cderun",
			"--cderun-image", "alpine",
			"bash",
		}

		errPreSubcmd := ExecuteContextWithOptions(context.Background(), argsPreSubcmd, nil)
		require.Error(t, errPreSubcmd)
		assert.Contains(t, errPreSubcmd.Error(), "must be placed after the subcommand")
	})

	t.Run("hoisting_interleaved_security_and_resource_flags", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"cderun",
			"go",
			"test",
			"--cderun-image", "golang:1.22",
			"--cderun-security-opt", "seccomp=unconfined",
			"--cderun-gpus", "all",
			"--cderun-shm-size", "2g",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
			"./...",
		}

		mockRuntime := runtime.NewMockRuntime()
		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}

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

		var dryRunOutput map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &dryRunOutput)
		require.NoError(t, err)

		assert.Equal(t, "golang:1.22", dryRunOutput["image"])
		assert.Equal(t, "all", dryRunOutput["gpus"])
		assert.Equal(t, "2g", dryRunOutput["shm_size"])

		secOpt, ok := dryRunOutput["security_opt"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"seccomp=unconfined"}, secOpt)

		cmdArgs, ok := dryRunOutput["command"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"test", "./..."}, cmdArgs)
	})
}

func TestUnit_Command_EarlyLogger_ValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid_log_level", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_LOG_LEVEL": "invalid_level",
			},
		}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level: \"invalid_level\"")
	})

	t.Run("invalid_log_format", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_LOG_FORMAT": "xml",
			},
		}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log format: \"xml\"")
	})

	t.Run("invalid_log_timestamp_boolean", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_LOG_TIMESTAMP": "notabool",
			},
		}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid boolean value for log-timestamp: \"notabool\"")
	})
}

func TestUnit_Command_WriteFormatted_ErrorsAndFormats(t *testing.T) {
	t.Parallel()

	o := &rootOptions{}
	o.ensureHooks()

	t.Run("unsupported_format", func(t *testing.T) {
		t.Parallel()
		buf := &bytes.Buffer{}
		err := o.writeFormatted(buf, "xml", map[string]string{"foo": "bar"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format: \"xml\"")
	})

	t.Run("json_marshal_error", func(t *testing.T) {
		t.Parallel()
		customOpts := &rootOptions{
			jsonMarshalIndent: func(v any, prefix, indent string) ([]byte, error) {
				return nil, errors.New("custom json marshal error")
			},
		}
		customOpts.ensureHooks()

		buf := &bytes.Buffer{}
		err := customOpts.writeFormatted(buf, "json", map[string]string{"foo": "bar"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom json marshal error")
	})

	t.Run("yaml_marshal_error", func(t *testing.T) {
		t.Parallel()
		customOpts := &rootOptions{
			yamlMarshal: func(v any) ([]byte, error) {
				return nil, errors.New("custom yaml marshal error")
			},
		}
		customOpts.ensureHooks()

		buf := &bytes.Buffer{}
		err := customOpts.writeFormatted(buf, "yaml", map[string]string{"foo": "bar"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom yaml marshal error")
	})
}

func TestUnit_Command_DiagnosisAndPrune_Scenarios(t *testing.T) {
	t.Parallel()

	t.Run("diagnosis_output_formats", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}
		mockRuntime := runtime.NewMockRuntime()

		// Simple format
		outBufSimple := &bytes.Buffer{}
		errSimple := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--diagnosis-format", "simple"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			cmd.SetOut(outBufSimple)
		})
		require.NoError(t, errSimple)
		assert.Contains(t, outBufSimple.String(), "Runtime: docker")

		// JSON format
		outBufJSON := &bytes.Buffer{}
		errJSON := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--diagnosis-format", "json"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			cmd.SetOut(outBufJSON)
		})
		require.NoError(t, errJSON)
		var diagData map[string]any
		require.NoError(t, json.Unmarshal(outBufJSON.Bytes(), &diagData))
	})

	t.Run("prune_dry_run_and_empty", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}
		mockRuntime := runtime.NewMockRuntime()

		// Prune dry run
		outBufDry := &bytes.Buffer{}
		errDry := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--prune", "--dry-run"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			cmd.SetOut(outBufDry)
		})
		require.NoError(t, errDry)
		assert.Contains(t, outBufDry.String(), "Dry-run mode: Would prune stopped orphan containers")

		// Prune empty list
		outBufEmpty := &bytes.Buffer{}
		errEmpty := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--prune"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.fs = mfs
			cmd.SetOut(outBufEmpty)
		})
		require.NoError(t, errEmpty)
		assert.Contains(t, outBufEmpty.String(), "No orphan containers found to prune.")
	})
}

func TestUnit_Command_NestedExecution_ControlSocketFallback(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/workspace/.cderun.yaml": []byte("hostContext:\n  level: 1\n  controlSocket: /nonexistent/socket.sock\n"),
		},
	}

	mockRuntime := runtime.NewMockRuntime()

	errBuf := &bytes.Buffer{}

	// Execute with HostContext pointing to a non-existent ControlSocket to trigger connection fallback warning
	args := []string{
		"cderun",
		"echo", "hello",
		"--cderun-image", "alpine",
		"--cderun-log-level", "debug",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(e, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetErr(errBuf)
	})

	require.NoError(t, err)
	assert.Contains(t, errBuf.String(), "Failed to connect to Control Socket")
	assert.Contains(t, errBuf.String(), "falling back to raw socket")
}
