package command

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/argument-parsing.md: Phase 1 (P1) Internal Overrides Hoisting
// Verifies that complex hoisting configurations involving multiple value-taking and boolean options
// are properly rearranged before Cobra parses, preserving the absolute order of passthrough arguments.
func TestUnit_Command_Preprocessor_AdvancedHoistingComplexMatrix(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	args := []string{
		"cderun",
		"python",
		"script.py",
		"--cderun-image", "python:3.12-alpine",
		"--cderun-tty=true",
		"arg1",
		"--cderun-read-only",
		"arg2",
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

	// Ensure overrides are applied correctly
	assert.Equal(t, "python:3.12-alpine", cfg.Image)
	assert.True(t, cfg.TTY)
	assert.True(t, cfg.ReadOnly)

	// Ensure passthrough command/args preserve their absolute relative positions
	assert.Equal(t, []string{"script.py", "arg1", "arg2"}, cfg.Command)
}

// docs/features/argument-parsing.md: Symlink Mode & Polyglot Entry Point
// Verifies that standard options (P2) placed after a symlink-derived subcommand are strictly preserved
// as literal passthrough arguments and not hoisted, while --cderun- prefixed overrides are hoisted.
func TestUnit_Command_SymlinkMode_StrictPreservationOfStandardFlags(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	// Executed via symlink node
	args := []string{
		"./node",
		"--tty", // Non-prefixed (standard flag) should stay as passthrough in symlink mode
		"index.js",
		"--cderun-read-only", // Prefixed (P1 override) should be hoisted
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

	// Overrides applied
	assert.True(t, cfg.ReadOnly)

	// Standard flag --tty remains as literal passthrough
	assert.Equal(t, []string{"--tty", "index.js"}, cfg.Command)
}

// docs/features/stdin-synchronization.md: Stdin synchronization and interactive flow
// Test interactive with empty input reader and non-interactive input separation.
func TestUnit_Command_Stdin_EmptyAndSeparationEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("interactive mode with empty reader exits cleanly", func(t *testing.T) {
		t.Parallel()
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-empty-stdin"
		mock.ExitCode = 0

		var stdout bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(strings.NewReader("")) // Empty stdin
			cmd.SetOut(&stdout)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		assert.Empty(t, stdout.String())
	})

	t.Run("non-interactive ignores stdin entirely", func(t *testing.T) {
		t.Parallel()
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-non-interactive-stdin"
		mock.ExitCode = 0

		var stdout bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(strings.NewReader("some stdin data")) // Stdin supplied but interactive is false
			cmd.SetOut(&stdout)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		assert.Empty(t, stdout.String(), "stdout must be empty because stdin copying is disabled in non-interactive mode")
	})
}

// docs/features/signal-handling-security.md: Signal Name Validation
// Verifies signal validation rules directly via the runtime's SignalContainer implementation or mock boundaries.
func TestUnit_Command_Signal_NameValidationRules(t *testing.T) {
	t.Parallel()

	// While runtime-level signalRegex performs character whitelisting,
	// let's ensure our command layer cleanly handles SignalContainer validation rejections.
	t.Run("SignalContainer reject malicious signal structures", func(t *testing.T) {
		t.Parallel()
		rtInstance := &runtime.MockRuntime{}

		// Safe alphanumeric signal
		err := rtInstance.SignalContainer(context.Background(), "container-123", "SIGTERM")
		assert.NoError(t, err)

		// Invalid signal with control chars / semicolons
		err2 := rtInstance.SignalContainer(context.Background(), "container-123", "SIGTERM; rm -rf /")
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "invalid signal")

		// Invalid signal with whitespaces
		err3 := rtInstance.SignalContainer(context.Background(), "container-123", "SIG INT")
		assert.Error(t, err3)
		assert.Contains(t, err3.Error(), "invalid signal")
	})
}
