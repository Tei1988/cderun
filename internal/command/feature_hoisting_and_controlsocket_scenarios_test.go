package command

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_PreprocessArgs_SpaceSeparatedAndSubcommandInvariants(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(&rootOptions{})

	t.Run("space separated cderun flags hoisting", func(t *testing.T) {
		t.Parallel()
		rawArgs := []string{"cderun", "run", "--cderun-image", "alpine:latest", "echo", "hello"}
		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)

		// Expected order: binary ("cderun"), overrides ("--cderun-image", "alpine:latest"), subcommand ("run"), remaining ("echo", "hello")
		expected := []string{"cderun", "--cderun-image", "alpine:latest", "run", "echo", "hello"}
		assert.Equal(t, expected, processed)
	})

	t.Run("pre-subcommand cderun flag error in standard mode", func(t *testing.T) {
		t.Parallel()
		rawArgs := []string{"cderun", "--cderun-image", "alpine:latest", "run", "echo", "hello"}
		_, err := preprocessArgs(cmd, rawArgs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cderun internal override flag \"--cderun-image\" must be placed after the subcommand")
	})

	t.Run("polyglot mode hoisting without subcommand restriction", func(t *testing.T) {
		t.Parallel()
		rawArgs := []string{"python3", "--cderun-image", "python:3.11", "-c", "print(1)"}
		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)

		// In polyglot mode, binary becomes "cderun", overrides hoisted, then execName ("python3"), then remaining flags
		expected := []string{"cderun", "--cderun-image", "python:3.11", "python3", "-c", "print(1)"}
		assert.Equal(t, expected, processed)
	})

	t.Run("findSubcommandIndex with equals and shorthand flags", func(t *testing.T) {
		t.Parallel()

		testCmd := &cobra.Command{Use: "cderun"}
		testCmd.PersistentFlags().String("config", "", "config path")
		testCmd.PersistentFlags().Bool("verbose", false, "verbose output")
		testCmd.Flags().StringP("output", "o", "", "output file")

		subCmd := &cobra.Command{Use: "run"}
		testCmd.AddCommand(subCmd)

		// Standard mode with flags taking arguments
		args1 := []string{"cderun", "--config=test.yaml", "-o", "out.txt", "run", "arg1"}
		idx1 := findSubcommandIndex(testCmd, args1, false)
		assert.Equal(t, 4, idx1)

		// Polyglot mode always treats index 0 as subcommand position
		args2 := []string{"git", "status"}
		idx2 := findSubcommandIndex(testCmd, args2, true)
		assert.Equal(t, 0, idx2)
	})
}
