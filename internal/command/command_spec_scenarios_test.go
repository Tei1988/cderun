package command

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// TestUnit_Command_Hoisting_BeforeAndAfterSubcommand verifies P1 argument hoisting rules
// under standard cderun CLI invocations. Placement of "--cderun-*" overrides before the
// subcommand must be cleanly rejected with a validation error, while placement after
// the subcommand must successfully hoist and apply overrides as intended.
func TestUnit_Command_Hoisting_BeforeAndAfterSubcommand(t *testing.T) {
	t.Parallel()

	// 1. Placing --cderun-* before the subcommand must be rejected
	t.Run("cderun override placed before subcommand fails", func(t *testing.T) {
		t.Parallel()

		args := []string{
			"cderun",
			"--cderun-image=alpine",
			"sh",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cderun internal override flag")
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	// 2. Placing --cderun-* after the subcommand must be successfully hoisted and resolved
	t.Run("cderun override placed after subcommand is hoisted", func(t *testing.T) {
		t.Parallel()

		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"--image=ubuntu",
			"sh",
			"--cderun-image=alpine",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		// Hoisted override took absolute precedence
		assert.Equal(t, "alpine", cfg.Image)
	})
}

// TestUnit_Command_HostVsNestedContextPriority verifies resolution of magic words
// {{BASE_HOME}} and {{BASE_PWD}} in Level 0 (host context) vs Level 1 (nested execution context),
// ensuring proper mapping to host values during dry-run executions.
func TestUnit_Command_HostVsNestedContextPriority(t *testing.T) {
	t.Parallel()

	t.Run("Level 0 - BASE_HOME and BASE_PWD fall back to local HOME/PWD", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/local/work",
			HomeDir: "/local/home",
		}

		args := []string{
			"cderun",
			"--image=alpine",
			"--dry-run",
			"--dry-run-format=json",
			"--workdir={{BASE_PWD}}/nested",
			"sh",
		}

		var buf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.exitFunc = func(code int) {}
			cmd.SetOut(&buf)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, `"/local/work/nested"`)
	})

	t.Run("Level 1 - BASE_HOME and BASE_PWD evaluate to host values", func(t *testing.T) {
		t.Parallel()

		// Place `.cderun.yaml` containing the Level 1 HostContext into the mock file system
		// so that the hierarchical configuration loader automatically merges it.
		mfs := &config.MockFileSystem{
			WD:      "/container/work",
			HomeDir: "/container/home",
			Files: map[string][]byte{
				"/container/work/.cderun.yaml": []byte(`
hostContext:
  level: 1
  homeDir: /host/home
  workingDir: /host/work
`),
			},
			Dirs: map[string]bool{
				"/container/work": true,
			},
		}

		args := []string{
			"cderun",
			"--image=alpine",
			"--dry-run",
			"--dry-run-format=json",
			"--workdir={{BASE_PWD}}/nested",
			"sh",
		}

		var buf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.exitFunc = func(code int) {}
			cmd.SetOut(&buf)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		output := buf.String()
		// Since level is 1, BASE_PWD resolves to host working dir /host/work
		assert.Contains(t, output, `"/host/work/nested"`)
	})
}

// TestUnit_Command_Diagnostics_FormatValidation verifies diagnostics execution
// fails cleanly when format configuration fails early or uses unsupported parameters.
func TestUnit_Command_Diagnostics_FormatValidation(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/work",
		HomeDir: "/home",
	}

	args := []string{
		"cderun",
		"--diagnosis",
		"--diagnosis-format=unsupported_format",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported diagnosis format")
}
