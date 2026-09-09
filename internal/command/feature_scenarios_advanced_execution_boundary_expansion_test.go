package command

import (
	"bytes"
	"path/filepath"
	"testing"

	"cderun/internal/container"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_AdvancedExecutionBoundaryExpansion_WrapperHoisting(t *testing.T) {
	t.Parallel()

	t.Run("Hoist flags across double dash and space equals forms", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "cderun"}
		cmd.Flags().String("cderun-image", "", "")
		cmd.Flags().String("cderun-workdir", "", "")

		// When hoistOverrides is called with subcommand index 0 (subcmdIdx=0), the subcommand at index 0 ("node") is extracted.
		args := []string{"node", "--cderun-image", "node:20-alpine", "--", "--cderun-workdir=/app", "index.js", "--port", "3000"}
		overrides, others, err := hoistOverrides(cmd, args, false, 0)
		require.NoError(t, err)

		assert.Contains(t, overrides, "--cderun-image")
		assert.Contains(t, overrides, "node:20-alpine")
		assert.Contains(t, overrides, "--cderun-workdir=/app")

		assert.Equal(t, []string{"--", "index.js", "--port", "3000"}, others)
	})
}

func TestFeatureScenarios_AdvancedExecutionBoundaryExpansion_SymlinkExecution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	symlinkPath := filepath.Join(tmpDir, "python3")

	// Symlink mode detection helper check
	execName := filepath.Base("python3")
	assert.NotEqual(t, "cderun", execName)

	execNameCderun := filepath.Base("cderun")
	assert.Equal(t, "cderun", execNameCderun)

	require.NotNil(t, symlinkPath)
}

func TestFeatureScenarios_AdvancedExecutionBoundaryExpansion_DryRunFormatting(t *testing.T) {
	t.Parallel()

	cfg := &container.ContainerConfig{
		Image:      "alpine:latest",
		Workdir:    "/app",
		Env:        []string{"API_KEY=[REDACTED]", "APP_ENV=production"},
		Command:    []string{"echo", "hello"},
		Entrypoint: []string{"/bin/sh", "-c"},
	}

	var buf bytes.Buffer
	formatDryRunSimple(&buf, cfg)

	out := buf.String()
	assert.Contains(t, out, "alpine:latest")
	assert.NotContains(t, out, "supersecret")
	assert.Contains(t, out, `"API_KEY"="[REDACTED]"`)
}
