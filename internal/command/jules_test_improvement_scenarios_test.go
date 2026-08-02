package command

import (
	"context"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_RapidConsecutiveSignals_Scenarios(t *testing.T) {
	t.Parallel()

	state := newExecutionState()
	state.MarkStartupBegun() // Startup has begun

	// First signal: should forward immediate, but NOT cancel context
	cancelCtx, forwardImmediate, _, _ := state.HandleSignal("SIGINT")
	assert.False(t, cancelCtx)
	assert.True(t, forwardImmediate)

	// Second consecutive signal: should cancel context on host
	cancelCtx, forwardImmediate, _, _ = state.HandleSignal("SIGINT")
	assert.True(t, cancelCtx)
	assert.False(t, forwardImmediate)
}

func TestUnit_Command_SymlinkMode_CleanedRelativePaths(t *testing.T) {
	t.Parallel()

	o := &rootOptions{}
	cmd := newRootCmd(o)

	// In Symlink/Polyglot mode, if the command is executed with complex relative paths containing non-ASCII/unicode characters,
	// the executable name should be correctly cleaned to its base name.
	args := []string{"../../some/unicode_dir/日本語/🔥_python", "-c", "print('こんにちは!')"}
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	// processed args should have the base name "🔥_python" as the subcommand
	expected := []string{"cderun", "🔥_python", "-c", "print('こんにちは!')"}
	assert.Equal(t, expected, processed)
}

func TestUnit_Command_P1Override_ValueTakingSpaceSeparated(t *testing.T) {
	t.Parallel()

	o := &rootOptions{}
	cmd := newRootCmd(o)

	// Value-taking P1 override flags can use space separation format.
	// The preprocessor must successfully extract both the flag and its value.
	valueFlags := []string{
		"--cderun-image",
		"--cderun-network",
		"--cderun-socket-path",
		"--cderun-workdir",
		"--cderun-memory",
		"--cderun-hang-timeout",
	}

	for _, flg := range valueFlags {
		args := []string{"cderun", "sh", flg, "some-value"}
		actual, err := preprocessArgs(cmd, args)
		require.NoError(t, err, "expected no error for flag %q", flg)
		expected := []string{"cderun", flg, "some-value", "sh"}
		assert.Equal(t, expected, actual)
	}
}

func TestUnit_Command_HangTimeout_NegativeValue(t *testing.T) {
	t.Parallel()

	args := []string{"cderun", "--image", "alpine", "sh", "--cderun-hang-timeout=-5s"}
	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration error")
	assert.Contains(t, err.Error(), "duration cannot be negative")
}
