package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_CLIArgBuilder_JoinedFlagsAndDoubleDash(t *testing.T) {
	t.Run("formats flags as --flag=value and inserts -- before positionals", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--net", "bridge", false)
		builder.AddJoinedFlag("--user", "1000:1000", true)
		builder.AddJoinedFlag("--workdir", "", true) // empty omitEmpty=true
		builder.AddBoolFlag("--tty", true)
		builder.AddBoolFlag("--interactive", false)
		builder.AddRepeatJoinedFlag("--env", []string{"FOO=bar", "BAZ=qux"})
		builder.AddPositionals("alpine:latest", "echo", "hello")

		argv := builder.Build()

		expected := []string{
			"--net=bridge",
			"--user=1000:1000",
			"--tty",
			"--env=FOO=bar",
			"--env=BAZ=qux",
			"--",
			"alpine:latest",
			"echo",
			"hello",
		}

		assert.Equal(t, expected, argv)

		err := builder.VerifyStructure(argv, 5)
		require.NoError(t, err)
	})

	t.Run("VerifyStructure detects missing double dash when positionals exist", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--net", "bridge", false)
		builder.AddPositionals("alpine:latest")

		malformedArgv := []string{"--net=bridge", "alpine:latest"}
		err := builder.VerifyStructure(malformedArgv, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positionals present but '--' separator missing")
	})

	t.Run("VerifyStructure detects flag argument without leading dash", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		malformedArgv := []string{"invalid_flag", "--", "alpine:latest"}

		err := builder.VerifyStructure(malformedArgv, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not start with '-'")
	})
}
