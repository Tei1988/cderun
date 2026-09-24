package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"
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

		args := []string{"cderun", "python", "-c", "import sys", "--cderun-dry-run", "--cderun-dry-run-format=simple", "--", "--cderun-workdir=/tmp"}
		ctx := context.Background()

		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
		})
		require.NoError(t, err)

		output := outBuf.String()
		assert.Contains(t, output, "Workdir: /tmp")
		assert.Contains(t, output, "Image: python:latest")
	})

	t.Run("Symlink mode non-prefixed flag passthrough execution", func(t *testing.T) {
		t.Parallel()

		mockRT := &runtime.MockRuntime{}
		args := []string{"/usr/local/bin/python", "-v", "--version"}
		ctx := context.Background()

		err := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRT, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		cfg := mockRT.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "python:latest", cfg.Image)
		assert.Equal(t, []string{"-v", "--version"}, cfg.Command)
	})
}
