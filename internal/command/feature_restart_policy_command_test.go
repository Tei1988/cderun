package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_RestartPolicy_DryRunSimple(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"--dry-run",
		"--dry-run-format", "simple",
		"--image", "alpine",
		"--restart", "always",
		"--remove=false",
		"sh",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Restart: always")
}
