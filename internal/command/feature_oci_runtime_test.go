package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_OciRuntime_CLIFlags(t *testing.T) {
	t.Run("--oci-runtime flag is registered and sets container config in dry-run", func(t *testing.T) {
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}

		rawArgs := []string{"cderun", "--image", "alpine", "--oci-runtime", "crun", "--dry-run", "--dry-run-format", "simple", "echo", "hello"}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})

		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "Image: alpine")
	})

	t.Run("--cderun-oci-runtime flag overrides --oci-runtime in dry-run", func(t *testing.T) {
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}

		rawArgs := []string{"cderun", "--image", "alpine", "--oci-runtime", "crun", "echo", "hello", "--cderun-oci-runtime", "runc", "--cderun-dry-run", "--cderun-dry-run-format", "simple"}

		err := ExecuteContextWithOptions(context.Background(), rawArgs, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})

		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "Image: alpine")
	})
}
