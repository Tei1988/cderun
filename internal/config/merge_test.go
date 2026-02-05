package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHierarchicalMerge(t *testing.T) {
	// Create a temporary directory structure
	// tmp/
	//   .cderun.yaml (parent)
	//   .tools.yaml (parent)
	//   child/
	//     .cderun.yaml (child)
	//     .tools.yaml (child)

	tmpDir, err := os.MkdirTemp("", "cderun-merge-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	childDir := filepath.Join(tmpDir, "child")
	err = os.MkdirAll(childDir, 0755)
	require.NoError(t, err)

	// Parent configs
	parentCDERun := `
runtime: docker
defaults:
  tty: false
  network: bridge
`
	parentTools := `
node:
  image: node:14
  env: ["PARENT=1"]
`
	err = os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(parentCDERun), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, ".tools.yaml"), []byte(parentTools), 0644)
	require.NoError(t, err)

	// Child configs
	childCDERun := `
defaults:
  tty: true
`
	childTools := `
node:
  image: node:16
python:
  image: python:3.9
`
	err = os.WriteFile(filepath.Join(childDir, ".cderun.yaml"), []byte(childCDERun), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(childDir, ".tools.yaml"), []byte(childTools), 0644)
	require.NoError(t, err)

	// Change working directory to childDir
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	defer func() { _ = os.Chdir(originalWd) }()

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Contains(t, paths[0], filepath.Join("child", ".cderun.yaml"))
		assert.Contains(t, paths[1], filepath.Join(".cderun.yaml"))

		assert.Equal(t, "docker", cfg.Runtime)          // From parent
		assert.True(t, *cfg.Defaults.TTY)               // From child (overridden)
		assert.Equal(t, "bridge", cfg.Defaults.Network) // From parent
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image) // From child (overridden)
		// Note: 既存の ToolConfig に対して mergo.Merge を使って深いマージを行う (LoadToolsConfig 内)。
		// yaml.Unmarshal はファイルの読み込みに使用され、実際の深いマージは mergo.Merge が担当する。
		assert.Equal(t, []string{"PARENT=1"}, node.Env) // From parent (preserved by deep merge)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image) // From child
	})
}
