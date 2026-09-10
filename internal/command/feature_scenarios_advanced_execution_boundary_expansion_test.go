package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"cderun/internal/config"
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
	targetBinary := filepath.Join(tmpDir, "cderun")
	err := os.WriteFile(targetBinary, []byte("#!/bin/sh\necho cderun"), 0755)
	require.NoError(t, err)

	symlinkPath := filepath.Join(tmpDir, "python3")
	err = os.Symlink(targetBinary, symlinkPath)
	require.NoError(t, err)

	cmd := &cobra.Command{Use: "cderun"}

	// Invoke cderun's actual command-resolution preprocessArgs function for symlink/polyglot execution mode
	processedSymlink, err := preprocessArgs(cmd, []string{symlinkPath, "--version"})
	require.NoError(t, err)
	// Polyglot execution mode prepends "cderun" and extracts the tool name "python3" from args[0]
	assert.Equal(t, "cderun", processedSymlink[0])
	assert.Contains(t, processedSymlink, "python3")
	assert.Contains(t, processedSymlink, "--version")

	// Invoke preprocessArgs for standard direct wrapper mode
	processedDirect, err := preprocessArgs(cmd, []string{targetBinary, "python3", "--version"})
	require.NoError(t, err)
	assert.Equal(t, targetBinary, processedDirect[0])
}

func TestFeatureScenarios_AdvancedExecutionBoundaryExpansion_DryRunFormatting(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "cderun"}
	opts := &rootOptions{
		rootFlags: rootFlags{
			dryRunFormat: "json",
		},
	}

	cfg := &container.ContainerConfig{
		Image:      "alpine:latest",
		Workdir:    "/app",
		Env:        []string{"API_KEY=supersecretrawkey", "APP_ENV=production"},
		Command:    []string{"echo", "hello"},
		Entrypoint: []string{"/bin/sh", "-c"},
	}

	resolved := &config.ResolvedConfig{
		SensitiveEnv: []string{"*KEY*"},
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := opts.handleDryRun(cmd, cfg, resolved)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "alpine:latest")
	assert.NotContains(t, out, "supersecretrawkey")
	assert.Contains(t, out, "API_KEY=[REDACTED]")
}
