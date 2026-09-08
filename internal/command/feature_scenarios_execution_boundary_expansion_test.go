package command

import (
	"bytes"
	"encoding/json"
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_ExecutionBoundary_HoistingAndParsing(t *testing.T) {
	t.Parallel()

	t.Run("WrapperMode_HoistingAcrossDoubleDash", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "cderun"}
		opts := &rootOptions{}
		registerFlags(cmd, opts)

		args := []string{"node", "--cderun-image=node:20-alpine", "--", "index.js", "--cderun-workdir", "/app"}

		overrides, others, err := hoistOverrides(cmd, args, false, 0)
		require.NoError(t, err)

		assert.Contains(t, overrides, "--cderun-image=node:20-alpine")
		assert.Contains(t, overrides, "--cderun-workdir")
		assert.Contains(t, overrides, "/app")

		assert.Contains(t, others, "--")
		assert.Contains(t, others, "index.js")
	})

	t.Run("SymlinkMode_PassthroughExecution", func(t *testing.T) {
		t.Parallel()

		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{
			Image: "node:20-alpine",
		}
		passthroughArgs := []string{"node", "index.js", "--port", "3000"}

		cc, err := opts.buildContainerConfig(resolved, passthroughArgs, nil)
		require.NoError(t, err)
		require.NotNil(t, cc)

		assert.Equal(t, "node:20-alpine", cc.Image)
		assert.Equal(t, []string{"node", "index.js", "--port", "3000"}, cc.Command)
	})

	t.Run("DryRunMode_RedactionAndJSONFormatting", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cc := &container.ContainerConfig{
			Image: "alpine:latest",
			Env: []string{
				"API_KEY=[REDACTED]",
				"SECRET_TOKEN=[REDACTED]",
			},
		}

		formatDryRunSimple(&buf, cc)

		out := buf.String()
		assert.Contains(t, out, "Image: alpine:latest")
		assert.Contains(t, out, `"API_KEY"="[REDACTED]"`)

		jsonBuf := &bytes.Buffer{}
		encoder := json.NewEncoder(jsonBuf)
		encoder.SetIndent("", "  ")
		err := encoder.Encode(cc)
		require.NoError(t, err)

		jsonStr := jsonBuf.String()
		assert.Contains(t, jsonStr, `"image": "alpine:latest"`)
		assert.Contains(t, jsonStr, `API_KEY=[REDACTED]`)
		assert.NotContains(t, jsonStr, `supersecret`)
	})
}
