package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"cderun/internal/config"
	"cderun/internal/runtime"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_Command_DryRunFormattingAndHoisting(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("python:\n  image: python:latest\n"),
		},
	}

	t.Run("Dry-run JSON output structure", func(t *testing.T) {
		t.Parallel()

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)

		args := []string{"cderun", "--dry-run-format=json", "python", "-c", "print('hello')", "--cderun-dry-run"}
		ctx := context.Background()

		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})
		require.NoError(t, err)

		var dryRunData map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &dryRunData)
		require.NoError(t, err, "Output should be valid JSON: %s", outBuf.String())

		assert.Equal(t, "python:latest", dryRunData["image"])
		cmdList, ok := dryRunData["command"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"-c", "print('hello')"}, cmdList)
	})

	t.Run("Wrapper Mode argument hoisting with double dash divider", func(t *testing.T) {
		t.Parallel()

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)

		args := []string{"cderun", "python", "-c", "import sys", "--cderun-dry-run", "--", "--cderun-workdir=/tmp"}
		ctx := context.Background()

		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})
		require.NoError(t, err)

		output := outBuf.String()
		assert.Contains(t, output, "workdir: /tmp")
		assert.Contains(t, output, "python:latest")
	})

	t.Run("Symlink mode non-prefixed flag passthrough", func(t *testing.T) {
		t.Parallel()

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)

		mockRT := &runtime.MockRuntime{}

		args := []string{"/usr/local/bin/python", "-v", "--version", "--cderun-dry-run"}
		ctx := context.Background()

		err := ExecuteContextWithOptions(ctx, args, withMockRuntime(mockRT, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		}))
		require.NoError(t, err)

		output := outBuf.String()
		assert.Contains(t, output, "python:latest")
		assert.Contains(t, output, "-v")
		assert.Contains(t, output, "--version")
	})
}
