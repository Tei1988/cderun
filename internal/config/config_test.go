package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCDERunConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cderun-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change working directory to tmpDir
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })

	t.Run("not found", func(t *testing.T) {
		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, paths)
	})

	t.Run("found in current dir", func(t *testing.T) {
		content := `
runtime: docker
defaults:
  tty: true
`
		err := os.WriteFile(".cderun.yaml", []byte(content), 0644)
		require.NoError(t, err)
		defer func() { _ = os.Remove(".cderun.yaml") }()

		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".cderun.yaml")
		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
	})

	t.Run("found in home dir", func(t *testing.T) {
		homeDir, err := os.MkdirTemp("", "cderun-home-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(homeDir) }()

		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir)

		configDir := filepath.Join(homeDir, ".config", "cderun")
		err = os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		content := `
runtime: podman
`
		err = os.WriteFile(filepath.Join(configDir, ".cderun.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".cderun.yaml")
		assert.Equal(t, "podman", cfg.Runtime)
	})

	t.Run("found in run dir", func(t *testing.T) {
		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		content := `
runtime: docker
defaults:
  network: host
`
		err = os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".cderun.yaml")
		assert.Equal(t, "host", cfg.Defaults.Network)
	})

	t.Run("priority: home over run", func(t *testing.T) {
		homeDir, err := os.MkdirTemp("", "cderun-home-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(homeDir) }()
		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir)

		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		// Home config
		homeConfigDir := filepath.Join(homeDir, ".config", "cderun")
		require.NoError(t, os.MkdirAll(homeConfigDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(homeConfigDir, ".cderun.yaml"), []byte("runtime: podman"), 0644))

		// Run config
		require.NoError(t, os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte("runtime: docker"), 0644))

		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.Equal(t, 2, len(paths))
		// Priority: higher first. Home > Run.
		assert.Contains(t, paths[0], homeDir)
		assert.Contains(t, paths[1], runDir)
		assert.Equal(t, "podman", cfg.Runtime)
	})

	t.Run("HostContext is loaded and merged", func(t *testing.T) {
		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		content := `
hostContext:
  level: 1
  snapshotDir: /tmp/snap
  mounts:
    - source: /host
      target: /container
      level: 1
`
		err = os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, _, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.HostContext)
		assert.Equal(t, 1, cfg.HostContext.Level)
		assert.Equal(t, "/tmp/snap", cfg.HostContext.SnapshotDir)
		require.Len(t, cfg.HostContext.Mounts, 1)
		assert.Equal(t, "/host", cfg.HostContext.Mounts[0].Source)
	})
}

func TestLoadToolsConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cderun-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change working directory to tmpDir
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })

	t.Run("not found", func(t *testing.T) {
		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, paths)
	})

	t.Run("found in current dir", func(t *testing.T) {
		content := `
node:
  image: node:20-alpine
  tty: true
`
		err := os.WriteFile(".tools.yaml", []byte(content), 0644)
		require.NoError(t, err)
		defer func() { _ = os.Remove(".tools.yaml") }()

		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".tools.yaml")
		tool, ok := cfg["node"]
		assert.True(t, ok)
		assert.Equal(t, "node:20-alpine", tool.Image)
		assert.True(t, *tool.TTY)
	})

	t.Run("found in run dir", func(t *testing.T) {
		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		content := `
node:
  image: node:18-alpine
`
		err = os.WriteFile(filepath.Join(runDir, ".tools.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".tools.yaml")
		assert.Equal(t, "node:18-alpine", cfg["node"].Image)
	})
}
