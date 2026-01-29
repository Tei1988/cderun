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
	defer os.RemoveAll(tmpDir)

	// Change working directory to tmpDir
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(oldWd)

	t.Run("not found", func(t *testing.T) {
		cfg, path, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, path)
	})

	t.Run("found in current dir", func(t *testing.T) {
		content := `
runtime: docker
defaults:
  tty: true
`
		err := os.WriteFile(".cderun.yaml", []byte(content), 0644)
		require.NoError(t, err)
		defer os.Remove(".cderun.yaml")

		cfg, path, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, ".cderun.yaml", path)
		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
	})

	t.Run("found in home dir", func(t *testing.T) {
		homeDir, err := os.MkdirTemp("", "cderun-home-*")
		require.NoError(t, err)
		defer os.RemoveAll(homeDir)

		t.Setenv("HOME", homeDir)

		configDir := filepath.Join(homeDir, ".config", "cderun")
		err = os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		content := `
runtime: podman
`
		err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, path, err := LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Contains(t, path, "config.yaml")
		assert.Equal(t, "podman", cfg.Runtime)
	})
}

func TestLoadToolsConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cderun-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to tmpDir
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(oldWd)

	t.Run("not found", func(t *testing.T) {
		cfg, path, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, path)
	})

	t.Run("found in current dir", func(t *testing.T) {
		content := `
node:
  image: node:20-alpine
  tty: true
`
		err := os.WriteFile(".tools.yaml", []byte(content), 0644)
		require.NoError(t, err)
		defer os.Remove(".tools.yaml")

		cfg, path, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, ".tools.yaml", path)
		tool, ok := cfg["node"]
		assert.True(t, ok)
		assert.Equal(t, "node:20-alpine", tool.Image)
		assert.True(t, *tool.TTY)
	})
}
