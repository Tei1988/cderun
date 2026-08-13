package command

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/argument-parsing.md: Preprocessor Hoisting and Subcommand detection
// Verify that preprocessor hoisting behaves exactly as expected under complex flag/value structures.
func TestUnit_Command_Preprocessor_ExtremeHoisting(t *testing.T) {
	t.Parallel()

	t.Run("hoists multiple P1 overrides with space-separated values", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Standard CLI mode: executable is "cderun", subcommand is "node"
		// The command-line is: cderun node --cderun-image alpine --cderun-network host src/main.js
		args := []string{"cderun", "node", "--cderun-image", "alpine", "--cderun-network", "host", "src/main.js"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Expected output: overrides should be hoisted to follow the cderun executable immediately
		expected := []string{"cderun", "--cderun-image", "alpine", "--cderun-network", "host", "node", "src/main.js"}
		assert.Equal(t, expected, processed)
	})

	t.Run("subcommand detection with complex long and short flag combinations", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Command-line is: cderun -t -p 8080:80 node app.js
		// Shorthand grouping and values should be handled to locate the true subcommand ("node" at index 4)
		args := []string{"cderun", "-t", "-p", "8080:80", "node", "app.js"}
		subcmdIdx := findSubcommandIndex(cmd, args, false)
		assert.Equal(t, 4, subcmdIdx)
	})

	t.Run("rejects P1 override placed before the subcommand in standard mode", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Command-line: cderun --cderun-image alpine node app.js
		args := []string{"cderun", "--cderun-image", "alpine", "node", "app.js"}
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("ensures double-dash separator does not disable hoisting of P1 overrides", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Command-line: cderun node -- --cderun-image alpine
		args := []string{"cderun", "node", "--", "--cderun-image", "alpine"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{"cderun", "--cderun-image", "alpine", "node", "--"}
		assert.Equal(t, expected, processed)
	})

	t.Run("fails when value-taking P1 override lacks a corresponding value", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Command-line: cderun node --cderun-image
		args := []string{"cderun", "node", "--cderun-image"}
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})
}

// docs/features/argument-parsing.md: Symlink Mode Execution
// Verify argument translation under symlink mode (polyglot wrapper mode).
func TestUnit_Command_SymlinkMode_Extremes(t *testing.T) {
	t.Parallel()

	t.Run("translates polyglot symlink arguments to standard cderun overrides layout", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Executable is "node" (symlink). The command-line: node --cderun-image alpine src/index.js
		args := []string{"/usr/local/bin/node", "--cderun-image", "alpine", "src/index.js"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Expected translation:
		// "node" is treated as the subcommand. Overrides hoisted to the front.
		expected := []string{"cderun", "--cderun-image", "alpine", "node", "src/index.js"}
		assert.Equal(t, expected, processed)
	})

	t.Run("preserves CJK/Unicode paths and non-prefixed arguments in polyglot mode", func(t *testing.T) {
		cmd := newRootCmd(&rootOptions{})

		// Input: python3 --cderun-image python:3.9 ユーザーコード/テスト.py
		args := []string{"python3", "--cderun-image", "python:3.9", "ユーザーコード/テスト.py"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{"cderun", "--cderun-image", "python:3.9", "python3", "ユーザーコード/テスト.py"}
		assert.Equal(t, expected, processed)
	})
}

type quickEOFReader struct{}

func (quickEOFReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}

// Verify interactive stdin with empty or quick EOF behaviors
func TestUnit_Command_StdinAndSignals_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("interactive mode with empty reader exits cleanly with no-op read", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-empty-stdin"
		mock.ExitCode = 0

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var stdout strings.Builder
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(quickEOFReader{})
			cmd.SetOut(&stdout)
			o.isTerminal = func(fd int) bool { return false }
		})

		require.NoError(t, err)
		assert.Empty(t, stdout.String())
	})

	t.Run("consecutive rapid signal context cancellation", func(t *testing.T) {
		// Verify that cancelling the parent execution context triggers an immediate, graceful return.
		ctx, cancel := context.WithCancel(context.Background())
		mock := runtime.NewMockRuntime()
		mock.CreatedContainerID = "test-cancellation"

		// Cancel the context beforehand to simulate an early signal host-cancellation
		cancel()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sleep", "10"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
		})

		// Should fail or complete with context.Canceled error
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}
